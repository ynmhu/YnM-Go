package plugins

import (
    "fmt"
    "strings"
    "sync"
	"time"
    "github.com/ynmhu/YnM-Go/irc"
)

type AdminPlugin struct {
    mu           sync.Mutex
    userBanUntil map[string]time.Time
    userRequestTimes map[string][]time.Time
    userBanNotified map[string]bool
}

func NewAdminPlugin() *AdminPlugin {
    return &AdminPlugin{
        userBanUntil:     make(map[string]time.Time),
        userRequestTimes: make(map[string][]time.Time),
        userBanNotified:  make(map[string]bool),
    }
}

func (p *AdminPlugin) HandleMessage(msg irc.Message) string {
    cmd := strings.TrimSpace(strings.ToLower(msg.Text))

    if strings.HasPrefix(cmd, "!clear ") {
        parts := strings.SplitN(cmd, " ", 2)
        if len(parts) == 2 {
            targetUser := strings.ToLower(strings.TrimSpace(parts[1]))

            p.mu.Lock()
            defer p.mu.Unlock()

            delete(p.userBanUntil, targetUser)
            delete(p.userRequestTimes, targetUser)
            delete(p.userBanNotified, targetUser)

            return fmt.Sprintf("Tiltás és korlátozás törölve %s felhasználóra.", targetUser)
        }
    }

    return ""
}


func (p *AdminPlugin) OnTick() []irc.Message {
    return nil
}
