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

package owner

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
	"log"
	_ "github.com/mattn/go-sqlite3"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

// GenerateRandomString generates a random string of specified length
func (p *YnmAdminPlugin) GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// GenerateSessionID creates a unique session ID
func (p *YnmAdminPlugin) GenerateSessionID() string {
    return fmt.Sprintf("%d_%s", time.Now().UnixNano(), p.GenerateRandomString(16))
}

// GenerateSessionKey creates a short session key for user display
func (p *YnmAdminPlugin) GenerateSessionKey() string {
    const keyCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
    
    b := make([]byte, 8)
    for i := range b {
        b[i] = keyCharset[seededRand.Intn(len(keyCharset))]
    }
    return string(b)
}

// owner plugin CreateSession függvénye - JAVÍTVA
func (p *YnmAdminPlugin) CreateSession(originalHost, loggedInAs string) (string, string) {
    p.mu.Lock()
    defer p.mu.Unlock()
    simplifiedOriginalHost := YnMModule.SimplifyHostmask(originalHost)
    loggedInHost := p.getUserHostmask(loggedInAs)
    sessionID := p.GenerateSessionID()
    sessionKey := p.GenerateSessionKey()
    session := &Session{
        OriginalHost:    simplifiedOriginalHost,  
        LoggedInAs:      loggedInAs,              
        LoggedInHost:    loggedInHost,          
        LoginTime:       time.Now(),
        LastActivity:    time.Now(),
        SessionKey:      sessionKey,
    }
    
    p.sessions[sessionID] = session
    p.hostSessions[simplifiedOriginalHost] = sessionID  // Login hostmask
    p.hostSessions[loggedInHost] = sessionID            // Regisztrált hostmask
    
    p.sessionKeys[sessionKey] = sessionID
    
    fmt.Printf("[DEBUG CreateSession] Created session for both hostmasks:\n")
    fmt.Printf("  Login hostmask: %s -> session\n", simplifiedOriginalHost)
    fmt.Printf("  Registered hostmask: %s -> session\n", loggedInHost)
    fmt.Printf("  User: %s, Key: %s\n", loggedInAs, sessionKey)
    
    return sessionID, sessionKey
}
func (p *YnmAdminPlugin) getUserHostmask(nick string) string {
    info, err := p.Db.GetUserInfoByNick(nick)
    if err != nil || info == nil {
         return nick + "!*@*"
    }
    if !strings.HasPrefix(info.Hostmask, "*!*@") {
        return YnMModule.SimplifyHostmask(info.Hostmask)
    }  
    return info.Hostmask
}


func (p *YnmAdminPlugin) GetSessionByHost(hostmask string) (*Session, bool) {

	if YnMModule.IsServerHostmask(hostmask) {
		return nil, false
	}
	
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	// ✅ GYORS KILÉPÉS: Ha nincs session, ne keressünk
	if len(p.hostSessions) == 0 {
		return nil, false
	}
	
	// Próbáljuk meg közvetlenül
	sessionID, exists := p.hostSessions[hostmask]
	if !exists {
		return nil, false
	}
	
	session, exists := p.sessions[sessionID]
	if exists {
		session.LastActivity = time.Now()
	}
	return session, exists
}
// DeleteSession removes a session by sessionID
func (p *YnmAdminPlugin) DeleteSession(sessionID string) {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    if session, exists := p.sessions[sessionID]; exists {
        delete(p.hostSessions, session.OriginalHost)
        delete(p.sessions, sessionID)
        
        // Session key is törlése is
        if session.SessionKey != "" {
            delete(p.sessionKeys, session.SessionKey)
        }
    }
}

func (p *YnmAdminPlugin) HandleLoginCommand(fullHostmask, username, password string) {
    nick := strings.Split(fullHostmask, "!")[0]
    
    // Ellenőrizzük a hitelesítést - JAVÍTOTT
    valid, err := p.Db.VerifyPassword(username, password)
    if err != nil || !valid {
        p.Bot.SendMessage(nick, "❌ Hibás felhasználónév vagy jelszó.")
        return
    }
    
    // Simplified hostmask-ot használunk a session tárolásához
    simplifiedHostmask := YnMModule.SimplifyHostmask(fullHostmask)
    
    // Session létrehozása (visszakapjuk a sessionID-t és a sessionKey-t)
    sessionID, sessionKey := p.CreateSession(simplifiedHostmask, username)
    _ = sessionID // használatlan változó
    
    p.Bot.SendMessage(nick, "✅ Sikeres bejelentkezés "+username+" felhasználóként!")
    p.Bot.SendMessage(nick, "Session Key: "+sessionKey+" (24 óráig érvényes)")
}

