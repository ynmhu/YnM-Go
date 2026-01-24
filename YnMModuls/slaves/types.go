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

package main

import (
	"net"
	"sync"
	"time"
	"strings"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMCmd"
)

type SlaveConfig struct {
    Name     string   `json:"name"`
    Server   string   `json:"server"`
    Port     int      `json:"port"`
    SSL      bool     `json:"ssl"`
    Nickname string   `json:"nickname"`
    Username string   `json:"username"`
    Realname string   `json:"realname"`
    Channels []string `json:"channels"`
    TopicChannel        string `json:"topic_channel"`         // "#ynm"
    TopicUpdateInterval string `json:"topic_update_interval"` // "12h"
}

// MasterMessage - Master és Slave közötti üzenet formátum
type MasterMessage struct {
	Type     string      `json:"type"`
	BotName  string      `json:"bot_name,omitempty"`
	Channel  string      `json:"channel,omitempty"`
	User     string      `json:"user,omitempty"`
	Hostmask string      `json:"hostmask,omitempty"`
	Message  string      `json:"message,omitempty"`
	Command  string      `json:"command,omitempty"`
	Reply    string      `json:"reply,omitempty"`
	Source   string      `json:"source,omitempty"` // "master" vagy "slave-BotName"
	Data     interface{} `json:"data,omitempty"`   // ÚJ: Session adatok
	Action   string      `json:"action,omitempty"` // ÚJ: Session műveletek
}

// SlaveBot - Slave bot fő struktúra
type SlaveBot struct {
    config         SlaveConfig
    ircClient      *YnMIrC.Client
    masterConn     net.Conn
    socketPath     string
    running        bool
    reconnecting   bool
    standalone     bool
    handlerMutex   sync.Mutex
    handlerRunning bool
    sessionMutex   sync.RWMutex
    sessions       map[string]*UserSession
    startTime      time.Time
    topicUpdater   *YnMCmd.TopicUpdaterPlugin
}

// UserSession - Felhasználói session
type UserSession struct {
	Username  string                 `json:"username"`
	Hostmask  string                 `json:"hostmask"`
	LoggedIn  bool                   `json:"logged_in"`
	LoginTime time.Time              `json:"login_time"`
	LastSeen  time.Time              `json:"last_seen"`
	Data      map[string]interface{} `json:"data"` // Egyéb session adatok
}

// SessionRequest - Session kérés a master felé
type SessionRequest struct {
	Type     string      `json:"type"`
	Action   string      `json:"action"`   // "get", "set", "delete", "check", "login"
	BotName  string      `json:"bot_name"`
	Hostmask string      `json:"hostmask"`
	Channel  string      `json:"channel,omitempty"`
	User     string      `json:"user,omitempty"`
	Data     interface{} `json:"data,omitempty"`
}

// SessionResponse - Session válasz a mastertől
type SessionResponse struct {
	Type     string      `json:"type"`
	Action   string      `json:"action"`
	BotName  string      `json:"bot_name"`
	Hostmask string      `json:"hostmask"`
	Success  bool        `json:"success"`
	Session  *UserSession `json:"session,omitempty"`
	Message  string      `json:"message,omitempty"`
}

// NewSlaveBot - Új slave bot létrehozása
func NewSlaveBot(config SlaveConfig, socketPath string) *SlaveBot {
    // ✅ Normalizálás AZONNAL a konstruktorban
    for i, ch := range config.Channels {
        config.Channels[i] = strings.ToLower(ch)
    }
    
    if config.TopicChannel != "" {
        config.TopicChannel = strings.ToLower(config.TopicChannel)
    }
    
    return &SlaveBot{
        config:     config,
        socketPath: socketPath,
        running:    true,
        sessions:   make(map[string]*UserSession),
        startTime:  time.Now(),
    }
}


// Helper functions for session management

// GetSession - Session lekérése lokális cache-ből
func (sb *SlaveBot) GetSession(hostmask string) *UserSession {
	sb.sessionMutex.RLock()
	defer sb.sessionMutex.RUnlock()
	
	if session, exists := sb.sessions[hostmask]; exists {
		return session
	}
	return nil
}

// SetSession - Session beállítása lokális cache-ben
func (sb *SlaveBot) SetSession(hostmask string, session *UserSession) {
	sb.sessionMutex.Lock()
	defer sb.sessionMutex.Unlock()
	
	sb.sessions[hostmask] = session
}

// DeleteSession - Session törlése lokális cache-ből
func (sb *SlaveBot) DeleteSession(hostmask string) {
	sb.sessionMutex.Lock()
	defer sb.sessionMutex.Unlock()
	
	delete(sb.sessions, hostmask)
}

// IsUserLoggedIn - Ellenőrzi, hogy a felhasználó be van-e jelentkezve
func (sb *SlaveBot) IsUserLoggedIn(hostmask string) bool {
	session := sb.GetSession(hostmask)
	return session != nil && session.LoggedIn
}

// UpdateLastSeen - Frissíti a session last_seen mezőjét
func (sb *SlaveBot) UpdateLastSeen(hostmask string) {
	session := sb.GetSession(hostmask)
	if session != nil {
		session.LastSeen = time.Now()
		sb.SetSession(hostmask, session)
	}
}