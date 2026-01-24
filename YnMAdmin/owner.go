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

package owner

import (
	"fmt"
	"strings"
	"sync"
	"time"
//	"log"
	_ "github.com/mattn/go-sqlite3"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMLang"
	"git.ynm.hu/markus/YnM-Go/YnMDb"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

type Session struct {
    OriginalHost    string    // Az eredeti hostmask (pl: Priv!~o@2zyce3647r4e4.ynm)
    LoggedInAs      string    // Kit jelentkezett be (pl: Markus)
    LoggedInHost    string    // A bejelentkezett user hostmask-ja (pl: *!*@markus.ynm.hu)
    LoginTime       time.Time
    LastActivity    time.Time
    IPAddress       string 
	SessionKey      string
}

type YnmAdminPlugin struct {
    Bot *YnMIrC.Client
    Db  *YnMDb.AdminDB
    Cfg *YnMConfig.Config
    mu            sync.RWMutex
    loggedInUsers map[string]string
    userModes     map[string]map[string]string
    sessions      map[string]*Session    
    hostSessions  map[string]string      
	sessionKeys   map[string]string  
	channelsMu        sync.Mutex
	channelsPending   map[string]bool
	channelsReplyTo   string
	channelsTimeoutID int64


}

// Constructor
func NewYnmAdminPlugin(cfg *YnMConfig.Config) (*YnmAdminPlugin, error) {
    db, err := YnMDb.NewAdminDB()
    if err != nil {
        return nil, fmt.Errorf("couldn't create admin DB: %v", err)
    }

    return &YnmAdminPlugin{
        Bot:              nil,
        Db:               db,
        Cfg:              cfg,
        loggedInUsers:    make(map[string]string),
        userModes:        make(map[string]map[string]string),
        sessions:         make(map[string]*Session),
        hostSessions:     make(map[string]string),
        sessionKeys:      make(map[string]string),
    }, nil
}

					
func (p *YnmAdminPlugin) Initialize(bot *YnMIrC.Client) {
    if bot == nil {
        return
    }
    p.Bot = bot

    // ✅ 315 END OF WHO callback bekötése (100%-os !ynm channels)
    p.Bot.OnEndOfWho = func(ch string) {
        p.onEndOfWho(ch)
    }

    // Database initialization
    db, err := YnMDb.NewAdminDB()
    if err != nil {
        return
    }
    p.Db = db
}

									  
func (p *YnmAdminPlugin) GetDB() *YnMDb.AdminDB {
    return p.Db
}

											   
func (p *YnmAdminPlugin) GetPrefixForHost(hostmask string) string {
	info, err := p.Db.GetUserInfoByHost(hostmask)
	if err != nil || info == nil {
		return "!"
	}
	if info.MyChar != nil && *info.MyChar != "" {
		return *info.MyChar
	}
	return "!"
}
										  
func nullToStr(s *string) string {
	if s == nil || *s == "" {
		return "<none>"
	}
	return *s
}


func (p *YnmAdminPlugin) HandleMessage(msg YnMIrC.Message) string {
	 if msg.Channel == "" || msg.Channel == "*" {
        return ""
    }
    if !strings.HasPrefix(msg.Channel, "#") && !strings.HasPrefix(msg.Channel, "&") && msg.Channel != p.Cfg.NickName {
        return ""
    }	
    if p.Bot == nil {
        return "Bot not initialized"
    }												  
    if len(msg.Text) == 0 {
        return ""
    }
    botNick := ""
    if p.Bot != nil {
        botNick = p.Bot.GetNick()
    }
    if botNick == "" {
        botNick = p.Cfg.NickName
    }

	firstChar := msg.Text[0]
	isCommandChar := firstChar == '!' || firstChar == '-' || firstChar == '.' || firstChar == '@'
	isBotNickCommand := strings.HasPrefix(msg.Text, botNick+" ") || msg.Text == botNick


	isValidCommand := false
	if isCommandChar {
		// Levágjuk a parancs karaktert és nézzük, maradt-e valami
		restOfText := strings.TrimSpace(msg.Text[1:])
		isValidCommand = len(restOfText) > 0
	}

	if !isValidCommand && !isBotNickCommand && strings.HasPrefix(msg.Channel, "#") {
		return ""
	}
    
											 
    fullHostmask := msg.Sender
	
																	   
    effectiveUser, effectiveHost := p.GetEffectiveUser(fullHostmask)
    _ = effectiveHost
    
						  
    originalNick := strings.Split(fullHostmask, "!")[0]
    if effectiveUser != originalNick {
        fmt.Printf("[DEBUG SessionActive] %s is logged in as %s\n", fullHostmask, effectiveUser)
    }
   																	 
    simplifiedHostmask := YnMModule.SimplifyHostmask(fullHostmask)
    hostmask := simplifiedHostmask
    prefix := p.GetPrefixForHost(hostmask)
	userRole, err := p.Db.GetUserRoleInChannel(msg.Nick, fullHostmask, msg.Channel)
	if err != nil {
		return ""
	}

	if userRole == "" {
		parts := strings.Fields(strings.TrimSpace(msg.Text))
		if len(parts) == 1 && strings.HasSuffix(parts[0], "ynm") && !p.Db.HasAnyowner() {
			return p.handleYnmCommand(msg.Sender, msg.Text)
		}
		return ""
	}

	userLevel := RoleHierarchy[strings.ToLower(userRole)]
	minLevel := RoleHierarchy["vip"]  // vagy bármilyen minimális role

	if userLevel < minLevel {
		return ""
	}

    nick := effectiveUser
    
    issuingChannel := ""
    if len(msg.Params) > 0 {
        issuingChannel = msg.Params[0]
    }

    // Mode parancsok kezelése
    if strings.HasPrefix(msg.Text, prefix+"mode ") {
        p.HandleModeCommand(nick, hostmask, issuingChannel, msg.Text)
        return ""
    }
							  
    if strings.HasPrefix(msg.Text, prefix+"clearmode") {
        p.HandleClearModeCommand(nick, hostmask, issuingChannel, msg.Text)
        return ""
    }
		if strings.HasPrefix(msg.Text, prefix) {
			parts := strings.Fields(msg.Text)
			cmd := strings.TrimPrefix(parts[0], prefix)
			args := parts[1:]

			switch cmd {
			case "o", "h", "v":
				    if cmd == "o" && userLevel < RoleHierarchy["mod"] {
						return "Csak adminok adhatnak op jogot"
					}
					// H parancs mod vagy magasabb
					if cmd == "h" && userLevel < RoleHierarchy["mod"] {
						return "Csak modok vagy magasabbak adhatnak halfop jogot"
					}
					// V parancs vip vagy magasabb
					if cmd == "v" && userLevel < RoleHierarchy["vip"] {
						return "Csak vip-ek vagy magasabbak adhatnak voice jogot"
					}

				targetChannel := issuingChannel
				targetNicks := args  // Kezdetben az összes arg
				
				if !strings.HasPrefix(msg.Channel, "#") {
					// Privát üzenet - keressük a csatornát
					for i, arg := range args {
						if strings.HasPrefix(arg, "#") {
							targetChannel = arg
							// Távolítsuk el a csatornát, a maradék nick lesz
							targetNicks = append(args[:i], args[i+1:]...)
							break
						}
					}
				}
				
				// Ha nincs target nick megadva, üres slice-ot adunk át
				// A SetChannelMode majd behelyettesíti a saját nicket
				return p.SetChannelMode(msg.Sender, targetChannel, cmd, targetNicks)

			case "t":
				    if userLevel < RoleHierarchy["vip"] {
						return "Csak modok vagy magasabbak változtathatják a topict"
					}
				targetChannel := issuingChannel
				topicText := strings.Join(args, " ")
				
				if !strings.HasPrefix(msg.Channel, "#") {
					// Privát üzenet - keressük a csatornát
					for i, arg := range args {
						if strings.HasPrefix(arg, "#") {
							targetChannel = arg
							// Topic a maradék szöveg
							topicText = strings.Join(append(args[:i], args[i+1:]...), " ")
							break
						}
					}
				}
				
				return p.SetChannelTopic(msg.Sender, targetChannel, topicText)
			}
		}
	
    // YnmStatus külön kezelése
					  

    if strings.HasPrefix(msg.Text, "!ynmstatus") {
        return "YnmAdminPlugin is loaded and operational"
    }

    
    // Parancs feldolgozása
	text := ""
	hasPrefix := false

    
    fmt.Printf("[DEBUG] botNick='%s', msg.Text='%s', prefix='%s'\n", botNick, msg.Text, prefix)
    
    prefixYnm := prefix + "ynm"
    if msg.Text == prefixYnm || strings.HasPrefix(msg.Text, prefixYnm+" ") {
        text = msg.Text[len(prefix):]
        hasPrefix = true
        fmt.Printf("[DEBUG] Matched prefixYnm, text='%s'\n", text)
    } else if strings.HasPrefix(msg.Text, botNick+" ") || msg.Text == botNick {
        text = msg.Text[len(botNick):]
        text = strings.TrimSpace(text)
        if !strings.HasPrefix(text, "ynm ") && text != "ynm" {
            text = "ynm " + text
        }
        hasPrefix = false
        fmt.Printf("[DEBUG] Matched botNick, text='%s'\n", text)
    } else {
        fmt.Printf("[DEBUG] No match, returning empty\n")
        return ""
    }
    
    parts := strings.Fields(text)
    if len(parts) == 0 {
        return ""
    }
    
    if parts[0] == "ynm" && len(parts) == 1 && issuingChannel != p.Cfg.ConsoleChannel {
        return ""
    }
    
    if parts[0] == "ynm" {
		    if userLevel < RoleHierarchy["vip"] {
				return "Nincs jogod YNM parancsokhoz (VIP szükséges)"
			}
			if len(parts) < 2 {
				return p.handleYnmCommand(msg.Sender, msg.Text)
			}
			
        // Handle plugin activation/deactivation with + or - prefix
        if strings.HasPrefix(parts[1], "+") || strings.HasPrefix(parts[1], "-") {
            return p.handlePluginCommand(msg.Sender, parts, issuingChannel)
        }
        
        switch parts[1] {
        case "info":
            if len(parts) > 2 {
                return p.handleInfoCommand(msg.Sender, parts[2])
            }
            return p.handleInfoCommand(msg.Sender)
        case "set":
            if len(parts) < 3 {
                return p.GetSetUsage(nick, hasPrefix, prefix)
            }
            field := strings.ToLower(parts[2])
            value := strings.Join(parts[3:], " ")
            return p.handleSetCommand(msg.Sender, field, value)
        
        case "channels":
            return p.handleChannelsCommand(msg.Sender, issuingChannel)
        
        case "add":
            if len(parts) < 3 {
                return "Usage: !ynm add chan #channel OR !ynm add user #channel nick role OR !ynm add vip #channel nick"
            }
            subcommand := parts[2]
            switch subcommand {
            case "chan":
                if len(parts) < 4 {
                    return "Usage: !ynm add chan #channel"
                }
                return p.handleAddRoomCommand(msg.Sender, parts[3], issuingChannel)
            case "user":
                if len(parts) < 5 {
                    return "Usage: !ynm add user #channel nick role OR !ynm add user #channel nick hostmask role"
                }
                userArgs := parts[3:]
                return p.handleAddUserToChannelCommandFlexible(msg.Sender, userArgs, issuingChannel)
			case "vip":
				if len(parts) < 3 {
					return "Usage: !ynm add vip [#channel] nick1 [nick2 ...]\n  - Csatornával: lokális VIP jog\n  - Csatorna nélkül: GLOBÁLIS VIP jog (csak owner!)"												 
				}
				vipArgs := parts[3:]
				return p.handleAddVipCommand(msg.Sender, vipArgs, issuingChannel)
			case "admin":
				if len(parts) < 4 { 
					return "Usage: !ynm add admin [#channel] nick1 [nick2 ...]"
				}
				adminArgs := parts[3:]
				return p.handleAddAdminCommand(msg.Sender, adminArgs, issuingChannel)

			case "mod":
				if len(parts) < 4 {  
					return "Usage: !ynm add mod [#channel] nick1 [nick2 ...]"
				}
				modArgs := parts[3:]
				return p.handleAddModCommand(msg.Sender, modArgs, issuingChannel)                
            default:
                return "Unknown add subcommand"
																	   
            }
	// ========== DEL COMMAND HANDLER ==========
	case "del":
		if len(parts) < 4 {
			return "Usage:\n" +
				   "  !ynm del chan #channel\n" +
				   "  !ynm del user #channel nick  (lokális törlés)\n" +
				   "  !ynm del user nick           (teljes törlés)\n" +
				   "  !ynm del vip [#channel] nick (VIP jog törlése)\n" +
				   "  !ynm del admin [#channel] nick (Admin jog törlése)\n" +
				   "  !ynm del mod [#channel] nick (Mod jog törlése)"
		}
		subcommand := parts[2]
		switch subcommand {
		case "chan":
			return p.handleRemoveRoomCommand(msg.Sender, parts[3], issuingChannel)
		case "user":
			if len(parts) == 4 {
				// Mindenhol törlés, ha csak a nick van megadva
				targetNick := parts[3]
				return p.handleRemoveUserEverywhereCommand(msg.Sender, targetNick)
			} else if len(parts) >= 5 {
				// Csatornához kötött törlés
				userArgs := parts[3:]
				return p.handleRemoveUserFromChannelCommandFlexible(msg.Sender, userArgs)
			} else {
				return "Usage: !ynm del user #channel nick OR !ynm del user nick"
			}
		case "vip":
			if len(parts) < 4 {
				return "Usage: !ynm del vip [#channel] nick1 [nick2 ...]\n  - Csatornával: lokális VIP jog törlése\n  - Csatorna nélkül: GLOBÁLIS VIP jog törlése"
			}
			vipArgs := parts[3:]
			return p.handleRemoveVipCommand(msg.Sender, vipArgs, issuingChannel)
		case "admin":
			if len(parts) < 4 {
				return "Usage: !ynm del admin [#channel] nick1 [nick2 ...]\n  - Csatornával: lokális Admin jog törlése\n  - Csatorna nélkül: GLOBÁLIS Admin jog törlése"
			}
			adminArgs := parts[3:]
			return p.handleRemoveAdminCommand(msg.Sender, adminArgs, issuingChannel)
		case "mod":
			if len(parts) < 4 {
				return "Usage: !ynm del mod [#channel] nick1 [nick2 ...]\n  - Csatornával: lokális Mod jog törlése\n  - Csatorna nélkül: GLOBÁLIS Mod jog törlése"
			}
			modArgs := parts[3:]
			return p.handleRemoveModCommand(msg.Sender, modArgs, issuingChannel)
		default:
			return "Unknown del subcommand."
		}
		default:
			return p.handleYnmCommand(msg.Sender, msg.Text)
			}
	} 
		return ""
}

// Get localized message
func (p *YnmAdminPlugin) GetMessage(hostmask string, key YnMLang.MessageKey, args ...interface{}) string {
	// Get user's language preference
	lang := p.getUserLanguage(hostmask)
	
	// Get the message template
	langMessages, exists := YnMLang.Messages[lang]
	if !exists {
		langMessages = YnMLang.Messages["En"] // fallback to English
	}
	
	template, exists := langMessages[key]
	if !exists {
		template = YnMLang.Messages["En"][key] // fallback to English message
	}
	
	// Format the message with arguments if provided
	if len(args) > 0 {
		return fmt.Sprintf(template, args...)
	}
	return template
}

// Get user's language preference from database
func (p *YnmAdminPlugin) getUserLanguage(hostmask string) string {
	info, err := p.Db.GetUserInfoByHost(hostmask)
	if err != nil || info == nil {
		return "En" // default to English
	}
	return info.Lang
}

// Get password usage message based on context
func (p *YnmAdminPlugin) GetPasswordUsage(nick string, isNew bool, hasPrefix bool, prefix string) string {
	var usage string
	if hasPrefix {
		if isNew {
			usage = prefix + "ynm pass <new>"
		} else {
			usage = prefix + "ynm pass <new> or " + prefix + "ynm pass <old> <new>"
		}
	} else {
		if isNew {
			usage = "pass <new>"
		} else {
			usage = "pass <new> or pass <old> <new>"
		}
	}
	
	if isNew {
		return p.GetMessage(nick, YnMLang.MsgNoPasswordSet, usage)
	}
	return p.GetMessage(nick, YnMLang.MsgPasswordUsage, usage)
}

// Get set command usage message
func (p *YnmAdminPlugin) GetSetUsage(nick string, hasPrefix bool, prefix string) string {
	var usage string
	if hasPrefix {
		usage = prefix + "ynm set <field> <value>"
	} else {
		usage = "set <field> <value>"
	}
	return p.GetMessage(nick, YnMLang.MsgUsageSet, usage)
}


func (p *YnmAdminPlugin) OnJoin(channel, nick, hostmask string) {
    if nick == p.Bot.GetNick() {
        return
    }   
    go func() {
        time.Sleep(3 * time.Second)
        p.AutoApplyUserModes(nick, hostmask, channel)
    }()
}

func (p *YnmAdminPlugin) OnTick() []YnMIrC.Message {
    // Clean up expired sessions every hour
    if time.Now().Minute() == 0 {
        p.CleanupExpiredSessions()
    }
    return nil
}