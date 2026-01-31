// ==================================================
//  Szerzői jog © 2025 Markus (markus@ynm.hu)
//  https://ynm.hu   – főoldal
//  https://forum.ynm.hu   – hivatalos fórum
//  https://ynm-go.ynm.hu     – bot oldala és dokumentáció
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
    "database/sql"
    "strings"
    "fmt"
    "time"

    _ "github.com/mattn/go-sqlite3"
)

func (p *YnMApiPlugin) handleChannelsMode(w http.ResponseWriter, r *http.Request) {
    db, err := p.getDB()
    if err != nil || db == nil {
        http.Error(w, "Database not available", http.StatusServiceUnavailable)
        return
    }
    
    username := r.Header.Get("X-Username")
    if strings.TrimSpace(username) == "" {
        http.Error(w, "User authentication required", http.StatusUnauthorized)
        return
    }

    globalRole := strings.ToLower(strings.TrimSpace(p.getUserRole(username)))

    // Jogosultság ellenőrzése
    canModifyModes := false
    if globalRole == "owner" || globalRole == "admin" || globalRole == "mod" {
        canModifyModes = true
    }

    switch r.Method {
    case http.MethodGet:
        // Channel modes lista lekérése a channel_modes táblából
        query := `
            SELECT 
                cm.id,
                cm.channel,
                cm.modes,
                cm.mode,
                cm.mode_params,
                cm.enabled,
                cm.set_by,
                cm.set_by_host,
                cm.created_at,
                cm.updated_at,
                cm.active
            FROM channel_modes cm
            WHERE cm.active = 1
        `
        args := []interface{}{}

        // Ha nem admin/owner, csak saját csatornák
        if globalRole != "admin" && globalRole != "owner" {
            query += `
                AND cm.channel IN (
                    SELECT DISTINCT channel
                    FROM channel_users
                    WHERE nick = ? COLLATE NOCASE
                )
            `
            args = append(args, username)
        }

        query += ` ORDER BY cm.channel ASC`

        rows, err := db.Query(query, args...)
        if err != nil {
            http.Error(w, "Database error: " + err.Error(), http.StatusInternalServerError)
            return
        }
        defer rows.Close()

        var modes []map[string]interface{}
        for rows.Next() {
            var id int
            var enabled, active bool
            var channel, modesStr, mode, modeParams, setBy, setByHost, createdAt, updatedAt string
            var modesNull, modeNull, modeParamsNull, setByNull, setByHostNull, createdAtNull, updatedAtNull sql.NullString

            if err := rows.Scan(
                &id,
                &channel,
                &modesNull,
                &modeNull,
                &modeParamsNull,
                &enabled,
                &setByNull,
                &setByHostNull,
                &createdAtNull,
                &updatedAtNull,
                &active,
            ); err != nil {
                fmt.Printf("ERROR scanning row: %v\n", err)
                continue
            }

            // Null értékek kezelése
            if modesNull.Valid {
                modesStr = modesNull.String
            }
            if modeNull.Valid {
                mode = modeNull.String
            }
            if modeParamsNull.Valid {
                modeParams = modeParamsNull.String
            }
            if setByNull.Valid {
                setBy = setByNull.String
            }
            if setByHostNull.Valid {
                setByHost = setByHostNull.String
            }
            if createdAtNull.Valid {
                createdAt = createdAtNull.String
            }
            if updatedAtNull.Valid {
                updatedAt = updatedAtNull.String
            }

            modes = append(modes, map[string]interface{}{
                "id":            id,
                "channel":       channel,
                "modes":         modesStr,
                "mode":          mode,
                "mode_params":   modeParams,
                "enabled":       enabled,
                "set_by":        setBy,
                "set_by_host":   setByHost,
                "created_at":    createdAt,
                "updated_at":    updatedAt,
                "active":        active,
                "can_edit":      canModifyModes,
            })
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": true,
            "modes":   modes,
            "can_edit": canModifyModes,
            "total":   len(modes),
        })

    case http.MethodPut:
        // Channel mode frissítése vagy hozzáadása
        if !canModifyModes {
            http.Error(w, "Only Mod/Admin/Owner can modify channel modes", http.StatusForbidden)
            return
        }

        var req struct {
            ChannelID   int    `json:"id"`
            ChannelName string `json:"channel_name"`
            Mode        string `json:"mode"`
            Param       string `json:"param"`
            Action      string `json:"action"` // "add" vagy "remove"
        }

        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }

        // Channel név meghatározása
        var channelName string
        if req.ChannelName != "" {
            channelName = req.ChannelName
        } else if req.ChannelID > 0 {
            err := db.QueryRow("SELECT name FROM channels WHERE id = ?", req.ChannelID).Scan(&channelName)
            if err != nil {
                http.Error(w, "Channel not found", http.StatusNotFound)
                return
            }
        } else {
            http.Error(w, "Channel ID or name is required", http.StatusBadRequest)
            return
        }

        if req.Mode == "" {
            http.Error(w, "Mode is required", http.StatusBadRequest)
            return
        }

        // Ha nem admin/owner, ellenőrizzük, hogy hozzáfér-e a csatornához
        if globalRole != "admin" && globalRole != "owner" {
            var count int
            err := db.QueryRow(`
                SELECT COUNT(*) 
                FROM channel_users 
                WHERE nick = ? COLLATE NOCASE 
                AND channel = ?
            `, username, channelName).Scan(&count)
            
            if err != nil || count == 0 {
                http.Error(w, "You don't have access to this channel", http.StatusForbidden)
                return
            }
        }

        // Bot MODE parancs küldése
        botModeSuccess := false
        botModeMessage := "IRC bot not connected"
        
        if p.client != nil && p.client.IsConnected() {
            modeCommand := ""
            
            if req.Action == "remove" {
                // Mode eltávolítása
                modeToRemove := strings.Replace(req.Mode, "+", "-", 1)
                modeCommand = fmt.Sprintf("MODE %s %s", channelName, modeToRemove)
            } else {
                // Mode hozzáadása (alapértelmezett)
                modeCommand = fmt.Sprintf("MODE %s %s", channelName, req.Mode)
                if req.Param != "" {
                    modeCommand += " " + req.Param
                }
            }
            
            p.client.SendRaw(modeCommand)
            
            if req.Action == "remove" {
                botModeMessage = fmt.Sprintf("Mode removed in %s: %s", channelName, req.Mode)
            } else {
                botModeMessage = fmt.Sprintf("Mode set in %s: %s", channelName, req.Mode)
                if req.Param != "" {
                    botModeMessage += fmt.Sprintf(" (%s)", req.Param)
                }
            }
            
            botModeSuccess = true
            time.Sleep(300 * time.Millisecond)
        }

        // Adatbázis kezelése - channel_modes tábla
        var existingID int
        var existingModes, existingMode, existingParams string
        
        // Ellenőrizzük, hogy van-e már bejegyzés
        err = db.QueryRow(`
            SELECT id, modes, mode, mode_params 
            FROM channel_modes 
            WHERE channel = ? AND active = 1
        `, channelName).Scan(&existingID, &existingModes, &existingMode, &existingParams)

        if err == sql.ErrNoRows {
            // Új bejegyzés létrehozása
            newModes := parseCombinedModes("", req.Mode, req.Action == "remove")

            _, err = db.Exec(`
                INSERT INTO channel_modes 
                (channel, modes, mode, mode_params, enabled, set_by, set_by_host, created_at, updated_at, active)
                VALUES (?, ?, ?, ?, 1, ?, ?, datetime('now'), datetime('now'), 1)
            `, channelName, newModes, req.Mode, req.Param, username, "WebAPI")

            if err != nil {
                http.Error(w, "Failed to create mode entry: "+err.Error(), http.StatusInternalServerError)
                return
            }

        } else if err != nil {
            http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
            return
        } else {
            // Meglévő bejegyzés frissítése - kombinált mode kezelés
            newModes := parseCombinedModes(existingModes, req.Mode, req.Action == "remove")
            
            // Parse existing params JSON
            paramsMap := parseParamsJSON(existingParams)
            
            // Update params for this specific mode
            modeChar := strings.TrimPrefix(strings.TrimPrefix(req.Mode, "+"), "-")
            if req.Action == "remove" || strings.HasPrefix(req.Mode, "-") {
                delete(paramsMap, modeChar)
            } else if req.Param != "" {
                paramsMap[modeChar] = req.Param
            }
            
            // Convert back to JSON
            newParams := buildParamsJSON(paramsMap)
            
            _, err = db.Exec(`
                UPDATE channel_modes 
                SET modes = ?,
                    mode = ?,
                    mode_params = ?,
                    set_by = ?,
                    updated_at = datetime('now')
                WHERE id = ?
            `, newModes, req.Mode, newParams, username, existingID)

            if err != nil {
                http.Error(w, "Failed to update mode: "+err.Error(), http.StatusInternalServerError)
                return
            }
        }

        p.logAudit(username, "🔧 MODE_UPDATED", r.RemoteAddr,
            fmt.Sprintf("Channel: %s, Mode: %s, Param: %s, Action: %s, IRC: %v", 
                channelName, req.Mode, req.Param, req.Action, botModeSuccess))

        // ✅ Lekérjük az összes módot a frissítés után
        queryModes := `
            SELECT 
                cm.id,
                cm.channel,
                cm.modes,
                cm.mode,
                cm.mode_params,
                cm.enabled,
                cm.set_by,
                cm.set_by_host,
                cm.created_at,
                cm.updated_at,
                cm.active
            FROM channel_modes cm
            WHERE cm.active = 1
        `
        argsModes := []interface{}{}

        if globalRole != "admin" && globalRole != "owner" {
            queryModes += `
                AND cm.channel IN (
                    SELECT DISTINCT channel
                    FROM channel_users
                    WHERE nick = ? COLLATE NOCASE
                )
            `
            argsModes = append(argsModes, username)
        }

        queryModes += ` ORDER BY cm.channel ASC`

        rowsModes, err := db.Query(queryModes, argsModes...)
        if err != nil {
            http.Error(w, "Database error", http.StatusInternalServerError)
            return
        }
        defer rowsModes.Close()

        var modes []map[string]interface{}
        for rowsModes.Next() {
            var id int
            var enabled, active bool
            var channel, modesStr, mode, modeParams, setBy, setByHost, createdAt, updatedAt string
            var modesNull, modeNull, modeParamsNull, setByNull, setByHostNull, createdAtNull, updatedAtNull sql.NullString

            if err := rowsModes.Scan(
                &id,
                &channel,
                &modesNull,
                &modeNull,
                &modeParamsNull,
                &enabled,
                &setByNull,
                &setByHostNull,
                &createdAtNull,
                &updatedAtNull,
                &active,
            ); err != nil {
                fmt.Printf("ERROR scanning row: %v\n", err)
                continue
            }

            // Null értékek kezelése
            if modesNull.Valid {
                modesStr = modesNull.String
            }
            if modeNull.Valid {
                mode = modeNull.String
            }
            if modeParamsNull.Valid {
                modeParams = modeParamsNull.String
            }
            if setByNull.Valid {
                setBy = setByNull.String
            }
            if setByHostNull.Valid {
                setByHost = setByHostNull.String
            }
            if createdAtNull.Valid {
                createdAt = createdAtNull.String
            }
            if updatedAtNull.Valid {
                updatedAt = updatedAtNull.String
            }

            modes = append(modes, map[string]interface{}{
                "id":            id,
                "channel":       channel,
                "modes":         modesStr,
                "mode":          mode,
                "mode_params":   modeParams,
                "enabled":       enabled,
                "set_by":        setBy,
                "set_by_host":   setByHost,
                "created_at":    createdAt,
                "updated_at":    updatedAt,
                "active":        active,
                "can_edit":      canModifyModes,
            })
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": true,
            "message": "Mode updated successfully",
            "modes":   modes,
            "bot_action": map[string]interface{}{
                "updated": botModeSuccess,
                "message": botModeMessage,
                "channel": channelName,
            },
        })

    case http.MethodDelete:
        // Channel mode törlése vagy inaktiválása
        if !canModifyModes {
            http.Error(w, "Only Mod/Admin/Owner can delete channel modes", http.StatusForbidden)
            return
        }

        var req struct {
            ID   int    `json:"id"`
            Mode string `json:"mode"` // Opcionális: csak egy bizonyos mode törlése
        }

        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }

        if req.ID == 0 {
            http.Error(w, "Mode ID is required", http.StatusBadRequest)
            return
        }

        // Lekérjük a channel nevét
        var channelName, currentModes string
        err := db.QueryRow(`
            SELECT channel, modes FROM channel_modes WHERE id = ?
        `, req.ID).Scan(&channelName, &currentModes)

        if err != nil {
            http.Error(w, "Mode entry not found", http.StatusNotFound)
            return
        }

        botModeSuccess := false
        botModeMessage := "IRC bot not connected"

        if req.Mode != "" {
            // Egy adott mode törlése
            if p.client != nil && p.client.IsConnected() {
                modeToRemove := strings.Replace(req.Mode, "+", "-", 1)
                p.client.SendRaw(fmt.Sprintf("MODE %s %s", channelName, modeToRemove))
                botModeMessage = fmt.Sprintf("Mode %s removed from %s", req.Mode, channelName)
                botModeSuccess = true
                time.Sleep(300 * time.Millisecond)
            }

            // Mode eltávolítása a stringből
            newModes := removeModeFromString(currentModes, req.Mode)
            
            if newModes == "" {
                // Ha üres lett, inaktiváljuk a bejegyzést
                _, err = db.Exec(`UPDATE channel_modes SET active = 0, updated_at = datetime('now') WHERE id = ?`, req.ID)
            } else {
                // Csak a modes frissítése
                _, err = db.Exec(`UPDATE channel_modes SET modes = ?, updated_at = datetime('now') WHERE id = ?`, newModes, req.ID)
            }

        } else {
            // Teljes bejegyzés törlése (inaktiválás)
            if p.client != nil && p.client.IsConnected() {
                // Minden mode törlése a listából
                modes := strings.Fields(currentModes)
                for _, mode := range modes {
                    if strings.HasPrefix(mode, "+") {
                        modeToRemove := strings.Replace(mode, "+", "-", 1)
                        p.client.SendRaw(fmt.Sprintf("MODE %s %s", channelName, modeToRemove))
                        time.Sleep(200 * time.Millisecond)
                    }
                }
                botModeMessage = fmt.Sprintf("All modes removed from %s", channelName)
                botModeSuccess = true
                time.Sleep(500 * time.Millisecond)
            }

            _, err = db.Exec(`UPDATE channel_modes SET active = 0, updated_at = datetime('now') WHERE id = ?`, req.ID)
        }

        if err != nil {
            http.Error(w, "Failed to delete mode: "+err.Error(), http.StatusInternalServerError)
            return
        }

        p.logAudit(username, "🗑️ MODE_DELETED", r.RemoteAddr,
            fmt.Sprintf("Channel: %s, Mode ID: %d, Mode: %s, IRC: %v", 
                channelName, req.ID, req.Mode, botModeSuccess))

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": true,
            "message": "Mode deleted successfully",
            "bot_action": map[string]interface{}{
                "updated": botModeSuccess,
                "message": botModeMessage,
                "channel": channelName,
            },
        })

    case http.MethodPost:
        // ✅ Bot OP esetén mode beállítása adatbázisból
        var req struct {
            Channel string `json:"channel"`
            Action  string `json:"action"`
        }

        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }

        if req.Channel == "" {
            http.Error(w, "Channel is required", http.StatusBadRequest)
            return
        }

        // Csak bot vagy admin hívhatja
        if !strings.HasPrefix(username, "Bot_") && globalRole != "admin" && globalRole != "owner" {
            http.Error(w, "Only bot or admin can use this endpoint", http.StatusForbidden)
            return
        }

        // Lekérjük a mentett mode-okat az adatbázisból
        rows, err := db.Query(`
            SELECT modes, mode_params 
            FROM channel_modes 
            WHERE channel = ? AND active = 1 AND enabled = 1
        `, req.Channel)

        if err != nil {
            http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
            return
        }
        defer rows.Close()

        botModeSuccess := false
        botModeMessage := "IRC bot not connected"
        appliedModes := []string{}

        if p.client != nil && p.client.IsConnected() {
            for rows.Next() {
                var modesStr, modeParams sql.NullString
                if err := rows.Scan(&modesStr, &modeParams); err != nil {
                    continue
                }

                if modesStr.Valid && modesStr.String != "" {
                    // Minden mode alkalmazása
                    modes := strings.Fields(modesStr.String)
                    for _, mode := range modes {
                        modeCommand := fmt.Sprintf("MODE %s %s", req.Channel, mode)
                        
                        // Ha van paraméter, és a mode +k vagy +l
                        if modeParams.Valid && modeParams.String != "" && 
                           (mode == "+k" || mode == "+l") {
                            modeCommand += " " + modeParams.String
                        }
                        
                        p.client.SendRaw(modeCommand)
                        appliedModes = append(appliedModes, mode)
                        time.Sleep(200 * time.Millisecond)
                    }
                    
                    botModeSuccess = true
                }
            }
            
            if len(appliedModes) > 0 {
                botModeMessage = fmt.Sprintf("Modes loaded to %s: %s", req.Channel, strings.Join(appliedModes, ", "))
            } else {
                botModeMessage = fmt.Sprintf("No modes to load for %s", req.Channel)
            }
        }

        rows.Close() // Bezárjuk a cursor-t

        p.logAudit(username, "🔄 MODE_LOADED", r.RemoteAddr,
            fmt.Sprintf("Channel: %s, Modes: %s", req.Channel, strings.Join(appliedModes, ", ")))

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": true,
            "message": "Modes loaded from database",
            "channel": req.Channel,
            "modes":   appliedModes,
            "bot_action": map[string]interface{}{
                "updated": botModeSuccess,
                "message": botModeMessage,
                "channel": req.Channel,
            },
        })

    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

