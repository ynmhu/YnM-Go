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
package ynmapi

import (
	"log"
	"fmt"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
)
func (p *YnMApiPlugin) GetConfig() *YnMConfig.YnMApiConfig {
    p.configMutex.RLock()
    defer p.configMutex.RUnlock()
    return p.config
}
func (p *YnMApiPlugin) SetConfigReloadCallback(callback func() error) {
    // Ezt nem használjuk közvetlenül, de a control plugin hívhatja
    // a ReloadConfig metódust
}
func (p *YnMApiPlugin) ReloadConfig() error {
    YnMApiCfg, err := YnMConfig.LoadYnMApiConfig("YnMConfig/ynm-api.yaml")
    if err != nil {
        return fmt.Errorf("failed to load ynm-api.yaml: %v", err)
    }
    
    p.configMutex.Lock()
    oldPort := p.config.YnM.Port
    p.config = YnMApiCfg
    p.configMutex.Unlock()
    
    log.Printf("[YnMApI] ✅ Configuration reloaded from ynm-api.yaml")
    log.Printf("[YnMApI]    Port: %d", YnMApiCfg.YnM.Port)
    log.Printf("[YnMApI]    Session Lifetime: %d seconds", YnMApiCfg.YnM.Session.Lifetime)
    log.Printf("[YnMApI]    Password Expiry: %d minutes", YnMApiCfg.YnM.Password.ExpiryMinutes)
    
    // Ha a port változott, figyelmeztetés
    if oldPort != YnMApiCfg.YnM.Port {
        log.Printf("[YnMApI] ⚠️ Port changed from %d to %d - requires restart!", oldPort, YnMApiCfg.YnM.Port)
    }
    
    return nil
}