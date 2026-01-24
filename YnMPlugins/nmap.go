package ynm

import (
//	"bufio"
	"context"
	"crypto/tls"
//	"encoding/json"
	"fmt"
	"net"
//	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"log"
	"regexp"
	"sort"
	"hash/fnv"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
)

// Configuration structure for better maintainability
type NmapConfig struct {
	MaxConcurrentScans    int           `json:"max_concurrent_scans"`
	MaxWorkersPerScan     int           `json:"max_workers_per_scan"`
	MaxPortsPerScan       int           `json:"max_ports_per_scan"`
	DefaultTimeout        time.Duration `json:"default_timeout"`
	BannerReadTimeout     time.Duration `json:"banner_read_timeout"`
	ScanCooldownPerUser   time.Duration `json:"scan_cooldown_per_user"`
	GlobalScanCooldown    time.Duration `json:"global_scan_cooldown"`
	CacheExpiry           time.Duration `json:"cache_expiry"`
	MaxBannerLength       int           `json:"max_banner_length"`
	ProgressUpdateInterval time.Duration `json:"progress_update_interval"`
}

// Rate limiting structures
type rateLimiter struct {
	mu            sync.Mutex
	userLastScan  map[string]time.Time
	globalLastScan time.Time
	activeScansByUser map[string]int
	totalActiveScans  int
}

// Cached scan result
type cachedResult struct {
	result    *ScanResult
	timestamp time.Time
}

// Enhanced scan result structure
type ScanResult struct {
	Host           string          `json:"host"`
	Network        string          `json:"network"`
	OpenPorts      []PortResult    `json:"open_ports"`
	ClosedPorts    int             `json:"closed_ports"`
	FilteredPorts  int             `json:"filtered_ports"`
	ScanDuration   time.Duration   `json:"scan_duration"`
	ScanTimestamp  time.Time       `json:"scan_timestamp"`
	TotalPorts     int             `json:"total_ports"`
}

type PortResult struct {
	Port      int    `json:"port"`
	State     string `json:"state"` // open, closed, filtered
	Service   string `json:"service"`
	Version   string `json:"version,omitempty"`
	Banner    string `json:"banner,omitempty"`
	SSL       bool   `json:"ssl,omitempty"`
	ResponseTime time.Duration `json:"response_time"`
}

// Enhanced plugin structure
type NmapPlugin struct {
	bot          *YnMIrC.Client
	adminPlugin  *owner.YnmAdminPlugin
	config       *NmapConfig
	rateLimiter  *rateLimiter
	mu           sync.RWMutex
	cache        map[string]*cachedResult
	commonPorts  []int
	scanning     map[string]*ScanProgress
	serviceMap   map[int]ServiceInfo
	scanSpeeds   map[string]time.Duration
	logger       *log.Logger
	bannedHosts  map[string]bool // Internal/private networks
	ctx          context.Context
	cancel       context.CancelFunc
}

type ServiceInfo struct {
	Name        string
	Description string
	Timeout     time.Duration
	Banner      bool
	SSL         bool
}

type ScanProgress struct {
	Host         string
	TotalPorts   int
	ScannedPorts int
	StartTime    time.Time
	Channel      string
	User         string
	Cancel       context.CancelFunc
}

// Private network detection
var (
	privateNetworks = []*net.IPNet{
	//	{IP: net.IPv4(10, 0, 0, 0), Mask: net.CIDRMask(8, 32)},        // 10.0.0.0/8
	//	{IP: net.IPv4(172, 16, 0, 0), Mask: net.CIDRMask(12, 32)},     // 172.16.0.0/12
	//	{IP: net.IPv4(192, 168, 0, 0), Mask: net.CIDRMask(16, 32)},    // 192.168.0.0/16
	//	{IP: net.IPv4(127, 0, 0, 0), Mask: net.CIDRMask(8, 32)},       // 127.0.0.0/8
	//	{IP: net.IPv4(169, 254, 0, 0), Mask: net.CIDRMask(16, 32)},    // 169.254.0.0/16
	}
)

