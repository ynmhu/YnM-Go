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
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

// DNSPlugin provides DNS lookup functionality for IRC bot
type DNSPlugin struct {
	bot         *YnMIrC.Client
	adminPlugin *owner.YnmAdminPlugin
	timeout     time.Duration
	maxRecords  int
	mu          sync.RWMutex
	cache       map[string]*dnsCache
	rateLimiter map[string]time.Time
}

// dnsCache represents cached DNS lookup results
type dnsCache struct {
	result    string
	timestamp time.Time
}

// DNSConfig holds configuration for the DNS plugin
type DNSConfig struct {
	Timeout     time.Duration
	MaxRecords  int
	CacheTTL    time.Duration
	RateLimit   time.Duration
}

// DefaultDNSConfig returns default configuration
func DefaultDNSConfig() DNSConfig {
	return DNSConfig{
		Timeout:     10 * time.Second,
		MaxRecords:  10,
		CacheTTL:    5 * time.Minute,
		RateLimit:   2 * time.Second,
	}
}

// NewDNSPlugin creates a new DNS plugin instance with configuration
func NewDNSPlugin(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin, config DNSConfig) *DNSPlugin {
	return &DNSPlugin{
		bot:         bot,
		adminPlugin: adminPlugin,
		timeout:     config.Timeout,
		maxRecords:  config.MaxRecords,
		cache:       make(map[string]*dnsCache),
		rateLimiter: make(map[string]time.Time),
	}
}

// NewDNSPluginDefault creates a new DNS plugin with default configuration
func NewDNSPluginDefault(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin) *DNSPlugin {
	return NewDNSPlugin(bot, adminPlugin, DefaultDNSConfig())
}

func (p *DNSPlugin) HandleMessage(msg YnMIrC.Message) string {
    text := strings.ToLower(strings.TrimSpace(msg.Text))
    hostmask := YnMModule.SimplifyHostmask(msg.Sender)
    prefix := p.adminPlugin.GetPrefixForHost(hostmask)
    nick, hostmask := YnMModule.GetEffectiveNickAndHost(p.adminPlugin, msg.Sender)    
    minLevel := 1 
    
    if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, msg.Channel, minLevel) {
        return ""
    }
    

    if !strings.HasPrefix(text, strings.ToLower(prefix+"dns")) && !strings.HasPrefix(text, strings.ToLower(prefix+"nslookup")) {
        return ""
    }
    
    
    if !p.checkRateLimit(nick) {
        return "⏱️ Please wait before making another DNS request."
    }

    // Parse command
    parts := strings.Fields(msg.Text)
    if len(parts) < 2 {
        return p.getUsageHelp()
    }

    //command := strings.ToLower(parts[0])
    target := strings.TrimSpace(parts[1])

    // Validate input
    if !p.isValidDomain(target) && !p.isValidIP(target) {
        return "❌ Invalid domain name or IP address format."
    }

    // Determine lookup type
    recordType := "A"
    if len(parts) >= 3 {
        recordType = strings.ToUpper(parts[2])
    }

    // Synchronous lookup - ez a változtatás a kulcs!
    result := p.performDNSLookupSync(target, recordType, msg.Channel, nick)
    return result
}

// Új függvény: szinkron DNS lookup
func (p *DNSPlugin) performDNSLookupSync(target, recordType, channel, requester string) string {
    cacheKey := fmt.Sprintf("%s:%s", target, recordType)
    
    // Check cache first
    if cached, found := p.getCachedResult(cacheKey); found {
        return fmt.Sprintf("🔄 [Cached] %s", cached)
    }

    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
    defer cancel()

    var result string
    var err error

    // Perform lookup based on record type
    switch recordType {
    case "A":
        result, err = p.lookupA(ctx, target)
    case "AAAA":
        result, err = p.lookupAAAA(ctx, target)
    case "CNAME":
        result, err = p.lookupCNAME(ctx, target)
    case "MX":
        result, err = p.lookupMX(ctx, target)
    case "TXT":
        result, err = p.lookupTXT(ctx, target)
    case "NS":
        result, err = p.lookupNS(ctx, target)
    case "PTR":
        result, err = p.lookupPTR(ctx, target)
    case "ALL":
        result, err = p.lookupAll(ctx, target)
    default:
        result = fmt.Sprintf("❌ Unsupported record type: %s", recordType)
        err = nil
    }

    // Handle results
    if err != nil {
        return fmt.Sprintf("🔴 DNS lookup failed for %s (%s): %v", target, recordType, err)
    }

    // Cache and return successful result
    p.setCachedResult(cacheKey, result)
    return result
}

// checkRateLimit implements simple rate limiting per user
func (p *DNSPlugin) checkRateLimit(nick string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	if lastRequest, exists := p.rateLimiter[nick]; exists {
		if now.Sub(lastRequest) < 2*time.Second {
			return false
		}
	}
	
	p.rateLimiter[nick] = now
	return true
}

