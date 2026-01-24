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

package YnMIrC

import (
	"net"
	"sync"
	"time"
//	"fmt"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"database/sql"
	
)

// ──────────────────────── Típusok ────────────────────────────

// Message represents an incoming IRC message
type Message struct {
    Sender      string
    Nick        string
    Hostmask    string
    Channel     string
    Text        string
    Command     string
    IsCommand   bool   
    CommandText string 
    Params      []string
    Time        time.Time
}

// WhoisData contains information from a WHOIS response
type WhoisData struct {
	Nick     string
	Username string
	Hostname string
	Realname string
	Hostmask string   // nick!user@host format
	
	// Channel információk
	Channels []string  // channels the user is in
	
	// Server információk
	Server     string  // server the user is connected to
	ServerInfo string  // server description/info
	
	// Időzítés
	Idle       int     // idle time in seconds
	SignonTime int64   // Unix timestamp when user connected
	
	// Account/Auth (IRC services)
	Account    string  // NickServ account name (RPL_WHOISACCOUNT - 330)
	IsLoggedIn bool    // whether user is identified
	
	// Kapcsolat információk
	IPAddress  string  // IP address (if visible, some servers show this)
	IsSecure   bool    // using TLS/SSL connection
	
	// Operator/Mode információk
	IsOper     bool    // IRC operator
	IsBot      bool    // marked as bot
	IsAway     bool    // away status
	AwayMsg    string  // away message
	
	// Extra információk (szerver-specifikus)
	Modes      string  // user modes (+i, +x, etc)
	Extra      map[string]string // egyéb szerver-specifikus adatok
	
	// Metaadatok
	Raw        []string // raw WHOIS response lines (debugging)
}

// Az irc.Client-hez adjuk hozzá ezt az interfészt:
type JoinHandler interface {
    OnJoin(channel, nick, hostmask string)
}

// ChannelUser represents a user in a channel with their modes
type ChannelUser struct {
	Nick     string
	Hostmask string
	Modes    string // "ov" for op+voice, "o" for just op, etc.
	JoinTime time.Time
}

// Channel represents an IRC channel
type Channel struct {
	Name       string
	Topic      string
	TopicBy    string
	TopicTime  time.Time
	Users      map[string]*ChannelUser
	Modes      string
	Key        string // channel key if any
	Limit      int    // user limit if any
	CreatedAt  time.Time
	mu         sync.RWMutex
}

// NewChannel creates a new Channel instance
func NewChannel(name string) *Channel {
	return &Channel{
		Name:      name,
		Users:     make(map[string]*ChannelUser),
		CreatedAt: time.Now(),
	}
}

// AddUser adds a user to the channel
func (ch *Channel) AddUser(nick, hostmask, modes string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	
	ch.Users[nick] = &ChannelUser{
		Nick:     nick,
		Hostmask: hostmask,
		Modes:    modes,
		JoinTime: time.Now(),
	}
}

// RemoveUser removes a user from the channel
func (ch *Channel) RemoveUser(nick string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	delete(ch.Users, nick)
}

// GetUser gets a user from the channel
func (ch *Channel) GetUser(nick string) (*ChannelUser, bool) {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	user, exists := ch.Users[nick]
	return user, exists
}

// UpdateUserModes updates a user's modes in the channel
func (ch *Channel) UpdateUserModes(nick, modes string, add bool) {
    ch.mu.Lock()
    defer ch.mu.Unlock()
    
    user, exists := ch.Users[nick]
    if !exists {
        return
    }
    
    if add {
        // Add modes that don't exist
        for _, mode := range modes {
            if !containsRune(user.Modes, mode) {
                user.Modes += string(mode)
            }
        }
    } else {
        // Remove modes
        newModes := ""
        for _, existingMode := range user.Modes {
            if !containsRune(modes, existingMode) {
                newModes += string(existingMode)
            } else {
            }
        }
        user.Modes = newModes
    } 
}
// GetUserCount returns the number of users in the channel
func (ch *Channel) GetUserCount() int {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return len(ch.Users)
}