// DeleteSessionByHost removes a session by hostmask
func (p *YnmAdminPlugin) DeleteSessionByHost(hostmask string) {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    sessionID, exists := p.hostSessions[hostmask]
    if exists {
        // Session key is törlése
        if session, exists := p.sessions[sessionID]; exists && session.SessionKey != "" {
            delete(p.sessionKeys, session.SessionKey)
        }
        
        delete(p.hostSessions, hostmask)
        delete(p.sessions, sessionID)
    }
}

// CleanupExpiredSessions removes old sessions
func (p *YnmAdminPlugin) CleanupExpiredSessions() {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    now := time.Now()
    maxAge := 24 * time.Hour // 24 hours session duration
    
    for sessionID, session := range p.sessions {
        if now.Sub(session.LastActivity) > maxAge {
            delete(p.hostSessions, session.OriginalHost)
            
            // Session key is törlése
            if session.SessionKey != "" {
                delete(p.sessionKeys, session.SessionKey)
            }
            
            delete(p.sessions, sessionID)
        }
    }
}

func (p *YnmAdminPlugin) GetEffectiveUser(hostmask string) (string, string) {
	originalNick := strings.Split(hostmask, "!")[0]
	
	// ✅ GYORS KILÉPÉS: Ha nincs session, ne keressünk
	p.mu.RLock()
	hasAnySessions := len(p.hostSessions) > 0
	p.mu.RUnlock()
	
	if !hasAnySessions {
		// Nincs session, visszaadjuk az eredeti nicket és simplified hostot
		simplifiedHostmask := YnMModule.SimplifyHostmask(hostmask)
		return originalNick, simplifiedHostmask
	}
	
	// Van session, keressük meg
	simplifiedHostmask := YnMModule.SimplifyHostmask(hostmask)
	
	// Először simplified hostmask-al próbáljuk
	if session, exists := p.GetSessionByHost(simplifiedHostmask); exists {
		return session.LoggedInAs, session.LoggedInHost
	}
	
	// Ha nincs session simplified-del, próbáljuk a teljes hostmask-al
	if session, exists := p.GetSessionByHost(hostmask); exists {
		return session.LoggedInAs, session.LoggedInHost
	}
	
	// Nincs session, visszaadjuk az eredetit simplified formában
	return originalNick, simplifiedHostmask
}

// HasAccessWithSession checks permissions with session support
func (p *YnmAdminPlugin) HasAccessWithSession(hostmask, command string) bool {
    effectiveUser, _ := p.GetEffectiveUser(hostmask)
    log.Printf("🔍 HasAccessWithSession: hostmask=%s, effectiveUser=%s, command=%s", hostmask, effectiveUser, command)
    
    commandRoles := map[string]string{
        "setpass":  "user",
        "setmail":  "user",
        "login":    "user",
        "logout":   "user",
        "auth":     "user",
        "adduser":  "admin",
        "deluser":  "admin",
        "op":       "admin",
        "halfop":   "admin",
        "voice":    "admin",
        "shutdown": "owner",
        "restart":  "owner",
    }
    requiredRole, exists := commandRoles[command]
    if !exists {
        log.Printf("🔍 Command '%s' not found in commandRoles", command)
        return false
    }
    
    // Használjuk a tényleges user hostmask-ját
    effectiveHostmask := p.GetEffectiveHostmask(hostmask)
    role := YnMModule.GetUserGlobalRoleWithDB(p.Db, effectiveUser, effectiveHostmask)
    log.Printf("🔍 Role check: effectiveHostmask=%s, role=%s, requiredRole=%s", effectiveHostmask, role, requiredRole)
    
    result := p.HasRole(role, requiredRole)
    log.Printf("🔍 HasRole result: %v (userLevel: %d >= requiredLevel: %d)", result, RoleHierarchy[role], RoleHierarchy[requiredRole])
    return result
}

