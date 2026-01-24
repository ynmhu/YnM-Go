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
    "encoding/json"
    "net/http"
)

func (p *YnMApiPlugin) handlePlugins(w http.ResponseWriter, r *http.Request) {
    // Mock data - később integrálhatod a valódi plugin manager-rel
    plugins := []map[string]interface{}{
        {"name": "weather", "enabled": true},
        {"name": "chatgpt", "enabled": true},
        {"name": "xp", "enabled": true},
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "plugins": plugins,
    })
}