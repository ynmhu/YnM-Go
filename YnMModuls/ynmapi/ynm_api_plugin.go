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
    "time"
    "sync"
    "database/sql"
    "log"
    "fmt"
	"runtime"
    "net/http"
    "strings"
    _ "github.com/mattn/go-sqlite3"
    "git.ynm.hu/markus/YnM-Go/YnMIrC"
    "git.ynm.hu/markus/YnM-Go/YnMModule"
    "git.ynm.hu/markus/YnM-Go/YnMAdmin"
    "git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMDb"
)


var RoleLevels = map[string]int{
    "owner": 5,
    "admin": 4,
    "mod":   3,
    "vip":   2,
    "user":  1,
}

// ===== STRUKTÚRÁK =====
type YnMApiPlugin struct {
    client      *YnMIrC.Client
    db          *sql.DB
	adminDB     *YnMDb.AdminDB
    quit        chan struct{}
    adminPlugin *owner.YnmAdminPlugin   
    passwords   map[string]*PasswordEntry
	roleLevels map[string]int
    mutex       sync.RWMutex    
    dbReady     bool
    dbMutex     sync.RWMutex
    startTime   time.Time
    config      *YnMConfig.YnMApiConfig
	cfg         *YnMConfig.Config
    configMutex sync.RWMutex
	statusQuit       chan struct{}
    startedAt        time.Time
    Version          string
    GoVersion        string
    repository       string
	reconnectMu  sync.Mutex
	reconnecting bool
}
// Channel struktúra a channels táblához
type Channel struct {
    ID              int       `json:"id"`
    Name            string    `json:"name"`
    AutoOp          bool      `json:"auto_op"`
    AutoVoice       bool      `json:"auto_voice"`
    AutoHalfop      bool      `json:"auto_halfop"`
    Owner           string    `json:"owner"`
    OwnerHostmask   string    `json:"owner_hostmask"`
    CreatedAt       time.Time `json:"created_at"`
}

