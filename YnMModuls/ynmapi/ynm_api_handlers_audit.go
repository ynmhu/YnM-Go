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
    "strconv"
    "log"
	"strings"

)

func (p *YnMApiPlugin) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
    username := r.Header.Get("X-Username")
    if username == "" {
        username = "YnM-Go"
    }

	db, err := p.getDB()
	if err != nil || db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

    switch r.Method {
    case http.MethodGet:
        // ===== EGYESÍTETT AUDIT LOGOK LEKÉRDEZÉSE =====
        log.Printf("🔵 GET /audit-logs request from: %s", username)
        
        // Query paraméterek
        limit := 100
        if l := r.URL.Query().Get("limit"); l != "" {
            if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
                limit = parsed
            }
        }
        
        actionFilter := r.URL.Query().Get("action")
        usernameFilter := r.URL.Query().Get("username")
        
        log.Printf("📊 Filters - Limit: %d, Action: %s, Username: %s", limit, actionFilter, usernameFilter)
        
        // ===== EGYESÍTETT LEKÉRDEZÉS UNION-nal =====
        // Mindkét táblából lekérjük az adatokat és egyesítjük őket
        
        query := `
        SELECT id, username, action, hostmask as ip_address, details, timestamp, 'bot' as source
        FROM bot_logs
        WHERE 1=1
        `
        
        args := []interface{}{}
        
        if actionFilter != "" {
            query += ` AND action = ?`
            args = append(args, actionFilter)
        }
        
        if usernameFilter != "" {
            query += ` AND username LIKE ?`
            args = append(args, "%"+usernameFilter+"%")
        }
        
        query += `
        UNION ALL
        SELECT id, username, action, ip_address, details, timestamp, 'web' as source
        FROM web_logs
        WHERE 1=1
        `

        if actionFilter != "" {
            query += ` AND action = ?`
            args = append(args, actionFilter)
        }
        
        if usernameFilter != "" {
            query += ` AND username LIKE ?`
            args = append(args, "%"+usernameFilter+"%")
        }
        
        // Időrend szerinti rendezés és limit
        query += ` ORDER BY timestamp DESC LIMIT ?`
        args = append(args, limit)
        
        log.Printf("🔍 SQL Query: %s", query)
        log.Printf("🔍 SQL Args: %v", args)
        
        // Adatbázis lekérdezés
        rows, err := db.Query(query, args...)
        if err != nil {
            log.Printf("❌ Bot logs query error: %v", err)
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]interface{}{
                "success": false,
                "error":   "Database query failed: " + err.Error(),
            })
            return
        }
        defer rows.Close()
        
        // Logok gyűjtése
        var logs []map[string]interface{}
        for rows.Next() {
            var id int
            var uname, action, ipAddress, details, timestamp, source string
            
            err := rows.Scan(&id, &uname, &action, &ipAddress, &details, &timestamp, &source)
            if err != nil {
                log.Printf("❌ Row scan error: %v", err)
                continue
            }
            
            logs = append(logs, map[string]interface{}{
                "id":         id,
                "username":   uname,
                "action":     action,
                "ip_address": ipAddress,
                "details":    details,
                "timestamp":  timestamp,
                "source":     source, // 'bot' vagy 'web'
            })
        }
        
        if logs == nil {
            logs = []map[string]interface{}{}
        }
        
        log.Printf("✅ Returning %d unified bot logs", len(logs))
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": true,
            "logs":    logs,
            "total":   len(logs),
        })

	case http.MethodPost:
        // ===== AUDIT LOG ÍRÁSA =====
        log.Printf("🔵 POST /audit-logs request from: %s", username)
        
        var req struct {
            Username  string `json:"username"`
            Action    string `json:"action"`
            IPAddress string `json:"ip_address"`
            Details   string `json:"details"`
            Source    string `json:"source"` // 'bot' vagy 'web'
        }
        
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            log.Printf("❌ Invalid JSON: %v", err)
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }
        
        // Alapértelmezett értékek
        if req.Username == "" {
            req.Username = username
        }
        if req.IPAddress == "" {
            req.IPAddress = r.RemoteAddr
        }
        if req.Source == "" {
            req.Source = "web" // Alapértelmezetten web source
        }
        
        // Válasszuk ki a megfelelő táblát
		if req.Source == "web" {
			_, err = db.Exec(`
				INSERT INTO web_logs (username, action, ip_address, details, timestamp)
				VALUES (?, ?, ?, ?, datetime('now'))
			`, req.Username, req.Action, req.IPAddress, req.Details)
		} else {
			// Bot lognál a hostmask-ot kell használni!
			hostmask := req.IPAddress // vagy külön hostmask paraméter
			_, err = db.Exec(`
				INSERT INTO bot_logs (username, action, hostmask, details, timestamp)
				VALUES (?, ?, ?, ?, datetime('now'))
			`, req.Username, req.Action, hostmask, req.Details)
		}
        
        if err != nil {
            log.Printf("❌ Failed to write bot log: %v", err)
            http.Error(w, "Failed to write bot log", http.StatusInternalServerError)
            return
        }
        
        log.Printf("✅ Bot log written to %s: %s - %s", req.Source, req.Username, req.Action)
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": true,
            "message": "Bot log written",
        })

