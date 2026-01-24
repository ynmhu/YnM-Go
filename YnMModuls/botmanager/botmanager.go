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

package botmanager

import (
//	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"gopkg.in/yaml.v3"
)

type BotManagerPlugin struct {
	bot          *YnMIrC.Client
	config       *SlavesConfig
	slaves       map[string]*ManagedSlave
	shuttingDown bool
	mutex        sync.RWMutex
	socketPath   string
	listener     net.Listener
	stopChan     chan struct{}
	stateFile    string
	dataDir      string
	pluginManager PluginManagerInterface
	adminPlugin *owner.YnmAdminPlugin
	startTime    time.Time 
}

type PluginManagerInterface interface {
	HandleMessage(msg YnMIrC.Message) string
}

type PrivateMessageHandler interface {
    HandlePrivateMessage(nick, target, msg, hostmask string, isPrivate bool) string
}
type SlavesConfig struct {
	Slaves    map[string]SlaveConfig `yaml:"slaves"`
	Socket    SocketConfig           `yaml:"socket"`
	StateFile string                 `yaml:"state_file"`
}

type SlaveConfig struct {
	Server    string   `yaml:"server"`
	Port      int      `yaml:"port"`
	SSL       bool     `yaml:"ssl"`
	Nickname  string   `yaml:"nickname"`
	Username  string   `yaml:"username"`
	Realname  string   `yaml:"realname"`
	Channels  []string `yaml:"channels"`
	AutoStart bool     `yaml:"auto_start"`
	TopicChannel        string `yaml:"topic_channel"`
	TopicUpdateInterval string `yaml:"topic_update_interval"`
}

type SocketConfig struct {
	Path    string `yaml:"path"`
	Timeout string `yaml:"timeout"`
}

type ManagedSlave struct {
	Name      string
	Config    SlaveConfig
	PID       int
	Status    string
	StartedAt time.Time
	Conn      net.Conn
	Process   *os.Process
}

type SlaveState struct {
	Name            string    `json:"name"`
	PID             int       `json:"pid"`
	Status          string    `json:"status"`
	SocketConnected bool      `json:"socket_connected"`
	StartedAt       time.Time `json:"started_at"`
}

type StateFile struct {
	Slaves map[string]SlaveState `json:"slaves"`
}

// NewBotManagerPlugin inicializálja a bot manager plugint
func NewBotManagerPlugin(bot *YnMIrC.Client, configPath string, pluginManager PluginManagerInterface, adminPlugin *owner.YnmAdminPlugin) (*BotManagerPlugin, error) {
	config, err := loadSlavesConfig(configPath)
    if err != nil {
        return nil, fmt.Errorf("slaves.yaml betöltési hiba: %v", err)
    }

    dataDir := "data"
    if err := os.MkdirAll(dataDir, 0755); err != nil {
        return nil, fmt.Errorf("data könyvtár létrehozási hiba: %v", err)
    }

    socketPath := config.Socket.Path
    if socketPath == "" {
        socketPath = "data/ynm-go.sock"
    }

    stateFile := config.StateFile
    if stateFile == "" {
        stateFile = "data/slaves_state.json"
    }

    bm := &BotManagerPlugin{
        bot:           bot,
        config:        config,
        slaves:        make(map[string]*ManagedSlave),
        socketPath:    socketPath,
        stopChan:      make(chan struct{}),
        stateFile:     stateFile,
        dataDir:       dataDir,
        pluginManager: pluginManager,
        adminPlugin:   adminPlugin, 
		 startTime:     time.Now(),
    }

    socketDir := filepath.Dir(bm.socketPath)
    if err := os.MkdirAll(socketDir, 0755); err != nil {
        return nil, fmt.Errorf("socket mappa létrehozási hiba: %v", err)
    }

    bm.loadState()

    //log.Printf("✅ BotManager plugin inicializálva")
    //log.Printf("   📁 Slave botok: %d db", len(config.Slaves))
    //log.Printf("   🔌 Socket: %s", bm.socketPath)
    //log.Printf("   💾 State fájl: %s", bm.stateFile)
    //log.Printf("   📂 Data könyvtár: %s", bm.dataDir)
    
    if adminPlugin != nil {
        //log.Printf("   🔐 owner plugin: ✅ csatlakoztatva")
    } else {
        //log.Printf("   🔐 owner plugin: ❌ nincs csatlakoztatva")
    }

    return bm, nil
}

