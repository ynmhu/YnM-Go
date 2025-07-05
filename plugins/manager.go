package plugins

import (
	"YnM-Go/irc"

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
	return &Manager{}
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