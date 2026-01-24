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
package ynm

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    
    "git.ynm.hu/markus/YnM-Go/YnMIrC"
)

type AlexaPlugin struct {
    bot        *YnMIrC.Client
    nodeRedURL string
}

func NewAlexaPlugin(bot *YnMIrC.Client, nodeRedURL string) (*AlexaPlugin, error) {
    if nodeRedURL == "" {
        return nil, fmt.Errorf("nodeRedURL nem lehet üres")
    }
    
    return &AlexaPlugin{
        bot:        bot,
        nodeRedURL: nodeRedURL,
    }, nil
}

func (p *AlexaPlugin) HandleMessage(msg YnMIrC.Message) string {
    lowerText := strings.ToLower(msg.Text)
    
    // 1. !kell parancs → "New movie was requested"
    if strings.HasPrefix(lowerText, "!kell") {
        go p.sendToAlexa("New movie was requested")
        return "" // Nem válaszol IRC-en
    }
    
    // 2. Ha valaki említi "Markus"-t → "Markus, you were highlighted"
    if strings.Contains(lowerText, "markus") {
        go p.sendToAlexa("Markus, you were highlighted")
        return "" // Nem válaszol IRC-en
    }
    
    return ""
}

func (p *AlexaPlugin) sendToAlexa(text string) {
    payload := map[string]string{
        "text": text,
    }
    
    jsonData, _ := json.Marshal(payload)
    
    resp, err := http.Post(
        p.nodeRedURL,
        "application/json",
        bytes.NewBuffer(jsonData),
    )
    
    if err != nil {
        fmt.Printf("[Alexa] HTTP hiba: %v\n", err)
        return
    }
    defer resp.Body.Close()
    
    fmt.Printf("[Alexa] Üzenet elküldve: %s (HTTP %d)\n", text, resp.StatusCode)
}

func (p *AlexaPlugin) OnTick() []YnMIrC.Message {
    return nil
}