func NewNmapPlugin(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin) *NmapPlugin {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Default configuration
	config := &NmapConfig{
		MaxConcurrentScans:     3,
		MaxWorkersPerScan:      12,
		MaxPortsPerScan:        10000, // Increased for full scans
		DefaultTimeout:         3 * time.Second,
		BannerReadTimeout:      2 * time.Second,
		ScanCooldownPerUser:    30 * time.Second,
		GlobalScanCooldown:     5 * time.Second,
		CacheExpiry:           10 * time.Minute,
		MaxBannerLength:       50,
		ProgressUpdateInterval: 60 * time.Second,
	}

	plugin := &NmapPlugin{
		bot:         bot,
		adminPlugin: adminPlugin,
		config:      config,
		rateLimiter: &rateLimiter{
			userLastScan:      make(map[string]time.Time),
			activeScansByUser: make(map[string]int),
		},
		cache:    make(map[string]*cachedResult),
		scanning: make(map[string]*ScanProgress),
		ctx:      ctx,
		cancel:   cancel,
		bannedHosts: map[string]bool{
			"localhost": false,
			"127.0.0.1": false,
			"::1":       false,
		},
		scanSpeeds: map[string]time.Duration{
			"fast":   500 * time.Millisecond,
			"normal": 2 * time.Second,
			"slow":   5 * time.Second,
			"stealth": 10 * time.Second,
		},
		commonPorts: []int{
			// Most common ports first for faster results
			80, 443, 22, 21, 23, 25, 53, 110, 143, 993, 995,
			3389, 5900, 8080, 8443, 3306, 5432, 6379, 27017,
			111, 135, 139, 445, 389, 636, 88, 464, 749, 1433,
			1521, 2049, 2121, 3128, 5000, 5001, 8000, 8008, 
			8081, 8888, 9000, 9090, 10000, 11211,
			// IRC ports
			6667, 6697, 6660, 6661, 6662, 6663, 6664, 6665, 6666, 6668, 6669,
			// Gaming and media
			25565, 25566, 25567, // Minecraft
			7777, 7778, // Gaming
			1935, 554, // RTMP, RTSP
			// VPN and tunneling
			1194, 1723, 4500, 500, // OpenVPN, PPTP, IPSec
			// Additional databases
			1433, 1434, // SQL Server
			5984, 9200, 9300, // CouchDB, Elasticsearch
			// Web alternatives
			81, 82, 83, 8000, 8001, 8002, 8003, 8181, 8888, 9080, 9443,
			// Mail
			109, 465, 587, 2525, // POP2, SMTPS, submission
			// FTP alternatives  
			20, 990, 989, // FTP data, FTPS
			// Remote access
			23, 513, 514, 515, 543, 544, 5985, 5986, // Telnet, rlogin, WinRM
			// Misc services
			161, 162, 179, 194, 443, 631, 993, 995, 1080, 3128, 8080, 9050,
		},
		serviceMap: map[int]ServiceInfo{
			21:    {Name: "FTP", Description: "File Transfer Protocol", Timeout: 3 * time.Second, Banner: true},
			22:    {Name: "SSH", Description: "Secure Shell", Timeout: 3 * time.Second, Banner: true},
			23:    {Name: "Telnet", Description: "Telnet Protocol", Timeout: 2 * time.Second, Banner: true},
			25:    {Name: "SMTP", Description: "Simple Mail Transfer", Timeout: 5 * time.Second, Banner: true},
			53:    {Name: "DNS", Description: "Domain Name System", Timeout: 1 * time.Second, Banner: false},
			80:    {Name: "HTTP", Description: "Hypertext Transfer Protocol", Timeout: 3 * time.Second, Banner: true},
			110:   {Name: "POP3", Description: "Post Office Protocol v3", Timeout: 3 * time.Second, Banner: true},
			143:   {Name: "IMAP", Description: "Internet Message Access", Timeout: 3 * time.Second, Banner: true},
			443:   {Name: "HTTPS", Description: "HTTP over TLS/SSL", Timeout: 5 * time.Second, Banner: false, SSL: true},
			993:   {Name: "IMAPS", Description: "IMAP over SSL", Timeout: 5 * time.Second, Banner: true, SSL: true},
			995:   {Name: "POP3S", Description: "POP3 over SSL", Timeout: 5 * time.Second, Banner: true, SSL: true},
			3306:  {Name: "MySQL", Description: "MySQL Database", Timeout: 3 * time.Second, Banner: true},
			3389:  {Name: "RDP", Description: "Remote Desktop Protocol", Timeout: 3 * time.Second, Banner: false},
			5432:  {Name: "PostgreSQL", Description: "PostgreSQL Database", Timeout: 3 * time.Second, Banner: true},
			5900:  {Name: "VNC", Description: "Virtual Network Computing", Timeout: 3 * time.Second, Banner: true},
			6379:  {Name: "Redis", Description: "Redis Database", Timeout: 2 * time.Second, Banner: true},
			6667:  {Name: "IRC", Description: "Internet Relay Chat", Timeout: 3 * time.Second, Banner: true},
			6697:  {Name: "IRC-SSL", Description: "IRC over SSL", Timeout: 5 * time.Second, Banner: true, SSL: true},
			27017: {Name: "MongoDB", Description: "MongoDB Database", Timeout: 3 * time.Second, Banner: true},
			25565: {Name: "Minecraft", Description: "Minecraft Server", Timeout: 3 * time.Second, Banner: true},
			1194:  {Name: "OpenVPN", Description: "OpenVPN", Timeout: 2 * time.Second, Banner: false},
			1723:  {Name: "PPTP", Description: "Point-to-Point Tunneling", Timeout: 3 * time.Second, Banner: false},
			9200:  {Name: "Elasticsearch", Description: "Elasticsearch HTTP", Timeout: 3 * time.Second, Banner: true},
		},
	}

	// Start background tasks
	go plugin.cacheCleanup()
	go plugin.progressReporter()
	
	return plugin
}

// Enhanced host validation
func (p *NmapPlugin) validateHost(host string) error {
	// Check banned hosts
	if p.bannedHosts[strings.ToLower(host)] {
		return fmt.Errorf("scanning localhost/loopback is not allowed")
	}

	// Try to resolve the host
	ips, err := net.LookupIP(host)
	if err != nil {
		// If lookup fails, try to parse as IP
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("invalid host or IP address")
		}
		ips = []net.IP{ip}
	}

	// Check if any IP is in private networks
	for _, ip := range ips {
		for _, privateNet := range privateNetworks {
			if privateNet.Contains(ip) {
				return fmt.Errorf("scanning private/internal networks is not allowed")
			}
		}
	}

	return nil
}

// Rate limiting check
func (p *NmapPlugin) checkRateLimit(user string) error {
	p.rateLimiter.mu.Lock()
	defer p.rateLimiter.mu.Unlock()

	now := time.Now()

	// Check global cooldown
	if now.Sub(p.rateLimiter.globalLastScan) < p.config.GlobalScanCooldown {
		return fmt.Errorf("global scan cooldown active, wait %v", 
			p.config.GlobalScanCooldown-now.Sub(p.rateLimiter.globalLastScan))
	}

	// Check user cooldown
	if lastScan, exists := p.rateLimiter.userLastScan[user]; exists {
		if now.Sub(lastScan) < p.config.ScanCooldownPerUser {
			return fmt.Errorf("user scan cooldown active, wait %v", 
				p.config.ScanCooldownPerUser-now.Sub(lastScan))
		}
	}

	// Check concurrent scan limits
	if p.rateLimiter.totalActiveScans >= p.config.MaxConcurrentScans {
		return fmt.Errorf("maximum concurrent scans reached (%d)", p.config.MaxConcurrentScans)
	}

	if userScans, exists := p.rateLimiter.activeScansByUser[user]; exists && userScans >= 1 {
		return fmt.Errorf("you already have an active scan")
	}

	// Update rate limiting
	p.rateLimiter.userLastScan[user] = now
	p.rateLimiter.globalLastScan = now
	p.rateLimiter.activeScansByUser[user]++
	p.rateLimiter.totalActiveScans++

	return nil
}

