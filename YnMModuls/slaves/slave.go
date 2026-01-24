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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	"strings"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMCmd"
)

// Start - Slave bot indítása
func (sb *SlaveBot) Start() error {
    // ✅ Csatorna nevek normalizálása RÖGTÖN az elején
    sb.normalizeChannels()
    
    log.Printf("[%s] 📋 Normalized channels: %v", sb.config.Name, sb.config.Channels)
    log.Printf("[%s] 📋 Normalized topic channel: %s", sb.config.Name, sb.config.TopicChannel)
    
    ircConfig := &YnMConfig.Config{
        Server:                sb.config.Server,
        Port:                  fmt.Sprintf("%d", sb.config.Port),
        UseTLS:                sb.config.SSL,
        NickName:              sb.config.Nickname,
        UserName:              sb.config.Username,
        RealName:              sb.config.Realname,
        Channels:              sb.config.Channels,      // ← már normalizált
        ReconnectOnDisconnect: 5,
        UseSASL:               false,
        Version:               "YnM-Go Slave Bot 1.0",
        ConsoleChannel:        sb.config.TopicChannel,  // ← már normalizált
        TopicUpdateInterval:   sb.config.TopicUpdateInterval,
    }

    sb.ircClient = YnMIrC.NewClient(ircConfig)
    sb.ircClient.SetChannelModeHandler(&YnMIrC.EmptyChannelModeHandler{})
    sb.setupIRCHandlers()
    
// ✅ Topic updater inicializálás ADMINPLUGIN NÉLKÜL
    if sb.config.TopicChannel != "" {
        log.Printf("[%s] 📋 Initializing topic updater for %s (interval: %s)", 
            sb.config.Name, sb.config.TopicChannel, sb.config.TopicUpdateInterval)
        
        // ✅ NIL adminPlugin átadása - slave botnak nincs rá szüksége
        sb.topicUpdater = YnMCmd.NewTopicUpdaterPlugin(sb.ircClient, ircConfig, nil)
        
        sb.topicUpdater.StartAutoInit(15 * time.Second)
        go sb.topicTickerLoop()
    }

    log.Printf("[%s] 🔌 Connecting to IRC server %s:%d...", sb.config.Name, sb.config.Server, sb.config.Port)

    go func() {
        if err := sb.ircClient.Connect(); err != nil {
            log.Printf("[%s] ❌ IRC connection error: %v", sb.config.Name, err)
        } else {
            log.Printf("[%s] ✅ IRC connection successful", sb.config.Name)
        }
    }()

    go sb.connectToMaster()

    go func() {
        time.Sleep(30 * time.Second)
        if sb.masterConn == nil {
            sb.standalone = true
            log.Printf("[%s] 🔶 STANDALONE MODE ACTIVATED (master not available)", sb.config.Name)
        }
    }()

    log.Printf("[%s] ✅ Slave bot started", sb.config.Name)
    return nil
}

func (sb *SlaveBot) topicTickerLoop() {
    if sb.topicUpdater == nil {
        return
    }
    
    log.Printf("[%s] 📋 Topic ticker loop started", sb.config.Name)
    
    ticker := time.NewTicker(10 * time.Second) // 10 másodpercenként ellenőriz
    defer ticker.Stop()
    
    for sb.running {
        select {
        case <-ticker.C:
            if sb.topicUpdater != nil {
                // ✅ OnTick() meghívása - ez hiányzott!
                sb.topicUpdater.OnTick()
            }
        }
    }
    
    log.Printf("[%s] 📋 Topic ticker loop stopped", sb.config.Name)
}

func (sb *SlaveBot) normalizeChannels() {
    // Channels normalizálása
    for i, ch := range sb.config.Channels {
        sb.config.Channels[i] = strings.ToLower(ch)
    }
    
    // TopicChannel normalizálása
    if sb.config.TopicChannel != "" {
        sb.config.TopicChannel = strings.ToLower(sb.config.TopicChannel)
    }
}


// Stop - Slave bot leállítása
func (sb *SlaveBot) Stop() {
    log.Printf("[%s] 🛑 Stopping slave bot...", sb.config.Name)
    sb.running = false

    time.Sleep(1 * time.Second)

    // Topic updater leállítása (ha fut)
    if sb.topicUpdater != nil {
        sb.topicUpdater = nil
        log.Printf("[%s] 📋 Topic updater stopped", sb.config.Name)
    }

    sb.handlerMutex.Lock()
    if sb.masterConn != nil {
        sb.masterConn.Close()
        sb.masterConn = nil
    }
    sb.handlerRunning = false
    sb.handlerMutex.Unlock()

    if sb.ircClient != nil {
        sb.ircClient.Disconnect()
    }

    log.Printf("[%s] ✅ Slave bot stopped", sb.config.Name)
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	configFile := flag.String("config", "", "Slave bot configuration JSON")
	socketPath := flag.String("socket", "data/ynm-go.sock", "Master socket path")
	botName := flag.String("name", "", "Bot name")
	flag.Parse()

	if *configFile == "" {
		log.Fatal("Usage: slave -config <config.json> -socket <socket_path> [-name <bot_name>]")
	}

	// Konfiguráció betöltése
	data, err := os.ReadFile(*configFile)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	var config SlaveConfig
	if err := json.Unmarshal(data, &config); err != nil {
		log.Fatalf("Failed to parse config: %v", err)
	}

	if config.Name == "" {
		if *botName != "" {
			config.Name = *botName
		} else {
			config.Name = config.Nickname
		}
	}

	if config.Name == "" {
		log.Fatal("Bot name is required!")
	}

	// ABSZOLÚT útvonal
	fullSocketPath := *socketPath
	if !filepath.IsAbs(fullSocketPath) {
		cwd, _ := os.Getwd()
		fullSocketPath = filepath.Join(cwd, fullSocketPath)
	}

	log.Printf("🤖 Slave Bot Starting (PID: %d)", os.Getpid())
	log.Printf("   Name: %s", config.Name)
	log.Printf("   Server: %s:%d", config.Server, config.Port)
	log.Printf("   Socket: %s", fullSocketPath)
	log.Printf("   Channels: %v", config.Channels)

	syscall.Setsid()

	bot := NewSlaveBot(config, fullSocketPath)
	if err := bot.Start(); err != nil {
		log.Fatalf("Failed to start slave bot: %v", err)
	}

	log.Printf("[%s] ✅ Slave bot started successfully", config.Name)

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("[%s] Received signal: %v", config.Name, sig)

	log.Println("Shutting down slave bot...")
	bot.Stop()

	log.Printf("[%s] Slave bot stopped", config.Name)
}