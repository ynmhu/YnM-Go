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

package discord

import (
  //  "log"
	"strings"
    "github.com/bwmarrin/discordgo"
    "git.ynm.hu/markus/YnM-Go/YnMIrC" 
)

// ============ Discord Adapter ============

type DiscordAdapter struct {
    Token          string
    Session        *discordgo.Session
    MessageHandler func(channelID, authorID, content string)
}

func (d *DiscordAdapter) Start() error {
    dg, err := discordgo.New("Bot " + d.Token)
    if err != nil {
        return err
    }
    
    d.Session = dg
    
    // Intents beállítása
    dg.Identify.Intents = discordgo.IntentsGuildMessages | 
                          discordgo.IntentsGuilds |
                          discordgo.IntentsMessageContent
    
    // Ready event
    dg.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
      //  log.Printf("✅ Bejelentkezve: %s#%s", s.State.User.Username, s.State.User.Discriminator)
        //log.Printf("🎮 %d szerveren", len(r.Guilds))
    })
    
    // Message handler
    dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
        // Ignore saját üzenetek
        if m.Author.ID == s.State.User.ID {
            return
        }
        
        if d.MessageHandler != nil {
            d.MessageHandler(m.ChannelID, m.Author.ID, m.Content)
        }
    })
    
    // Kapcsolódás
    err = dg.Open()
    if err != nil {
        return err
    }
    
    return nil
}

func (d *DiscordAdapter) SendMessage(channelID, message string) error {
    if d.Session != nil {
        _, err := d.Session.ChannelMessageSend(channelID, message)
        return err
    }
    return nil
}

func (d *DiscordAdapter) Stop() {
    if d.Session != nil {
        d.Session.Close()
    }
}

// ============ Discord Plugin ============

// MessageInterface definiálja az üzenet struktúrát
type MessageInterface interface {
    GetNick() string
    GetChannel() string
    GetText() string
}

// PluginManagerInterface definiálja a plugin manager metódusokat
type PluginManagerInterface interface {
    HandleMessage(msg YnMIrC.Message) string
}

// DiscordPlugin - a Discord plugin implementációja
type DiscordPlugin struct {
    Adapter *DiscordAdapter
    manager PluginManagerInterface
}


// YnMMessage wrapper
type YnMMessageWrapper struct {
    Nick    string
    Channel string
    Text    string
}

func (m YnMMessageWrapper) GetNick() string {
    return m.Nick
}

func (m YnMMessageWrapper) GetChannel() string {
    return m.Channel
}

func (m YnMMessageWrapper) GetText() string {
    return m.Text
}

func NewDiscordPlugin(token string, pm PluginManagerInterface) *DiscordPlugin {
    plugin := &DiscordPlugin{
        manager: pm,
    }
    
   plugin.Adapter = &DiscordAdapter{
    Token: token,
    MessageHandler: func(channelID, authorID, content string) {
        //log.Printf("[Discord→IRC] Channel: %s | User: %s | Msg: %s",
          //  channelID, authorID, content)

        // Csinálunk egy YnMIrC.Message-et
        msg := YnMIrC.Message{
            Nick:    authorID,
            Channel: channelID,
            Text:    content,
            //Network: "discord",
        }

        // Átküldjük a plugin managernek
        response := pm.HandleMessage(msg)

        // ✨ Többsoros válaszkezelés
        if response != "" {
            if strings.Contains(response, "\n") {
                for _, line := range strings.Split(response, "\n") {
                    if strings.TrimSpace(line) != "" {
                        plugin.Adapter.SendMessage(channelID, line)
                    }
                }
            } else {
                plugin.Adapter.SendMessage(channelID, response)
            }
        }
    },

    }
    
    return plugin
}

func (dp *DiscordPlugin) Name() string {
    return "Discord"
}

func (dp *DiscordPlugin) Start() error {
    return dp.Adapter.Start()
}

func (dp *DiscordPlugin) Stop() error {
    dp.Adapter.Stop()
    return nil
}
func (dp *DiscordPlugin) Close() {
    dp.Stop()
}

func (dp *DiscordPlugin) IsEnabled() bool {
    return true
}

// HandleMessage - implements the Plugin interface (IRC -> Discord)
func (dp *DiscordPlugin) HandleMessage(msg YnMIrC.Message) string {
    // This would handle IRC messages going TO Discord
    // For now, we don't bridge IRC -> Discord, only Discord -> IRC
    return ""
}

// OnTick - implements the Plugin interface
func (dp *DiscordPlugin) OnTick() []YnMIrC.Message {
    // Discord plugin doesn't generate scheduled messages
    return nil
}