// Release rate limiting
func (p *NmapPlugin) releaseRateLimit(user string) {
	p.rateLimiter.mu.Lock()
	defer p.rateLimiter.mu.Unlock()

	if p.rateLimiter.activeScansByUser[user] > 0 {
		p.rateLimiter.activeScansByUser[user]--
	}
	if p.rateLimiter.totalActiveScans > 0 {
		p.rateLimiter.totalActiveScans--
	}
}

// Enhanced port range parsing with validation
func parsePortRange(rangeStr string) ([]int, error) {
	if strings.Contains(rangeStr, ",") {
		var allPorts []int
		ranges := strings.Split(rangeStr, ",")
		
		for _, r := range ranges {
			ports, err := parsePortRange(strings.TrimSpace(r))
			if err != nil {
				return nil, err
			}
			allPorts = append(allPorts, ports...)
		}
		return allPorts, nil
	}

	if strings.Contains(rangeStr, "-") {
		parts := strings.Split(rangeStr, "-")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid range format")
		}
		start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid start port: %v", err)
		}
		end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("invalid end port: %v", err)
		}
		
		if start > end || start < 1 || end > 65535 {
			return nil, fmt.Errorf("ports must be 1-65535 and start <= end")
		}

		if end-start+1 > 1000 {
			return nil, fmt.Errorf("port range too large (max 1000 ports)")
		}

		var ports []int
		for i := start; i <= end; i++ {
			ports = append(ports, i)
		}
		return ports, nil
	}

	port, err := strconv.Atoi(rangeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port number: %v", err)
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("port must be 1-65535")
	}
	return []int{port}, nil
}

// Generate cache key
func (p *NmapPlugin) getCacheKey(host string, ports []int, network string) string {
	h := fnv.New64a()
	h.Write([]byte(fmt.Sprintf("%s:%s:%v", host, network, ports)))
	return fmt.Sprintf("%x", h.Sum64())
}

// Check cache
func (p *NmapPlugin) getCachedResult(key string) *ScanResult {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	if cached, exists := p.cache[key]; exists {
		if time.Since(cached.timestamp) < p.config.CacheExpiry {
			return cached.result
		}
		delete(p.cache, key)
	}
	return nil
}

// Store in cache
func (p *NmapPlugin) setCachedResult(key string, result *ScanResult) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.cache[key] = &cachedResult{
		result:    result,
		timestamp: time.Now(),
	}
}

// Enhanced message handler
func (p *NmapPlugin) HandleMessage(msg YnMIrC.Message) string {
	text := strings.ToLower(strings.TrimSpace(msg.Text))

	
	hostmask := YnMModule.SimplifyHostmask(msg.Sender)
	prefix := p.adminPlugin.GetPrefixForHost(hostmask)
	nick, hostmask := YnMModule.GetEffectiveNickAndHost(p.adminPlugin, msg.Sender)	
	minLevel := 1 
	
	if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, msg.Channel, minLevel) {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(text), strings.ToLower(prefix+"nmap")) {
		return ""
	}
	parts := strings.Fields(msg.Text)
	if len(parts) < 2 {
		return p.getUsageHelp()
	}

	// Handle special commands
	switch strings.ToLower(parts[1]) {
	case "help", "-h", "--help":
		return p.getDetailedHelp()
	case "status":
		return p.getScanStatus()
	case "stop":
		return p.stopUserScans(nick, msg.Channel)
	case "cache":
		return p.getCacheStatus()
	}

	host := parts[1]

	// Validate host
	if err := p.validateHost(host); err != nil {
		return fmt.Sprintf("❌ %v", err)
	}

	// Check rate limiting
	if err := p.checkRateLimit(nick); err != nil {
		return fmt.Sprintf("⏳ %v", err)
	}

	// Parse arguments
	scanArgs, err := p.parseArguments(parts[2:])
	if err != nil {
		p.releaseRateLimit(nick)
		return fmt.Sprintf("❌ %v", err)
	}

	// Check cache first
	cacheKey := p.getCacheKey(host, scanArgs.Ports, scanArgs.Network)
	if cached := p.getCachedResult(cacheKey); cached != nil {
		p.releaseRateLimit(nick)
		return p.formatCachedResult(cached, host)
	}

	// Start scan
	go p.performScan(host, scanArgs, nick, msg.Channel, cacheKey)
	
	return fmt.Sprintf("🔍 Starting %s scan of %s (%d ports, speed=%s, %s)...", 
		strings.ToUpper(scanArgs.Network), host, len(scanArgs.Ports), 
		scanArgs.SpeedName, scanArgs.Network)
}

// Scan arguments structure
type ScanArguments struct {
	Ports      []int
	Speed      time.Duration
	SpeedName  string
	Network    string
	ScanType   string
}

// Enhanced argument parsing
func (p *NmapPlugin) parseArguments(args []string) (*ScanArguments, error) {
	result := &ScanArguments{
		Ports:     p.commonPorts,
		Speed:     p.config.DefaultTimeout,
		SpeedName: "normal",
		Network:   "tcp",
		ScanType:  "connect",
	}

	for _, arg := range args {
		arg = strings.ToLower(arg)
		
		if strings.HasPrefix(arg, "speed=") {
			speedStr := strings.TrimPrefix(arg, "speed=")
			if speed, ok := p.scanSpeeds[speedStr]; ok {
				result.Speed = speed
				result.SpeedName = speedStr
			} else {
				return nil, fmt.Errorf("invalid speed: %s (use: fast, normal, slow, stealth)", speedStr)
			}
			continue
		}
		
		if arg == "ipv6" {
			result.Network = "tcp6"
			continue
		}
		
		if arg == "ipv4" {
			result.Network = "tcp"
			continue
		}
		
		if arg == "udp" {
			result.Network = "udp"
			continue
		}
		
		if arg == "full" || arg == "all" {
			// Generate all ports 1-65535 (this will be limited by MaxPortsPerScan)
			result.Ports = make([]int, 65535)
			for i := 0; i < 65535; i++ {
				result.Ports[i] = i + 1
			}
			continue
		}
		
		if arg == "top1000" {
			result.Ports = p.getTop1000Ports()
			continue
		}
		
		// Try parsing as port range
		if ports, err := parsePortRange(arg); err == nil {
			if len(ports) > p.config.MaxPortsPerScan {
				return nil, fmt.Errorf("too many ports (max %d, got %d)", p.config.MaxPortsPerScan, len(ports))
			}
			result.Ports = ports
		}
	}
	
	// Final validation for full scans
	if len(result.Ports) > p.config.MaxPortsPerScan {
		return nil, fmt.Errorf("port range too large (max %d ports). Use 'top1000' for common ports or specify smaller ranges", p.config.MaxPortsPerScan)
	}
	
	return result, nil
}