// GetUserRoleWithSession returns role considering session
func (p *YnmAdminPlugin) GetUserRoleWithSession(hostmask string) string {
    effectiveUser, _ := p.GetEffectiveUser(hostmask)
    return YnMModule.GetUserGlobalRoleWithDB(p.Db, effectiveUser, effectiveUser+"!*@*")
}

func (p *YnmAdminPlugin) HandleSessions(nick string, isPrivate bool) {
    hostmask := nick + "!*@*"
    if !p.HasAccessWithSession(hostmask, "adduser") {
        p.Bot.SendMessage(nick, "❌ Nincs jogosultságod ehhez a parancshoz.")
        return
    }

    p.mu.RLock()
    defer p.mu.RUnlock()

    if len(p.sessions) == 0 {
        p.Bot.SendMessage(nick, "Nincsenek aktív session-ök.")
        return
    }

    p.Bot.SendMessage(nick, "Aktív session-ök:")
    for _, session := range p.sessions {
        duration := time.Since(session.LoginTime).Round(time.Minute)
        p.Bot.SendMessage(nick, fmt.Sprintf("- Key: %s | %s → %s (%v)", 
            session.SessionKey, session.OriginalHost, session.LoggedInAs, duration))
    }
}

func (p *YnmAdminPlugin) HandleSessionInfo(nick string, isPrivate bool) {
    hostmask := nick + "!*@*"
    if session, exists := p.GetSessionByHost(hostmask); exists {
        duration := time.Since(session.LoginTime).Round(time.Minute)
        p.Bot.SendMessage(nick, fmt.Sprintf("Session információk:"))
        p.Bot.SendMessage(nick, fmt.Sprintf("Bejelentkezve mint: %s", session.LoggedInAs))
        p.Bot.SendMessage(nick, fmt.Sprintf("Bejelentkezés ideje: %s", session.LoginTime.Format("2006-01-02 15:04:05")))
        p.Bot.SendMessage(nick, fmt.Sprintf("Időtartam: %v", duration))
        p.Bot.SendMessage(nick, fmt.Sprintf("Utolsó aktivitás: %s", session.LastActivity.Format("15:04:05")))
    } else {
        p.Bot.SendMessage(nick, "Nincs aktív session-öd.")
    }
}

// Add this debug method to check session state
func (p *YnmAdminPlugin) DebugSessionState(hostmask string) {
    fmt.Printf("[DEBUG SessionState] Checking session for hostmask: %s\n", hostmask)
    
    p.mu.RLock()
    defer p.mu.RUnlock()
    
    // Check sessions map
    fmt.Printf("[DEBUG SessionState] Total sessions: %d\n", len(p.sessions))
    for sessionID, session := range p.sessions {
        fmt.Printf("[DEBUG SessionState] Session %s: %s -> %s\n", 
            sessionID[:8], session.OriginalHost, session.LoggedInAs)
    }
    
    // Check hostSessions map
    fmt.Printf("[DEBUG SessionState] Total host sessions: %d\n", len(p.hostSessions))
    for host, sessionID := range p.hostSessions {
        fmt.Printf("[DEBUG SessionState] Host %s -> Session %s\n", host, sessionID[:8])
    }
    
    // Check specific host
    if sessionID, exists := p.hostSessions[hostmask]; exists {
        fmt.Printf("[DEBUG SessionState] %s has active session: %s\n", hostmask, sessionID[:8])
        if session, exists := p.sessions[sessionID]; exists {
            fmt.Printf("[DEBUG SessionState] Session details: %s -> %s (login: %s)\n",
                session.OriginalHost, session.LoggedInAs, session.LoginTime.Format("15:04:05"))
        }
    } else {
        fmt.Printf("[DEBUG SessionState] %s has NO active session\n", hostmask)
    }
}

func (p *YnmAdminPlugin) HandleSessionDebug(nick string, isPrivate bool) {
    hostmask := nick + "!*@*"
    effectiveUser, effectiveHost := p.GetEffectiveUser(hostmask)
    
    p.Bot.SendMessage(nick, fmt.Sprintf("Session Debug:"))
    p.Bot.SendMessage(nick, fmt.Sprintf("Original host: %s", hostmask))
    p.Bot.SendMessage(nick, fmt.Sprintf("Effective user: %s", effectiveUser))
    p.Bot.SendMessage(nick, fmt.Sprintf("Effective host: %s", effectiveHost))
    
    if session, exists := p.GetSessionByHost(hostmask); exists {
        p.Bot.SendMessage(nick, fmt.Sprintf("Session active: YES"))
        p.Bot.SendMessage(nick, fmt.Sprintf("Logged in as: %s", session.LoggedInAs))
        p.Bot.SendMessage(nick, fmt.Sprintf("Using host: %s", session.LoggedInHost))
        p.Bot.SendMessage(nick, fmt.Sprintf("Login time: %s", session.LoginTime.Format("15:04:05")))
    } else {
        p.Bot.SendMessage(nick, fmt.Sprintf("Session active: NO"))
    }
}