// ChannelMode struktúra a channel_modes táblához
type ChannelMode struct {
    ID          int       `json:"id"`
    Channel     string    `json:"channel"`
    Modes       string    `json:"modes"`
    ModeParams  string    `json:"mode_params"`
    SetBy       string    `json:"set_by"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    Active      bool      `json:"active"`
    Enabled     bool      `json:"enabled"`
    Mode        bool      `json:"mode"`
}

// ChannelUser struktúra a channel_users táblához
type ChannelUser struct {
    ID          int       `json:"id"`
    Nick        string    `json:"nick"`
    Hostmask    string    `json:"hostmask"`
    Channel     string    `json:"channel"`
    Role        string    `json:"role"`
    AutoOp      bool      `json:"auto_op"`
    AutoVoice   bool      `json:"auto_voice"`
    AutoHalfop  bool      `json:"auto_halfop"`
    CreatedAt   time.Time `json:"created_at"`
    AddedBy     string    `json:"added_by"`
}
type ChannelUserRequest struct {
    ID          int    `json:"id,omitempty"`
    Nick        string `json:"nick"`
    Hostmask    string `json:"hostmask,omitempty"`
    Channel     string `json:"channel"`
    Role        string `json:"role,omitempty"`
    AutoOp      *bool  `json:"auto_op,omitempty"`
    AutoVoice   *bool  `json:"auto_voice,omitempty"`
    AutoHalfop  *bool  `json:"auto_halfop,omitempty"`
    AddedBy     string `json:"added_by,omitempty"`
    CurrentRole string `json:"current_role,omitempty"`
}
type ChannelUserUpdateRequest struct {
    ID      int    `json:"id"`
    Nick    string `json:"nick"`
    Channel string `json:"channel"`
    Field   string `json:"field"`
    Value   int    `json:"value"` // 0 vagy 1
}
type ChannelAddRequest struct {
    Name         string `json:"name"`
    owner        string `json:"owner"`
    ownerHostmask string `json:"owner_hostmask"`
    AutoOp       int    `json:"auto_op"`
    AutoVoice    int    `json:"auto_voice"`
    AutoHalfop   int    `json:"auto_halfop"`
    RequestedBy  string `json:"requested_by"`
    CurrentRole  string `json:"current_role"`
}
type ChannelUserDeleteRequest struct {
    ID          int    `json:"id"`
    Nick        string `json:"nick,omitempty"`
    Channel     string `json:"channel,omitempty"`
    DeletedBy   string `json:"deleted_by,omitempty"`
    CurrentRole string `json:"current_role,omitempty"`
	CurrentUser      string `json:"current_user,omitempty"`       
    CurrentUserRole  string `json:"current_user_role,omitempty"`  
	User             string `json:"user,omitempty"`             
    Role             string `json:"role,omitempty"`            
}
// ChannelWithStats - Csatorna statisztikákkal
type ChannelWithStats struct {
    Channel
    UserCount   int `json:"user_count"`
    OwnerCount  int `json:"owner_count"`
    AdminCount  int `json:"admin_count"`
    ModCount    int `json:"mod_count"`
    VipCount    int `json:"vip_count"`
}

type PasswordEntry struct {
    Username   string    `json:"username"`
    Password   string    `json:"password"`
    CreatedAt  time.Time `json:"created_at"`
    ExpiresAt  time.Time `json:"expires_at"`
    UsedCount  int       `json:"used_count"`
    MaxUses    int       `json:"max_uses"`
    RequestedBy string   `json:"requested_by"`
}

type AuthRequest struct {
    Username string `json:"username"`
    Password string `json:"password"`
    Command  string `json:"command"`
}

type AuthResponse struct {
    Success   bool   `json:"success"`
    Message   string `json:"message"`
    Token     string `json:"token,omitempty"`
    ExpiresIn int64  `json:"expires_in,omitempty"`
    Role      string `json:"role,omitempty"`
    Username  string `json:"username,omitempty"`
    UserID    int    `json:"user_id,omitempty"`
}

type UserProfile struct {
    Username  string    `json:"username"`
    Email     string    `json:"email"`
    Role      string    `json:"role"`
    Language  string    `json:"language"`
    LastLogin time.Time `json:"last_login"`
}

// ✅ JAVÍTOTT: UpdateProfileRequest - minden mező amit a PHP küld
type UpdateProfileRequest struct {
    Email      string `json:"email"`
    Role       string `json:"role"`
    Lang       string `json:"lang"`
    MyChar     string `json:"mychar"`
    Welcome    string `json:"welcome"`
	Website    string `json:"website"` 
    DiscordID  string `json:"discord_id"`
    TelegramID string `json:"telegram_id"`
    Facebook   string `json:"facebook"`
    AvatarType string `json:"avatar_type"`
    AvatarURL  string `json:"avatar_url"`
}

// ✅ JAVÍTOTT: UserProfileUpdate - pointer mezők az opcionális frissítésekhez
type UserProfileUpdate struct {
    Nick       *string    `json:"nick,omitempty"`
    Hostmask   *string    `json:"hostmask,omitempty"`
    Role       *string    `json:"role,omitempty"`
    AddedBy    *string    `json:"added_by,omitempty"`
    Lang       *string    `json:"lang,omitempty"`
    MyChar     *string    `json:"mychar,omitempty"`
    CreatedAt  *time.Time `json:"created_at,omitempty"`
    Email      *string    `json:"email,omitempty"`
    Welcome    *string    `json:"welcome,omitempty"`
	Website    *string    `json:"website,omitempty"`  
    DiscordID  *string    `json:"discord_id,omitempty"`
    TelegramID *string    `json:"telegram_id,omitempty"`
    Facebook   *string    `json:"facebook,omitempty"`
    AvatarType *string    `json:"avatar_type,omitempty"`
    AvatarURL  *string    `json:"avatar_url,omitempty"`
}

func NewYnMApiPlugin(client *YnMIrC.Client, cfg *YnMConfig.Config, adminPlugin *owner.YnmAdminPlugin, adminDB *YnMDb.AdminDB) *YnMApiPlugin {

    YnMApiCfg, err := YnMConfig.LoadYnMApiConfig("YnMConfig/ynm-api.yaml")
    if err != nil {
        log.Printf("[YnMApI] ⚠️ Failed to load ynm-api.yaml: %v, using defaults", err)
        // Használj default értékeket
        YnMApiCfg = &YnMConfig.YnMApiConfig{}
        YnMApiCfg.YnM.Port = 2525
        YnMApiCfg.YnM.WebsiteURL = "https://ynm-go.ynm.hu"  
        YnMApiCfg.YnM.Session.Lifetime = 3600
        YnMApiCfg.YnM.Password.ExpiryMinutes = 60
        
        // Default expiry options
        YnMApiCfg.YnM.Password.ExpiryOptions = map[int]string{
            30:     "30 perc",
            60:     "1 óra",
            180:    "3 óra",
            1440:   "24 óra",
            10080:  "1 hét",
            43200:  "1 hónap",
            525600: "1 év",
            0:      "Soha ne járjon le",
        }
    }
    
    plugin := &YnMApiPlugin{
        client:      client,
		db: adminPlugin.Db.SQL,
		adminDB:     adminDB,
        quit:        make(chan struct{}),
        adminPlugin: adminPlugin,
        passwords:   make(map[string]*PasswordEntry),
        dbReady:     false,
        startTime:   time.Now(),
        config:      YnMApiCfg,
		cfg:              cfg,
		statusQuit:  make(chan struct{}),
        startedAt:   time.Now(),
        Version:		owner.YnMVersion,
        GoVersion:   runtime.Version(),
        repository:  "https://git.ynm.hu/markus/YnM-Go",
    }

    // Initialize database connection with retry logic
    go plugin.initializeDatabase()
    
    // Cleanup goroutine
    go plugin.cleanupLoop()
    
    // HTTP server
    go plugin.startHTTPServer()
    
    // Debug info
    fmt.Printf("[YnMApI] Website URL configured: %s\n", YnMApiCfg.YnM.WebsiteURL)
    fmt.Printf("[YnMApI] Login page: %s/login\n", YnMApiCfg.YnM.WebsiteURL)

    return plugin
}
func (p *YnMApiPlugin) startHTTPServer() {
    cfg := p.GetConfig()
    
    mux := http.NewServeMux()

    corsHandler := func(h http.HandlerFunc) http.HandlerFunc {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            origin := r.Header.Get("Origin")
            
            for _, allowed := range cfg.YnM.AllowedOrigins {
                if origin == strings.TrimSpace(allowed) {
                    w.Header().Set("Access-Control-Allow-Origin", origin)
                    break
                }
            }
            
            w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
            w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
            w.Header().Set("Access-Control-Allow-Credentials", "true")

            if r.Method == "OPTIONS" {
                w.WriteHeader(http.StatusOK)
                return
            }
            h(w, r)
        })
    }

    // ✅ Prefix-ek a konfigból
    botPrefix := cfg.YnM.Routing.BotPrefix
    
    // ===== PUBLIC ENDPOINTS =====
    mux.HandleFunc(botPrefix+"/auth", corsHandler(p.handleAuth))
    mux.HandleFunc(botPrefix+"/status", corsHandler(p.handleStatus))
	mux.HandleFunc(botPrefix+"/bot-stats", corsHandler(p.handleBotStats))
    mux.HandleFunc(botPrefix+"/max-uses-options", corsHandler(p.requireRoleOrLocalhost(p.handleMaxUsesOptions)))
    
    // ===== USER ENDPOINTS (VIP+) =====
    mux.HandleFunc(botPrefix+"/dashboard", corsHandler(p.requireRoleOrLocalhost(p.handleDashboard)))
    mux.HandleFunc(botPrefix+"/profile", corsHandler(p.requireRoleOrLocalhost(p.handleProfile, "vip", "mod", "admin", "owner")))
	mux.HandleFunc(botPrefix+"/profile/avatar", corsHandler(p.requireRoleOrLocalhost(p.handleProfileAvatar, "vip", "mod", "admin", "owner")))
    mux.HandleFunc(botPrefix+"/permissions", corsHandler(p.requireRoleOrLocalhost(p.handlePermissions)))
    
    // ===== ADMIN ENDPOINTS (MOD+) =====
    mux.HandleFunc(botPrefix+"/stats", corsHandler(p.requireRoleOrLocalhost(p.handleStats, "vip", "mod", "admin", "owner")))
    mux.HandleFunc(botPrefix+"/audit", corsHandler(p.requireRoleOrLocalhost(p.handleAudit, "vip", "mod", "admin", "owner")))
	mux.HandleFunc(botPrefix+"/audit-logs", corsHandler(p.requireRoleOrLocalhost(p.handleAuditLogs, "vip", "mod", "admin", "owner")))
    mux.HandleFunc(botPrefix+"/users", corsHandler(p.requireRoleOrLocalhost(func(w http.ResponseWriter, r *http.Request) {p.handleUsers(w, r)}, "vip", "mod", "admin", "owner")))
	mux.HandleFunc(botPrefix+"/users/",corsHandler(p.requireRoleOrLocalhost(func(w http.ResponseWriter, r *http.Request) {switch r.Method { case http.MethodPut: p.handleUsersUpdate(w, r); case http.MethodDelete: p.handleUsersDelete(w, r); default: http.Error(w, "Method not allowed", http.StatusMethodNotAllowed) }},  "vip", "mod", "admin", "owner")))
    mux.HandleFunc(botPrefix+"/database", corsHandler(p.requireRoleOrLocalhost(p.handleDatabase, "vip", "mod", "admin", "owner")))
    
    // ===== BOT CONTROL ENDPOINTS (MOD+) =====
    mux.HandleFunc(botPrefix+"/channels", corsHandler(p.requireRoleOrLocalhost(p.handleChannels, "vip", "mod", "admin", "owner")))
	mux.HandleFunc(botPrefix+"/channels/topic", corsHandler(p.requireRoleOrLocalhost(p.handleChannelsTopic, "vip", "mod", "admin", "owner")))
	mux.HandleFunc(botPrefix+"/channels/mode", corsHandler(p.requireRoleOrLocalhost(p.handleChannelsMode, "vip", "mod", "admin", "owner")))
    mux.HandleFunc(botPrefix+"/channels/sync", corsHandler(p.requireRoleOrLocalhost(p.handleChannelSync, "vip", "mod", "admin", "owner")))
    mux.HandleFunc(botPrefix+"/channels/detail", corsHandler(p.requireRoleOrLocalhost(p.handleChannelDetail, "vip", "mod", "admin", "owner")))
    mux.HandleFunc(botPrefix+"/control", corsHandler(p.requireRoleOrLocalhost(p.handleBotControl, "vip", "mod", "admin", "owner")))
    mux.HandleFunc(botPrefix+"/memory", corsHandler(p.requireRoleOrLocalhost(p.handleMemoryStats, "vip", "mod", "admin", "owner")))
	
	// ===== CHANNEL USERS ENDPOINTS =====
    mux.HandleFunc(botPrefix+"/channel-users", corsHandler(p.requireRoleOrLocalhost(p.handleChannelUsers, "vip", "mod", "admin", "owner")))
    mux.HandleFunc(botPrefix+"/channel-users/update", corsHandler(p.requireRoleOrLocalhost(p.handleChannelUsersUpdate, "vip", "mod", "admin", "owner")))  
	mux.HandleFunc(botPrefix+"/channel-users/add", corsHandler(p.requireRoleOrLocalhost(p.handleChannelUsersAdd, "vip", "mod", "admin", "owner")))     
    mux.HandleFunc(botPrefix+"/channel-users/delete", corsHandler(p.requireRoleOrLocalhost(p.handleChannelUsersDelete,  "vip", "mod", "admin", "owner"))) 
	// ===== CHANNELS EXTENDED =====
	mux.HandleFunc(botPrefix+"/channels/list-stats", corsHandler(p.requireRoleOrLocalhost(p.handleChannelsListWithStats, "vip", "mod", "admin", "owner")))
	mux.HandleFunc(botPrefix+"/channels/full-detail", corsHandler(p.requireRoleOrLocalhost(p.handleChannelFullDetail, "vip", "mod", "admin", "owner")))
	mux.HandleFunc(botPrefix+"/channels/with-users", corsHandler(p.requireRoleOrLocalhost(p.handleChannelWithUsers,  "vip", "mod", "admin", "owner")))

		    // ===== CHANNEL USERS ENDPOINTS =====
	
	// ===== DATABASE ENDPOINTS =====
    mux.HandleFunc(botPrefix+"/database/passwords", corsHandler(p.requireRoleOrLocalhost(p.handleDatabasePasswords, "vip", "mod", "admin", "owner")))
    mux.HandleFunc(botPrefix+"/database/stats", corsHandler(p.requireRoleOrLocalhost(p.handleDatabaseStats, "vip", "mod", "admin", "owner")))
    mux.HandleFunc(botPrefix+"/database/generate", corsHandler(p.requireRoleOrLocalhost(p.handleDatabaseGenerate, "vip", "mod", "admin", "owner")))
	mux.HandleFunc(botPrefix+"/user/password/change", corsHandler(p.requireRoleOrLocalhost(p.handleUserPasswordChange, "vip", "mod", "admin", "owner")))
	// ===== JELSZÓ KEZELÉS ENDPOINTS (MOD+) =====
	
	mux.HandleFunc(botPrefix+"/password/change", corsHandler(p.requireRoleOrLocalhost(p.handlePasswordChange, "vip", "mod", "admin", "owner")))
	mux.HandleFunc(botPrefix+"/password/add", corsHandler(p.requireRoleOrLocalhost(p.handlePasswordAdd, "vip", "mod", "admin", "owner")))
	mux.HandleFunc(botPrefix+"/password/list", corsHandler(p.requireRoleOrLocalhost(p.handlePasswordList, "vip", "mod", "admin", "owner")))
	mux.HandleFunc(botPrefix+"/password/delete", corsHandler(p.requireRoleOrLocalhost(p.handlePasswordDelete, "vip", "mod", "admin", "owner")))
	    // ✅ ÚJ ENDPOINT hozzáadása:
    mux.HandleFunc(botPrefix+"/users/effective-role", corsHandler(p.requireRoleOrLocalhost(p.handleEffectiveRole, "vip", "mod", "admin", "owner")))
	
    serverAddr := fmt.Sprintf(":%d", cfg.YnM.Port)
    fmt.Printf("[YnMApI] HTTP API server starting on %s\n", serverAddr)
    fmt.Println("[YnMApI] Configuration:")
    fmt.Printf("  Bot API: %s\n", botPrefix)
    fmt.Printf("  Port: %d\n", cfg.YnM.Port)
    
    fmt.Println("\n[YnMApI] Available endpoints:")
    fmt.Println("\n  📢 PUBLIC:")
    fmt.Printf("    POST   %s/auth\n", botPrefix)
    fmt.Printf("    GET    %s/status\n", botPrefix)
    fmt.Printf("    GET    %s/max-uses-options\n", botPrefix) 
    
    fmt.Println("\n  👤 USER (VIP+):")
    fmt.Printf("    GET    %s/dashboard\n", botPrefix)
    fmt.Printf("    GET    %s/profile\n", botPrefix)
    fmt.Printf("    GET    %s/permissions\n", botPrefix)
    
    fmt.Println("\n  ⚙️ ADMIN (MOD+):")
    fmt.Printf("    GET    %s/stats\n", botPrefix)
    fmt.Printf("    GET    %s/audit\n", botPrefix)
    fmt.Printf("    GET    %s/users\n", botPrefix)
    fmt.Printf("    GET    %s/database (owner)\n", botPrefix)
    fmt.Printf("    *      %s/password/change (owner)\n", botPrefix)
    fmt.Printf("    POST   %s/password/add (owner)\n", botPrefix)
    fmt.Printf("    GET    %s/password/list (owner)\n", botPrefix)
    fmt.Printf("    DELETE %s/password/delete (owner)\n", botPrefix)
    
    fmt.Println("\n  🤖 BOT CONTROL (MOD+):")
    fmt.Printf("    *      %s/channels\n", botPrefix)
    fmt.Printf("    POST   %s/channels/sync\n", botPrefix)
	fmt.Printf("    GET    %s/channels/topic\n", botPrefix)
	fmt.Printf("    GET    %s/channels/mode\n", botPrefix)
	fmt.Printf("   POST   %s/channels/topic-update\n", botPrefix)
	fmt.Printf("   POST   %s/channels/mode-update\n", botPrefix)
    fmt.Printf("    GET    %s/channels/detail\n", botPrefix)
    fmt.Printf("    POST  %s/control (owner)\n", botPrefix)
    fmt.Printf("    GET    %s/memory (owner)\n", botPrefix)
    fmt.Printf("    GET    %s/channels/list-stats\n", botPrefix)
    fmt.Printf("    GET    %s/channels/full-detail\n", botPrefix)
    fmt.Printf("    GET    %s/channels/with-users\n", botPrefix)
    
    fmt.Println("\n  👥 CHANNEL USERS (MOD+):")
    fmt.Printf("    *      %s/channel-users\n", botPrefix)
    fmt.Printf("    POST   %s/channel-users/update\n", botPrefix)
	fmt.Printf("    POST   %s/channel-users/add\n", botPrefix)      
    fmt.Printf("    DELETE %s/channel-users/delete\n", botPrefix) 
    
    fmt.Println("\n  💾 DATABASE (MOD+):")
    fmt.Printf("    GET    %s/database/passwords\n", botPrefix)
    fmt.Printf("    GET    %s/database/stats\n", botPrefix)
    fmt.Printf("    POST   %s/database/generate\n", botPrefix)
    
    fmt.Printf("\n[YnMApI] Total registered endpoints: %d\n", 
        // You might want to calculate this dynamically or update manually
        21) // Példa szám - frissítsd a tényleges endpointok számára
    fmt.Println("[YnMApI] Server initialized successfully!")
    
    if err := http.ListenAndServe(serverAddr, mux); err != nil {
        fmt.Printf("[YnMApI] HTTP server error: %v\n", err)
    }
}

func (p *YnMApiPlugin) HandleMessage(msg YnMIrC.Message) string {
    if !strings.HasPrefix(msg.Text, "!web") {
        return ""
    }

    if !p.isDatabaseReady() {
        fmt.Println("[YnMApI] Cannot handle !web: database not ready")
        return "Service temporarily unavailable, please try again in a moment."
    }

    nick, hostmask := YnMModule.GetEffectiveNickAndHost(p.adminPlugin, msg.Sender)
    role := YnMModule.GetUserGlobalRoleWithDB(p.adminPlugin.Db, nick, hostmask)

    // Csak VIP+, mod, admin, owner használhatja
    allowedRoles := []string{"vip", "mod", "admin", "owner"}
    allowed := false
    for _, r := range allowedRoles {
        if strings.EqualFold(role, r) {
            allowed = true
            break
        }
    }
    
    if !allowed {
        p.logAudit(nick, "🚫", hostmask, "Command: !web - Insufficient role")
        return ""
    }

    // Default értékek: 60 perc (1 óra), 10 használat
    expiryMinutes := 60
    maxUses := 10

    // Jelszó generálása
    password := p.generatePasswordForUserWithMaxUses(nick, nick, expiryMinutes, maxUses)
    if password == "" {
        return fmt.Sprintf("Error generating password for %s!", nick)
    }

    // Website URL
    cfg := p.GetConfig()
    loginURL := cfg.YnM.WebsiteURL + "/login"

    // Csak 3 sor - semmi extra
    p.client.SendMessage(nick, fmt.Sprintf("🔐 Web jelszó: %s", nick))
    p.client.SendMessage(nick, fmt.Sprintf("🔑 %s", password))
    p.client.SendMessage(nick, fmt.Sprintf("⏰ 1 óra | 🌐 %s", loginURL))

    p.logAudit(nick, "🔑", hostmask, fmt.Sprintf("Role: %s, Expiry: 60 minutes, MaxUses: 10", role))

    return "" // Nincs szükség visszatérési értékre
}

// ==================== ROLE HELPER FUNCTIONS ====================
// ✅ IDE (a fájl végére, az összes handler után):

// GetEffectiveRole - számolja az effective role-t (globális vs channel)
func (p *YnMApiPlugin) GetEffectiveRole(nick, channel string) string {
    // 1. Globális role lekérése
    globalRole := p.GetUserGlobalRole(nick)
    if globalRole == "" {
        globalRole = "user"
    }
    
    // 2. Channel-specific role lekérése
    channelRole := p.GetUserChannelRole(nick, channel)
    if channelRole == "" {
        channelRole = "user"
    }
    
    // 3. A magasabbat visszaadjuk
    globalLevel := RoleLevels[globalRole]
    channelLevel := RoleLevels[channelRole]
    
    if channelLevel > globalLevel {
        return channelRole
    }
    return globalRole
}

// HasChannelAccess - ellenőrzi, hogy bemehet-e a csatornába
func (p *YnMApiPlugin) HasChannelAccess(nick, channel string) bool {
    effectiveRole := p.GetEffectiveRole(nick, channel)
    level := RoleLevels[effectiveRole]
    
    // Minimum VIP (level 2) kell a channel-ökbe
    return level >= 2
}

// GetUserGlobalRole - lekéri a globális role-t
func (p *YnMApiPlugin) GetUserGlobalRole(nick string) string {
    db, err := p.getDB()
    if err != nil || db == nil {
        return "user"
    }

    var role string
    err = db.QueryRow("SELECT role FROM users WHERE nick = ?", nick).Scan(&role)
    if err != nil {
        if err == sql.ErrNoRows {
            return "user"
        }
        log.Printf("[YnMApi] Error getting global role for %s: %v", nick, err)
        return "user"
    }
    return role
}

func (p *YnMApiPlugin) GetUserChannelRole(nick, channel string) string {
    db, err := p.getDB()
    if err != nil || db == nil {
        return "user"
    }

    var role string
    err = db.QueryRow(
        "SELECT role FROM channel_users WHERE nick = ? AND channel = ?",
        nick, channel,
    ).Scan(&role)
    if err != nil {
        if err == sql.ErrNoRows {
            return "user"
        }
        log.Printf("[YnMApi] Error getting channel role for %s in %s: %v", nick, channel, err)
        return "user"
    }
    return role
}

// CanModifyUser - ellenőrzi, hogy módosítható-e a felhasználó
func (p *YnMApiPlugin) CanModifyUser(currentNick, currentRole, targetNick, targetChannel string) bool {
    targetEffectiveRole := p.GetEffectiveRole(targetNick, targetChannel)
    currentEffectiveRole := p.GetEffectiveRole(currentNick, targetChannel)
    
    currentLevel := RoleLevels[currentEffectiveRole]
    targetLevel := RoleLevels[targetEffectiveRole]
    
    // Csak magasabb role módosíthat alacsonyabbat
    return currentLevel > targetLevel
}


func (p *YnMApiPlugin) GetActivePasswordsCount() int {
    p.mutex.RLock()
    defer p.mutex.RUnlock()
    return len(p.passwords)
}
func (p *YnMApiPlugin) GetDatabaseStatus() bool {
    return p.isDatabaseReady()
}

func (p *YnMApiPlugin) Stop() {
    close(p.quit)
}

func (p *YnMApiPlugin) OnTick() []YnMIrC.Message {
    return nil
}