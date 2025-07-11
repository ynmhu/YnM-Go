package plugins

import (
	"github.com/ynmhu/YnM-Go/irc"
)

type ScheduledMessage = irc.Message

type Plugin interface {
	HandleMessage(msg irc.Message) string
	OnTick() []irc.Message
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

// Új metódus: plugin lista lekérése (pl. OnTick hívásokhoz)
func (m *Manager) GetPlugins() []Plugin {
	return m.plugins
}