// Get top 1000 most common ports
func (p *NmapPlugin) getTop1000Ports() []int {
	// Top 1000 ports based on nmap's port frequency data
	top1000 := []int{
		1, 3, 4, 6, 7, 9, 13, 17, 19, 20, 21, 22, 23, 24, 25, 26, 30, 32, 33, 37, 42, 43, 49, 53, 70, 79, 80, 81, 82, 83, 84, 85, 88, 89, 90, 99, 100, 106, 109, 110, 111, 113, 119, 125, 135, 139, 143, 144, 146, 161, 163, 179, 199, 211, 212, 222, 254, 255, 256, 259, 264, 280, 301, 306, 311, 340, 366, 389, 406, 407, 416, 417, 425, 427, 443, 444, 445, 458, 464, 465, 481, 497, 500, 512, 513, 514, 515, 524, 541, 543, 544, 545, 548, 554, 555, 563, 587, 593, 616, 617, 625, 631, 636, 646, 648, 666, 667, 668, 683, 687, 691, 700, 705, 711, 714, 720, 722, 726, 749, 765, 777, 783, 787, 800, 801, 808, 843, 873, 880, 888, 898, 900, 901, 902, 903, 911, 912, 981, 987, 990, 992, 993, 995, 999, 1000, 1001, 1002, 1007, 1009, 1010, 1011, 1021, 1022, 1023, 1024, 1025, 1026, 1027, 1028, 1029, 1030, 1031, 1032, 1033, 1034, 1035, 1036, 1037, 1038, 1039, 1040, 1041, 1042, 1043, 1044, 1045, 1046, 1047, 1048, 1049, 1050, 1051, 1052, 1053, 1054, 1055, 1056, 1057, 1058, 1059, 1060, 1061, 1062, 1063, 1064, 1065, 1066, 1067, 1068, 1069, 1070, 1071, 1072, 1073, 1074, 1075, 1076, 1077, 1078, 1079, 1080, 1081, 1082, 1083, 1084, 1085, 1086, 1087, 1088, 1089, 1090, 1091, 1092, 1093, 1094, 1095, 1096, 1097, 1098, 1099, 1100, 1102, 1104, 1105, 1106, 1107, 1108, 1110, 1111, 1112, 1113, 1114, 1117, 1119, 1121, 1122, 1123, 1124, 1126, 1130, 1131, 1132, 1137, 1138, 1141, 1145, 1147, 1148, 1149, 1151, 1152, 1154, 1163, 1164, 1165, 1166, 1169, 1174, 1175, 1183, 1185, 1186, 1187, 1192, 1198, 1199, 1201, 1213, 1216, 1217, 1218, 1233, 1234, 1236, 1244, 1247, 1248, 1259, 1271, 1272, 1277, 1287, 1296, 1300, 1301, 1309, 1310, 1311, 1322, 1328, 1334, 1352, 1417, 1433, 1434, 1443, 1455, 1461, 1494, 1500, 1501, 1503, 1521, 1524, 1533, 1556, 1580, 1583, 1594, 1600, 1641, 1658, 1666, 1687, 1688, 1700, 1717, 1718, 1719, 1720, 1721, 1723, 1755, 1761, 1782, 1783, 1801, 1805, 1812, 1839, 1840, 1862, 1863, 1864, 1875, 1900, 1914, 1935, 1947, 1971, 1972, 1974, 1984, 1998, 1999, 2000, 2001, 2002, 2003, 2004, 2005, 2006, 2007, 2008, 2009, 2010, 2013, 2020, 2021, 2022, 2030, 2033, 2034, 2035, 2038, 2040, 2041, 2042, 2043, 2045, 2046, 2047, 2048, 2049, 2065, 2068, 2099, 2100, 2103, 2105, 2106, 2107, 2111, 2119, 2121, 2126, 2135, 2144, 2160, 2161, 2170, 2179, 2190, 2191, 2196, 2200, 2222, 2251, 2260, 2288, 2301, 2323, 2366, 2381, 2382, 2383, 2393, 2394, 2399, 2401, 2492, 2500, 2522, 2525, 2557, 2601, 2602, 2604, 2605, 2607, 2608, 2638, 2701, 2702, 2710, 2717, 2718, 2725, 2800, 2809, 2811, 2869, 2875, 2909, 2910, 2920, 2967, 2968, 2998, 3000, 3001, 3003, 3005, 3006, 3007, 3011, 3013, 3017, 3030, 3031, 3052, 3071, 3077, 3128, 3168, 3211, 3221, 3260, 3261, 3268, 3269, 3283, 3300, 3301, 3306, 3322, 3323, 3324, 3325, 3333, 3351, 3367, 3369, 3370, 3371, 3372, 3389, 3390, 3404, 3476, 3493, 3517, 3527, 3546, 3551, 3580, 3659, 3689, 3690, 3703, 3737, 3766, 3784, 3800, 3801, 3809, 3814, 3826, 3827, 3828, 3851, 3869, 3871, 3878, 3880, 3889, 3905, 3914, 3918, 3920, 3945, 3971, 3986, 3995, 3998, 4000, 4001, 4002, 4003, 4004, 4005, 4006, 4045, 4111, 4125, 4126, 4129, 4224, 4242, 4279, 4321, 4343, 4443, 4444, 4445, 4446, 4449, 4550, 4567, 4662, 4848, 4899, 4900, 4998, 5000, 5001, 5002, 5003, 5004, 5009, 5030, 5033, 5050, 5051, 5054, 5060, 5061, 5080, 5087, 5100, 5101, 5102, 5120, 5190, 5200, 5214, 5221, 5222, 5225, 5226, 5269, 5280, 5298, 5357, 5405, 5414, 5431, 5432, 5440, 5500, 5510, 5544, 5550, 5555, 5560, 5566, 5631, 5633, 5666, 5678, 5679, 5718, 5730, 5800, 5801, 5802, 5810, 5811, 5815, 5822, 5825, 5850, 5859, 5862, 5877, 5900, 5901, 5902, 5903, 5904, 5906, 5907, 5910, 5911, 5915, 5922, 5925, 5950, 5952, 5959, 5960, 5961, 5962, 5963, 5987, 5988, 5989, 5998, 5999, 6000, 6001, 6002, 6003, 6004, 6005, 6006, 6007, 6009, 6025, 6059, 6100, 6101, 6106, 6112, 6123, 6129, 6156, 6346, 6389, 6502, 6510, 6543, 6547, 6565, 6566, 6567, 6580, 6646, 6666, 6667, 6668, 6669, 6689, 6692, 6699, 6779, 6788, 6789, 6792, 6839, 6881, 6901, 6969, 7000, 7001, 7002, 7004, 7007, 7019, 7025, 7070, 7100, 7103, 7106, 7200, 7201, 7402, 7435, 7443, 7496, 7512, 7625, 7627, 7676, 7741, 7777, 7778, 7800, 7911, 7920, 7921, 7937, 7938, 7999, 8000, 8001, 8002, 8007, 8008, 8009, 8010, 8011, 8021, 8022, 8031, 8042, 8045, 8080, 8081, 8082, 8083, 8084, 8085, 8086, 8087, 8088, 8089, 8090, 8093, 8099, 8100, 8180, 8181, 8192, 8193, 8194, 8200, 8222, 8254, 8290, 8291, 8292, 8300, 8333, 8383, 8400, 8402, 8443, 8500, 8600, 8649, 8651, 8652, 8654, 8701, 8800, 8873, 8888, 8899, 8994, 9000, 9001, 9002, 9003, 9009, 9010, 9011, 9040, 9050, 9071, 9080, 9081, 9090, 9091, 9099, 9100, 9101, 9102, 9103, 9110, 9111, 9200, 9207, 9220, 9290, 9415, 9418, 9485, 9500, 9502, 9503, 9535, 9575, 9593, 9594, 9595, 9618, 9666, 9876, 9877, 9878, 9898, 9900, 9917, 9929, 9943, 9944, 9968, 9998, 9999, 10000, 10001, 10002, 10003, 10004, 10009, 10010, 10012, 10024, 10025, 10082, 10180, 10215, 10243, 10566, 10616, 10617, 10621, 10626, 10628, 10629, 10778, 11110, 11111, 11967, 12000, 12174, 12265, 12345, 13456, 13722, 13782, 13783, 14000, 14238, 14441, 14442, 15000, 15002, 15003, 15004, 15660, 15742, 16000, 16001, 16012, 16016, 16018, 16080, 16113, 16992, 16993, 17877, 17988, 18040, 18101, 18988, 19101, 19283, 19315, 19350, 19780, 19801, 19842, 20000, 20005, 20031, 20221, 20222, 20828, 21571, 22939, 23502, 24444, 24800, 25734, 25735, 26214, 27000, 27352, 27353, 27355, 27356, 27715, 28201, 30000, 30718, 30951, 31038, 31337, 32768, 32769, 32770, 32771, 32772, 32773, 32774, 32775, 32776, 32777, 32778, 32779, 32780, 32781, 32782, 32783, 32784, 32785, 33354, 33899, 34571, 34572, 34573, 35500, 38292, 40193, 40911, 41511, 42510, 44176, 44442, 44443, 44501, 45100, 48080, 49152, 49153, 49154, 49155, 49156, 49157, 49158, 49159, 49160, 49161, 49163, 49165, 49167, 49175, 49176, 49400, 49999, 50000, 50001, 50002, 50003, 50006, 50300, 50389, 50500, 50636, 50800, 51103, 51493, 52673, 52822, 52848, 52869, 54045, 54328, 55055, 55056, 55555, 55600, 56737, 56738, 57294, 57797, 58080, 60020, 60443, 61532, 61900, 62078, 63331, 64623, 64680, 65000, 65129, 65389,
	}
	
	return top1000
	}
	