// GetUserList returns a slice of all user nicks
func (ch *Channel) GetUserList() []string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	
	users := make([]string, 0, len(ch.Users))
	for nick := range ch.Users {
		users = append(users, nick)
	}
	return users
}

// HasUser checks if a user is in the channel
func (ch *Channel) HasUser(nick string) bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	_, exists := ch.Users[nick]
	return exists
}

// SetTopic sets the channel topic
func (ch *Channel) SetTopic(topic, setBy string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.Topic = topic
	ch.TopicBy = setBy
	ch.TopicTime = time.Now()
}

// Client represents the main IRC client structure
type Client struct {
	conn            net.Conn
	config          *YnMConfig.Config
	OnConnect       func()
	OnMessage       func(Message)
	OnJoin          func(channel, nick, hostmask string)
	OnPart          func(channel, nick, reason string)
	OnQuit          func(nick, reason string)
	OnKick          func(channel, kicked, kicker, reason string)
	OnNickChange    func(oldNick, newNick string)
	OnTopic         func(channel, topic, setBy string)
	OnMode          func(channel, modes, args string, setBy string)
	OnPong          func(pongID string)
	OnLoginFailed   func(reason string)
	OnLoginSuccess  func()
	OnChannelJoined func(channel string)
	OnChannelParted func(channel string)
	OnNames        func(channel string, names []string)
	OnWho 			func(channel, nick, hostmask string) 
	OnEndOfWho func(channel string)
	
	mu              sync.RWMutex
	connected       bool
	disconnectChan  chan struct{}
	reconnecting    bool
	loggedIn        bool
	
	// Undernet 
	undernetLoginSent bool
    undernetLoggedIn  bool
	
	// Enhanced channel and user tracking
	channels       map[string]*Channel
	nick           string
	channelModeHandler ChannelModeHandler
	
	// SASL settings
	useSASL  bool
	saslUser string
	saslPass string
	
	// Message sending queue (optimization)
	sendQueue chan string
	sendDone  chan struct{}
	
	// WHOIS handling
	whoisData     map[string]*WhoisData
	whoisMutex    sync.Mutex
	whoisChannels map[string][]chan *WhoisData  
	
	// Bot prefix cache
	whoBotPrefixMu sync.RWMutex
	whoBotPrefix   map[string]string

	// Rate limiting
	lastMessageTime time.Time
	messageDelay    time.Duration
	lastMessage    time.Time
	messageMutex     sync.Mutex
	startedAt       time.Time
	plugins []interface{}
    pluginMu sync.RWMutex
	db *sql.DB
	
	// ISUPPORT PREFIX parsing (005 PREFIX=...)
    prefixToMode map[rune]rune
    modeToPrefix map[rune]rune
    prefixMu     sync.RWMutex
    
    // ✅ WHO RESPONSE CACHE
    whoResponseCache map[string]time.Time  // channel -> last WHO time
    whoCacheMutex    sync.RWMutex
    
    // ✅ STARTUP WHO TRACKING
    startupWhoSent   bool
    startupWhoMutex  sync.Mutex
}

// IRCClient interface for better testability and abstraction
type IRCClient interface {
	Connect() error
	Disconnect()
	SendMessage(target, text string)
	SendRaw(msg string) error
	Join(channel string)
	Part(channel string, reason string)
	GetNick() string
	GetChannels() []string
	GetChannelUsers(channel string) []string
	GetUserModes(channel, nick string) string
	GetChannelUserHostmask(channel, nick string) string
	GetWhoisData(nick string) *WhoisData
	IsConnected() bool
	IsLoggedIn() bool
	IsChannel(name string) bool
	Config() *YnMConfig.Config
}

// Helper function
func containsRune(s string, r rune) bool {
	for _, char := range s {
		if char == r {
			return true
		}
	}
	return false
}

func (c *Client) AddPlugin(plugin interface{}) {
    c.pluginMu.Lock()
    defer c.pluginMu.Unlock()
    c.plugins = append(c.plugins, plugin)
}

type ChannelModeHandler interface {
    HandleModeChange(channel, setter, modes string, args []string)
    GetSavedModes(channel string) (string, error)
}


func (c *Client) SetChannelModeHandler(h ChannelModeHandler) {
    c.channelModeHandler = h
}