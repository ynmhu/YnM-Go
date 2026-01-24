// ==================================================
//  Szerzői jog © 2025 Markus (markus@ynm.hu)
//  https://ynm.hu   – főoldal
//  https://forum.ynm.hu   – hivatalos fórum
//  https://bot.ynm.hu     – bot oldala és dokumentáció
//
//  Minden jog fenntartva. A kód Markus tulajdona, tilos terjeszteni vagy
//  módosítani a szerző írásos engedélye nélkül.
//
//  Ez a fájl a YnM-Go IRC-bot rendszerének része.
// ==================================================
package ynm

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

const (
	// API constants
	ipAPIURL         = "http://ip-api.com/json/%s"
	ipAPIFields      = "status,message,country,countryCode,regionName,city,isp,org,as,reverse,lat,lon,timezone"
	apiTimeout       = 10 * time.Second
	messageDelay     = 250 * time.Millisecond
	requiredMinLevel = 3

	// Commands
	cmdIP = "!ip"

	// Error messages
	errInsufficientPermissions = ""// "❌ You need VIP or higher permissions."
	errInvalidUsage           = "Usage: !ip <IP address>"
	errInvalidIP              = "❌ Invalid IP address format"
	errAPICall                = "🔴 Error during API call: %v"
	errAPIStatus              = "🔴 API returned error status: %d"
	errDecoding               = "🔴 Error decoding API response: %v"
	errAPIResponse            = "🔴 Error: %s"
)

// IPPlugin handles IP lookup functionality for IRC bot
type IPPlugin struct {
	bot         *YnMIrC.Client
	adminPlugin *owner.YnmAdminPlugin
	timeout     time.Duration
	mu          sync.RWMutex
	rateLimiter map[string]time.Time
}

// IpAPIResponse represents the response from ip-api.com
type IpAPIResponse struct {
	Status      string  `json:"status"`
	Message     string  `json:"message,omitempty"`
	Country     string  `json:"country"`
	CountryCode string  `json:"countryCode"`
	RegionName  string  `json:"regionName"`
	City        string  `json:"city"`
	ISP         string  `json:"isp"`
	Org         string  `json:"org"`
	AS          string  `json:"as"`
	Reverse     string  `json:"reverse"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Timezone    string  `json:"timezone"`
}

// NewIPPlugin creates a new IP lookup plugin instance
func NewIPPlugin(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin, timeout time.Duration) *IPPlugin {
	if timeout == 0 {
		timeout = apiTimeout
	}

	return &IPPlugin{
		bot:         bot,
		adminPlugin: adminPlugin,
		timeout:     timeout,
		rateLimiter: make(map[string]time.Time),
	}
}

// HandleMessage processes incoming IRC messages for IP lookup commands
func (p *IPPlugin) HandleMessage(msg YnMIrC.Message) string {
	if !p.isIPCommand(msg.Text) {
		return ""
	}

    hostmask := YnMModule.SimplifyHostmask(msg.Sender)
    prefix := p.adminPlugin.GetPrefixForHost(hostmask)
    nick, hostmask := YnMModule.GetEffectiveNickAndHost(p.adminPlugin, msg.Sender)
    minLevel := 1

    if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, msg.Channel, minLevel) {
        return ""
    }
	text := strings.TrimSpace(msg.Text)
	if !strings.HasPrefix(strings.ToLower(text), strings.ToLower(prefix+"ip")) {
		return ""
	}

	ip, err := p.parseIPFromMessage(msg.Text)
	if err != nil {
		return err.Error()
	}

	if p.isRateLimited(nick) {
		return "⏱️ Please wait before making another IP lookup request"
	}

	go p.performIPLookup(ip, msg.Channel, nick)
	return ""
}

// OnTick is required by the plugin interface
func (p *IPPlugin) OnTick() []YnMIrC.Message {
	return nil
}

// isIPCommand checks if the message is an IP lookup command
func (p *IPPlugin) isIPCommand(text string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(text)), cmdIP)
}

// extractNick extracts the nickname from a full IRC hostmask
func (p *IPPlugin) extractNick(sender string) string {
	if idx := strings.Index(sender, "!"); idx != -1 {
		return sender[:idx]
	}
	return sender
}

// hasPermission checks if the user has sufficient permissions
func (p *IPPlugin) hasPermission(nick, hostmask, channel string) bool {
	return YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, channel, requiredMinLevel)
}

// parseIPFromMessage extracts and validates the IP address from the message
func (p *IPPlugin) parseIPFromMessage(text string) (string, error) {
	parts := strings.Fields(text)
	if len(parts) < 2 {
		return "", fmt.Errorf(errInvalidUsage)
	}

	ip := strings.TrimSpace(parts[1])
	if !p.isValidIP(ip) {
		return "", fmt.Errorf(errInvalidIP)
	}

	return ip, nil
}

// isValidIP validates if the provided string is a valid IP address
func (p *IPPlugin) isValidIP(ip string) bool {
	// Check for IPv4 or IPv6
	if net.ParseIP(ip) != nil {
		return true
	}

	// Check for domain names (basic validation)
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,61}[a-zA-Z0-9]?(\.[a-zA-Z0-9][a-zA-Z0-9-]{0,61}[a-zA-Z0-9]?)*$`)
	return domainRegex.MatchString(ip) && len(ip) <= 253
}

