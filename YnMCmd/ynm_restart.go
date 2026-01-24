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
package YnMCmd

import (
	"os"
	"strings"
	"time"
	"fmt"
	"os/exec"
	"path/filepath"
	"log"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

type ControlPlugin struct {
	bot         *YnMIrC.Client
	adminPlugin *owner.YnmAdminPlugin
	StopChan    chan struct{}
	ConfigReloadCallback func() error
}

func NewControlPlugin(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin, stopChan chan struct{}) *ControlPlugin {
	return &ControlPlugin{
		bot:         bot,
		adminPlugin: adminPlugin,
		StopChan:    stopChan,
	}
}

// SetConfigReloadCallback beállítja a callback függvényt a config újratöltéshez
func (p *ControlPlugin) SetConfigReloadCallback(callback func() error) {
	p.ConfigReloadCallback = callback
}

func (p *ControlPlugin) HandleMessage(msg YnMIrC.Message) string {
	// MÓDOSÍTÁS: Parancs azonosítása csatornán és privátban különbözően
	var command string
	text := strings.ToLower(strings.TrimSpace(msg.Text))
	
	// Privát üzenet ellenőrzése (ha a csatorna megegyezik a bot nickével)
	isPrivate := strings.EqualFold(msg.Channel, p.bot.GetNick())
	
	if isPrivate {
		// Privátban: ! nélkül is működjenek a parancsok
		command = text
	} else {
		// Csatornában: csak !-el kezdődő parancsok
		if !strings.HasPrefix(text, "!") {
			return ""
		}
		command = strings.TrimPrefix(text, "!")
	}
	
	// Parancs ellenőrzése
	if command != "restart" && command != "die" && command != "reload" && command != "rehash" && command != "reboot" && command != "rebuild" {
		return ""
	}
	
	// Csatorna ellenőrzés - kis/nagybetű érzéketlen
	// Engedélyezett: ConsoleChannel VAGY privát üzenet
	if !strings.EqualFold(msg.Channel, p.adminPlugin.Cfg.ConsoleChannel) && 
	   !isPrivate {
		return ""
	}
	
	// Login Check
	nick, hostmask := YnMModule.GetEffectiveNickAndHost(p.adminPlugin, msg.Sender)
	role := YnMModule.GetUserGlobalRoleWithDB(p.adminPlugin.Db, nick, hostmask)
	if role != "owner" {
		return ""
	}

	// Válasz céljának meghatározása
	responseTarget := msg.Channel
	if isPrivate {
		responseTarget = msg.Sender // Privát üzenetre válaszoljunk a küldőnek
	}

	switch command {
	case "restart", "reboot":
		go func() {
			quitMsg := "♻️ A bot újraindul… https://bot.ynm.hu"
			if p.bot.OnQuit != nil {
				p.bot.OnQuit("YnM-Go", quitMsg)
			} else {
				p.bot.SendRaw("QUIT :" + quitMsg)
			}
			time.Sleep(500 * time.Millisecond)
			close(p.StopChan)
			os.Exit(0)
		}()
		
	case "die":
		go func() {
			quitMsg := "🛑 A bot leáll… https://bot.ynm.hu"
			if p.bot.OnQuit != nil {
				p.bot.OnQuit("YnM-Go", quitMsg)
			} else {
				p.bot.SendRaw("QUIT :" + quitMsg)
			}
			time.Sleep(500 * time.Millisecond)
			close(p.StopChan)
			os.Exit(42)
		}()
		
	case "reload", "rebuild":
		go func() {
			p.bot.SendMessage(responseTarget, "⚙️ Build és újraindítás folyamatban…")
			// Folyamat futó könyvtár automatikusan
			dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
			if err != nil {
				log.Printf("Hiba a working directory lekérésénél: %v", err)
				dir = "." // fallback
			}
			// PATH bővítése tipikus helyekkel
			paths := []string{"/usr/local/go/bin", "/usr/bin", "/bin"}
			os.Setenv("PATH", strings.Join(paths, ":")+":"+os.Getenv("PATH"))
			// Ellenőrizzük, hogy a go elérhető-e
			goBinary, err := exec.LookPath("go")
			if err != nil {
				p.bot.SendMessage(responseTarget, "🔴 'go' parancs nem található a PATH-ban.")
				return
			}
			// Build
			cmd := exec.Command(goBinary, "build")
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				// Soronként küldjük a build hibákat a csatornára
				for _, line := range strings.Split(string(out), "\n") {
					line = strings.TrimSpace(line)
					if line != "" {
						p.bot.SendMessage(responseTarget, "🔴 "+line)
						time.Sleep(200 * time.Millisecond)
					}
				}
				return
			}
			p.bot.SendMessage(responseTarget, "✅ Build kész, bot újraindul… https://bot.ynm.hu")
			quitMsg := "♻️ Bot reload && restart ... https://bot.ynm.hu"
			if p.bot.OnQuit != nil {
				p.bot.OnQuit("YnM-Go", quitMsg)
			} else {
				p.bot.SendRaw("QUIT :" + quitMsg)
			}
			time.Sleep(500 * time.Millisecond)
			close(p.StopChan)
			os.Exit(0)
		}()
		
	case "rehash":
		go func() {
			p.bot.SendMessage(responseTarget, "🔧 Konfiguráció újratöltése folyamatban…")
			
			if p.ConfigReloadCallback == nil {
				p.bot.SendMessage(responseTarget, "🔴 Config reload callback nincs beállítva!")
				return
			}
			
			// Config újratöltése
			if err := p.ConfigReloadCallback(); err != nil {
				p.bot.SendMessage(responseTarget, fmt.Sprintf("✅ Nincs változá: %v", err))
				return
			}
			
			p.bot.SendMessage(responseTarget, "✅ Konfiguráció sikeresen újratöltve!")
		}()
	}
	return ""
}
func (p *ControlPlugin) OnTick() []YnMIrC.Message {
	return nil
}