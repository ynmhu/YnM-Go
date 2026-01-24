package ynm

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

const (
	// Plugin constants
	pluginName           = "PingHostPlugin"
	commandPrefix        = "!pinghost"
	requiredAdminLevel   = 3
	maxPingTimeout       = 10 * time.Second
	defaultPingTimeout   = 5 * time.Second
	maxConcurrentPings   = 5
	pingCountPerHost     = 1
	
	// Response messages
	msgUsage             = "Usage: !pinghost <host or IP>"
	msgNoIPv6            = "⚠️ This server has no IPv6 connectivity, skipping IPv6 ping: %s"
	msgHostResponded     = "🟢 %s responded in %.2f ms"
	msgHostNoResponse    = "🔴 %s did not respond: %s"
	msgUnknownResponse   = "⚠️ Unknown response from %s"
	msgInvalidHost       = "❌ Invalid hostname or IP address: %s"
	msgTimeout           = "⏱️ Ping to %s timed out"
	msgRateLimited       = "⚠️ Too many ping requests in progress, please try again later"
)

var (
	// Regex patterns for validation
	ipv4Pattern = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	ipv6Pattern = regexp.MustCompile(`^([0-9a-fA-F]{0,4}:){1,7}[0-9a-fA-F]{0,4}$`)
	hostPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
)

// PingResult represents the result of a ping operation
type PingResult struct {
	Host     string
	Success  bool
	Duration time.Duration
	Error    error
}

// PingHostPlugin provides network connectivity testing functionality
type PingHostPlugin struct {
	bot             *YnMIrC.Client
	adminPlugin     *owner.YnmAdminPlugin
	timeout         time.Duration
	activePings     chan struct{} // Semaphore for rate limiting
	ipv6Available   *bool         // Cache for IPv6 availability
	ipv6CheckTime   time.Time     // Last time IPv6 was checked
}

// NewPingHostPlugin creates a new instance of the PingHost plugin
func NewPingHostPlugin(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin, timeout time.Duration) *PingHostPlugin {
	// Validate and adjust timeout
	if timeout <= 0 || timeout > maxPingTimeout {
		timeout = defaultPingTimeout
	}

	return &PingHostPlugin{
		bot:         bot,
		adminPlugin: adminPlugin,
		timeout:     timeout,
		activePings: make(chan struct{}, maxConcurrentPings),
	}
}

// Name returns the plugin name
func (p *PingHostPlugin) Name() string {
	return pluginName
}

// HandleMessage processes incoming IRC messages for ping commands
func (p *PingHostPlugin) HandleMessage(msg YnMIrC.Message) string {
	// Check if message starts with our command
	if !strings.HasPrefix(strings.ToLower(msg.Text), commandPrefix+" ") {
		return ""
	}

	// Only allow in channels, not in private messages
	if !p.isChannelMessage(msg) {
		return ""
	}

	// Extract user information
	nick := strings.Split(msg.Sender, "!")[0]
	hostmask := YnMModule.SimplifyHostmask(msg.Sender)

	// Check admin permissions
	if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, msg.Channel, requiredAdminLevel) {
		return "" // Silently ignore unauthorized requests
	}

	// Parse command arguments
	parts := strings.Fields(msg.Text)
	if len(parts) < 2 {
		return msgUsage
	}

	host := strings.TrimSpace(parts[1])
	
	// Validate host format
	if !p.isValidHost(host) {
		return fmt.Sprintf(msgInvalidHost, host)
	}

	// Check rate limiting
	select {
	case p.activePings <- struct{}{}:
		// Slot available, proceed with ping
		go p.performPingOperation(host, msg.Channel)
		return ""
	default:
		// No slots available
		return msgRateLimited
	}
}

// OnTick handles periodic plugin operations (not used in this plugin)
func (p *PingHostPlugin) OnTick() []YnMIrC.Message {
	return nil
}

// isChannelMessage checks if the message was sent to a channel
func (p *PingHostPlugin) isChannelMessage(msg YnMIrC.Message) bool {
	return msg.Channel != "" && msg.Channel != strings.Split(msg.Sender, "!")[0]
}

