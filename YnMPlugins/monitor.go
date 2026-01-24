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
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
)

// --------------------------------------------------
// KONFIG STRUKTÚRA
// --------------------------------------------------

type ServiceMonitorConfig struct {
	ServicesToMonitor []string `yaml:"services_to_monitor"`
	LogFile           string   `yaml:"log_file"`
	AlertInterval     int      `yaml:"alert_interval"`
	CheckInterval     int      `yaml:"check_interval"`
	ConfirmChecks     int      `yaml:"confirm_checks"`
	MChan             []string `yaml:"MChan"`
}

// --------------------------------------------------
// PLUGIN STRUCT
// --------------------------------------------------

type ServiceMonitorPlugin struct {
	bot           *YnMIrC.Client
	channels      []string
	config        *ServiceMonitorConfig
	lastAlerts    map[string]time.Time
	failCount     map[string]int
	mutex         sync.RWMutex
	ticker        *time.Ticker
	stopChan      chan struct{}
	checkInterval time.Duration
}

// --------------------------------------------------
// CONFIG BETÖLTÉS
// --------------------------------------------------

func LoadServiceMonitorConfig() (*ServiceMonitorConfig, error) {
	configPath := "./YnMConfig/monitor.yaml"
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("nem sikerült megnyitni a config fájlt (%s): %w", configPath, err)
	}
	defer file.Close()

	var config ServiceMonitorConfig
	decoder := yaml.NewDecoder(file)

	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("YAML feldolgozási hiba: %w", err)
	}

	if config.CheckInterval <= 0 {
		return nil, fmt.Errorf("check_interval értéke nem lehet 0 vagy negatív")
	}
	if config.AlertInterval <= 0 {
		return nil, fmt.Errorf("alert_interval értéke nem lehet 0 vagy negatív")
	}
	if config.ConfirmChecks <= 0 {
		config.ConfirmChecks = 3
	}

	return &config, nil
}

// --------------------------------------------------
// PLUGIN LÉTREHOZÁS
// --------------------------------------------------

func NewServiceMonitorPlugin(bot *YnMIrC.Client, channels []string) (*ServiceMonitorPlugin, error) {
	config, err := LoadServiceMonitorConfig()
	if err != nil {
		return nil, err
	}

	checkInterval := time.Duration(config.CheckInterval) * time.Minute

	p := &ServiceMonitorPlugin{
		bot:           bot,
		channels:      channels,
		config:        config,
		lastAlerts:    make(map[string]time.Time),
		failCount:     make(map[string]int),
		stopChan:      make(chan struct{}),
		checkInterval: checkInterval,
	}

	p.clearLogFile()
	p.loadAlertsFromLog()

	return p, nil
}

// --------------------------------------------------
// START / STOP
// --------------------------------------------------

func (p *ServiceMonitorPlugin) Start() {
	p.ticker = time.NewTicker(p.checkInterval)

	go func() {
		for {
			select {
			case <-p.ticker.C:
				p.checkServices()
			case <-p.stopChan:
				p.ticker.Stop()
				return
			}
		}
	}()
}

func (p *ServiceMonitorPlugin) Stop() {
	close(p.stopChan)
	if p.ticker != nil {
		p.ticker.Stop()
	}
}

// --------------------------------------------------
// SZOLGÁLTATÁSOK ELLENŐRZÉSE
// --------------------------------------------------

func (p *ServiceMonitorPlugin) checkServices() {
	now := time.Now()

	// Éjszaka ne spam-eljen
	if now.Hour() >= 22 {
		return
	}

	for _, service := range p.config.ServicesToMonitor {
		isActive := p.checkServiceStatus(service)

		p.mutex.Lock()

		if !isActive {
			p.failCount[service]++

			if p.failCount[service] == p.config.ConfirmChecks {
				lastAlert, exists := p.lastAlerts[service]
				if !exists || now.Sub(lastAlert).Seconds() > float64(p.config.AlertInterval) {
					p.lastAlerts[service] = now
					p.mutex.Unlock()
					p.sendAlert(service, "down")
					p.logAlert(service, "down")
				} else {
					p.mutex.Unlock()
				}
			} else {
				p.mutex.Unlock()
			}

		} else {
			p.failCount[service] = 0

			if _, exists := p.lastAlerts[service]; exists {
				delete(p.lastAlerts, service)
				p.mutex.Unlock()
				p.sendAlert(service, "up")
				p.removeServiceFromLog(service)
			} else {
				p.mutex.Unlock()
			}
		}
	}
}

