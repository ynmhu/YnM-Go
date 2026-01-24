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

package main

import (

	"time"
	"log"
	"fmt"

)
// In slave's commands.go, replace handleMasterCommand with this:

func (sb *SlaveBot) handleMasterCommand(msg MasterMessage) {
    log.Printf("[%s] 🔧 Executing master command: %s", sb.config.Name, msg.Action)
    
    switch msg.Action {
    case "cycle":
        sb.executeCycleCommand(msg.Channel)
        
    case "mode":
        // ✅ NEW: Generic mode handler
        // msg.Message contains the mode type: "o", "h", "v"
        modeType := msg.Message
        if modeType == "" {
            log.Printf("[%s] ❌ Mode command missing mode type", sb.config.Name)
            return
        }
        
        log.Printf("[%s] 🔧 Executing MODE %s for %s on %s", 
            sb.config.Name, modeType, msg.User, msg.Channel)
        sb.executeModeCommand(msg.Channel, msg.User, modeType)
        

    default:
        log.Printf("[%s] ❌ Unknown command: %s", sb.config.Name, msg.Action)
    }
}

// ✅ NEW: Generic mode execution
func (sb *SlaveBot) executeModeCommand(channel, user, modeType string) {
    if sb.ircClient == nil {
        log.Printf("[%s] ❌ IRC client not available", sb.config.Name)
        return
    }
    
    // Map mode type to IRC mode character
    modeChar := modeType
    modeName := modeType
    
    switch modeType {
    case "o":
        modeName = "OP"
    case "h":
        modeName = "HALFOP"
    case "v":
        modeName = "VOICE"
    }
    
    log.Printf("[%s] 🔧 Executing %s for %s on %s", sb.config.Name, modeName, user, channel)
    
    // Send MODE command
    modeCmd := fmt.Sprintf("MODE %s +%s %s", channel, modeChar, user)
    sb.ircClient.SendRaw(modeCmd)
    
    log.Printf("[%s] ✅ %s given to %s on %s", sb.config.Name, modeName, user, channel)
}

// Keep the old executeOpCommand for backward compatibility
func (sb *SlaveBot) executeOpCommand(channel, user string) {
    sb.executeModeCommand(channel, user, "o")
}

// Keep the old executeVoiceCommand for backward compatibility  
func (sb *SlaveBot) executeVoiceCommand(channel, user string) {
    sb.executeModeCommand(channel, user, "v")
}

func (sb *SlaveBot) executeCycleCommand(channel string) {
    log.Printf("[%s] 🔄 Executing CYCLE for channel: %s", sb.config.Name, channel)
    
    if sb.ircClient != nil {
        // PART és JOIN
        sb.ircClient.Part(channel, "Cycle")
        time.Sleep(2 * time.Second)
        sb.ircClient.Join(channel)
        
        log.Printf("[%s] ✅ CYCLE completed for %s", sb.config.Name, channel)
    }
}