// isRateLimited checks if the user is rate limited
func (p *IPPlugin) isRateLimited(nick string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	lastRequest, exists := p.rateLimiter[nick]
	if !exists || time.Since(lastRequest) > 30*time.Second {
		p.rateLimiter[nick] = time.Now()
		return false
	}

	return true
}

// performIPLookup performs the actual IP lookup and sends results
func (p *IPPlugin) performIPLookup(ip, channel, nick string) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	data, err := p.queryIPAPI(ctx, ip)
	if err != nil {
		p.sendErrorMessage(channel, fmt.Sprintf(errAPICall, err))
		return
	}

	if data.Status != "success" {
		p.sendErrorMessage(channel, fmt.Sprintf(errAPIResponse, data.Message))
		return
	}

	p.sendIPInfo(channel, ip, data)
}

// queryIPAPI queries the IP API and returns the response
func (p *IPPlugin) queryIPAPI(ctx context.Context, ip string) (*IpAPIResponse, error) {
	url := fmt.Sprintf("%s?fields=%s", fmt.Sprintf(ipAPIURL, ip), ipAPIFields)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", "YnM-Go-Bot/1.0")

	client := &http.Client{Timeout: p.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var data IpAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &data, nil
}

// sendIPInfo formats and sends IP information to the channel
func (p *IPPlugin) sendIPInfo(channel, ip string, data *IpAPIResponse) {
	lines := p.formatIPInfo(ip, data)
	
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			p.bot.SendMessage(channel, line)
			time.Sleep(messageDelay)
		}
	}
}

// formatIPInfo formats the IP information into readable lines
func (p *IPPlugin) formatIPInfo(ip string, data *IpAPIResponse) []string {
	var lines []string

	lines = append(lines, fmt.Sprintf("🔍 IP: %s", ip))
	
	if data.Country != "" {
		countryInfo := data.Country
		if data.CountryCode != "" {
			countryInfo = fmt.Sprintf("%s (%s)", data.Country, data.CountryCode)
		}
		lines = append(lines, fmt.Sprintf("🌍 Country: %s", countryInfo))
	}

	if data.RegionName != "" || data.City != "" {
		location := p.formatLocation(data.RegionName, data.City)
		if location != "" {
			lines = append(lines, fmt.Sprintf("📍 Location: %s", location))
		}
	}

	if data.ISP != "" {
		lines = append(lines, fmt.Sprintf("🌐 ISP: %s", data.ISP))
	}

	if data.Org != "" && data.Org != data.ISP {
		lines = append(lines, fmt.Sprintf("🏢 Organization: %s", data.Org))
	}

	if data.AS != "" {
		lines = append(lines, fmt.Sprintf("🔗 AS: %s", data.AS))
	}

	if data.Reverse != "" {
		lines = append(lines, fmt.Sprintf("🔄 Reverse DNS: %s", data.Reverse))
	}

	if data.Lat != 0 || data.Lon != 0 {
		lines = append(lines, fmt.Sprintf("📌 Coordinates: %.4f, %.4f", data.Lat, data.Lon))
	}

	if data.Timezone != "" {
		lines = append(lines, fmt.Sprintf("🕐 Timezone: %s", data.Timezone))
	}

	return lines
}

// formatLocation formats region and city information
func (p *IPPlugin) formatLocation(region, city string) string {
	if region != "" && city != "" {
		return fmt.Sprintf("%s, %s", region, city)
	}
	if region != "" {
		return region
	}
	if city != "" {
		return city
	}
	return ""
}

// sendErrorMessage sends an error message to the channel
func (p *IPPlugin) sendErrorMessage(channel, message string) {
	p.bot.SendMessage(channel, message)
}