// Main scan function
func (p *NmapPlugin) performScan(host string, args *ScanArguments, user, channel, cacheKey string) {
	defer p.releaseRateLimit(user)
	
	// Resolve IPs
	ips, err := p.resolveHost(host, args.Network)
	if err != nil {
		p.bot.SendMessage(channel, fmt.Sprintf("❌ %s: %v", host, err))
		return
	}

	for _, ip := range ips {
		ipStr := ip.String()
		
		// Check if already scanning this IP
		p.mu.Lock()
		if _, exists := p.scanning[ipStr]; exists {
			p.mu.Unlock()
			p.bot.SendMessage(channel, fmt.Sprintf("⚠️ Scan already in progress for %s", ipStr))
			continue
		}
		
		// Create scan context
		ctx, cancel := context.WithTimeout(p.ctx, 5*time.Minute)
		progress := &ScanProgress{
			Host:       ipStr,
			TotalPorts: len(args.Ports),
			StartTime:  time.Now(),
			Channel:    channel,
			User:       user,
			Cancel:     cancel,
		}
		p.scanning[ipStr] = progress
		p.mu.Unlock()

		// Perform the actual scan
		result := p.scanHost(ctx, ipStr, args)
		
		// Cleanup
		p.mu.Lock()
		delete(p.scanning, ipStr)
		p.mu.Unlock()
		cancel()

		// Cache and send results
		if result != nil {
			p.setCachedResult(cacheKey, result)
			p.sendScanResults(channel, result)
		}
	}
}

