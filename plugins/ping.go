package plugins

import (
 "YnM-Go/irc"
	"strings"
)

type PingPlugin struct{}

func (p *PingPlugin) HandleMessage(msg irc.Message) string {
    if strings.ToLower(msg.Text) == "!ping" {
        return "Pong!"
    }
    return ""
}

func (p *PingPlugin) OnTick() []irc.Message {
    return nil  // No scheduled messages for ping plugin
}