// parseParamsJSON - Parse mode_params JSON to map
func parseParamsJSON(paramsStr string) map[string]string {
    paramsMap := make(map[string]string)
    
    if paramsStr == "" {
        return paramsMap
    }
    
    // Try to parse as JSON
    if err := json.Unmarshal([]byte(paramsStr), &paramsMap); err == nil {
        return paramsMap
    }
    
    // Fallback: treat as single value (backward compatibility)
    // If it's just a plain string, assume it's for +k or +l
    paramsMap["legacy"] = paramsStr
    return paramsMap
}

// buildParamsJSON - Convert params map to JSON
func buildParamsJSON(paramsMap map[string]string) string {
    if len(paramsMap) == 0 {
        return ""
    }
    
    jsonBytes, err := json.Marshal(paramsMap)
    if err != nil {
        return ""
    }
    
    return string(jsonBytes)
}

// parseCombinedModes - IRC mode kombinálás helyes kezelése
// Példa: "+nt" + "+s" = "+nts", "+nts" - "s" = "+nt"
func parseCombinedModes(currentModes, newMode string, remove bool) string {
    // Parse current modes into a map
    modeMap := make(map[rune]bool)
    
    // Feldolgozzuk a jelenlegi mode-okat
    currentModes = strings.ReplaceAll(currentModes, " ", "")
    var currentSign rune = '+'
    for _, char := range currentModes {
        if char == '+' || char == '-' {
            currentSign = char
        } else {
            // Csak a + mode-okat tároljuk (aktív mode-ok)
            if currentSign == '+' {
                modeMap[char] = true
            }
        }
    }
    
    // Feldolgozzuk az új mode-ot
    newMode = strings.TrimSpace(newMode)
    var newSign rune = '+'
    for _, char := range newMode {
        if char == '+' || char == '-' {
            newSign = char
        } else {
            if remove || newSign == '-' {
                // Mode eltávolítása
                delete(modeMap, char)
            } else {
                // Mode hozzáadása
                modeMap[char] = true
            }
        }
    }
    
    // Visszaalakítjuk string-é
    if len(modeMap) == 0 {
        return ""
    }
    
    // Sorba rendezzük a mode-okat
    modes := make([]rune, 0, len(modeMap))
    for mode := range modeMap {
        modes = append(modes, mode)
    }
    
    // Egyszerű rendezés
    for i := 0; i < len(modes); i++ {
        for j := i + 1; j < len(modes); j++ {
            if modes[i] > modes[j] {
                modes[i], modes[j] = modes[j], modes[i]
            }
        }
    }
    
    return "+" + string(modes)
}

// removeModeFromString - Eltávolít egy mode-ot a modes stringből (kompatibilitás)
func removeModeFromString(modesStr, modeToRemove string) string {
    return parseCombinedModes(modesStr, modeToRemove, true)
}