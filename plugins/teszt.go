package plugins

import (
	"strings"
	"github.com/ynmhu/YnM-Go/irc"
)

// TestPlugin egy egyszerű példa plugin különféle jogosultságokkal

type TestPlugin struct {
    admin *AdminPlugin
}

// Konstruktor
func NewTestPlugin(admin *AdminPlugin) *TestPlugin {
    return &TestPlugin{admin: admin}
}



// HandleMessage a bejövő üzeneteket kezeli
func (p *TestPlugin) HandleMessage(msg irc.Message) string {
    nick := strings.Split(msg.Sender, "!")[0]
    hostmask := msg.Sender
    level := p.admin.store.GetAdminLevel(nick, hostmask)

    switch strings.TrimSpace(msg.Text) {
    case "!teszt":
        if level == AdminLevelOwner {
            return "Teszt parancs - csak Owner "
        }
        return ""

    case "!teszt1":
        if level >= AdminLevelAdmin {
            return "Teszt1 parancs - Owner és Admin"
        }
        return ""

    case "!teszt2":
        if level >= AdminLevelVIP {
            return "Teszt2 parancs - Owner, Admin, VIP "
        }
        return ""

    case "!teszt4":
        return "Teszt4 parancs - mindeki "
    }

    return ""
}

func isAdmin(sender string) bool {
	// TODO: ellenőrizd, hogy sender Admin-e
	return false
}

func isVIP(sender string) bool {
	// TODO: ellenőrizd, hogy sender VIP-e
	return false
}


func (p *TestPlugin) OnTick() []irc.Message {
    return nil
}