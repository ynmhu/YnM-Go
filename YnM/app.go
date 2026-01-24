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
// ==================================================package YnM
package YnM

import (

	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	 "strings"
	 "fmt"

	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMDb"

)

type App struct {
	config        *YnMConfig.Config
	bot           *YnMIrC.Client
	pluginManager *PluginManager
	eventHandler  *EventHandler
	logger        *Logger
	adminDB       *YnMDb.AdminDB  
	adminPlugin *owner.YnmAdminPlugin
}


func New(cfg *YnMConfig.Config) *App {
	return &App{
		config: cfg,
	}
}

func (a *App) GetBot() *YnMIrC.Client {
    return a.bot
}

func (a *App) initialize() error {
	if err := a.validateConfig(); err != nil {
		return err
	}
	if err := os.MkdirAll(a.config.LogDir, 0o755); err != nil {
		return err
	}
	a.bot = YnMIrC.NewClient(a.config)
	
	adminDB, err := YnMDb.NewAdminDB()
	if err != nil {
		return err
	}
	a.adminDB = adminDB
	a.adminPlugin = &owner.YnmAdminPlugin{
		Db:  a.adminDB,
		Bot: a.bot, 
	}
	
	a.bot.OnJoin = func(channel, nick, hostmask string) {
    fmt.Printf("*** JOIN ESEMÉNY: nick=%s, channel=%s, hostmask=%s ***\n", nick, channel, hostmask)

    botNick := a.bot.GetNick()

    if nick == botNick {

        return
    }  
    fmt.Printf("*** ADMINPLUGIN CHECK: %v ***\n", a.adminPlugin != nil)
    
    if a.adminPlugin != nil {
        fmt.Printf("*** AUTOAPPLY INDÍTÁSA... ***\n")
        log.Printf("*** AUTOAPPLY INDÍTÁSA... ***")
        
        // Közvetlenül hívjuk meg, goroutine nélkül a teszteléshez
        a.adminPlugin.AutoApplyUserModes(nick, hostmask, channel)
        
        fmt.Printf("*** AUTOAPPLY BEFEJEZVE ***\n")
    } else {
        fmt.Printf("*** ADMINPLUGIN NIL! ***\n")
        log.Printf("*** ADMINPLUGIN NIL! ***")
    }
}
	
	a.logger = NewLogger(a.config.LogDir)
	a.pluginManager = NewPluginManager(a.adminDB)
	a.eventHandler = NewEventHandler(a.bot, a.config, a.pluginManager, a.logger)
	
	// app.go initialize() függvényben - keressed meg ezt a részt és cseréld le:

	a.bot.OnChannelJoined = func(channel string) {
   // fmt.Printf("*** OnChannelJoined CALLBACK: channel=%s ***\n", channel)
    //log.Printf("*** OnChannelJoined CALLBACK: channel=%s ***", channel)
    
    go func() {
       // fmt.Printf("*** OnChannelJoined GOROUTINE START: channel=%s ***\n", channel)
        
        time.Sleep(3 * time.Second)
        
        if strings.ToLower(channel) == strings.ToLower(a.config.ConsoleChannel) {
            hasowner := a.adminDB.HasAnyowner()
            if !hasowner {
                key := a.config.ConsoleKey
                a.bot.SendRaw(fmt.Sprintf("MODE %s +k %s", channel, key))
                a.bot.SendRaw(fmt.Sprintf("MODE %s +nts", channel))
                a.bot.SendRaw(fmt.Sprintf("TOPIC %s :A csatorna zárva, amíg nincs regisztrált owner.", channel))
                a.bot.SendMessage(channel, "A csatorna zárva, amíg nincs regisztrált owner. A belépéshez szükséges kulcs: "+key+". Regisztrálj ownerként !ynm paranccsal.")
            }
        }
        
        //fmt.Printf("*** OnChannelJoined: Várakozás 5 másodperc... ***\n")
        time.Sleep(5 * time.Second)
        
        //fmt.Printf("*** OnChannelJoined: ApplySavedChannelModes hívása... ***\n")
        //log.Printf("*** OnChannelJoined: ApplySavedChannelModes hívása channel=%s ***", channel)
        
        a.adminDB.ApplySavedChannelModes(a.bot, channel)
        
        //fmt.Printf("*** OnChannelJoined: ApplySavedChannelModes befejezve ***\n")
        
        time.Sleep(2 * time.Second)
        //fmt.Printf("*** OnChannelJoined GOROUTINE END ***\n")
    }()
}
	
	a.eventHandler.Setup()

	// ⚠️ JAVÍTÁS: Ne írjuk felül, hanem BŐVÍTSÜK a callbacket
	a.bot.OnLoginSuccess = func() {
		// Először hívjuk meg a handler-t (loginSuccessHandled flag beállítása)
		a.eventHandler.handleLoginSuccess()
		
		// Aztán a csatornákhoz csatlakozás
		if err := a.joinSavedChannels(); err != nil {
			// log.Printf("Hiba a csatornákhoz csatlakozás közben: %v", err)
		}
	}
	
	
	a.bot.SetChannelModeHandler(&YnMIrC.EmptyChannelModeHandler{})
	originalOnMessage := a.bot.OnMessage
	a.bot.OnMessage = func(message YnMIrC.Message) {
		if originalOnMessage != nil {
			originalOnMessage(message)
		}
		if len(message.Command) == 3 && message.Command >= "300" && message.Command <= "399" {
			// log.Printf("[DEBUG] IRC numerikus üzenet - Command: '%s', Params: %v", message.Command, message.Params)
		}
		if message.Command == "311" && len(message.Params) >= 4 {
			targetNick := message.Params[1]
			channels, err := a.adminDB.GetAllChannels()
			if err != nil {
				channels = []string{"#YnM", "#ynm"}
			}
			allChannels := make([]string, 0, len(channels)*2)
			for _, ch := range channels {
				allChannels = append(allChannels, ch)
				if strings.ToLower(ch) != ch {
					allChannels = append(allChannels, strings.ToLower(ch))
				}
				if strings.ToUpper(ch) != ch {
					allChannels = append(allChannels, strings.ToUpper(ch))
				}
			}
			found := false
			for _, channel := range allChannels {
				users := a.bot.GetChannelUsers(channel)
				for _, user := range users {
					if strings.EqualFold(user, targetNick) {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				allBotChannels := a.bot.GetChannels()
				for _, ch := range allBotChannels {
					users := a.bot.GetChannelUsers(ch)
					for _, user := range users {
						if strings.EqualFold(user, targetNick) {
							found = true
							break
						}
					}
					if found {
						break
					}
				}
			}
		}
	}
	
	if err := a.pluginManager.RegisterAll(a.bot, a.config); err != nil {
		return err
	}
	a.startScheduledTasks()
	return nil
}


func (a *App) RegisterPlugins() error {
	return a.pluginManager.RegisterAll(a.bot, a.config)
}

func (a *App) validateConfig() error {
	if a.config.LogDir == "" {
		// log.Fatal("Log könyvtár nincs megadva a configban!")
	}
	if a.config.ConsoleChannel == "" {
		// log.Fatal("A 'console_channel' nincs megadva a config.yaml‑ben!")
	}
	return nil
}

func (a *App) startScheduledTasks() {
	// Időzített plugin ticker
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			a.pluginManager.HandleTick(a.bot)
		}
	}()
}