// Host resolution
func (p *NmapPlugin) resolveHost(host string, network string) ([]net.IP, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		if ip := net.ParseIP(host); ip != nil {
			ips = []net.IP{ip}
		} else {
			return nil, fmt.Errorf("could not resolve host")
		}
	}

	var validIPs []net.IP
	for _, ip := range ips {
		switch network {
		case "tcp6", "udp6":
			if ip.To16() != nil && ip.To4() == nil {
				validIPs = append(validIPs, ip)
			}
		default: // tcp, udp
			if ip.To4() != nil {
				validIPs = append(validIPs, ip)
			}
		}
	}

	if len(validIPs) == 0 {
		return nil, fmt.Errorf("no suitable IP addresses found for %s", network)
	}

	return validIPs, nil
}

// Enhanced port scanning with better error handling and service detection
func (p *NmapPlugin) scanHost(ctx context.Context, host string, args *ScanArguments) *ScanResult {
	result := &ScanResult{
		Host:          host,
		Network:       args.Network,
		ScanTimestamp: time.Now(),
		TotalPorts:    len(args.Ports),
	}

	startTime := time.Now()
	var mu sync.Mutex
	var wg sync.WaitGroup
	
	// Sort ports by likelihood (common ports first)
	sortedPorts := p.sortPortsByPriority(args.Ports)
	
	// Create worker pool
	portChan := make(chan int, len(sortedPorts))
	
	// Update progress tracking
	p.mu.Lock()
	if progress, exists := p.scanning[host]; exists {
		progress.ScannedPorts = 0
	}
	p.mu.Unlock()

	// Start workers
	for i := 0; i < p.config.MaxWorkersPerScan; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case port, ok := <-portChan:
					if !ok {
						return
					}
					
					portResult := p.scanPort(ctx, host, port, args)
					
					mu.Lock()
					switch portResult.State {
					case "open":
						result.OpenPorts = append(result.OpenPorts, *portResult)
					case "closed":
						result.ClosedPorts++
					case "filtered":
						result.FilteredPorts++
					}
					
					// Update progress
					p.mu.Lock()
					if progress, exists := p.scanning[host]; exists {
						progress.ScannedPorts++
					}
					p.mu.Unlock()
					
					mu.Unlock()
					
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Send ports to workers
	go func() {
		defer close(portChan)
		for _, port := range sortedPorts {
			select {
			case portChan <- port:
			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
	result.ScanDuration = time.Since(startTime)
	
	// Sort results by port number
	sort.Slice(result.OpenPorts, func(i, j int) bool {
		return result.OpenPorts[i].Port < result.OpenPorts[j].Port
	})

	return result
}

// Enhanced port scanning with service detection
func (p *NmapPlugin) scanPort(ctx context.Context, host string, port int, args *ScanArguments) *PortResult {
	result := &PortResult{
		Port:  port,
		State: "filtered",
	}

	timeout := args.Speed
	if serviceInfo, exists := p.serviceMap[port]; exists {
		timeout = serviceInfo.Timeout
		result.Service = serviceInfo.Name
	}

	startTime := time.Now()
	address := net.JoinHostPort(host, strconv.Itoa(port))
	
	// Create dialer with timeout
	dialer := &net.Dialer{
		Timeout: timeout,
	}

	var conn net.Conn
	var err error

	// Handle different network types
	switch args.Network {
	case "tcp", "tcp6":
		conn, err = dialer.DialContext(ctx, args.Network, address)
	case "udp", "udp6":
		conn, err = dialer.DialContext(ctx, args.Network, address)
		if err == nil {
			// For UDP, send a probe and wait for response
			conn.SetDeadline(time.Now().Add(timeout))
			_, err = conn.Write([]byte("probe"))
			if err == nil {
				buffer := make([]byte, 1024)
				_, err = conn.Read(buffer)
			}
		}
	}

	result.ResponseTime = time.Since(startTime)

	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			result.State = "filtered"
		} else {
			result.State = "closed"
		}
		return result
	}

	defer conn.Close()
	result.State = "open"

	// Service detection for open ports
	if serviceInfo, exists := p.serviceMap[port]; exists {
		if serviceInfo.SSL {
			result.SSL = true
			result.Version = p.detectSSLVersion(conn, timeout)
		} else if serviceInfo.Banner {
			result.Banner = p.readBanner(conn, timeout)
		}
		
		// Enhanced service detection
		result.Version = p.detectServiceVersion(conn, port, timeout)
	} else {
		// Unknown service - try banner grabbing
		result.Banner = p.readBanner(conn, timeout)
		result.Service = p.identifyUnknownService(result.Banner, port)
	}

	return result
}

// Enhanced banner reading with protocol awareness
func (p *NmapPlugin) readBanner(conn net.Conn, timeout time.Duration) string {
	conn.SetReadDeadline(time.Now().Add(timeout))
	
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		return ""
	}

	banner := string(buffer[:n])
	banner = strings.TrimSpace(banner)
	banner = regexp.MustCompile(`[^\x20-\x7E]`).ReplaceAllString(banner, "")
	
	if len(banner) > p.config.MaxBannerLength {
		banner = banner[:p.config.MaxBannerLength] + "..."
	}

	return banner
}

// SSL/TLS version detection
func (p *NmapPlugin) detectSSLVersion(conn net.Conn, timeout time.Duration) string {
	tlsConn := tls.Client(conn, &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         "",
	})
	
	tlsConn.SetDeadline(time.Now().Add(timeout))
	err := tlsConn.Handshake()
	if err != nil {
		return ""
	}
	
	state := tlsConn.ConnectionState()
	return fmt.Sprintf("TLS %s", p.tlsVersionString(state.Version))
}

