// ============================================================================
//  Szerzői jog © 2024 Markus (markus@ynm.hu)
//  https://ynm.hu   – főoldal
//  https://forum.ynm.hu   – hivatalos fórum
//  https://bot.ynm.hu     – bot oldala és dokumentáció
//
//  Minden jog fenntartva. A kód Markus tulajdona, tilos terjeszteni vagy
//  módosítani a szerző írásos engedélye nélkül.
//
//  Ez a fájl a YnM-Go IRC-bot rendszerének része.
// ============================================================================

package irc

import (
	"fmt"
	"time"
)

// IdentifyNickServ elküldi az azonosítási parancsot NickServ-nek.
func (c *Client) IdentifyNickServ() error {
	if !c.config.AutoLogin {
		if c.config.AutoJoinWithoutLogin {
			// Autologin kikapcsolva, de engedett az autojoin, tehát belépünk a csatornákra
			if c.OnLoginSuccess != nil {
				c.OnLoginSuccess()
			}
		}
		return nil
	}

	if c.config.NickservBotnick == "" || c.config.NickservNick == "" || c.config.NickservPass == "" {
		return fmt.Errorf("NickServ adatok hiányoznak a konfigurációból")
	}

	// Várjunk, hogy a kapcsolat stabil legyen
	time.Sleep(5 * time.Second)

	msg := fmt.Sprintf("PRIVMSG %s :IDENTIFY %s %s", c.config.NickservBotnick, c.config.NickservNick, c.config.NickservPass)
	if err := c.SendRaw(msg); err != nil {
		if c.OnLoginFailed != nil {
			c.OnLoginFailed("Nem sikerült azonosítani: " + err.Error())
		}
		return err
	}

	nickChangeCmd := fmt.Sprintf("NICK %s", c.config.NickservNick)
	if err := c.SendRaw(nickChangeCmd); err != nil {
		if c.OnLoginFailed != nil {
			c.OnLoginFailed("Nem sikerült nicket váltani: " + err.Error())
		}
		return err
	}

	// **Itt NE hívd meg OnLoginSuccess-t!**
	// Ezt majd a szerver visszajelzésére várva a readLoop-ban kezeljük.

	return nil
}