func (p *YnmAdminPlugin) GetEffectiveHostmask(hostmask string) string {
    if session, exists := p.GetSessionByHost(hostmask); exists {
        return session.LoggedInHost  // ✅ JÓÓÓÓ!
    }
    return hostmask
}
func isServerHostmask(hostmask string) bool {
    serverPatterns := []string{
        "irc.ynm.hu",
        "services.",
        "authserv.",
        "chanserv.", 
        "nickserv.",
        "memoserv.",
        "operserv.",
        "hostserv.",
        ".",
    }
    
    // Ha a hostmask pontot tartalmaz (nem valid user hostmask)
    if strings.Contains(hostmask, ".") && !strings.Contains(hostmask, "!") {
        return true
    }
    
    lowerHostmask := strings.ToLower(hostmask)
    for _, pattern := range serverPatterns {
        if strings.Contains(lowerHostmask, strings.ToLower(pattern)) {
            return true
        }
    }
    return false
}
func (p *YnmAdminPlugin) DebugDumpSessions() {
    p.mu.RLock()
    defer p.mu.RUnlock()
    
    fmt.Printf("==========================================\n")
    fmt.Printf("SESSION DEBUG DUMP\n")
    fmt.Printf("==========================================\n")
    fmt.Printf("Total sessions: %d\n", len(p.sessions))
    fmt.Printf("Total host sessions: %d\n", len(p.hostSessions))
    fmt.Printf("Total session keys: %d\n", len(p.sessionKeys))
    fmt.Printf("------------------------------------------\n")
    
    // Dump all sessions with full details
    for sessionID, session := range p.sessions {
        fmt.Printf("Session ID: %s\n", sessionID)
        fmt.Printf("  OriginalHost: '%s'\n", session.OriginalHost)
        fmt.Printf("  LoggedInAs: '%s'\n", session.LoggedInAs)
        fmt.Printf("  LoggedInHost: '%s'\n", session.LoggedInHost)
        fmt.Printf("  SessionKey: '%s'\n", session.SessionKey)
        fmt.Printf("  LoginTime: %s\n", session.LoginTime.Format("15:04:05"))
        fmt.Printf("  LastActivity: %s\n", session.LastActivity.Format("15:04:05"))
        fmt.Printf("\n")
    }
    
    fmt.Printf("------------------------------------------\n")
    fmt.Printf("HOST TO SESSION MAPPINGS:\n")
    for host, sessionID := range p.hostSessions {
        fmt.Printf("  '%s' -> %s\n", host, sessionID[:16])
    }
    
    fmt.Printf("------------------------------------------\n")
    fmt.Printf("SESSION KEY TO SESSION MAPPINGS:\n")
    for key, sessionID := range p.sessionKeys {
        fmt.Printf("  '%s' -> %s\n", key, sessionID[:16])
    }
    
    fmt.Printf("==========================================\n")
}

// GetSessionCount returns the number of active sessions
func (p *YnmAdminPlugin) GetSessionCount() int {
    p.mu.RLock()
    defer p.mu.RUnlock()
    return len(p.sessions)
}

// GetAllSessionInfo returns formatted session info for display
func (p *YnmAdminPlugin) GetAllSessionInfo() []string {
    p.mu.RLock()
    defer p.mu.RUnlock()
    
    var info []string
    info = append(info, fmt.Sprintf("📊 Total sessions: %d", len(p.sessions)))
    
    for _, session := range p.sessions {
        duration := time.Since(session.LoginTime).Round(time.Minute)
        line := fmt.Sprintf("  • %s -> %s (key: %s, duration: %v)",
            session.OriginalHost, session.LoggedInAs, session.SessionKey, duration)
        info = append(info, line)
    }
    
    return info
}