case http.MethodDelete:
    // ===== Bot LOG(OK) TÖRLÉSE =====
    log.Printf("🔵 DELETE /audit-logs request from: %s", username)
    
    // Kétféleképpen kaphatjuk az ID-ket:
    // 1. Query paraméterként: ?ids=1,2,3
    // 2. JSON body-ban: {"ids": [1,2,3]}
    
    var ids []int
    
    // 1. Próbáljuk query paraméterként
    if queryIDs := r.URL.Query().Get("ids"); queryIDs != "" {
        // "1,2,3" formátum feldolgozása
        idStrs := strings.Split(queryIDs, ",")
        for _, idStr := range idStrs {
            if id, err := strconv.Atoi(strings.TrimSpace(idStr)); err == nil && id > 0 {
                ids = append(ids, id)
            }
        }
    }
    
    // 2. Ha query paraméterben nem volt, próbáljuk JSON body-t
    if len(ids) == 0 && r.ContentLength > 0 {
        var req struct {
            IDs []int `json:"ids"`
        }
        
        if err := json.NewDecoder(r.Body).Decode(&req); err == nil && len(req.IDs) > 0 {
            ids = req.IDs
            log.Printf("📦 Got IDs from JSON body: %v", ids)
        }
    }
    
    if len(ids) == 0 {
        log.Printf("❌ No valid IDs provided")
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": false,
            "error":   "No valid IDs provided. Use ?ids=1,2,3 or JSON body",
        })
        return
    }
    
    log.Printf("🗑️ Deleting %d log(s): %v", len(ids), ids)
    
    var deletedCount int64 = 0
    
    // Töröljük egyesével mindkét táblából
    for _, id := range ids {
        // audit_logs táblából
        result1, err1 := db.Exec("DELETE FROM bot_logs WHERE id = ?", id)
        if err1 == nil {
            if rows, _ := result1.RowsAffected(); rows > 0 {
                deletedCount += rows
                log.Printf("✅ Deleted from bot_logs: ID %d", id)
            }
        } else {
            log.Printf("❌ Error deleting from bot_logs ID %d: %v", id, err1)
        }
        
        // web_logs táblából
        result2, err2 := db.Exec("DELETE FROM web_logs WHERE id = ?", id)
        if err2 == nil {
            if rows, _ := result2.RowsAffected(); rows > 0 {
                deletedCount += rows
                log.Printf("✅ Deleted from web_logs: ID %d", id)
            }
        } else {
            log.Printf("❌ Error deleting from web_logs ID %d: %v", id, err2)
        }
    }
    
    log.Printf("✅ Total deleted: %d log(s)", deletedCount)
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success":       true,
        "deleted_count": deletedCount,
        "requested_ids": len(ids),
    })


    default:
        log.Printf("❌ Method not allowed: %s", r.Method)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusMethodNotAllowed)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": false,
            "error":   "Method not allowed",
        })
    }
}

