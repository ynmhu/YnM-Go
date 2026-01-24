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
	"strings"
//	"time"
//	"log"
	_ "github.com/mattn/go-sqlite3"
//	"git.ynm.hu/markus/YnM-Go/YnMConfig"
//	"git.ynm.hu/markus/YnM-Go/YnMIrC"
//	"git.ynm.hu/markus/YnM-Go/YnMLang"
//	"git.ynm.hu/markus/YnM-Go/YnMDb"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

func (p *YnmAdminPlugin) HandlePrivateMessage(fullHostmask, message string) {
	nick := strings.Split(fullHostmask, "!")[0]
	
	// ynm parancs kezelése privátban
	if response := p.handleYnmCommand(fullHostmask, message); response != "" {
		// PRIVMSG formátum: PRIVMSG nick :message
		p.Bot.SendRaw(fmt.Sprintf("PRIVMSG %s :%s", nick, response))
	}

}

func (p *YnmAdminPlugin) HandleLoginWithHostmask(hostmask string, args []string, isPrivate bool) {
    originalNick := strings.Split(hostmask, "!")[0]
    
    if !isPrivate {
        p.Bot.SendMessage(originalNick, "A bejelentkezés csak privát üzenetben lehetséges.")
        return
    }

    if len(args) < 2 {
        p.Bot.SendMessage(originalNick, "Használat: login <nick> <jelszo>")
        return
    }

    targetNick := args[0]
    password := args[1]

    // Check if this host already has an active session
    if existingSession, exists := p.GetSessionByHost(hostmask); exists {
        p.Bot.SendMessage(originalNick, fmt.Sprintf("Már be vagy jelentkezve mint: %s", existingSession.LoggedInAs))
        return
    }

    // Verify password for TARGET nick
    valid, err := p.Db.VerifyPassword(targetNick, password)
    if err != nil {
        p.Bot.SendMessage(originalNick, "Hiba a bejelentkezés során.")
        return
    }

    if valid {
        // Create session based on HOSTMASK - JAVÍTOTT
        sessionID, sessionKey := p.CreateSession(hostmask, targetNick)
        _ = sessionID // használatlan változó
        
        p.Bot.SendMessage(originalNick, fmt.Sprintf("✅ Sikeres bejelentkezés %s felhasználóként!", targetNick))
        p.Bot.SendMessage(originalNick, fmt.Sprintf("Session Key: %s (24 óráig érvényes)", sessionKey))
        fmt.Printf("[DEBUG] Host %s logged in as %s with session %s\n", hostmask, targetNick, sessionID)
    } else {
        p.Bot.SendMessage(originalNick, "❌ Hibás jelszó!")
    }
}
func (p *YnmAdminPlugin) HandleLogoutWithHostmask(hostmask string, isPrivate bool) {
    originalNick := strings.Split(hostmask, "!")[0]
    
    // Simplified hostmask-ot használjunk a kereséshez is
    simplifiedHostmask := YnMModule.SimplifyHostmask(hostmask)
    
    // Delete session by simplified hostmask
    if session, exists := p.GetSessionByHost(simplifiedHostmask); exists {
        p.DeleteSessionByHost(simplifiedHostmask)
        p.Bot.SendMessage(originalNick, fmt.Sprintf("✅ Sikeres kijelentkezés (%s)", session.LoggedInAs))
        fmt.Printf("[DEBUG] Session terminated for host %s (was logged in as %s)\n", simplifiedHostmask, session.LoggedInAs)
    } else {
        p.Bot.SendMessage(originalNick, "❌ Nincs aktív session.")
        fmt.Printf("[DEBUG] No session found for: %s (simplified: %s)\n", hostmask, simplifiedHostmask)
    }
}
