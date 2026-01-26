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
    "database/sql"
    "strings"
    "fmt"
    "time"

    _ "github.com/mattn/go-sqlite3"
)

func (p *YnMApiPlugin) handleChannelsTopic(w http.ResponseWriter, r *http.Request) {
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

    switch r.Method {
    case http.MethodGet:
        // Topic lista lekérése
        query := `
            SELECT id, name, current_topic, topic_set_by, topic_set_at
            FROM channels
        `
        args := []interface{}{}

        // Ha nem admin/owner, csak saját csatornák
        if globalRole != "admin" && globalRole != "owner" {
            query += `
                WHERE name IN (
                    SELECT DISTINCT channel
                    FROM channel_users
                    WHERE nick = ? COLLATE NOCASE
                )
            `
            args = append(args, username)
        }

        query += ` ORDER BY id ASC`

        rows, err := db.Query(query, args...)
        if err != nil {
            http.Error(w, "Database error", http.StatusInternalServerError)
            return
        }
        defer rows.Close()

        var topics []map[string]interface{}
        for rows.Next() {
            var id int
            var name string
            var currentTopic, topicSetBy, topicSetAt sql.NullString

            if err := rows.Scan(&id, &name, &currentTopic, &topicSetBy, &topicSetAt); err != nil {
                continue
            }

            topicStr := ""
            if currentTopic.Valid {
                topicStr = currentTopic.String
            }

            setByStr := ""
            if topicSetBy.Valid {
                setByStr = topicSetBy.String
            }

            setAtStr := ""
            if topicSetAt.Valid {
                setAtStr = topicSetAt.String
            }

            topics = append(topics, map[string]interface{}{
                "id":            id,
                "name":          name,
                "current_topic": topicStr,
                "topic_set_by":  setByStr,
                "topic_set_at":  setAtStr,
            })
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": true,
            "topics":  topics,
        })

    case http.MethodPut:
        // Topic frissítése
        var req struct {
            ID    int    `json:"id"`
            Topic string `json:"topic"`
        }

        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }

        // Lekérjük a csatorna nevét
        var channelName string
        err := db.QueryRow("SELECT name FROM channels WHERE id = ?", req.ID).Scan(&channelName)
        if err != nil {
            http.Error(w, "Channel not found", http.StatusNotFound)
            return
        }

        // ✅ Bot TOPIC - nézd meg van-e Topic() metódus a YnMIrC.Client-ben
        botTopicSuccess := false
        botTopicMessage := "IRC bot not connected"
        if p.client != nil && p.client.IsConnected() {
            // Ha van Topic() metódus:
            // p.client.Topic(channelName, req.Topic)
            
            // HA NINCS Topic() metódus, akkor SendRaw()-val:
            if req.Topic == "" {
                p.client.SendRaw(fmt.Sprintf("TOPIC %s :", channelName))
                botTopicMessage = fmt.Sprintf("Topic cleared in %s", channelName)
            } else {
                p.client.SendRaw(fmt.Sprintf("TOPIC %s :%s", channelName, req.Topic))
                botTopicMessage = fmt.Sprintf("Topic updated in %s", channelName)
            }
            botTopicSuccess = true
            time.Sleep(300 * time.Millisecond)
        }

        // Adatbázis frissítése
        _, err = db.Exec(`
            UPDATE channels 
            SET current_topic = ?,
                topic_set_by = ?,
                topic_set_at = datetime('now')
            WHERE id = ?
        `, req.Topic, username, req.ID)

        if err != nil {
            http.Error(w, "Failed to update topic", http.StatusInternalServerError)
            return
        }

        p.logAudit(username, "📝 TOPIC_UPDATED", r.RemoteAddr,
            fmt.Sprintf("Channel: %s, Topic: %s, IRC: %v", channelName, req.Topic, botTopicSuccess))

        // ✅ Lekérjük az összes topic-ot a frissítés után
        queryTopics := `
            SELECT id, name, current_topic, topic_set_by, topic_set_at
            FROM channels
        `
        argsTopics := []interface{}{}

        if globalRole != "admin" && globalRole != "owner" {
            queryTopics += `
                WHERE name IN (
                    SELECT DISTINCT channel
                    FROM channel_users
                    WHERE nick = ? COLLATE NOCASE
                )
            `
            argsTopics = append(argsTopics, username)
        }

        queryTopics += ` ORDER BY id ASC`

        rowsTopics, err := db.Query(queryTopics, argsTopics...)
        if err != nil {
            http.Error(w, "Database error", http.StatusInternalServerError)
            return
        }
        defer rowsTopics.Close()

        var topics []map[string]interface{}
        for rowsTopics.Next() {
            var id int
            var name string
            var currentTopic, topicSetBy, topicSetAt sql.NullString

            if err := rowsTopics.Scan(&id, &name, &currentTopic, &topicSetBy, &topicSetAt); err != nil {
                continue
            }

            topicStr := ""
            if currentTopic.Valid {
                topicStr = currentTopic.String
            }

            setByStr := ""
            if topicSetBy.Valid {
                setByStr = topicSetBy.String
            }

            setAtStr := ""
            if topicSetAt.Valid {
                setAtStr = topicSetAt.String
            }

            topics = append(topics, map[string]interface{}{
                "id":            id,
                "name":          name,
                "current_topic": topicStr,
                "topic_set_by":  setByStr,
                "topic_set_at":  setAtStr,
            })
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": true,
            "message": "Topic updated successfully",
            "topics":  topics,
            "bot_action": map[string]interface{}{
                "updated": botTopicSuccess,
                "message": botTopicMessage,
                "channel": channelName,
            },
        })

    case http.MethodPost:
        // ✅ ÚJ: Topic ellenőrzése és beállítása amikor bot OP lesz
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

        // Lekérjük a mentett topic-ot az adatbázisból
        var dbTopic, topicSetBy, topicSetAt sql.NullString
        err := db.QueryRow(`
            SELECT current_topic, topic_set_by, topic_set_at 
            FROM channels 
            WHERE name = ?
        `, req.Channel).Scan(&dbTopic, &topicSetBy, &topicSetAt)

        if err != nil {
            if err == sql.ErrNoRows {
                http.Error(w, "Channel not found in database", http.StatusNotFound)
                return
            }
            http.Error(w, "Database error", http.StatusInternalServerError)
            return
        }

        savedTopic := ""
        if dbTopic.Valid {
            savedTopic = dbTopic.String
        }

        // Ha nincs mentett topic, visszatérünk
        if savedTopic == "" {
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]interface{}{
                "success": true,
                "message": "No saved topic in database",
                "channel": req.Channel,
                "action":  "no_topic",
            })
            return
        }

        // Bot beállítja a topic-ot
        botTopicSuccess := false
        botTopicMessage := "IRC bot not connected"
        if p.client != nil && p.client.IsConnected() {
            p.client.SendRaw(fmt.Sprintf("TOPIC %s :%s", req.Channel, savedTopic))
            botTopicMessage = fmt.Sprintf("Topic loaded from database to %s", req.Channel)
            botTopicSuccess = true
            time.Sleep(300 * time.Millisecond)
        }

        p.logAudit(username, "🔄 TOPIC_LOADED", r.RemoteAddr,
            fmt.Sprintf("Channel: %s, Topic: %s", req.Channel, savedTopic))

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": true,
            "message": "Topic loaded from database",
            "channel": req.Channel,
            "topic":   savedTopic,
            "bot_action": map[string]interface{}{
                "updated": botTopicSuccess,
                "message": botTopicMessage,
                "channel": req.Channel,
            },
        })

    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}