// --------------------------------------------------
// SYSTEMCTL CHECKER
// --------------------------------------------------

func (p *ServiceMonitorPlugin) checkServiceStatus(serviceName string) bool {
	cmd := exec.Command("systemctl", "is-active", serviceName)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("❌ Hiba a %s szolgáltatás ellenőrzésekor: %v", serviceName, err)
		return false
	}
	return strings.TrimSpace(string(output)) == "active"
}

// --------------------------------------------------
// RIASZTÁS KÜLDÉSE
// --------------------------------------------------

func (p *ServiceMonitorPlugin) sendAlert(service, status string) {
	var msg string

	if status == "down" {
		msg = fmt.Sprintf("🚨 A %s szolgáltatás LEÁLLT! 🚨", service)
	} else {
		msg = fmt.Sprintf("🟢 A %s szolgáltatás ÚJRA FUT! 🟢", service)
	}

	chans := p.channels
	if len(p.config.MChan) > 0 {
		chans = p.config.MChan
	}

	for _, ch := range chans {
		p.bot.SendMessage(ch, msg)
		//log.Printf("✔ Üzenet elküldve: %s -> %s", ch, msg)
	}
}

// --------------------------------------------------
// LOG KEZELÉS
// --------------------------------------------------

func (p *ServiceMonitorPlugin) logAlert(service, status string) {
	if status != "down" {
		return
	}

	file, err := os.OpenFile(p.config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("❌ Log fájl megnyitási hiba: %v", err)
		return
	}
	defer file.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	entry := fmt.Sprintf("[%s] A %s szolgáltatás leállt!\n", timestamp, service)

	file.WriteString(entry)
}

func (p *ServiceMonitorPlugin) clearLogFile() {
	if _, err := os.Stat(p.config.LogFile); err == nil {
		os.Remove(p.config.LogFile)
	}
}

func (p *ServiceMonitorPlugin) loadAlertsFromLog() {
	file, err := os.Open(p.config.LogFile)
	if err != nil {
		return
	}
	defer file.Close()

	sc := bufio.NewScanner(file)
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, "leállt") {
			parts := strings.Fields(line)
			if len(parts) >= 5 {
				service := parts[4]
				timeStr := strings.Trim(parts[0], "[]") + " " + strings.Trim(parts[1], "[]")
				if ts, err := time.Parse("2006-01-02 15:04:05", timeStr); err == nil {
					p.lastAlerts[service] = ts
				}
			}
		}
	}
}

func (p *ServiceMonitorPlugin) removeServiceFromLog(service string) {
	file, err := os.Open(p.config.LogFile)
	if err != nil {
		return
	}
	defer file.Close()

	var lines []string

	sc := bufio.NewScanner(file)
	for sc.Scan() {
		if !strings.Contains(sc.Text(), service) {
			lines = append(lines, sc.Text())
		}
	}

	out, _ := os.OpenFile(p.config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	defer out.Close()

	for _, l := range lines {
		out.WriteString(l + "\n")
	}
}

// --------------------------------------------------
// BOT HOZZÁFÉRÉS
// --------------------------------------------------

func (p *ServiceMonitorPlugin) Name() string {
	return "Service Monitor"
}

func (p *ServiceMonitorPlugin) HandleMessage(msg YnMIrC.Message) string {
	return ""
}

func (p *ServiceMonitorPlugin) OnTick() []YnMIrC.Message {
	return nil
}

func (p *ServiceMonitorPlugin) HandleCommand(cmd, channel, nick string, args []string) {
	if cmd == "service_status" {
		p.bot.SendMessage(channel, "ℹ️ A szolgáltatásfigyelő plugin aktív.")
	}
}
