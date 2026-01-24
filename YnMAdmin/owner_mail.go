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


//	"golang.org/x/crypto/bcrypt"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
//	"git.ynm.hu/markus/YnM-Go/YnMConfig"
)

func (p *YnmAdminPlugin) HandleSetMail(nick string, hostmask string, args []string, isPrivate bool) {
	fmt.Printf("[DEBUG HandleSetMail] nick=%s, args=%v, isPrivate=%v\n", nick, args, isPrivate)

	if !isPrivate {
		p.Bot.SendMessage(nick, "Az e-mail beállítása csak privát üzenetben lehetséges. Írj nekem PRIVÁT üzenetben: setmail <uj_email>")
		fmt.Printf("[DEBUG] Parancs elutasítva: nem privát üzenet\n")
		return
	}

	if len(args) == 0 {
		usage := "setmail <uj_email>"
		p.Bot.SendMessage(nick, fmt.Sprintf("Használat: %s", usage))
		return
	}

	newMail := args[0]

	// Egyszerű validáció (ellenőrizzük, hogy van benne @ jel)
	if !strings.Contains(newMail, "@") {
		p.Bot.SendMessage(nick, "Hibás e-mail cím formátum.")
		return
	}

	// Ellenőrizzük, hogy a felhasználó létezik-e
	exists, err := p.Db.UserExists(nick)
	if err != nil {
		fmt.Printf("[DEBUG] Error checking user existence: %v\n", err)
		p.Bot.SendMessage(nick, "Hiba a felhasználó ellenőrzése közben.")
		return
	}

	if !exists {
		// A felhasználó nem található az adatbázisban
		return
	}

	// ✅ JAVÍTVA: Ellenőrizzük, hogy van-e már e-mail cím
	hasMail, err := p.Db.UserHasMail(nick)
	
	// 🔍 DEBUG: Nézzük meg mit ad vissza
	fmt.Printf("[DEBUG] UserHasMail('%s') returned: hasMail=%v, err=%v\n", nick, hasMail, err)
	
	if err != nil {
		fmt.Printf("[ERROR] Failed to check email for %s: %v\n", nick, err)
		p.Bot.SendMessage(nick, "Hiba az e-mail ellenőrzése közben.")
		return
	}
	
	// ✅ Ha már van e-mail, NE engedjük beállítani újra!
	if hasMail {
		p.Bot.SendMessage(nick, "❌ E-mail cím már létezik!")
		p.Bot.SendMessage(nick, "Használd a 'chgmail' parancsot a módosításhoz.")
		fmt.Printf("[INFO] %s tried to set email but already has one\n", nick)
		return
	}

	fmt.Printf("[DEBUG] User exists and has no email yet: %s\n", nick)

	// Mentés az adatbázisba
	err = p.Db.SetUserMail(nick, newMail)
	if err != nil {
		fmt.Printf("[DEBUG] Error saving email: %v\n", err)
		p.Bot.SendMessage(nick, "Hiba az e-mail mentése közben az adatbázisba.")
		return
	}

	fmt.Printf("[DEBUG] Email saved successfully for user: %s\n", nick)
	p.Bot.SendMessage(nick, "✅ E-mail cím sikeresen beállítva.")
}
// ==================================================
func (p *YnmAdminPlugin) HandleChgPass(nick string, hostmask string, args []string, isPrivate bool) {
	fmt.Printf("[DEBUG HandleChgPass] nick=%s, args=%v, isPrivate=%v\n", nick, args, isPrivate)

	if !isPrivate {
		p.Bot.SendMessage(nick, "A jelszó módosítása csak privát üzenetben lehetséges. Írj PRIVÁT üzenetben: chgpass <regi_jelszo> <uj_jelszo>")
		return
	}

	if len(args) < 2 {
		p.Bot.SendMessage(nick, "Használat: chgpass <regi_jelszo> <uj_jelszo>")
		return
	}

	oldPass := args[0]
	newPass := args[1]

	valid, err := p.Db.VerifyPassword(nick, oldPass)
	if err != nil {
		p.Bot.SendMessage(nick, "Hiba a régi jelszó ellenőrzése közben.")
		return
	}
	if !valid {
		p.Bot.SendMessage(nick, "Hibás a régi jelszó!")
		return
	}

	hash, err := HashPassword(newPass)
	if err != nil {
		p.Bot.SendMessage(nick, "Hiba az új jelszó hash-elése közben.")
		return
	}

	err = p.Db.SetUserPassword(nick, hash)
	if err != nil {
		p.Bot.SendMessage(nick, "Hiba az új jelszó mentése közben.")
		return
	}

	// E-mail kiküldése az új jelszóval
	email, err := p.Db.GetUserMail(nick)
	if err == nil && email != "" {
		err = YnMModule.SendEmail(p.Cfg, email, "Jelszó módosítás", fmt.Sprintf("Az új jelszavad: %s", newPass))
		if err != nil {
			p.Bot.SendMessage(nick, "⚠️ A jelszó sikeresen módosítva, de az e-mail küldése sikertelen volt.")
			return
		}
	}

	p.Bot.SendMessage(nick, "✅ Jelszó sikeresen módosítva és elküldve e-mailben.")
}



// ==================================================
func (p *YnmAdminPlugin) HandleForgetPass(nick string, hostmask string, isPrivate bool) {
    fmt.Printf("[DEBUG HandleForgetPass] nick=%s, isPrivate=%v\n", nick, isPrivate)

    // Napi limit ellenőrzése
    count, err := p.Db.CountForgetPassInLast24Hours(nick)
    if err != nil {
        fmt.Printf("[DEBUG] Hiba a kísérletek számlálása közben: %v\n", err)
        return
    }
    if count >= 2 {
        // Már túl sok próbálkozás, nem reagálunk
        fmt.Printf("[DEBUG] Napi limit elérve: %s\n", nick)
        return
    }

    email, err := p.Db.GetUserMail(nick)
    if err != nil || email == "" {
        p.Bot.SendMessage(nick, "Nincs e-mail cím beállítva, nem lehet visszaállítani a jelszót.")
        return
    }

    newPass := YnMModule.GenerateRandomPassword(12)
    hash, err := HashPassword(newPass)
    if err != nil {
        p.Bot.SendMessage(nick, "Hiba az új jelszó generálása közben.")
        return
    }

    err = p.Db.SetUserPassword(nick, hash)
    if err != nil {
        p.Bot.SendMessage(nick, "Hiba az új jelszó mentése közben.")
        return
    }

    err = YnMModule.SendEmail(p.Cfg, email, "Jelszó visszaállítás", fmt.Sprintf("Az új jelszavad: %s", newPass))
    if err != nil {
        p.Bot.SendMessage(nick, "Hiba történt az e-mail küldésekor.")
        return
    }

    // Naplózzuk a sikeres kérést
    _ = p.Db.LogForgetPassAttempt(nick)

    p.Bot.SendMessage(nick, "✅ Új jelszó elküldve e-mailben.")
}
