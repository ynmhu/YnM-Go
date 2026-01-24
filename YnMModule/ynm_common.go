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

package YnMModule

import (
	"strings"
)
func SimplifyHostmask(fullHostmask string) string {

    parts := strings.Split(fullHostmask, "!")
    if len(parts) < 2 {
        return fullHostmask
    }
    
    userHost := parts[1]
    atPos := strings.Index(userHost, "@")
    if atPos == -1 {
        return fullHostmask
    }
    
    host := userHost[atPos+1:]
    return "*!*@" + host
}

func IsServerHostmask(hostmask string) bool {
    // Ha nincs "!" a hostmask-ban, akkor valószínűleg szerver
    if !strings.Contains(hostmask, "!") {
        return true
    }
    
    serverPatterns := []string{
        "irc.ynm.hu",
        "services.",
        "authserv.",
        "chanserv.",
        "nickserv.", 
        "memoserv.",
        "operserv.",
        "hostserv.",
    }
    
    lowerHostmask := strings.ToLower(hostmask)
    for _, pattern := range serverPatterns {
        if strings.Contains(lowerHostmask, strings.ToLower(pattern)) {
            return true
        }
    }
    return false
}

func ShouldDebugForHostmask(hostmask string) bool {
    return !IsServerHostmask(hostmask)
}

func IsServerMessage(sender string) bool {
    serverHosts := []string{
        "irc.ynm.hu",
        "services.",
        "authserv.",
        "chanserv.",
        "nickserv.",
        "memoserv.",
        "operserv.",
        "hostserv.",
    }

    sender = strings.ToLower(sender)

    for _, server := range serverHosts {
        if strings.Contains(sender, strings.ToLower(server)) {
            return true
        }
    }
    return false
}