// isValidDomain validates domain name format
func (p *DNSPlugin) isValidDomain(domain string) bool {
	if len(domain) == 0 || len(domain) > 253 {
		return false
	}
	
	// Basic domain validation regex
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
	return domainRegex.MatchString(domain)
}

// isValidIP validates IP address format (IPv4 or IPv6)
func (p *DNSPlugin) isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// getCachedResult retrieves cached DNS result if valid
func (p *DNSPlugin) getCachedResult(key string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if cached, exists := p.cache[key]; exists {
		if time.Since(cached.timestamp) < 5*time.Minute {
			return cached.result, true
		}
		// Clean up expired cache entry
		delete(p.cache, key)
	}
	return "", false
}

// setCachedResult stores DNS result in cache
func (p *DNSPlugin) setCachedResult(key, result string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cache[key] = &dnsCache{
		result:    result,
		timestamp: time.Now(),
	}

	// Clean up old cache entries (simple cleanup)
	if len(p.cache) > 100 {
		for k, v := range p.cache {
			if time.Since(v.timestamp) > 10*time.Minute {
				delete(p.cache, k)
			}
		}
	}
}

// performDNSLookup executes the DNS lookup operation
func (p *DNSPlugin) performDNSLookup(target, recordType, channel, requester string) {
	cacheKey := fmt.Sprintf("%s:%s", target, recordType)
	
	// Check cache first
	if cached, found := p.getCachedResult(cacheKey); found {
		p.bot.SendMessage(channel, fmt.Sprintf("🔄 [Cached] %s", cached))
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	var result string
	var err error

	// Perform lookup based on record type
	switch recordType {
	case "A":
		result, err = p.lookupA(ctx, target)
	case "AAAA":
		result, err = p.lookupAAAA(ctx, target)
	case "CNAME":
		result, err = p.lookupCNAME(ctx, target)
	case "MX":
		result, err = p.lookupMX(ctx, target)
	case "TXT":
		result, err = p.lookupTXT(ctx, target)
	case "NS":
		result, err = p.lookupNS(ctx, target)
	case "PTR":
		result, err = p.lookupPTR(ctx, target)
	case "ALL":
		result, err = p.lookupAll(ctx, target)
	default:
		result = fmt.Sprintf("❌ Unsupported record type: %s", recordType)
		err = nil
	}

	// Handle results
	if err != nil {
		errorMsg := fmt.Sprintf("🔴 DNS lookup failed for %s (%s): %v", target, recordType, err)
		p.bot.SendMessage(channel, errorMsg)
		return
	}

	// Cache and send successful result
	p.setCachedResult(cacheKey, result)
	p.bot.SendMessage(channel, result)
}

// lookupA performs A record lookup
func (p *DNSPlugin) lookupA(ctx context.Context, domain string) (string, error) {
	resolver := &net.Resolver{}
	ips, err := resolver.LookupIPAddr(ctx, domain)
	if err != nil {
		return "", err
	}

	var ipv4s []string
	for _, ip := range ips {
		if ip.IP.To4() != nil {
			ipv4s = append(ipv4s, ip.IP.String())
			if len(ipv4s) >= p.maxRecords {
				break
			}
		}
	}

	if len(ipv4s) == 0 {
		return fmt.Sprintf("⚠️ No A records found for %s", domain), nil
	}

	return fmt.Sprintf("🟢 A records for %s: %s", domain, strings.Join(ipv4s, ", ")), nil
}

// lookupAAAA performs AAAA record lookup
func (p *DNSPlugin) lookupAAAA(ctx context.Context, domain string) (string, error) {
	resolver := &net.Resolver{}
	ips, err := resolver.LookupIPAddr(ctx, domain)
	if err != nil {
		return "", err
	}

	var ipv6s []string
	for _, ip := range ips {
		if ip.IP.To4() == nil {
			ipv6s = append(ipv6s, ip.IP.String())
			if len(ipv6s) >= p.maxRecords {
				break
			}
		}
	}

	if len(ipv6s) == 0 {
		return fmt.Sprintf("⚠️ No AAAA records found for %s", domain), nil
	}

	return fmt.Sprintf("🟢 AAAA records for %s: %s", domain, strings.Join(ipv6s, ", ")), nil
}

// lookupCNAME performs CNAME record lookup
func (p *DNSPlugin) lookupCNAME(ctx context.Context, domain string) (string, error) {
	resolver := &net.Resolver{}
	cname, err := resolver.LookupCNAME(ctx, domain)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("🟢 CNAME for %s: %s", domain, strings.TrimSuffix(cname, ".")), nil
}

// lookupMX performs MX record lookup
func (p *DNSPlugin) lookupMX(ctx context.Context, domain string) (string, error) {
	resolver := &net.Resolver{}
	mxRecords, err := resolver.LookupMX(ctx, domain)
	if err != nil {
		return "", err
	}

	if len(mxRecords) == 0 {
		return fmt.Sprintf("⚠️ No MX records found for %s", domain), nil
	}

	var mxList []string
	for i, mx := range mxRecords {
		if i >= p.maxRecords {
			break
		}
		mxList = append(mxList, fmt.Sprintf("%s (priority: %d)", 
			strings.TrimSuffix(mx.Host, "."), mx.Pref))
	}

	return fmt.Sprintf("🟢 MX records for %s: %s", domain, strings.Join(mxList, ", ")), nil
}

// lookupTXT performs TXT record lookup
func (p *DNSPlugin) lookupTXT(ctx context.Context, domain string) (string, error) {
	resolver := &net.Resolver{}
	txtRecords, err := resolver.LookupTXT(ctx, domain)
	if err != nil {
		return "", err
	}

	if len(txtRecords) == 0 {
		return fmt.Sprintf("⚠️ No TXT records found for %s", domain), nil
	}

	var txtList []string
	for i, txt := range txtRecords {
		if i >= p.maxRecords {
			break
		}
		// Truncate very long TXT records
		if len(txt) > 100 {
			txt = txt[:97] + "..."
		}
		txtList = append(txtList, fmt.Sprintf("\"%s\"", txt))
	}

	return fmt.Sprintf("🟢 TXT records for %s: %s", domain, strings.Join(txtList, ", ")), nil
}

// lookupNS performs NS record lookup
func (p *DNSPlugin) lookupNS(ctx context.Context, domain string) (string, error) {
	resolver := &net.Resolver{}
	nsRecords, err := resolver.LookupNS(ctx, domain)
	if err != nil {
		return "", err
	}

	if len(nsRecords) == 0 {
		return fmt.Sprintf("⚠️ No NS records found for %s", domain), nil
	}

	var nsList []string
	for i, ns := range nsRecords {
		if i >= p.maxRecords {
			break
		}
		nsList = append(nsList, strings.TrimSuffix(ns.Host, "."))
	}

	return fmt.Sprintf("🟢 NS records for %s: %s", domain, strings.Join(nsList, ", ")), nil
}

// lookupPTR performs reverse DNS lookup
func (p *DNSPlugin) lookupPTR(ctx context.Context, ip string) (string, error) {
	resolver := &net.Resolver{}
	names, err := resolver.LookupAddr(ctx, ip)
	if err != nil {
		return "", err
	}

	if len(names) == 0 {
		return fmt.Sprintf("⚠️ No PTR records found for %s", ip), nil
	}

	var nameList []string
	for i, name := range names {
		if i >= p.maxRecords {
			break
		}
		nameList = append(nameList, strings.TrimSuffix(name, "."))
	}

	return fmt.Sprintf("🟢 PTR records for %s: %s", ip, strings.Join(nameList, ", ")), nil
}

// lookupAll performs comprehensive DNS lookup
func (p *DNSPlugin) lookupAll(ctx context.Context, domain string) (string, error) {
	var results []string

	// A records
	if aResult, err := p.lookupA(ctx, domain); err == nil {
		results = append(results, aResult)
	}

	// AAAA records
	if aaaaResult, err := p.lookupAAAA(ctx, domain); err == nil {
		results = append(results, aaaaResult)
	}

	// CNAME records
	if cnameResult, err := p.lookupCNAME(ctx, domain); err == nil {
		results = append(results, cnameResult)
	}

	// MX records
	if mxResult, err := p.lookupMX(ctx, domain); err == nil {
		results = append(results, mxResult)
	}

	if len(results) == 0 {
		return fmt.Sprintf("⚠️ No DNS records found for %s", domain), nil
	}

	return strings.Join(results, " | "), nil
}

// getUsageHelp returns usage instructions
func (p *DNSPlugin) getUsageHelp() string {
	return "📖 Usage: !dns <domain/ip> [record_type] | Types: A, AAAA, CNAME, MX, TXT, NS, PTR, ALL | Example: !dns google.com MX"
}

// OnTick is called periodically (unused in this implementation)
func (p *DNSPlugin) OnTick() []YnMIrC.Message {
	// Cleanup old cache entries and rate limiter
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	
	// Clean cache
	for key, cached := range p.cache {
		if now.Sub(cached.timestamp) > 10*time.Minute {
			delete(p.cache, key)
		}
	}

	// Clean rate limiter
	for nick, lastTime := range p.rateLimiter {
		if now.Sub(lastTime) > 5*time.Minute {
			delete(p.rateLimiter, nick)
		}
	}

	return nil
}

// GetStats returns plugin statistics (optional helper method)
func (p *DNSPlugin) GetStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"cached_entries":    len(p.cache),
		"rate_limit_entries": len(p.rateLimiter),
		"timeout":           p.timeout.String(),
		"max_records":       p.maxRecords,
	}
}