func (a *App) setupGracefulShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		// log.Println("🛑 Leállítási jel érkezett...")
		
		// Pluginok leállítása
		a.pluginManager.Shutdown()
		
		// Bot leállítása
		a.bot.Disconnect()
		os.Exit(0)
	}()
}

func (a *App) Run() error {
    // Előkészítés
    if err := a.initialize(); err != nil {
        return err
    }

    // Bot indítása
    if err := a.bot.Connect(); err != nil {
        return err
    }
    defer a.bot.Disconnect()

    // ⚠️ ELTÁVOLÍTVA: Ne hívjuk meg itt a joinSavedChannels()-t
    // Az OnLoginSuccess callback fogja meghívni, amikor az azonosítás sikeres
    
    // Graceful shutdown beállítása
    a.setupGracefulShutdown()

    // Várakozás a leállítási jelre
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    return nil
}

func (a *App) joinSavedChannels() error {
    // ⚠️ VÉDELEM: Ellenőrizzük az azonosítást
    if a.config.Undernet.Enabled && !a.bot.IsUndernetAuthenticated() {
        // log.Println("🔒 joinSavedChannels: Undernet még nincs azonosítva, kihagyom")
        return nil
    }

    // log.Println("📋 Adatbázisból betöltött csatornákba lépés...")
    channels, err := a.adminDB.GetAllChannels()
    if err != nil {
        return err
    }

    for _, channel := range channels {
        // log.Printf("📍 DB JOIN: %s\n", channel)
        a.bot.Join(channel)
        time.Sleep(500 * time.Millisecond)
    }

    return nil
}