// isValidHost validates if the provided string is a valid hostname or IP address
func (p *PingHostPlugin) isValidHost(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}

	// Check if it's a valid IPv4 address
	if ipv4Pattern.MatchString(host) {
		ip := net.ParseIP(host)
		return ip != nil && ip.To4() != nil
	}

	// Check if it's a valid IPv6 address
	if strings.Contains(host, ":") {
		ip := net.ParseIP(host)
		return ip != nil && ip.To16() != nil
	}

	// Check if it's a valid hostname
	return hostPattern.MatchString(host) && !strings.HasPrefix(host, "-") && !strings.HasSuffix(host, "-")
}

// hasIPv6Connectivity checks if the system has IPv6 connectivity
// Results are cached for 5 minutes to avoid repeated checks
func (p *PingHostPlugin) hasIPv6Connectivity() bool {
	now := time.Now()
	
	// Use cached result if available and recent
	if p.ipv6Available != nil && now.Sub(p.ipv6CheckTime) < 5*time.Minute {
		return *p.ipv6Available
	}

	// Test IPv6 connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ping", "-6", "-c", "1", "-W", "2", "ipv6.google.com")
	err := cmd.Run()
	
	result := err == nil
	p.ipv6Available = &result
	p.ipv6CheckTime = now
	
	return result
}

// performPingOperation executes the ping command and sends results to the channel
func (p *PingHostPlugin) performPingOperation(host, channel string) {
	// Ensure we release the rate limiting slot when done
	defer func() {
		<-p.activePings
	}()

	// Check IPv6 connectivity for IPv6 addresses
	isIPv6 := strings.Contains(host, ":")
	if isIPv6 && !p.hasIPv6Connectivity() {
		p.bot.SendMessage(channel, fmt.Sprintf(msgNoIPv6, host))
		return
	}

	// Perform the ping
	result := p.executePing(host)
	
	// Send appropriate response based on result
	p.sendPingResult(channel, result)
}

// executePing performs the actual ping operation
func (p *PingHostPlugin) executePing(host string) PingResult {
	// Determine IP version and build command arguments
	isIPv6 := strings.Contains(host, ":")
	args := p.buildPingArgs(host, isIPv6)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout+time.Second)
	defer cancel()

	// Execute ping command
	cmd := exec.CommandContext(ctx, "ping", args...)
	start := time.Now()
	_, err := cmd.CombinedOutput()
	elapsed := time.Since(start)

	return PingResult{
		Host:     host,
		Success:  err == nil,
		Duration: elapsed,
		Error:    err,
	}
}

// buildPingArgs constructs the arguments for the ping command
func (p *PingHostPlugin) buildPingArgs(host string, isIPv6 bool) []string {
	args := []string{
		"-c", fmt.Sprintf("%d", pingCountPerHost),
		"-W", fmt.Sprintf("%.0f", p.timeout.Seconds()),
	}

	if isIPv6 {
		args = append([]string{"-6"}, args...)
	} else {
		args = append([]string{"-4"}, args...)
	}

	args = append(args, host)
	return args
}

// sendPingResult analyzes ping output and sends appropriate response to channel
func (p *PingHostPlugin) sendPingResult(channel string, result PingResult) {
	if !result.Success {
		// Handle different types of errors
		if result.Error != nil {
			if strings.Contains(result.Error.Error(), "context deadline exceeded") {
				p.bot.SendMessage(channel, fmt.Sprintf(msgTimeout, result.Host))
				return
			}
		}
		
		errorMsg := "unknown error"
		if result.Error != nil {
			errorMsg = result.Error.Error()
		}
		
		p.bot.SendMessage(channel, fmt.Sprintf(msgHostNoResponse, result.Host, errorMsg))
		return
	}

	// For successful pings, try to extract timing information from output
	// If that fails, use the measured elapsed time
	durationMs := float64(result.Duration.Nanoseconds()) / 1e6
	p.bot.SendMessage(channel, fmt.Sprintf(msgHostResponded, result.Host, durationMs))
}