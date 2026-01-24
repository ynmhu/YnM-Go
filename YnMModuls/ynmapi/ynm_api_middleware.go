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
    "strings"
    _ "github.com/mattn/go-sqlite3"
)
func (p *YnMApiPlugin) requireAuth(handler http.HandlerFunc) http.HandlerFunc {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, "Missing authorization", http.StatusUnauthorized)
            return
        }
        
        if strings.HasPrefix(token, "Bearer ") {
            token = strings.TrimPrefix(token, "Bearer ")
        }
        
        username := p.validateSessionToken(token)
        if username == "" {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }
        
        r.Header.Set("X-Username", username)
        handler(w, r)
    })
}

func (p *YnMApiPlugin) requireRole(handler http.HandlerFunc, allowedRoles ...string) http.HandlerFunc {
    return p.requireAuth(func(w http.ResponseWriter, r *http.Request) {
        username := r.Header.Get("X-Username")
        userRole := p.GetUserEffectiveRole(username)
        
        allowed := false
        for _, role := range allowedRoles {
            if strings.EqualFold(userRole, role) {
                allowed = true
                break
            }
        }
        
        if !allowed {
            p.logAudit(username, "🚫", r.RemoteAddr, 
                fmt.Sprintf("Required: %v, User role: %s", allowedRoles, userRole))
            http.Error(w, "Insufficient permissions", http.StatusForbidden)
            return
        }
        
        handler(w, r)
    })
}
func (p *YnMApiPlugin) requireGlobalRole(handler http.HandlerFunc, allowedRoles ...string) http.HandlerFunc {
    return p.requireAuth(func(w http.ResponseWriter, r *http.Request) {
        username := r.Header.Get("X-Username")
        userRole := p.getUserRole(username)
        
        allowed := false
        for _, role := range allowedRoles {
            if strings.EqualFold(userRole, role) {
                allowed = true
                break
            }
        }
        
        if !allowed {
            p.logAudit(username, "🚫", r.RemoteAddr, 
                fmt.Sprintf("Required: %v, User role: %s", allowedRoles, userRole))
            http.Error(w, "Insufficient permissions", http.StatusForbidden)
            return
        }
        
        handler(w, r)
    })
}
func (p *YnMApiPlugin) requireVIP(handler http.HandlerFunc) http.HandlerFunc {
    return p.requireAuth(func(w http.ResponseWriter, r *http.Request) {
        username := r.Header.Get("X-Username")
        userRole := p.getUserRole(username)
        
        // VIP, mod, admin és owner számára engedélyezzük a hozzáférést
        allowed := false
        for _, role := range []string{"vip", "mod", "admin", "owner"} {
            if strings.EqualFold(userRole, role) {
                allowed = true
                break
            }
        }
        
        if !allowed {
            http.Error(w, "VIP access required", http.StatusForbidden)
            return
        }
        
        handler(w, r)
    })
}
func (p *YnMApiPlugin) requireRoleOrLocalhost(handler http.HandlerFunc, allowedRoles ...string) http.HandlerFunc {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // ✅ CSAK Localhost bypass
        remoteAddr := r.RemoteAddr
        if remoteAddr == "127.0.0.1" || remoteAddr == "[::1]" || 
           strings.HasPrefix(remoteAddr, "127.0.0.1:") || 
           strings.HasPrefix(remoteAddr, "[::1]:") ||
           strings.HasPrefix(remoteAddr, "localhost:") {
            
            fmt.Printf("[YnMApI] ✅ Localhost bypass: %s %s\n", r.Method, r.URL.Path)
            handler(w, r)
            return
        }
        
        // Ha nem localhost, akkor normál auth
        fmt.Printf("[YnMApI] External request, checking auth: %s %s (from %s)\n", r.Method, r.URL.Path, remoteAddr)
        p.requireRole(handler, allowedRoles...)(w, r)
    })
}