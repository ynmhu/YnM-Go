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
package YnMDb

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"context"

	_ "github.com/mattn/go-sqlite3"
)

// ==================================================
// ADATBÁZIS STRUKTÚRÁK
// ==================================================

type AdminDB struct {
	db  *sql.DB
	SQL *sql.DB
}

// ==================================================
// FELHASZNÁLÓI STRUKTÚRÁK
// ==================================================

type UserInfo struct {
	Nick       string
	Hostmask   string
	Role       string
	AddedBy    string
	Lang       string
	MyChar     *string
	Welcome    *string
	Website    *string
	Pass       *string
	Email      *string
	DiscordID  *string
	TelegramID *string
	Facebook   *string
	Invites    int
	AvatarURL  *string
	AvatarType string
	LastLogin  *time.Time
	UpdatedAt  *time.Time
	CreatedAt  time.Time
}

type PasswordInfo struct {
	Username     string
	PasswordHash string
	ExpiresAt    time.Time
	Uses         int
	MaxUses      int
	LastUsed     time.Time
	CreatedAt    time.Time
}

// ==================================================
// CSATORNA STRUKTÚRÁK
// ==================================================

type ChannelInfo struct {
	ID            int
	Name          string
	AutoOp        bool
	AutoVoice     bool
	AutoHalfOp    bool
	Owner         *string
	OwnerHostmask *string
	CurrentTopic  *string
	TopicSetBy    *string
	TopicSetAt    *time.Time
	CurrentModes  *string
	ModesSetBy    *string
	ModesSetAt    *time.Time
	CreatedAt     time.Time
	MyPermission  string
}

type UserChannelRole struct {
	UserID        int
	Nick          string
	Hostmask      string
	Channel       string
	ChannelID     int
	Role          string
	AutoOp        bool
	AutoVoice     bool
	AutoHalfOp    bool
	Automode      string
	owner         string
	ownerHostmask string
	CreatedAt     time.Time
}

type ChannelBan struct {
	ID        int
	Channel   string
	Mask      string
	SetBy     string
	Reason    *string
	CreatedAt time.Time
	ExpiresAt *time.Time
	Active    bool
}

type ChannelModeHistory struct {
	ID        int
	Channel   string
	Modes     string
	SetBy     string
	CreatedAt time.Time
}

// ==================================================
// NAPLÓ STRUKTÚRÁK
// ==================================================

type BotLog struct {
	ID        int
	Username  string
	Action    string
	Hostmask  string
	Details   string
	Channel   *string
	Command   *string
	Timestamp time.Time
}

type WebLog struct {
	ID        int
	Username  string
	Action    string
	IPAddress string
	UserAgent *string
	Endpoint  *string
	Details   string
	Timestamp time.Time
}

// ==================================================
// PLUGIN STRUKTÚRÁK
// ==================================================

type PluginInfo struct {
	ID          int
	Name        string
	Description string
	CreatedAt   time.Time
}