// Enhanced service version detection
func (p *NmapPlugin) detectServiceVersion(conn net.Conn, port int, timeout time.Duration) string {
	switch port {
	case 80, 8080, 8081:
		return p.detectHTTPVersion(conn, timeout)
	case 21:
		return p.detectFTPVersion(conn, timeout)
	case 22:
		return p.detectSSHVersion(conn, timeout)
	case 25, 587:
		return p.detectSMTPVersion(conn, timeout)
	default:
		return ""
	}
}

// HTTP version detection
func (p *NmapPlugin) detectHTTPVersion(conn net.Conn, timeout time.Duration) string {
	conn.SetDeadline(time.Now().Add(timeout))
	
	request := "HEAD / HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"
	_, err := conn.Write([]byte(request))
	if err != nil {
		return ""
	}

	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		return ""
	}

	response := string(buffer[:n])
	lines := strings.Split(response, "\n")
	
	for _, line := range lines {
		if strings.HasPrefix(line, "Server:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Server:"))
		}
	}
	
	// Try to extract HTTP version from status line
	if len(lines) > 0 && strings.HasPrefix(lines[0], "HTTP/") {
		parts := strings.Fields(lines[0])
		if len(parts) >= 2 {
			return fmt.Sprintf("HTTP Server (%s)", parts[0])
		}
	}
	
	return ""
}

// FTP version detection
func (p *NmapPlugin) detectFTPVersion(conn net.Conn, timeout time.Duration) string {
	conn.SetReadDeadline(time.Now().Add(timeout))
	
	buffer := make([]byte, 512)
	n, err := conn.Read(buffer)
	if err != nil {
		return ""
	}
	
	banner := strings.TrimSpace(string(buffer[:n]))
	if strings.HasPrefix(banner, "220 ") {
		return strings.TrimPrefix(banner, "220 ")
	}
	
	return banner
}

// SSH version detection
func (p *NmapPlugin) detectSSHVersion(conn net.Conn, timeout time.Duration) string {
	conn.SetReadDeadline(time.Now().Add(timeout))
	
	buffer := make([]byte, 256)
	n, err := conn.Read(buffer)
	if err != nil {
		return ""
	}
	
	banner := strings.TrimSpace(string(buffer[:n]))
	if strings.HasPrefix(banner, "SSH-") {
		return banner
	}
	
	return ""
}

// SMTP version detection
func (p *NmapPlugin) detectSMTPVersion(conn net.Conn, timeout time.Duration) string {
	conn.SetReadDeadline(time.Now().Add(timeout))
	
	buffer := make([]byte, 512)
	n, err := conn.Read(buffer)
	if err != nil {
		return ""
	}
	
	banner := strings.TrimSpace(string(buffer[:n]))
	if strings.HasPrefix(banner, "220 ") {
		return strings.TrimPrefix(banner, "220 ")
	}
	
	return banner
}

// Identify unknown services based on banner patterns
func (p *NmapPlugin) identifyUnknownService(banner string, port int) string {
	if banner == "" {
		return "Unknown"
	}
	
	banner = strings.ToLower(banner)
	
	// Common service patterns
	patterns := map[string]string{
		"http":     "HTTP Server",
		"apache":   "Apache HTTP",
		"nginx":    "Nginx HTTP",
		"iis":      "Microsoft IIS",
		"ssh":      "SSH Server",
		"ftp":      "FTP Server",
		"smtp":     "SMTP Server",
		"mysql":    "MySQL Database",
		"postgres": "PostgreSQL Database",
		"redis":    "Redis Database",
		"mongodb":  "MongoDB Database",
		"elastic":  "Elasticsearch",
		"rabbitmq": "RabbitMQ",
		"memcached": "Memcached",
	}
	
	for pattern, service := range patterns {
		if strings.Contains(banner, pattern) {
			return service
		}
	}
	
	return "Unknown Service"
}

// TLS version string helper
func (p *NmapPlugin) tlsVersionString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "1.0"
	case tls.VersionTLS11:
		return "1.1"
	case tls.VersionTLS12:
		return "1.2"
	case tls.VersionTLS13:
		return "1.3"
	default:
		return "Unknown"
	}
}

// Sort ports by scanning priority (common ports first)
func (p *NmapPlugin) sortPortsByPriority(ports []int) []int {
	commonPortsMap := make(map[int]int)
	for i, port := range p.commonPorts {
		commonPortsMap[port] = i
	}
	
	sorted := make([]int, len(ports))
	copy(sorted, ports)
	
	sort.Slice(sorted, func(i, j int) bool {
		priorityI, existsI := commonPortsMap[sorted[i]]
		priorityJ, existsJ := commonPortsMap[sorted[j]]
		
		if existsI && existsJ {
			return priorityI < priorityJ
		}
		if existsI {
			return true
		}
		if existsJ {
			return false
		}
		return sorted[i] < sorted[j]
	})
	
	return sorted
}

// Background cache cleanup
func (p *NmapPlugin) cacheCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			p.mu.Lock()
			now := time.Now()
			for key, cached := range p.cache {
				if now.Sub(cached.timestamp) > p.config.CacheExpiry {
					delete(p.cache, key)
				}
			}
			p.mu.Unlock()
		case <-p.ctx.Done():
			return
		}
	}
}

// Progress reporter
func (p *NmapPlugin) progressReporter() {
	ticker := time.NewTicker(p.config.ProgressUpdateInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			p.mu.RLock()
			for _, progress := range p.scanning {
				if progress.TotalPorts > 0 {
					percent := (progress.ScannedPorts * 100) / progress.TotalPorts
					elapsed := time.Since(progress.StartTime)
					
					if percent > 0 && percent < 100 {
						eta := time.Duration((elapsed.Nanoseconds()/int64(percent))*(100-int64(percent))) * time.Nanosecond
						p.bot.SendMessage(progress.Channel,
							fmt.Sprintf("⏳ %s: %d%% complete (%d/%d ports, ETA: %v)",
								progress.Host, percent, progress.ScannedPorts,
								progress.TotalPorts, eta.Round(time.Second)))
					}
				}
			}
			p.mu.RUnlock()
		case <-p.ctx.Done():
			return
		}
	}
}

