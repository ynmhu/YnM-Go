// ============================================================================
//  Szerzői jog © 2024 Markus (markus@ynm.hu)
//  https://ynm.hu   – főoldal
//  https://forum.ynm.hu   – hivatalos fórum
//  https://bot.ynm.hu     – bot oldala és dokumentáció
//
//  Minden jog fenntartva. A kód Markus tulajdona, tilos terjeszteni vagy
//  módosítani a szerző írásos engedélye nélkül.
//
//  Ez a fájl a YnM-Go IRC-bot rendszerének része.
// ============================================================================


package plugins

import (
	"github.com/ynmhu/YnM-Go/irc"

)

type ScheduledMessage = irc.Message

type Plugin interface {
    HandleMessage(msg irc.Message) string
    OnTick() []irc.Message  // Add this required method
}

type Manager struct {
	plugins []Plugin
}

func NewManager() *Manager {
	return &Manager{
		plugins: make([]Plugin, 0),
	}
}

func (m *Manager) Register(plugin Plugin) {
	m.plugins = append(m.plugins, plugin)
}

func (m *Manager) HandleMessage(msg irc.Message) string {
	for _, plugin := range m.plugins {
		if response := plugin.HandleMessage(msg); response != "" {
			return response
		}
	}
	return ""
}