type PluginState struct {
	ID         int
	PluginName string
	Channel    string
	IsActive   bool
	UpdatedAt  time.Time
}
// 1. Migration struktúra a RunMigrations-hez
type Migration struct {
	Version     int
	Description string
	Up          func(*sql.DB) error
	Down        func(*sql.DB) error
}
// 2. getExpectedSchema függvény a VerifySchema-hoz
func getExpectedSchema() map[string][]struct {
	name string
	typ  string
	def  string
} {
	return map[string][]struct {
		name string
		typ  string
		def  string
	}{
		"bot_stats": {
			{"key", "TEXT", "PRIMARY KEY"},
			{"value", "INTEGER", "NOT NULL"},
			{"ram_used_mb", "REAL", ""},
			{"cpu_percent", "REAL", ""},
			{"process_memory_mb", "REAL", ""},
			{"load_avg", "TEXT", ""},
			{"disk_usage", "TEXT", ""},
			{"disk_io", "TEXT", ""},
			{"network_traffic", "TEXT", ""},
			{"thread_count", "INTEGER", ""},
			{"nick", "TEXT", ""},
			{"version", "TEXT", "NOT NULL"},
			{"go_version", "TEXT", "NOT NULL"},
			{"bot_uptime", "TEXT", "NOT NULL"},
			{"bot_max_uptime", "TEXT", "NOT NULL"},
		    {"bot_max_connect_time", "TEXT", "NOT NULL"},
			{"server_uptime", "TEXT", "NOT NULL"},
			{"channels", "TEXT", "NOT NULL"},
			{"server", "TEXT", "NOT NULL"},
			{"connected", "INTEGER", "NOT NULL"},
			{"last_updated", "DATETIME", "NOT NULL"},
			{"total_users", "INTEGER", "NOT NULL DEFAULT 0"},
			{"owner", "TEXT", "NOT NULL DEFAULT ''"},
			{"globaladmins", "TEXT", "NOT NULL DEFAULT '[]'"},
			{"globalmods", "TEXT", "NOT NULL DEFAULT '[]'"},
			{"globalvips", "TEXT", "NOT NULL DEFAULT '[]'"},
			{"admins", "TEXT", "NOT NULL DEFAULT '[]'"},
			{"mods", "TEXT", "NOT NULL DEFAULT '[]'"},
			{"vips", "TEXT", "NOT NULL DEFAULT '[]'"},
		},
		"users": {
			{"nick", "TEXT", "NOT NULL"},
			{"hostmask", "TEXT", "NOT NULL"},
			{"role", "TEXT", "NOT NULL DEFAULT 'user'"},
			{"added_by", "TEXT", "NOT NULL DEFAULT 'YnM-Go'"},
			{"lang", "TEXT", "NOT NULL DEFAULT 'en'"},
			{"mychar", "TEXT", ""},
			{"welcome", "TEXT", ""},
			{"website", "TEXT", ""},
			{"pass", "TEXT", ""},
			{"email", "TEXT", ""},
			{"discord_id", "TEXT", ""},
			{"telegram_id", "TEXT", ""},
			{"facebook", "TEXT", ""},
			{"avatar_url", "TEXT", ""},
			{"avatar_type", "TEXT", "DEFAULT 'initials'"},
			{"invites", "INTEGER", "DEFAULT 0"},
			{"password_expires", "DATETIME", ""},
			{"password_uses", "INTEGER", "DEFAULT 0"},
			{"password_max_uses", "INTEGER", "DEFAULT 10"},
			{"password_last_used", "DATETIME", ""},
			{"password_created", "DATETIME", ""},
			{"created_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
			{"last_login", "DATETIME", ""},
			{"updated_at", "DATETIME", ""},
		},
		"forget_pass_logs": {
			{"nick", "TEXT", "NOT NULL"},
			{"created_at", "DATETIME", "NOT NULL DEFAULT CURRENT_TIMESTAMP"},
		},
		"channels": {
			{"name", "TEXT", "NOT NULL"},
			{"auto_op", "BOOLEAN", "DEFAULT 0"},
			{"auto_voice", "BOOLEAN", "DEFAULT 0"},
			{"auto_halfop", "BOOLEAN", "DEFAULT 0"},
			{"owner", "TEXT", ""},
			{"owner_hostmask", "TEXT", ""},
			{"current_topic", "TEXT", ""},
			{"topic_set_by", "TEXT", ""},
			{"topic_set_at", "DATETIME", ""},
			{"current_modes", "TEXT", "DEFAULT '+nt'"},
			{"modes_set_by", "TEXT", "DEFAULT 'YnM-Go'"},
			{"modes_set_at", "DATETIME", ""},
			{"created_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
		},
		"channel_users": {
			{"nick", "TEXT", "NOT NULL"},
			{"hostmask", "TEXT", "NOT NULL"},
			{"channel", "TEXT", "NOT NULL"},
			{"role", "TEXT", "NOT NULL DEFAULT ''"},
			{"auto_op", "BOOLEAN", "DEFAULT 0"},
			{"auto_voice", "BOOLEAN", "DEFAULT 0"},
			{"auto_halfop", "BOOLEAN", "DEFAULT 0"},
			{"added_by", "TEXT", "NOT NULL DEFAULT 'YnM-Go'"},
			{"added_by_host", "TEXT", "NOT NULL DEFAULT 'YnM-Go'"},
			{"created_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
			{"modified_by", "TEXT", "NOT NULL DEFAULT 'YnM-Go'"},
			{"modified_by_host", "TEXT", "NOT NULL DEFAULT 'YnM-Go'"},
			{"changed_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
		},
		"channel_modes": {
			{"channel", "TEXT", "NOT NULL"},
			{"modes", "TEXT", "NOT NULL"},
			{"mode", "TEXT", "NOT NULL DEFAULT ''"},
			{"mode_params", "TEXT", ""},
			{"enabled", "BOOLEAN", "NOT NULL DEFAULT 1"},
			{"set_by", "TEXT", "NOT NULL"},
			{"set_by_host", "TEXT", "NOT NULL DEFAULT 'YnM-Go'"},
			{"created_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
			{"updated_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
			{"active", "BOOLEAN", "DEFAULT 1"},
		},
		"channel_bans": {
			{"channel", "TEXT", "NOT NULL"},
			{"mask", "TEXT", "NOT NULL"},
			{"set_by", "TEXT", "NOT NULL"},
			{"set_by_host", "TEXT", "NOT NULL DEFAULT 'YnM-Go'"},
			{"reason", "TEXT", ""},
			{"created_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
			{"expires_at", "DATETIME", ""},
			{"active", "BOOLEAN", "DEFAULT 1"},
		},
		"channel_mode_history": {
			{"channel", "TEXT", "NOT NULL"},
			{"modes", "TEXT", "NOT NULL"},
			{"set_by", "TEXT", "NOT NULL"},
			{"set_by_host", "TEXT", "NOT NULL DEFAULT 'YnM-Go'"},
			{"created_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
		},
		"plugins": {
			{"name", "TEXT", "NOT NULL"},
			{"description", "TEXT", ""},
			{"created_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
		},
		"plugin_states": {
			{"plugin_name", "TEXT", "NOT NULL"},
			{"channel", "TEXT", "NOT NULL"},
			{"is_active", "BOOLEAN", "NOT NULL DEFAULT 0"},
			{"updated_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
		},
		"bot_logs": {
			{"username", "TEXT", "NOT NULL"},
			{"action", "TEXT", "NOT NULL"},
			{"hostmask", "TEXT", "NOT NULL"},
			{"details", "TEXT", ""},
			{"channel", "TEXT", ""},
			{"command", "TEXT", ""},
			{"timestamp", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
		},
		"web_sessions": {
			{"token", "TEXT", "PRIMARY KEY"},
			{"username", "TEXT", "NOT NULL"},
			{"created_at", "DATETIME", "NOT NULL DEFAULT CURRENT_TIMESTAMP"},
			{"expires_at", "DATETIME", "NOT NULL"},
			{"ip_address", "TEXT", ""},
			{"user_agent", "TEXT", ""},
		},
		"web_logs": {
			{"username", "TEXT", "NOT NULL"},
			{"action", "TEXT", "NOT NULL"},
			{"ip_address", "TEXT", ""},
			{"timestamp", "DATETIME", "NOT NULL DEFAULT CURRENT_TIMESTAMP"},
			{"details", "TEXT", ""},
		},
	}
}

// ==================================================
// ADATBÁZIS INICIALIZÁLÁS
// ==================================================

func NewAdminDB() (*AdminDB, error) {
	if err := os.MkdirAll("data", 0755); err != nil {
		return nil, fmt.Errorf("couldn't create data dir: %v", err)
	}

	dbPath := filepath.Join("data", "ynm.db")
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=1&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("couldn't open database: %v", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("couldn't ping database: %v", err)
	}

	if err := createTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("couldn't create tables: %v", err)
	}

	checker := NewSchemaChecker(db)
	if err := checker.CheckAndAddColumns(); err != nil {
		fmt.Printf("Schema check error: %v\n", err)
	}

	return &AdminDB{db: db, SQL: db}, nil
}

func (a *AdminDB) Close() error {
	return a.db.Close()
}
// ==================================================
// TÁBLÁK LÉTREHOZÁSA
// ==================================================

func createTables(db *sql.DB) error {
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("couldn't enable foreign keys: %v", err)
	}

	tables := getTableDefinitions()
	for _, table := range tables {
		if _, err := db.Exec(table.query); err != nil {
			return fmt.Errorf("error creating %s table: %v", table.name, err)
		}
	}

	if err := createIndexes(db); err != nil {
		return err
	}

	if err := insertBasePlugins(db); err != nil {
		return err
	}

	return nil
}

func getTableDefinitions() []struct {
	name  string
	query string
} {
	return []struct {
		name  string
		query string
	}{
		{"bot_stats", `CREATE TABLE IF NOT EXISTS bot_stats (
			key TEXT PRIMARY KEY,
			value INTEGER NOT NULL,
			ram_used_mb REAL,
			cpu_percent REAL,
			process_memory_mb REAL,
			load_avg TEXT,
			disk_usage TEXT,
			disk_io  TEXT,
			network_traffic TEXT,
			thread_count INTEGER,
			pid INTEGER,
			exec_path TEXT,
			nick TEXT,
			version TEXT NOT NULL,
			go_version TEXT NOT NULL,
			bot_uptime TEXT NOT NULL,
			bot_max_uptime TEXT NOT NULL DEFAULT '0s',
			bot_max_connect_time TEXT NOT NULL DEFAULT '0s',
			server_uptime TEXT NOT NULL,
			channels TEXT NOT NULL,
			server TEXT NOT NULL,
			connected INTEGER NOT NULL,
			last_updated DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			total_users INTEGER NOT NULL DEFAULT 0,
			owner TEXT NOT NULL DEFAULT '',
			globaladmins TEXT NOT NULL DEFAULT '[]',
			globalmods TEXT NOT NULL DEFAULT '[]',
			globalvips TEXT NOT NULL DEFAULT '[]',
			admins TEXT NOT NULL DEFAULT '[]',
			mods TEXT NOT NULL DEFAULT '[]',
			vips TEXT NOT NULL DEFAULT '[]'
		)`},
		{"users", `CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nick TEXT NOT NULL UNIQUE COLLATE NOCASE,
			hostmask TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			added_by TEXT NOT NULL DEFAULT 'YnM-Go',
			lang TEXT NOT NULL DEFAULT 'en',
			mychar TEXT DEFAULT '!',
			welcome TEXT,
			website TEXT,
			pass TEXT,
			email TEXT,
			discord_id TEXT,
			telegram_id TEXT,
			facebook TEXT,
			avatar_url TEXT,
			avatar_type TEXT DEFAULT 'initials',
			invites INTEGER DEFAULT 0,
			password_expires DATETIME,
			password_uses INTEGER DEFAULT 0,
			password_max_uses INTEGER DEFAULT 10,
			password_last_used DATETIME,
			password_created DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_login DATETIME,
			updated_at DATETIME
		)`},
		{"forget_pass_logs", `CREATE TABLE IF NOT EXISTS forget_pass_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nick TEXT NOT NULL COLLATE NOCASE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`},
		{"channels", `CREATE TABLE IF NOT EXISTS channels (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL COLLATE NOCASE,
			auto_op BOOLEAN DEFAULT 0,
			auto_voice BOOLEAN DEFAULT 0,
			auto_halfop BOOLEAN DEFAULT 0,
			owner TEXT,
			owner_hostmask TEXT,
			current_topic TEXT,
			topic_set_by TEXT,
			topic_set_at DATETIME,
			current_modes TEXT DEFAULT '+nt',
			modes_set_by TEXT DEFAULT 'YnM-Go',
			modes_set_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(name)
		)`},
		{"channel_users", `CREATE TABLE IF NOT EXISTS channel_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nick TEXT NOT NULL COLLATE NOCASE,
			hostmask TEXT NOT NULL,
			channel TEXT NOT NULL COLLATE NOCASE,
			role TEXT NOT NULL DEFAULT '',
			auto_op BOOLEAN DEFAULT 0,
			auto_voice BOOLEAN DEFAULT 0,
			auto_halfop BOOLEAN DEFAULT 0,
			added_by TEXT NOT NULL DEFAULT 'YnM-Go',
			added_by_host TEXT NOT NULL DEFAULT 'YnM-Go',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			modified_by TEXT NOT NULL DEFAULT 'YnM-Go',
			modified_by_host TEXT NOT NULL DEFAULT 'YnM-Go',
			changed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(nick, channel)
		)`},
		{"channel_modes", `CREATE TABLE IF NOT EXISTS channel_modes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			channel TEXT NOT NULL COLLATE NOCASE,
			modes TEXT NOT NULL,
			mode TEXT NOT NULL DEFAULT '',
			mode_params TEXT,
			enabled BOOLEAN NOT NULL DEFAULT 1,
			set_by TEXT NOT NULL,
			set_by_host TEXT NOT NULL DEFAULT 'YnM-Go',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			active BOOLEAN DEFAULT 1,
			UNIQUE(channel)
		)`},
		{"channel_bans", `CREATE TABLE IF NOT EXISTS channel_bans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			channel TEXT NOT NULL COLLATE NOCASE,
			mask TEXT NOT NULL,
			set_by TEXT NOT NULL,
			set_by_host TEXT NOT NULL DEFAULT 'YnM-Go',
			reason TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME,
			active BOOLEAN DEFAULT 1,
			UNIQUE(channel, mask)
		)`},
		{"channel_mode_history", `CREATE TABLE IF NOT EXISTS channel_mode_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			channel TEXT NOT NULL COLLATE NOCASE,
			modes TEXT NOT NULL,
			set_by TEXT NOT NULL,
			set_by_host TEXT NOT NULL DEFAULT 'YnM-Go',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`},
		{"bot_logs", `CREATE TABLE IF NOT EXISTS bot_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL COLLATE NOCASE,
			action TEXT NOT NULL COLLATE NOCASE,
			hostmask TEXT NOT NULL,
			details TEXT,
			channel TEXT,
			command TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		)`},
		{"web_sessions", `CREATE TABLE IF NOT EXISTS web_sessions (
			token TEXT PRIMARY KEY,
			username TEXT NOT NULL COLLATE NOCASE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL,
			ip_address TEXT,
			user_agent TEXT
		)`},
		{"web_logs", `CREATE TABLE IF NOT EXISTS web_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL COLLATE NOCASE,
			action TEXT NOT NULL,
			ip_address TEXT,
			timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			details TEXT
		)`},
		{"plugins", `CREATE TABLE IF NOT EXISTS plugins (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(name)
		)`},
		{"plugin_states", `CREATE TABLE IF NOT EXISTS plugin_states (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			plugin_name TEXT NOT NULL,
			channel TEXT NOT NULL COLLATE NOCASE,
			is_active BOOLEAN NOT NULL DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(plugin_name) REFERENCES plugins(name),
			UNIQUE(plugin_name, channel)
		)`},
	}
}

func createIndexes(db *sql.DB) error {
	indexes := []string{
		// users tábla indexek
		`CREATE INDEX IF NOT EXISTS idx_users_nick ON users(nick COLLATE NOCASE)`,
		`CREATE INDEX IF NOT EXISTS idx_users_password_expires ON users(password_expires)`,
		`CREATE INDEX IF NOT EXISTS idx_users_password_created ON users(password_created DESC)`,
		
		// channels tábla indexek
		`CREATE INDEX IF NOT EXISTS idx_channels_name ON channels(name COLLATE NOCASE)`,
		`CREATE INDEX IF NOT EXISTS idx_channels_owner ON channels(owner)`,
		`CREATE INDEX IF NOT EXISTS idx_channels_created ON channels(created_at DESC)`,
		
		// channel_users tábla indexek
		`CREATE INDEX IF NOT EXISTS idx_channel_users_nick ON channel_users(nick COLLATE NOCASE)`,
		`CREATE INDEX IF NOT EXISTS idx_channel_users_channel ON channel_users(channel COLLATE NOCASE)`,
		`CREATE INDEX IF NOT EXISTS idx_channel_users_nick_channel ON channel_users(nick, channel)`,
		
		// channel_modes tábla indexek
		`CREATE INDEX IF NOT EXISTS idx_channel_modes_channel ON channel_modes(channel COLLATE NOCASE)`,
		`CREATE INDEX IF NOT EXISTS idx_channel_modes_active ON channel_modes(active)`,
		
		// channel_bans tábla indexek
		`CREATE INDEX IF NOT EXISTS idx_channel_bans_channel ON channel_bans(channel)`,
		`CREATE INDEX IF NOT EXISTS idx_channel_bans_expires ON channel_bans(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_channel_bans_active ON channel_bans(active)`,
		`CREATE INDEX IF NOT EXISTS idx_channel_bans_channel_active ON channel_bans(channel, active)`,
		
		// channel_mode_history tábla indexek
		`CREATE INDEX IF NOT EXISTS idx_mode_history_channel ON channel_mode_history(channel)`,
		`CREATE INDEX IF NOT EXISTS idx_mode_history_created ON channel_mode_history(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_mode_history_channel_created ON channel_mode_history(channel, created_at DESC)`,

		// bot_logs tábla indexek
		`CREATE INDEX IF NOT EXISTS idx_bot_logs_username ON bot_logs(username)`,
		`CREATE INDEX IF NOT EXISTS idx_bot_logs_timestamp ON bot_logs(timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_bot_logs_action ON bot_logs(action)`,
		`CREATE INDEX IF NOT EXISTS idx_bot_logs_channel ON bot_logs(channel) WHERE channel IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_bot_logs_command ON bot_logs(command) WHERE command IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_bot_logs_username_timestamp ON bot_logs(username, timestamp DESC)`,
		
		// web_sessions tábla indexek
		`CREATE INDEX IF NOT EXISTS idx_web_sessions_username ON web_sessions(username)`,
		`CREATE INDEX IF NOT EXISTS idx_web_sessions_expires ON web_sessions(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_web_sessions_username_expires ON web_sessions(username, expires_at)`,
		
		// web_logs tábla indexek
		`CREATE INDEX IF NOT EXISTS idx_web_logs_username ON web_logs(username)`,
		`CREATE INDEX IF NOT EXISTS idx_web_logs_timestamp ON web_logs(timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_web_logs_action ON web_logs(action)`,
		`CREATE INDEX IF NOT EXISTS idx_web_logs_username_timestamp ON web_logs(username, timestamp DESC)`,
		
		// forget_pass_logs tábla indexek
		`CREATE INDEX IF NOT EXISTS idx_forget_pass_logs_nick ON forget_pass_logs(nick COLLATE NOCASE)`,
		`CREATE INDEX IF NOT EXISTS idx_forget_pass_logs_created ON forget_pass_logs(created_at DESC)`,
		
		// bot_stats tábla indexek (PRIMARY KEY miatt nem szükséges külön index a key-re)
		`CREATE INDEX IF NOT EXISTS idx_bot_stats_last_updated ON bot_stats(last_updated DESC)`,
				
		// plugins tábla indexek
		`CREATE INDEX IF NOT EXISTS idx_plugins_name ON plugins(name)`,
		
		// plugin_states tábla indexek
		`CREATE INDEX IF NOT EXISTS idx_plugin_states_channel ON plugin_states(channel)`,
		`CREATE INDEX IF NOT EXISTS idx_plugin_states_active ON plugin_states(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_plugin_states_plugin_channel ON plugin_states(plugin_name, channel)`,
		
	}

	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			fmt.Printf("Warning: Could not create index: %v\n", err)
		}
	}

	return nil
}



// ==================================================
// SchemaChecker plugin – hiányzó oszlopok ellenőrzése
// ==================================================
type SchemaChecker struct {
	db *sql.DB
}

func NewSchemaChecker(db *sql.DB) *SchemaChecker {
	return &SchemaChecker{db: db}
}

// CheckAndAddColumns végigmegy a táblákon és oszlopokon, hiányzó oszlopokat hozzáad
func (s *SchemaChecker) CheckAndAddColumns() error {
	tables := map[string][]struct {
		name string
		typ  string
		def  string
	}{
		"bot_stats": {
			{"key", "TEXT", "PRIMARY KEY"},
			{"value", "INTEGER", "NOT NULL"},
			{"ram_used_mb", "REAL", ""},
			{"cpu_percent", "REAL", ""},
			{"process_memory_mb", "REAL", ""},
			{"load_avg", "TEXT", ""},
			{"disk_usage", "TEXT", ""},
			{"disk_io", "TEXT", ""},
			{"network_traffic", "TEXT", ""},
			{"thread_count", "INTEGER", ""},
			{"pid", "INTEGER", ""},
			{"exec_path", "TEXT", ""},
			{"nick", "TEXT", ""},
			{"version", "TEXT", "NOT NULL"},
			{"go_version", "TEXT", "NOT NULL"},
			{"bot_uptime", "TEXT", "NOT NULL"},
			{"bot_max_uptime", "TEXT", "NOT NULL DEFAULT '0s'"},
			{"bot_max_connect_time", "TEXT", "NOT NULL DEFAULT '0s'"},
			{"server_uptime", "TEXT", "NOT NULL"},
			{"channels", "TEXT", "NOT NULL"},
			{"server", "TEXT", "NOT NULL"},
			{"connected", "INTEGER", "NOT NULL"},
			{"last_updated", "DATETIME", "NOT NULL"},
			{"total_users", "INTEGER", "NOT NULL DEFAULT 0"},
			{"owner", "TEXT", "NOT NULL DEFAULT ''"},
			{"globaladmins", "TEXT", "NOT NULL DEFAULT '[]'"},
			{"globalmods", "TEXT", "NOT NULL DEFAULT '[]'"},
			{"globalvips", "TEXT", "NOT NULL DEFAULT '[]'"},
			{"admins", "TEXT", "NOT NULL DEFAULT '[]'"},
			{"mods", "TEXT", "NOT NULL DEFAULT '[]'"},
			{"vips", "TEXT", "NOT NULL DEFAULT '[]'"},
		},
		"users": {
			{"nick", "TEXT", "NOT NULL"},
			{"hostmask", "TEXT", "NOT NULL"},
			{"role", "TEXT", "NOT NULL DEFAULT 'user'"},
			{"added_by", "TEXT", "NOT NULL DEFAULT 'YnM-Go'"},
			{"lang", "TEXT", "NOT NULL DEFAULT 'en'"},
			{"mychar", "TEXT", ""},
			{"welcome", "TEXT", ""},
			{"website", "TEXT", ""},
			{"pass", "TEXT", ""},
			{"email", "TEXT", ""},
			{"discord_id", "TEXT", ""},
			{"telegram_id", "TEXT", ""},
			{"facebook", "TEXT", ""},
			{"avatar_url", "TEXT", ""},
			{"avatar_type", "TEXT", "DEFAULT 'initials'"},
			{"invites", "INTEGER", "DEFAULT 0"},
			{"password_expires", "DATETIME", ""},
			{"password_uses", "INTEGER", "DEFAULT 0"},
			{"password_max_uses", "INTEGER", "DEFAULT 10"},
			{"password_last_used", "DATETIME", ""},
			{"password_created", "DATETIME", ""},
			{"created_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
			{"last_login", "DATETIME", ""},
			{"updated_at", "DATETIME", ""},
		},	
		"forget_pass_logs": {
			{"nick", "TEXT", "NOT NULL"},
			{"created_at", "DATETIME", "NOT NULL DEFAULT CURRENT_TIMESTAMP"},		
		},
		"channels": {
			{"name", "TEXT", "NOT NULL"},
			{"auto_op", "BOOLEAN", "DEFAULT 0"},
			{"auto_voice", "BOOLEAN", "DEFAULT 0"},
			{"auto_halfop", "BOOLEAN", "DEFAULT 0"},
			{"owner", "TEXT", ""},
			{"owner_hostmask", "TEXT", ""},
			{"current_topic", "TEXT", ""},
			{"topic_set_by", "TEXT", ""},
			{"topic_set_at", "DATETIME", ""},
			{"current_modes", "TEXT", "DEFAULT '+nt'"},
			{"modes_set_by", "TEXT", "DEFAULT 'YnM-Go'"},
			{"modes_set_at", "DATETIME", ""},
			{"created_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
		},
		"channel_users": {
			{"nick", "TEXT", "NOT NULL"},
			{"hostmask", "TEXT", "NOT NULL"},
			{"channel", "TEXT", "NOT NULL"},
			{"role", "TEXT", "NOT NULL DEFAULT ''"},
			{"auto_op", "BOOLEAN", "DEFAULT 0"},
			{"auto_voice", "BOOLEAN", "DEFAULT 0"},
			{"auto_halfop", "BOOLEAN", "DEFAULT 0"},
			{"added_by", "TEXT", "NOT NULL DEFAULT 'YnM-Go'"},
			{"added_by_host", "TEXT", "NOT NULL DEFAULT 'YnM-Go'"},
			{"created_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
			{"modified_by", "TEXT", "NOT NULL DEFAULT 'YnM-Go'"},
			{"modified_by_host", "TEXT", "NOT NULL DEFAULT 'YnM-Go'"},
			{"changed_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
		},
		"channel_modes": {
			{"channel", "TEXT", "NOT NULL"},
			{"modes", "TEXT", "NOT NULL"},
			{"mode", "TEXT", "NOT NULL DEFAULT ''"},
			{"mode_params", "TEXT", ""},
			{"enabled", "BOOLEAN", "NOT NULL DEFAULT 1"},
			{"set_by", "TEXT", "NOT NULL"},
			{"set_by_host", "TEXT", "NOT NULL DEFAULT 'YnM-Go'"},
			{"created_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
			{"updated_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
			{"active", "BOOLEAN", "DEFAULT 1"},
		},
		"channel_bans": {
			{"channel", "TEXT", "NOT NULL"},
			{"mask", "TEXT", "NOT NULL"},
			{"set_by", "TEXT", "NOT NULL"},
			{"set_by_host", "TEXT", "NOT NULL DEFAULT 'YnM-Go'"},
			{"reason", "TEXT", ""},
			{"created_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
			{"expires_at", "DATETIME", ""},
			{"active", "BOOLEAN", "DEFAULT 1"},
		},
		"channel_mode_history": {
			{"channel", "TEXT", "NOT NULL"},
			{"modes", "TEXT", "NOT NULL"},
			{"set_by", "TEXT", "NOT NULL"},
			{"set_by_host", "TEXT", "NOT NULL DEFAULT 'YnM-Go'"},
			{"created_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
		},
		"bot_logs": {
			{"username", "TEXT", "NOT NULL"},
			{"action", "TEXT", "NOT NULL"},
			{"hostmask", "TEXT", "NOT NULL"},
			{"details", "TEXT", ""},
			{"channel", "TEXT", ""},
			{"command", "TEXT", ""},
			{"timestamp", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
		},
		"web_sessions": {
			{"token", "TEXT", "PRIMARY KEY"},
			{"username", "TEXT", "NOT NULL"},
			{"created_at", "DATETIME", "NOT NULL DEFAULT CURRENT_TIMESTAMP"},
			{"expires_at", "DATETIME", "NOT NULL"},
			{"ip_address", "TEXT", ""},
			{"user_agent", "TEXT", ""},
		},
		"web_logs": {
			{"username", "TEXT", "NOT NULL"},
			{"action", "TEXT", "NOT NULL"},
			{"ip_address", "TEXT", ""},
			{"timestamp", "DATETIME", "NOT NULL DEFAULT CURRENT_TIMESTAMP"},
			{"details", "TEXT", ""},
		},
		"plugins": {
			{"name", "TEXT", "NOT NULL"},
			{"description", "TEXT", ""},
			{"created_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
		},
		"plugin_states": {
			{"plugin_name", "TEXT", "NOT NULL"},
			{"channel", "TEXT", "NOT NULL"},
			{"is_active", "BOOLEAN", "NOT NULL DEFAULT 0"},
			{"updated_at", "DATETIME", "DEFAULT CURRENT_TIMESTAMP"},
		},
	}

	for table, cols := range tables {
		existingCols, err := s.getColumns(table)
		if err != nil {
			return fmt.Errorf("error getting columns for %s: %v", table, err)
		}

		for _, col := range cols {
			if _, ok := existingCols[col.name]; !ok {
				sqlStmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s %s", table, col.name, col.typ, col.def)
				_, err := s.db.Exec(sqlStmt)
				if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
					fmt.Printf("[SchemaChecker] Error adding column %s to %s: %v\n", col.name, table, err)
				} else {
					fmt.Printf("[SchemaChecker] Added missing column %s to %s\n", col.name, table)
				}
			}
		}
	}

	return nil
}
func (s *SchemaChecker) VerifySchema() ([]string, error) {
	issues := []string{}
	
	tables := getExpectedSchema()
	
	for tableName, expectedCols := range tables {
		// Ellenőrzi, hogy létezik-e a tábla
		var count int
		err := s.db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", 
			tableName,
		).Scan(&count)
		
		if err != nil {
			return nil, fmt.Errorf("error checking table %s: %v", tableName, err)
		}
		
		if count == 0 {
			issues = append(issues, fmt.Sprintf("Missing table: %s", tableName))
			continue
		}
		
		// Ellenőrzi az oszlopokat
		existingCols, err := s.getColumns(tableName)
		if err != nil {
			return nil, err
		}
		
		for _, col := range expectedCols {
			if _, ok := existingCols[col.name]; !ok {
				issues = append(issues, 
					fmt.Sprintf("Missing column: %s.%s", tableName, col.name))
			}
		}
	}
	
	return issues, nil
}

func (s *SchemaChecker) getColumns(table string) (map[string]struct{}, error) {
	rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols := make(map[string]struct{})
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols[name] = struct{}{}
	}
	return cols, nil
}
func (a *AdminDB) StartMaintenanceJob(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	
	go func() {
		for {
			select {
			case <-ticker.C:
				// Régi session-ök törlése
				a.db.Exec("DELETE FROM web_sessions WHERE expires_at < datetime('now')")
				
				// Inaktív banok törlése
				a.db.Exec(`UPDATE channel_bans 
					SET active = 0 
					WHERE expires_at IS NOT NULL 
					AND expires_at < datetime('now') 
					AND active = 1`)
				
				// Régi backupok tisztítása (30 nap)
				a.CleanOldBackups(30)
				
				fmt.Println("[Maintenance] Daily cleanup completed")
				
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}
func (a *AdminDB) OptimizeDatabase() error {
	fmt.Println("[Maintenance] Running VACUUM...")
	if _, err := a.db.Exec("VACUUM"); err != nil {
		return fmt.Errorf("VACUUM failed: %v", err)
	}
	
	fmt.Println("[Maintenance] Running ANALYZE...")
	if _, err := a.db.Exec("ANALYZE"); err != nil {
		return fmt.Errorf("ANALYZE failed: %v", err)
	}
	
	fmt.Println("[Maintenance] Optimization complete")
	return nil
}
func (a *AdminDB) CheckIntegrity() (bool, error) {
	var result string
	err := a.db.QueryRow("PRAGMA integrity_check").Scan(&result)
	if err != nil {
		return false, err
	}
	
	if result == "ok" {
		fmt.Println("[Integrity] Database integrity: OK")
		return true, nil
	}
	
	fmt.Printf("[Integrity] Database integrity issues: %s\n", result)
	return false, nil
}
func (a *AdminDB) GetDatabaseStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	
	// Tábla méretetek
	tables := []string{
		"users", "channels", "channel_users", "channel_modes",
		"channel_bans", "channel_mode_history", "bot_logs", "web_sessions", "web_logs", "forget_pass_logs", "bot_stats", "plugins", "plugin_states",
	}
	
	tableCounts := make(map[string]int)
	for _, table := range tables {
		var count int
		err := a.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err != nil {
			tableCounts[table] = -1
		} else {
			tableCounts[table] = count
		}
	}
	stats["table_counts"] = tableCounts
	
	// Adatbázis fájl mérete
	dbPath := filepath.Join("data", "ynm.db")
	if info, err := os.Stat(dbPath); err == nil {
		stats["db_size_mb"] = float64(info.Size()) / (1024 * 1024)
	}
	
	// WAL fájl mérete
	walPath := dbPath + "-wal"
	if info, err := os.Stat(walPath); err == nil {
		stats["wal_size_mb"] = float64(info.Size()) / (1024 * 1024)
	}
	
	return stats, nil
}
func (a *AdminDB) WithTransaction(fn func(*sql.Tx) error) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()
	
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	
	return tx.Commit()
}

// Használat példa:
// err := db.WithTransaction(func(tx *sql.Tx) error {
//     _, err := tx.Exec("INSERT INTO users ...")
//     if err != nil {
//         return err
//     }
//     _, err = tx.Exec("INSERT INTO bot_logs ...")
//     return err
// })
func (a *AdminDB) RunMigrations() error {
	// Migration verzió tábla létrehozása
	_, err := a.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return err
	}
	
	migrations := []Migration{
		// Példa migration
		{
			Version:     1,
			Description: "Add user_agent to web_logs",
			Up: func(db *sql.DB) error {
				_, err := db.Exec("ALTER TABLE web_logs ADD COLUMN user_agent TEXT")
				return err
			},
			Down: func(db *sql.DB) error {
				// SQLite nem támogatja a DROP COLUMN-t könnyen
				return nil
			},
		},
	}
	
	for _, m := range migrations {
		var count int
		err := a.db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", 
			m.Version).Scan(&count)
		
		if err != nil || count > 0 {
			continue
		}
		
		fmt.Printf("[Migration] Running: %s\n", m.Description)
		if err := m.Up(a.db); err != nil {
			return fmt.Errorf("migration %d failed: %v", m.Version, err)
		}
		
		_, err = a.db.Exec("INSERT INTO schema_migrations (version) VALUES (?)", m.Version)
		if err != nil {
			return err
		}
	}
	
	return nil
}
func (a *AdminDB) BackupDatabase() error {
	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join("data", "backups", fmt.Sprintf("ynm_%s.db", timestamp))
	
	if err := os.MkdirAll(filepath.Join("data", "backups"), 0755); err != nil {
		return fmt.Errorf("couldn't create backup dir: %v", err)
	}
	
	// SQLite backup parancs
	_, err := a.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", backupPath))
	if err != nil {
		return fmt.Errorf("backup failed: %v", err)
	}
	
	fmt.Printf("[Backup] Database backed up to: %s\n", backupPath)
	return nil
}

// CleanOldBackups - régi backupok törlése (pl. 30 napnál régebbiek)
func (a *AdminDB) CleanOldBackups(daysToKeep int) error {
	backupDir := filepath.Join("data", "backups")
	
	files, err := os.ReadDir(backupDir)
	if err != nil {
		return err
	}
	
	cutoffTime := time.Now().AddDate(0, 0, -daysToKeep)
	
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "ynm_") {
			info, err := file.Info()
			if err != nil {
				continue
			}
			
			if info.ModTime().Before(cutoffTime) {
				path := filepath.Join(backupDir, file.Name())
				if err := os.Remove(path); err != nil {
					fmt.Printf("Warning: couldn't delete old backup %s: %v\n", file.Name(), err)
				} else {
					fmt.Printf("[Cleanup] Deleted old backup: %s\n", file.Name())
				}
			}
		}
	}
	
	return nil
}