func loadSlavesConfig(path string) (*SlavesConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config SlavesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// Start elindítja a bot managert
func (bm *BotManagerPlugin) Start() {
	//log.Printf("🚀 [BOTMANAGER] Start() metódus meghívva!")

	go func() {
		defer func() {
			if r := recover(); r != nil {
				//log.Printf("❌❌❌ [PANIC] Socket szerver panic: %v", r)
			}
		}()
		//log.Printf("🔌 [GOROUTINE] Socket szerver goroutine elindult")
		bm.startSocketServer()
		//log.Printf("🔌 [GOROUTINE] Socket szerver goroutine véget ért")
	}()

	// ✅ ÚJ: Health check goroutine
	go bm.healthCheckLoop()

	bm.autoStartSlaves()

	//log.Printf("✅ [BOTMANAGER] Start() befejezve")
}

// ✅ ÚJ FÜGGVÉNY - Periodikus health check
func (bm *BotManagerPlugin) healthCheckLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	//log.Printf("🏥 Health check loop started (every 5 minutes)")
	
	for {
		select {
		case <-bm.stopChan:
			//log.Printf("🛑 Health check loop stopped")
			return
		case <-ticker.C:
			bm.performHealthCheck()
		}
	}
}
func (bm *BotManagerPlugin) performHealthCheck() {
	// ✅ PANIC VÉDELEM - ha bármi hiba van, ne álljon le
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌❌❌ [PANIC RECOVERY] Health check panic: %v", r)
		}
	}()
	
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()
	
	for name, slave := range bm.slaves {
		// ✅ VÉDETT MÓDON ellenőrizzük a connection-t
		func(slaveName string, s *ManagedSlave) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("❌ Health check panic for %s: %v", slaveName, r)
				}
			}()
			
			// Ellenőrzések
			if s == nil {
				log.Printf("⚠️ Health check: %s - Slave is nil", slaveName)
				return
			}
			
			if s.Conn == nil {
				log.Printf("⚠️ Health check: %s - No socket connection", slaveName)
				return
			}
			
			// Lokális másolat
			conn := s.Conn
			if conn == nil {
				return
			}
			
			// Connection teszt
			conn.SetReadDeadline(time.Now().Add(-1 * time.Second))
			buf := make([]byte, 1)
			_, err := conn.Read(buf)
			
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Timeout = OK, connection él
				} else {
					// Valódi hiba
					log.Printf("❌ Health check: %s - Connection dead: %v", slaveName, err)
					
					// Aszinkron bezárás
					go func() {
						bm.mutex.Lock()
						defer bm.mutex.Unlock()
						
						if s.Conn == conn && s.Conn != nil {
							s.Conn.Close()
							s.Conn = nil
						}
					}()
				}
			}
			
			// Deadline visszaállítás
			if s.Conn != nil && s.Conn == conn {
				conn.SetReadDeadline(time.Time{})
			}
		}(name, slave)
	}
}

// Stop leállítja a bot managert
func (bm *BotManagerPlugin) Stop() {
	log.Printf("⏹️  BotManager plugin leállítása...")
	
	bm.mutex.Lock()
	bm.shuttingDown = true
	bm.mutex.Unlock()
	
	close(bm.stopChan)
	
	if bm.listener != nil {
		bm.listener.Close()
	}

	bm.saveState()
	
	//log.Printf("✅ BotManager plugin leállt - slave botok tovább futnak standalone módban")
}

func (bm *BotManagerPlugin) Name() string {
	return "Bot Manager"
}

func (bm *BotManagerPlugin) OnTick() []YnMIrC.Message {
	return nil
}