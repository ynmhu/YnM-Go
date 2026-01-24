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


    "fmt"
    "net/http"
    "encoding/json"
	"strings"

)
func (p *YnMApiPlugin) handleEffectiveRole(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    nick := r.URL.Query().Get("nick")
    channel := r.URL.Query().Get("channel")
    
    if nick == "" || channel == "" {
        http.Error(w, "Missing nick or channel parameter", http.StatusBadRequest)
        return
    }
    
    // Get roles
    globalRole := p.GetUserGlobalRole(nick)
    channelRole := p.GetUserChannelRole(nick, channel)
    effectiveRole := p.GetEffectiveRole(nick, channel)
    canAccess := p.HasChannelAccess(nick, channel)
    
    response := map[string]interface{}{
        "success": true,
        "nick": nick,
        "channel": channel,
        "global_role": globalRole,
        "channel_role": channelRole,
        "effective_role": effectiveRole,
        "can_access": canAccess,
        "role_levels": RoleLevels,
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
// ✅ TELJES PERMISSION CHECK - creator + role hierarchy
func (p *YnMApiPlugin) CheckChannelUserPermission(
    currentNick, currentRole string,
    channelUserID int,
    action string, // "view", "edit", "delete"
) (bool, string) {

    // NORMALIZÁLÁS
    currentNick = strings.TrimSpace(currentNick)
    currentRole = strings.ToLower(strings.TrimSpace(currentRole)) // csak log/debug miatt tartjuk meg
    action = strings.ToLower(strings.TrimSpace(action))

    db, err := p.getDB()
    if err != nil || db == nil {
        return false, "Database not available"
    }

    // 1) Target rekord lekérése
    var targetNick, targetChannel, targetRole, addedBy, addedByRole string
    err = db.QueryRow(`
        SELECT cu.nick, cu.channel, cu.role, cu.added_by, COALESCE(u.role, '') as added_by_role
        FROM channel_users cu
        LEFT JOIN users u ON cu.added_by = u.nick
        WHERE cu.id = ?
    `, channelUserID).Scan(&targetNick, &targetChannel, &targetRole, &addedBy, &addedByRole)
    if err != nil {
        return false, "User not found"
    }

    targetNick = strings.TrimSpace(targetNick)
    targetChannel = strings.TrimSpace(targetChannel)
    targetRole = strings.ToLower(strings.TrimSpace(targetRole))
    addedBy = strings.TrimSpace(addedBy)
    addedByRole = strings.ToLower(strings.TrimSpace(addedByRole))

    // 2) Saját maga mindig
    if strings.EqualFold(targetNick, currentNick) {
        return true, "Self"
    }

    // 3) Effective role DB alapján (NE a kliens currentRole-ja alapján)
    currentEffectiveRole := strings.ToLower(strings.TrimSpace(p.GetEffectiveRole(currentNick, targetChannel)))
    targetEffectiveRole := strings.ToLower(strings.TrimSpace(p.GetEffectiveRole(targetNick, targetChannel)))

    // + globális owner fallback (ha a csatorna logika nem adná vissza)
    globalRole := strings.ToLower(strings.TrimSpace(p.GetUserGlobalRole(currentNick)))
    if currentEffectiveRole == "owner" || globalRole == "owner" {
        return true, "Owner"
    }

    // 4) Creator (case-insensitive)
    if strings.EqualFold(addedBy, currentNick) {
        switch action {
        case "view", "edit", "delete":
            return true, "Creator"
        }
    }

    // 5) Permission matrix (effective role alapján)
    permissionRules := map[string]map[string][]string{
        "view": {
            "owner": {"owner", "admin", "mod", "vip", "user"},
            "admin": {"admin", "mod", "vip", "user"},
            "mod":   {"mod", "vip", "user"},
            "vip":   {"vip", "user"},
            "user":  {"user"},
        },
        "edit": {
            "owner": {"owner", "admin", "mod", "vip", "user"},
            "admin": {"admin", "mod", "vip", "user"},
            "mod":   {"mod", "vip", "user"},
            "vip":   {"vip", "user"},
            "user":  {"user"},
        },
        "delete": {
            "owner": {"owner", "admin", "mod", "vip", "user"},
            "admin": {"admin", "mod", "vip", "user"},
            "mod":   {"mod", "vip", "user"},
            "vip":   {"vip", "user"},
            "user":  {"user"},
        },
    }

    // 6) Creator protection delete-nél (effective role szerint, nem currentRole szerint)
    if action == "delete" {
        if addedByRole == "admin" && !(currentEffectiveRole == "admin" || currentEffectiveRole == "owner") {
            return false, fmt.Sprintf("Only Admin or Owner can delete entries created by Admin (%s)", addedBy)
        }
        if addedByRole == "mod" && !(currentEffectiveRole == "mod" || currentEffectiveRole == "admin" || currentEffectiveRole == "owner") {
            return false, fmt.Sprintf("Only Mod, Admin or Owner can delete entries created by Mod (%s)", addedBy)
        }
    }

    // 7) Alap role check
    allowedTargets := permissionRules[action][currentEffectiveRole]
    if allowedTargets == nil {
        return false, fmt.Sprintf("No permission rules found (action=%s, your role=%s, raw role=%s)", action, currentEffectiveRole, currentRole)
    }

    for _, allowedRole := range allowedTargets {
        if targetEffectiveRole == allowedRole {
            return true, "Role based permission"
        }
    }

    return false, fmt.Sprintf(
        "%s cannot %s %s (your role: %s, target role: %s)",
        currentEffectiveRole, action, targetEffectiveRole,
        currentEffectiveRole, targetEffectiveRole,
    )
}
// ✅ ÚJ: Get user's effective role (considers channel-specific roles)
func (p *YnMApiPlugin) GetUserEffectiveRole(username string) string {
    // Start with global role
    globalRole := p.getUserRole(username)
    effectiveRole := globalRole
    
    // Check channel-specific roles
    rows, err := p.db.Query(`
        SELECT role FROM channel_users 
        WHERE nick = ? COLLATE NOCASE
    `, username)
    
    if err != nil {
        return effectiveRole
    }
    defer rows.Close()
    
    // Role hierarchy
    roleLevels := map[string]int{
        "owner": 5,
        "admin": 4,
        "mod":   3,
        "vip":   2,
        "user":  1,
    }
    
    for rows.Next() {
        var channelRole string
        rows.Scan(&channelRole)
        
        // If channel role is higher than current effective role
        if roleLevels[channelRole] > roleLevels[effectiveRole] {
            effectiveRole = channelRole
        }
    }
    
    return effectiveRole
}

// ✅ ÚJ: Get user's channel roles
func (p *YnMApiPlugin) getUserChannelRoles(username string) []map[string]interface{} {
    var roles []map[string]interface{}
    
    rows, err := p.db.Query(`
        SELECT channel, role 
        FROM channel_users 
        WHERE nick = ? COLLATE NOCASE
        ORDER BY channel
    `, username)
    
    if err != nil {
        return roles
    }
    defer rows.Close()
    
    for rows.Next() {
        var channel, role string
        rows.Scan(&channel, &role)
        
        roles = append(roles, map[string]interface{}{
            "channel": channel,  // kulcs: "channel"
            "role":    role,     // kulcs: "role"
        })
    }
    
    return roles
}

// ✅ ÚJ: Role-based statistics
func (p *YnMApiPlugin) getRoleBasedStats(username, effectiveRole string) map[string]interface{} {
    stats := map[string]interface{}{}
    
    switch effectiveRole {
    case "owner", "admin":
        // Full access
        stats["total_users"] = p.getTotalUsers()
        stats["total_channels"] = p.getTotalChannels()
        stats["recent_logs"] = p.getRecentAuditLogs(10)
        
    case "mod":
        // Limited access
        stats["channel_users"] = p.getUsersInSameChannels(username)
        stats["user_channels"] = len(p.getUserChannelRoles(username))
        stats["recent_activity"] = p.getUserRecentActivity(username, 5)
        
    case "vip":
        // Basic access
        stats["user_channels"] = len(p.getUserChannelRoles(username))
        stats["last_actions"] = p.getUserLastActions(username, 3)
        
    default: // user
        stats["user_channels"] = len(p.getUserChannelRoles(username))
    }
    
    return stats
}
// ✅ getTotalUsers - összes user számolása
func (p *YnMApiPlugin) getTotalUsers() int {
    var count int
    err := p.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
    if err != nil {
        return 0
    }
    return count
}

// ✅ getTotalChannels - összes channel számolása
func (p *YnMApiPlugin) getTotalChannels() int {
    var count int
    err := p.db.QueryRow("SELECT COUNT(*) FROM channels").Scan(&count)
    if err != nil {
        return 0
    }
    return count
}

// ✅ getRecentAuditLogs - legutóbbi audit logok
func (p *YnMApiPlugin) getRecentAuditLogs(limit int) []map[string]interface{} {
    var logs []map[string]interface{}
    
    rows, err := p.db.Query(`
        SELECT id, username, action, details, timestamp 
        FROM audit_logs 
        ORDER BY timestamp DESC 
        LIMIT ?
    `, limit)
    
    if err != nil {
        return logs
    }
    defer rows.Close()
    
    for rows.Next() {
        var id int
        var username, action, details, timestamp string
        
        rows.Scan(&id, &username, &action, &details, &timestamp)
        
        logs = append(logs, map[string]interface{}{
            "id":        id,
            "username":  username,
            "action":    action,
            "details":   details,
            "timestamp": timestamp,
        })
    }
    
    return logs
}

// ✅ getUsersInSameChannels - user channel társai
func (p *YnMApiPlugin) getUsersInSameChannels(username string) int {
    // 1. Get user's channels
    var channels []string
    rows, err := p.db.Query(`
        SELECT DISTINCT channel 
        FROM channel_users 
        WHERE nick = ? COLLATE NOCASE
    `, username)
    
    if err != nil {
        return 0
    }
    defer rows.Close()
    
    for rows.Next() {
        var channel string
        rows.Scan(&channel)
        channels = append(channels, channel)
    }
    
    if len(channels) == 0 {
        return 0
    }
    
    // 2. Count unique users in those channels
    query := `
        SELECT COUNT(DISTINCT nick) 
        FROM channel_users 
        WHERE channel IN (?` + strings.Repeat(",?", len(channels)-1) + `)
    `
    
    args := make([]interface{}, len(channels))
    for i, ch := range channels {
        args[i] = ch
    }
    
    var count int
    err = p.db.QueryRow(query, args...).Scan(&count)
    if err != nil {
        return 0
    }
    
    return count
}

// ✅ getUserRecentActivity - user legutóbbi tevékenységei
func (p *YnMApiPlugin) getUserRecentActivity(username string, limit int) []map[string]interface{} {
    var activities []map[string]interface{}
    
    rows, err := p.db.Query(`
        SELECT action, details, timestamp 
        FROM audit_logs 
        WHERE username = ? COLLATE NOCASE
        ORDER BY timestamp DESC 
        LIMIT ?
    `, username, limit)
    
    if err != nil {
        return activities
    }
    defer rows.Close()
    
    for rows.Next() {
        var action, details, timestamp string
        
        rows.Scan(&action, &details, &timestamp)
        
        activities = append(activities, map[string]interface{}{
            "action":    action,
            "details":   details,
            "timestamp": timestamp,
        })
    }
    
    return activities
}

// ✅ getUserLastActions - user utolsó X akciója
func (p *YnMApiPlugin) getUserLastActions(username string, limit int) []map[string]interface{} {
    // Ez ugyanaz mint a getUserRecentActivity, csak más néven
    return p.getUserRecentActivity(username, limit)
}