// Format and send scan results
func (p *NmapPlugin) sendScanResults(channel string, result *ScanResult) {
	if len(result.OpenPorts) == 0 {
		p.bot.SendMessage(channel,
			fmt.Sprintf("🔴 %s - No open ports found (%d closed, %d filtered, scan: %v)",
				result.Host, result.ClosedPorts, result.FilteredPorts,
				result.ScanDuration.Round(time.Millisecond)))
		return
	}

	// Send summary
	p.bot.SendMessage(channel,
		fmt.Sprintf("🟢 %s - %d open ports found (scan: %v)",
			result.Host, len(result.OpenPorts), result.ScanDuration.Round(time.Millisecond)))

	// Send detailed results in batches
	const maxPortsPerMessage = 8
	for i := 0; i < len(result.OpenPorts); i += maxPortsPerMessage {
		end := i + maxPortsPerMessage
		if end > len(result.OpenPorts) {
			end = len(result.OpenPorts)
		}
		
		var portDetails []string
		for _, port := range result.OpenPorts[i:end] {
			detail := fmt.Sprintf("%d", port.Port)
			
			if port.Service != "" && port.Service != "Unknown" {
				detail += fmt.Sprintf("(%s)", port.Service)
			}
			
			if port.Version != "" {
				detail += fmt.Sprintf("[%s]", port.Version)
			} else if port.Banner != "" {
				detail += fmt.Sprintf("[%s]", port.Banner)
			}
			
			if port.SSL {
				detail += "🔒"
			}
			
			if port.ResponseTime > 0 {
				detail += fmt.Sprintf(" (%v)", port.ResponseTime.Round(time.Millisecond))
			}
			
			portDetails = append(portDetails, detail)
		}
		
		p.bot.SendMessage(channel, fmt.Sprintf("  %s", strings.Join(portDetails, ", ")))
	}
}

// Format cached result message
func (p *NmapPlugin) formatCachedResult(result *ScanResult, host string) string {
	age := time.Since(result.ScanTimestamp)
	
	if len(result.OpenPorts) == 0 {
		return fmt.Sprintf("💾 %s - No open ports (cached %v ago)", host, age.Round(time.Second))
	}
	
	var portStrs []string
	for _, port := range result.OpenPorts {
		if port.Service != "" && port.Service != "Unknown" {
			portStrs = append(portStrs, fmt.Sprintf("%d(%s)", port.Port, port.Service))
		} else {
			portStrs = append(portStrs, strconv.Itoa(port.Port))
		}
	}
	
	return fmt.Sprintf("💾 %s - Open ports: %s (cached %v ago)",
		host, strings.Join(portStrs, ", "), age.Round(time.Second))
}

// Help messages
func (p *NmapPlugin) getUsageHelp() string {
	return "Usage: !nmap <host> [ports] [speed=fast|normal|slow|stealth] [ipv4|ipv6|udp] | !nmap help | !nmap status"
}

func (p *NmapPlugin) getDetailedHelp() string {
	help := []string{
		"🔍 Enhanced Nmap Plugin Help:",
		"Usage: !nmap <host> [options]",
		"",
		"Options:",
		"  ports: 80, 1-1000, 80,443,22 (max 1000 ports)",
		"  speed: fast(500ms), normal(2s), slow(5s), stealth(10s)",
		"  network: ipv4 (default), ipv6, udp",
		"",
		"Commands:",
		"  !nmap help - Show this help",
		"  !nmap status - Show active scans",
		"  !nmap stop - Stop your active scans",
		"  !nmap cache - Show cache status",
		"",
		"Examples:",
		"  !nmap google.com",
		"  !nmap 8.8.8.8 80,443 speed=fast",
		"  !nmap example.com 1-1000 ipv6",
		"",
		"Features: Service detection, SSL detection, Banner grabbing, Smart caching, Rate limiting",
	}
	
	return strings.Join(help, "\n")
}

// Scan status
func (p *NmapPlugin) getScanStatus() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	if len(p.scanning) == 0 {
		return "ℹ️ No active scans"
	}
	
	var status []string
	for host, progress := range p.scanning {
		elapsed := time.Since(progress.StartTime)
		percent := 0
		if progress.TotalPorts > 0 {
			percent = (progress.ScannedPorts * 100) / progress.TotalPorts
		}
		
		status = append(status,
			fmt.Sprintf("%s: %d%% (%d/%d) - %v elapsed (by %s)",
				host, percent, progress.ScannedPorts, progress.TotalPorts,
				elapsed.Round(time.Second), progress.User))
	}
	
	return fmt.Sprintf("📊 Active scans (%d):\n%s", len(p.scanning), strings.Join(status, "\n"))
}

// Stop user scans
func (p *NmapPlugin) stopUserScans(user, channel string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	stopped := 0
	for host, progress := range p.scanning {
		if progress.User == user {
			progress.Cancel()
			delete(p.scanning, host)
			stopped++
		}
	}
	
	if stopped == 0 {
		return "ℹ️ You have no active scans to stop"
	}
	
	return fmt.Sprintf("🛑 Stopped %d scan(s)", stopped)
}

// Cache status
func (p *NmapPlugin) getCacheStatus() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return fmt.Sprintf("💾 Cache: %d entries, expires in %v",
		len(p.cache), p.config.CacheExpiry)
}

// OnTick - required by interface
func (p *NmapPlugin) OnTick() []YnMIrC.Message {
	return nil
}

// Cleanup function (call this when shutting down)
func (p *NmapPlugin) Shutdown() {
	if p.cancel != nil {
		p.cancel()
	}
	
	// Cancel all active scans
	p.mu.Lock()
	for _, progress := range p.scanning {
		progress.Cancel()
	}
	p.scanning = make(map[string]*ScanProgress)
	p.mu.Unlock()
}