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
package YnMDebug

import "fmt"

var Channel = "#YnM"
var SendRawFunc func(msg string) error

// Belső flag a végtelen ciklus elkerülésére
var internalLogging = false

func Log(a ...any) {
    text := fmt.Sprintln(a...)
    fmt.Print(text) // konzolra mindig
    
    // Ha már belső logging folyik, akkor ne küldjön IRC-re
    if SendRawFunc != nil && !internalLogging {
        _ = SendRawFunc(fmt.Sprintf("PRIVMSG %s :%s", Channel, text))
    }
}

func Logf(format string, a ...any) {
    text := fmt.Sprintf(format, a...)
    fmt.Println(text) // konzolra mindig
    
    // Ha már belső logging folyik, akkor ne küldjön IRC-re
    if SendRawFunc != nil && !internalLogging {
        _ = SendRawFunc(fmt.Sprintf("PRIVMSG %s :%s", Channel, text))
    }
}

func LogInternal(a ...any) {
    internalLogging = true
    defer func() { internalLogging = false }()
    
    text := fmt.Sprintln(a...)
    fmt.Print(text) 
}

func LogfInternal(format string, a ...any) {
    internalLogging = true
    defer func() { internalLogging = false }()
    
    text := fmt.Sprintf(format, a...)
    fmt.Println(text) 
}