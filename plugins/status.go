package plugins

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/process"
	"github.com/ynmhu/YnM-Go/irc"
)

type StatusPlugin struct {
	client    *irc.Client
	startTime time.Time
}

var threadNames = []string{"MainThread", "uptime", "known_users", "message_sender", "auto_update"}

func NewStatusPlugin(client *irc.Client) *StatusPlugin {
	return &StatusPlugin{
		client:    client,
		startTime: time.Now(),
	}
}

func (p *StatusPlugin) HandleMessage(msg irc.Message) string {
	if strings.TrimSpace(msg.Text) == "!status" {
		p.StatusCommand(msg.Sender, msg.Channel)
		return ""
	}
	return ""
}

func (p *StatusPlugin) SendMessage(channel, text string) {
	p.client.SendMessage(channel, text)
}

func (p *StatusPlugin) getTotalMemoryMB() (float64, error) {
	v, err := mem.VirtualMemory()
	if err == nil {
		return float64(v.Total) / 1024 / 1024, nil
	}
	// fallback /proc/meminfo
	data, err2 := os.ReadFile("/proc/meminfo")
	if err2 != nil {
		return 0, err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0, fmt.Errorf("unexpected MemTotal line: %s", line)
			}
			kb, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				return 0, err
			}
			return kb / 1024.0, nil // MB
		}
	}
	return 0, fmt.Errorf("MemTotal not found")
}

func (p *StatusPlugin) getCpuUsagePercent() (float64, error) {
	percent, err := cpu.Percent(time.Second, false)
	if err != nil || len(percent) == 0 {
		return 0, fmt.Errorf("CPU usage not available")
	}
	return percent[0], nil
}

func (p *StatusPlugin) getProcessMemoryMB() float64 {
	pid := int32(os.Getpid())
	proc, err := process.NewProcess(pid)
	if err != nil {
		return 0
	}
	memInfo, err := proc.MemoryInfo()
	if err != nil {
		return 0
	}
	return float64(memInfo.RSS) / 1024.0 / 1024.0 // MB
}

func (p *StatusPlugin) StatusCommand(nick, channel string) {
	threadCount := runtime.NumGoroutine()
	threadList := strings.Join(threadNames, ", ")

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	gcObjects := memStats.Mallocs - memStats.Frees
	ramUsed := float64(memStats.Alloc) / 1024.0 / 1024.0

	totalMemMB, err := p.getTotalMemoryMB()
	if err != nil {
		totalMemMB = 0
	}

	cpuPercent, err := p.getCpuUsagePercent()
	if err != nil {
		cpuPercent = -1
	}

	processMemMB := p.getProcessMemoryMB()
    
		tlsStatus := "🔓 Insecure"
	if p.client.IsTLS() {
		tlsStatus = "🔐 TLS enabled"
	}
	osType := runtime.GOOS
	arch := runtime.GOARCH
	uptime := time.Since(p.startTime).Truncate(time.Second)

	// A botod adatainak lekérése (dummy értékek, cseréld saját adataidra)
	loggedUsers := len(p.client.GetLoggedUsers())    // vagy hasonló
	channels := len(p.client.GetJoinedChannels())    // ha van ilyen metódusod, különben dummy
	botNick := p.client.GetNick()                     // ha van getter, különben konstans

	p.SendMessage(channel, "📊 *Advanced Status Report*")
	p.SendMessage(channel, fmt.Sprintf("🔢 Threads: %d — %s", threadCount, threadList))
	p.SendMessage(channel, fmt.Sprintf("👥 Logged Users: %d | 🧑‍🤝‍🧑 Channels: %d", loggedUsers, channels))
	p.SendMessage(channel, fmt.Sprintf("📦 GC Objects: %d", gcObjects))
	p.SendMessage(channel, tlsStatus)
	p.SendMessage(channel, fmt.Sprintf("🧠 RAM (Go heap): %.2f MB | RAM (process): %.2f MB / %.0f MB", ramUsed, processMemMB, totalMemMB))
	p.SendMessage(channel, fmt.Sprintf("🔄 CPU: %s", func() string {
		if cpuPercent < 0 {
			return "n/a"
		}
		return fmt.Sprintf("%.2f%%", cpuPercent)
	}()))
	p.SendMessage(channel, fmt.Sprintf("💻 System: %s | CPU Arch: %s", osType, arch))
	p.SendMessage(channel, fmt.Sprintf("⏱️ Uptime: %s", uptime.String()))
	p.SendMessage(channel, fmt.Sprintf("🤖 Bot nick: %s", botNick))
}

func (p *StatusPlugin) OnTick() []irc.Message {
	return nil
}
