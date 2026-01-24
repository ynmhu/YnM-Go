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

package owner

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
//	"reflect"
	"strings"
	"sync"
	"time"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	_ "git.ynm.hu/markus/YnM-Go/YnMLang"
	"git.ynm.hu/markus/YnM-Go/YnMDb"
)

const (
	API_BASE_URL       = "https://ynm-go.ynm.hu/ynm/api"
	HEARTBEAT_INTERVAL = 5 * time.Minute
	REQUEST_TIMEOUT    = 15 * time.Second
	MAX_RETRIES        = 3
	RETRY_DELAY        = 15 * time.Second
)
var YnMVersion string = "YnM-v1.0.40.40"

type BotData struct {
	BotUUID   string `json:"bot_uuid"`
	Nick      string `json:"nick"`
	Uptime    int64  `json:"uptime"`
	Version   string `json:"version,omitempty"`
	IRCServer string `json:"irc_server,omitempty"`
	Runner    string `json:"runner,omitempty"`
	Country   string `json:"country,omitempty"`
}

// API Response struktúra
type APIResponse struct {
	Message string `json:"message,omitempty"`
	Status  string `json:"status,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Bot monitoring manager
type BotMonitor struct {
	uuid       string
	startTime  time.Time
	version    string
	client     *http.Client
	configPath string
	apiSecret  string
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	logger     *log.Logger
}
func (bm *BotMonitor) generateBotIdentifier() string {
	nick, ircServer, runner, _ := bm.getCurrentConfigData()
	absConfigPath, _ := filepath.Abs(bm.configPath)
	identifier := fmt.Sprintf("%s_%s_%s_%s", 
		nick, 
		strings.ReplaceAll(ircServer, ":", "_"), 
		runner,
		strings.ReplaceAll(absConfigPath, string(filepath.Separator), "_"))
	identifier = strings.ReplaceAll(identifier, ".", "_")
	identifier = strings.ReplaceAll(identifier, "/", "_")
	identifier = strings.ReplaceAll(identifier, "\\", "_")
	identifier = strings.ReplaceAll(identifier, " ", "_")
	
	// Hossz korlátozása
	if len(identifier) > 50 {
		identifier = identifier[:50]
	}
	
	return identifier
}

func (bm *BotMonitor) loadAPISecret() string {
	// 1. Környezeti változó (globális)
	if secret := os.Getenv("YNM_API_SECRET"); secret != "" {
		bm.logger.Println("API secret betöltve környezeti változóból")
		return secret
	}
	
	// 2. Bot-specifikus környezeti változó (UUID alapján, ha már van)
	if bm.uuid != "" {
		envVar := fmt.Sprintf("YNM_API_SECRET_%s", strings.ReplaceAll(bm.uuid, "-", "_"))
		if secret := os.Getenv(envVar); secret != "" {
			bm.logger.Printf("API secret betöltve bot-specifikus környezeti változóból: %s", envVar)
			return secret
		}
	}
	
	// 3. Config fájlból (.ynm_bot/api_secret.txt)
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		homeDir = "."
	}
	secretFile := filepath.Join("data", ".ynm_bot", "api_secret.txt")
	if data, err := os.ReadFile(secretFile); err == nil {
		secret := strings.TrimSpace(string(data))
		if secret != "" {
			bm.logger.Println("API secret betöltve config fájlból")
			return secret
		}
	}
	
	// 4. Bot-specifikus config fájl
	if bm.uuid != "" {
		botSecretFile := filepath.Join("data", ".ynm_bot", fmt.Sprintf("api_secret_%s.txt", bm.uuid))
		if data, err := os.ReadFile(botSecretFile); err == nil {
			secret := strings.TrimSpace(string(data))
			if secret != "" {
				bm.logger.Printf("API secret betöltve bot-specifikus fájlból: %s", botSecretFile)
				return secret
			}
		}
	}
	
	return ""
}

// Új monitor létrehozása
func NewBotMonitor(configPath string) *BotMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Biztonságos HTTP kliens TLS konfigurációval
	client := &http.Client{
		Timeout: REQUEST_TIMEOUT,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false, // Biztonság kedvéért
			},
		},
	}

	monitor := &BotMonitor{
		configPath: configPath,
		startTime:  time.Now(),
		version: YnMVersion, 
		client:     client,
		ctx:        ctx,
		cancel:     cancel,
		logger:     log.New(os.Stdout, "[Monitor] ", log.LstdFlags),
	}

	// UUID betöltése vagy generálása (API secret előtt!)
	if err := monitor.loadOrGenerateUUID(); err != nil {
		monitor.logger.Printf("UUID hiba: %v", err)
	}

	// API secret betöltése több forrásból
	monitor.apiSecret = monitor.loadAPISecret()
	if monitor.apiSecret == "" {
		monitor.apiSecret = "YnM@B0t!k25#2Xq" // Fallback, de VÁLTOZTATNI KELL!
	//	monitor.logger.Println("FIGYELEM: Alapértelmezett API secret használata!")
	}
	
	return monitor
}

// Graceful shutdown
func (bm *BotMonitor) Shutdown() {
	bm.cancel()
	bm.logger.Println("Monitoring leállítva")
}


func (bm *BotMonitor) getCurrentConfigData() (nick, ircServer, runner, country string) {
    // Alapértelmezett értékek
    nick = "YnM-Go"
    ircServer = "ynm.hu:6667"
    runner = "YnM"
    country = "HU"

    // Config újratöltése
    cfg, err := YnMConfig.Load(bm.configPath)
    if err != nil {
        bm.logger.Printf("Config betöltési hiba: %v", err)
        return
    }

    if cfg != nil {
        if cfg.NickName != "" {
            nick = cfg.NickName
        }

        if cfg.Server != "" {
            server := cfg.Server
            var port string
            if cfg.UseTLS {
                port = cfg.TLSPort
                if port == "" {
                    port = "6697"
                }
            } else {
                port = cfg.Port
                if port == "" {
                    port = "6667"
                }
            }
            ircServer = server + ":" + port
        }

        // 🔹 Runner felülírás az adatbázisból
if dbConn, err := YnMDb.NewAdminDB(); err == nil {
    defer dbConn.Close()

    if ownerNick, err := dbConn.GetAnyownerNick(); err == nil && ownerNick != "" {
        runner = ownerNick
    } else if cfg.UserName != "" {
        runner = cfg.UserName
    }
} else {
    bm.logger.Printf("DB kapcsolat hiba Runner lekéréshez: %v", err)
    if cfg.UserName != "" {
        runner = cfg.UserName
    }
}


        if envCountry := os.Getenv("YNM_COUNTRY"); envCountry != "" {
            country = envCountry
        }
    }

    //bm.logger.Printf("Friss config adatok: nick=%s, server=%s, runner=%s, country=%s",
    //    nick, ircServer, runner, country)

    return
}

func (bm *BotMonitor) loadOrGenerateUUID() error {
	configDir := filepath.Join("data", ".ynm_bot") 
	
	// Bot-specifikus UUID fájl név generálása a config alapján
	botIdentifier := bm.generateBotIdentifier()
	uuidFile := filepath.Join(configDir, fmt.Sprintf("bot_uuid_%s.txt", botIdentifier))

	// Könyvtár létrehozása
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("config könyvtár létrehozási hiba: %v", err)
	}

	//bm.logger.Printf("UUID fájl: %s", uuidFile)
	//bm.logger.Printf("Bot azonosító: %s", botIdentifier)

	// UUID betöltése
	if data, err := os.ReadFile(uuidFile); err == nil {
		uuid := strings.TrimSpace(string(data))
		if len(uuid) > 0 {
			bm.mu.Lock()
			bm.uuid = uuid
			bm.mu.Unlock()
			//bm.logger.Printf("UUID betöltve: %s", uuid)
			return nil
		}
	}

	// Új UUID generálása
	newUUID := generateUUID()
	
	if err := os.WriteFile(uuidFile, []byte(newUUID), 0644); err != nil {
		return fmt.Errorf("UUID mentési hiba: %v", err)
	}
	
	bm.mu.Lock()
	bm.uuid = newUUID
	bm.mu.Unlock()
	
	bm.logger.Printf("Új UUID generálva és mentve: %s", newUUID)
	return nil
}

// UUID generátor - javított
func generateUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback timestamp alapú UUID ha a random nem működik
		now := time.Now().UnixNano()
		return fmt.Sprintf("fallback-%x", now)
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// HTTP kérés küldése - retry mechanizmussal
func (bm *BotMonitor) sendRequest(endpoint string, data BotData) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("JSON marshaling hiba: %v", err)
	}

	url := API_BASE_URL + endpoint
	
	// Retry mechanizmus
	for attempt := 1; attempt <= MAX_RETRIES; attempt++ {
		select {
		case <-bm.ctx.Done():
			return fmt.Errorf("kérés megszakítva")
		default:
		}

		req, err := http.NewRequestWithContext(bm.ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("HTTP request hiba: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+bm.apiSecret)
		req.Header.Set("User-Agent", fmt.Sprintf("YnMBot-Monitor/%s", bm.version))

		resp, err := bm.client.Do(req)
		if err != nil {
			bm.logger.Printf("Kísérlet %d/%d sikertelen (%s): %v", attempt, MAX_RETRIES, endpoint, err)
			if attempt < MAX_RETRIES {
				time.Sleep(RETRY_DELAY * time.Duration(attempt))
				continue
			}
			return fmt.Errorf("HTTP kérés hiba %d kísérlet után: %v", MAX_RETRIES, err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		
		if err != nil {
			bm.logger.Printf("Válasz olvasási hiba (kísérlet %d/%d): %v", attempt, MAX_RETRIES, err)
			if attempt < MAX_RETRIES {
				time.Sleep(RETRY_DELAY * time.Duration(attempt))
				continue
			}
			return fmt.Errorf("válasz olvasási hiba: %v", err)
		}

		if resp.StatusCode >= 400 {
			var apiResp APIResponse
			if json.Unmarshal(body, &apiResp) == nil && apiResp.Error != "" {
				bm.logger.Printf("API hiba (%d) - kísérlet %d/%d: %s", resp.StatusCode, attempt, MAX_RETRIES, apiResp.Error)
			} else {
				bm.logger.Printf("API hiba (%d) - kísérlet %d/%d: %s", resp.StatusCode, attempt, MAX_RETRIES, string(body))
			}
			
			// 4xx hibáknál ne próbáljuk újra (kliens hiba)
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return fmt.Errorf("API kliens hiba (%d): %s", resp.StatusCode, string(body))
			}
			
			// 5xx hibáknál próbáljuk újra
			if attempt < MAX_RETRIES {
				time.Sleep(RETRY_DELAY * time.Duration(attempt))
				continue
			}
			return fmt.Errorf("API szerver hiba (%d): %s", resp.StatusCode, string(body))
		}

		// Sikeres válasz
		var apiResp APIResponse
		if json.Unmarshal(body, &apiResp) == nil {
			//bm.logger.Printf("API válasz (%s): %s", endpoint, apiResp.Message)
		} else {
			//bm.logger.Printf("API válasz (%s): %s", endpoint, string(body))
		}
		return nil
	}
	
	return fmt.Errorf("minden kísérlet sikertelen volt")
}

// Bot regisztrációja (friss config adatokkal)
func (bm *BotMonitor) Register() error {
	bm.mu.RLock()
	uuid := bm.uuid
	bm.mu.RUnlock()
	
	if uuid == "" {
		return fmt.Errorf("UUID nem található")
	}
	
	// Friss config adatok lekérése
	nick, ircServer, runner, country := bm.getCurrentConfigData()
	
	data := BotData{
		BotUUID:   uuid,
		Nick:      nick,
		Uptime:    0,
		Version:   bm.version,
		IRCServer: ircServer,
		Runner:    runner,
		Country:   country,
	}

	//bm.logger.Printf("Regisztráció: %s (%s) - %s", nick, uuid, ircServer)
	return bm.sendRequest("/register.php", data)
}

// Heartbeat küldése (friss config adatokkal)
func (bm *BotMonitor) SendHeartbeat() error {
	bm.mu.RLock()
	uuid := bm.uuid
	startTime := bm.startTime
	bm.mu.RUnlock()
	
	if uuid == "" {
		return fmt.Errorf("UUID nem található")
	}
	
	uptime := int64(time.Since(startTime).Seconds())
	
	// Friss config adatok lekérése minden heartbeat-nél
	nick, ircServer, runner, country := bm.getCurrentConfigData()
	
	data := BotData{
		BotUUID:   uuid,
		Nick:      nick,
		Uptime:    uptime,
		Version:   bm.version,
		IRCServer: ircServer,
		Runner:    runner,
		Country:   country,
	}

	//bm.logger.Printf("Heartbeat: %s - uptime: %ds", nick, uptime)
	return bm.sendRequest("/heartbeat.php", data)
}

// Monitoring indítása háttérben - javított hibakezelés
func (bm *BotMonitor) StartBackground() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				bm.logger.Printf("PÁNIK a monitoring goroutine-ban: %v", r)
			}
		}()
		
		// Kezdeti delay a bot inicializációja miatt
		select {
		case <-bm.ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}

		// Regisztráció retry-val
		for attempt := 1; attempt <= MAX_RETRIES; attempt++ {
			select {
			case <-bm.ctx.Done():
				return
			default:
			}
			
			if err := bm.Register(); err != nil {
				bm.logger.Printf("Regisztráció hiba (kísérlet %d/%d): %v", attempt, MAX_RETRIES, err)
				if attempt < MAX_RETRIES {
					time.Sleep(RETRY_DELAY * time.Duration(attempt))
					continue
				}
				bm.logger.Printf("Regisztráció véglegesen sikertelen")
			} else {
				break
			}
		}

		// Heartbeat timer
		ticker := time.NewTicker(HEARTBEAT_INTERVAL)
		defer ticker.Stop()

		//bm.logger.Printf("Elindítva, heartbeat: %v", HEARTBEAT_INTERVAL)

		// Első heartbeat
		if err := bm.SendHeartbeat(); err != nil {
			bm.logger.Printf("Első heartbeat hiba: %v", err)
		}

		// Periodikus heartbeat-ek
		for {
			select {
			case <-bm.ctx.Done():
				bm.logger.Println("Monitoring leállítás...")
				return
			case <-ticker.C:
				if err := bm.SendHeartbeat(); err != nil {
					bm.logger.Printf("Heartbeat hiba: %v", err)
				}
			}
		}
	}()
}
