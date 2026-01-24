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
	"fmt"
)

func (p *YnmAdminPlugin) LogSecurityEvent(username string, hostmask string, action string, details string) {
	db := p.Db
	if db == nil {
		return
	}
	
	// Konzolra is logolhatod (opcionális)
	//fmt.Printf("[BOT-LOG] %s@%s: %s - %s\n", username, hostmask, action, details)
	
	_, err := db.Exec(`
		INSERT INTO bot_logs (username, action, hostmask, details, timestamp)
		VALUES (?, ?, ?, ?, datetime('now'))
	`, username, action, hostmask, details)
	
	// Hiba kezelése (opcionális)
	if err != nil {
		fmt.Printf("[ERROR] Failed to write bot log: %v\n", err)
		
		 if p.Bot != nil && p.Bot.GetConfig().ConsoleChannel != "" {
            errMsg := fmt.Sprintf("🔴 [DB ERROR] Failed to write bot log: %v", err)
             p.Bot.SendMessage(p.Bot.GetConfig().ConsoleChannel, errMsg)
        }
	}
}

// Jogosultság ellenőrzés hibája
//p.LogSecurityEvent(
//    nick,
//    fullHostmask,
//    "PERMISSION_DENIED",
//    fmt.Sprintf("SETPASS command, role: %s, required: vip", userRole),
//)

// Sikeres parancs
//p.LogSecurityEvent(
//    nick, 
//    fullHostmask,
//    "COMMAND_EXECUTED",
//    "SETPASS command successful",
//)