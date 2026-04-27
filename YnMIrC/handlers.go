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
	"bufio"
	"encoding/base64"
	"container/list"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	"sync"
	
)
// ─────────────────────── Main Flood Prot ─────────────────────────
type GlobalFloodProtection struct {
	mu               sync.RWMutex
	recentFlooders   *list.List                    // Utolsó X floodoló
	flooderMap       map[string]*list.Element      // Gyors kereséshez
	globalLocked     bool                          // Globális zár aktív?
	globalLockUntil  time.Time                     // Zár lejárat
	lockDuration     time.Duration                 // Zár időtartam
	maxFlooders      int                           // Hány floodoló = zár (pl. 5)
	windowDuration   time.Duration                 // Időablak (pl. 10 másodperc)
}

type FlooderEntry struct {
	nick      string
	timestamp time.Time
}

var globalFloodProtection = &GlobalFloodProtection{
	recentFlooders:  list.New(),
	flooderMap:      make(map[string]*list.Element),
	lockDuration:    180 * time.Second,
	maxFlooders:     5,      // 5 különböző ember = zár
	windowDuration:  20 * time.Second, // 10 másodperc alatt

}
var (
	floodMutex       sync.RWMutex
	privFloodMap     = make(map[string][]time.Time)   // nick -> üzenet időbélyegek
	floodNoticeSent  = make(map[string]time.Time)     // nick -> utolsó NOTICE idő
	floodBlocked     = make(map[string]time.Time)     // nick -> blokkolt idő
	lastGlobalBlockLog time.Time
	globalLogMutex     sync.RWMutex
)

func checkGlobalFlood(c *Client, nick string) bool {
	globalFloodProtection.mu.Lock()
	defer globalFloodProtection.mu.Unlock()
	
	now := time.Now()
	
	// 1. Ha globális zár aktív
	if globalFloodProtection.globalLocked {
		if now.Before(globalFloodProtection.globalLockUntil) {
			return true // ✋ MINDENKI blokkolva
		} else {
			// Zár lejárt
			globalFloodProtection.globalLocked = false
			globalFloodProtection.recentFlooders.Init()
			globalFloodProtection.flooderMap = make(map[string]*list.Element)
			
			// ✅ Console log: zár lejárt
			if c.config.ConsoleChannel != "" {
				c.SendMessage(c.config.ConsoleChannel, 
					"🔓 [GLOBAL FLOOD] Lock expired, resetting flood protection")
			}
			//log.Printf("[GLOBAL FLOOD] Lock expired, resetting")
		}
	}
	
	// 2. Régi flooderek törlése (windowDuration-nál régebbiek)
	for e := globalFloodProtection.recentFlooders.Front(); e != nil; {
		next := e.Next()
		entry := e.Value.(*FlooderEntry)
		
		if now.Sub(entry.timestamp) > globalFloodProtection.windowDuration {
			globalFloodProtection.recentFlooders.Remove(e)
			delete(globalFloodProtection.flooderMap, entry.nick)
		}
		e = next
	}
	
	// 3. Új flooder hozzáadása/újítása
	if elem, exists := globalFloodProtection.flooderMap[nick]; exists {
		// Már volt, frissítjük az időt
		entry := elem.Value.(*FlooderEntry)
		entry.timestamp = now
		globalFloodProtection.recentFlooders.MoveToBack(elem)
	} else {
		// Új flooder
		entry := &FlooderEntry{
			nick:      nick,
			timestamp: now,
		}
		elem := globalFloodProtection.recentFlooders.PushBack(entry)
		globalFloodProtection.flooderMap[nick] = elem
	}
	
	// 4. Ellenőrizzük, hogy elértük-e a limitet
	flooderCount := globalFloodProtection.recentFlooders.Len()
	//log.Printf("[GLOBAL FLOOD] Recent flooders: %d/%d", 
	//	flooderCount, globalFloodProtection.maxFlooders)
	
	if flooderCount >= globalFloodProtection.maxFlooders {
		// 5+ különböző ember floodolt 10 másodperc alatt!
		globalFloodProtection.globalLocked = true
		globalFloodProtection.globalLockUntil = now.Add(globalFloodProtection.lockDuration)
		
		// ✅ Flooderek listájának összeállítása
		flooders := make([]string, 0, flooderCount)
		for e := globalFloodProtection.recentFlooders.Front(); e != nil; e = e.Next() {
			entry := e.Value.(*FlooderEntry)
			flooders = append(flooders, entry.nick)
		}
		
		// ✅ Console log: globális zár aktiválva
		if c.config.ConsoleChannel != "" {
			lockMsg := fmt.Sprintf(
				"🚨 [GLOBAL FLOOD LOCK] Activated! %d different flooders detected in %v: %s | Lock duration: %v", 
				flooderCount, 
				globalFloodProtection.windowDuration,
				strings.Join(flooders, ", "),
				globalFloodProtection.lockDuration,
			)
			c.SendMessage(c.config.ConsoleChannel, lockMsg)
		}
		
		log.Printf("[GLOBAL FLOOD LOCK] Activated! %d different flooders in %v", 
			flooderCount, globalFloodProtection.windowDuration)
		
		return true // ✋ Globális zár aktív
	}
	
	return false // Nincs globális zár
}

// ─────────────────────── Main Read Loop ─────────────────────────

func (c *Client) readLoop() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("💥 Panic a readLoop-ban [%s]: %v", c.config.Server, r)
		}

		log.Printf("❌ readLoop leállt [%s], auto reconnect indul", c.config.Server)
		c.Disconnect(false)
	}()

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		log.Printf("❌ Nincs aktív kapcsolat [%s]", c.config.Server)
		return
	}

	reader := bufio.NewReader(conn)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("❌ Olvasási hiba [%s]: %v", c.config.Server, err)
			return
		}

		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		// Null / CR védelem
		line = strings.ReplaceAll(line, "\r", "")
		line = strings.ReplaceAll(line, "\x00", "")

		// Debug raw opcionális
		// log.Printf("[RAW-IN] %s", line)

		// malformed
		if strings.Contains(strings.ToLower(line), "malformed") {
			log.Printf("❌ Malformed line [%s]: %s", c.config.Server, line)
			return
		}

		// túl hosszú flood / sérült packet
		if len(line) > 4000 {
			log.Printf("⚠️ Túl hosszú sor eldobva [%s]: %d byte", c.config.Server, len(line))
			continue
		}

		line = stripIRCTags(line)

		receivedTime := time.Now()

		// Ping-Pong
		if strings.HasPrefix(line, "PING ") {
			pong := strings.Replace(line, "PING", "PONG", 1)

			if err := c.SendRaw(pong); err != nil {
				log.Printf("❌ PONG küldési hiba [%s]: %v", c.config.Server, err)
				return
			}

			continue
		}

		// Handler panic védelem
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("💥 handleRawMessage panic [%s]: %v | LINE: %s", c.config.Server, r, line)
				}
			}()

			c.handleRawMessage(line, receivedTime)
		}()
	}
}


func stripIRCTags(line string) string {
    if strings.HasPrefix(line, "@") {
        if i := strings.IndexByte(line, ' '); i != -1 {
            return line[i+1:]
        }
    }
    return line
}
// ─────────────────────── Message Handling ─────────────────────────

func extractNickFromPrefix(prefix string) string {
    if idx := strings.Index(prefix, "!"); idx != -1 {
        return prefix[:idx]
    }
    return prefix
}


func (c *Client) handleRawMessage(line string, receivedTime time.Time) {
    // DEBUG: MODE üzenetek detektálása
    if strings.Contains(line, " MODE ") {
     //   fmt.Printf("[DEBUG] MODE üzenet érkezett: %s\n", line)
    }

	if strings.HasPrefix(line, ":") && strings.Contains(line, " MODE ") {
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == "MODE" {
			c.handleMode(line)
			return
		}
	}

	if strings.Contains(line, "You are now logged in as") ||
	   strings.Contains(line, "Welcome to the UnderNET") ||
	   strings.Contains(line, "registered and protected") ||
	   (strings.Contains(line, " NOTICE ") && strings.Contains(line, c.nick) && strings.Contains(line, "users.undernet.org")) {

		// max logolj, de NE állíts undernetLoggedIn-t és NE joinolj itt
		log.Printf("[DEBUG] undernet login hint: %s", line)

	}
    


	if strings.Contains(line, " NOTICE ") {

		if strings.HasPrefix(line, ":X!") &&
			strings.Contains(line, "AUTHENTICATION SUCCESSFUL") {

			c.mu.Lock()
			c.undernetLoggedIn = true
			c.mu.Unlock()

			log.Printf("✅ Undernet X auth OK (NOTICE). Waiting for 396 hidden host before joining.")
			return
		}

		if c.handleNickServNotice(line) {
			return
		}
	}

    // ========== NUMERIC RESPONSES ==========
    // Ezek MIND return-olnak, így nem zavarják a user MODE-okat
    switch {
    case strings.Contains(line, " 433 ") || strings.Contains(strings.ToUpper(line), "NICKNAME_RESERVED"):
        c.handleNickInUse()
        return
		
	case strings.Contains(line, " 396 "):
		low := strings.ToLower(line)
		my := strings.ToLower(c.nick)
		if strings.Contains(low, " 396 "+my+" ") {

			// 1) itt már rejtett host -> most lépünk be mindenhova
			go c.joinAllChannels()

			// 2) és csak ezután jelezzük a login sikert (kicsi késleltetéssel)
			go func() {
				time.Sleep(5 * time.Second)
				if c.OnLoginSuccess != nil {
					c.OnLoginSuccess()
				}
			}()
		}
		return

    case strings.Contains(line, " 001 "):
        c.handleWelcome()
        return
		
    case strings.Contains(line, " 352 "): 
        c.handleWhoResponse(line)
        return
		
	case strings.Contains(line, " 315 "):
		c.handleEndOfWho(line)
		return
         
    case strings.Contains(line, " 353 "):
        c.handleNamesReply(line)
        return  
		
    case strings.Contains(line, " 324 "): // RPL_CHANNELMODEIS
        c.handleChannelModeIs(line)
        return	

    case strings.Contains(line, " 366 "):
        c.handleEndOfNames(line)
        return
        
    case strings.Contains(line, " 331 ") || strings.Contains(line, " 332 "):
        c.handleTopicReply(line)
        return
        
    case strings.Contains(line, " 311 "):
	log.Printf("[WHOIS-RAW-311] %s", line)
        c.handleWhoisUser(line)
        return
        
	case strings.Contains(line, " 401 "):
    log.Printf("[WHOIS-RAW-401] %s", line)
    return	
	
    case strings.Contains(line, " 312 "):  
        c.handleWhoisServer(line)
        return
		
    case strings.Contains(line, " 313 "): 
        c.handleWhoisOperator(line)
        return
		
    case strings.Contains(line, " 317 "): 
        c.handleWhoisIdle(line)
        return
		
    case strings.Contains(line, " 318 "):
	 log.Printf("[WHOIS-RAW-318] %s", line)
        c.handleEndOfWhois(line)
        return
        
    case strings.Contains(line, " 319 "): 
        c.handleWhoisChannels(line)
        return
		
    case strings.Contains(line, " 330 "):  
        c.handleWhoisAccount(line)
        return
		
    case strings.Contains(line, " 671 "): 
        c.handleWhoisSecure(line)
        return
        
    // 005 (RPL_ISUPPORT) kezelése - tartalmazhat "MODES=" szöveget
    case strings.Contains(line, " 005 "):
        // Ez numeric response, nem user MODE változtatás
        // Kezeld, ha szükséges, vagy hagyd figyelmen kívül
        return
    }

    // SASL és NickServ (case-insensitive detektálás)
    if c.handleSASL(line) || c.handleNickServ(line) {
        return
    }

    // ========== CHANNEL EVENTS ==========
    // Ezek NEM numeric responses
    switch {
    case strings.Contains(line, " JOIN "):
        c.handleJoin(line)
        return
		
    case strings.Contains(line, " PART "):
        c.handlePart(line)
        return

    case strings.Contains(line, " QUIT "):
        c.handleQuit(line)
        return

    case strings.Contains(line, " KICK "):
        c.handleKick(line)
        return

    case strings.Contains(line, " NICK "):
        c.handleNickChange(line)
        return

    // USER MODE változtatás már kezelve lett feljebb
    // Itt NEM kell MODE case, mert az már feldolgozásra került
    
    case strings.Contains(line, " TOPIC "):
        c.handleTopicChange(line)
        return

    case strings.Contains(line, " PONG "):
        c.handlePong(line)
        return
    }

// ========== PRIVMSG FELDOLGOZÁS ==========
if msg := parseMessage(line); msg != nil {
    msg.Time = receivedTime

    if msg.Command == "PRIVMSG" {
        target := msg.Params[0]
        message := msg.Params[1]
        isPrivate := !c.IsChannel(target)

			if isPrivate {
				// ✅ 1. Először GLOBÁLIS flood ellenőrzés
				if checkGlobalFlood(c, msg.Nick) {
					// Globális zár aktív! MINDENKI blokkolva
					
					// ✅ Csak 30 másodpercenként logoljuk
					globalLogMutex.Lock()
					now := time.Now()
					shouldLog := false
					
					if now.Sub(lastGlobalBlockLog) > 30*time.Second {
						shouldLog = true
						lastGlobalBlockLog = now
					}
					globalLogMutex.Unlock()
					
					if shouldLog {
						log.Printf("[GLOBAL BLOCK] Global flood lock active (30s), blocking %s", msg.Nick)
					}
					
					return // ✋ STOP
				}
						
            // ✅ 2. Egyéni flood védelem
            floodMutex.Lock()
            now := time.Now()
            nick := msg.Nick
            
            // Ellenőrizd, hogy jelenleg blokkolva van-e
            if lastBlock, blocked := floodBlocked[nick]; blocked {
                if now.Sub(lastBlock) < 10*time.Second {
                    floodMutex.Unlock()
                    // Csak minden 10. másodpercben logolj
                    if int(now.Sub(lastBlock).Seconds())%20 == 0 {
                        log.Printf("[IRC FLOOD] %s still blocked (%.0fs remaining)", 
                            nick, 30-now.Sub(lastBlock).Seconds())
                    }
                    return
                }
                delete(floodBlocked, nick)
            }
            
            // Üzenetek lekérése
            messages := privFloodMap[nick]
            
            // Szűrés
            validMsgs := make([]time.Time, 0)
            for _, t := range messages {
                if now.Sub(t) <= 3*time.Second {
                    validMsgs = append(validMsgs, t)
                }
            }
            
            log.Printf("[IRC FLOOD] %s has %d messages in last 3s", nick, len(validMsgs))
            
            // Flood detektálás
            if len(validMsgs) >= 2 {
                shouldSendNotice := true
                
                if lastFloodTime, exists := floodNoticeSent[nick]; exists {
                    if now.Sub(lastFloodTime) < 30*time.Second {
                        shouldSendNotice = false
                    }
                }
                
                if shouldSendNotice {
                    c.SendRaw(fmt.Sprintf("NOTICE %s :⚠️ Flood védelem: 30 másodperc szünet", nick))
                    floodNoticeSent[nick] = now
                }
                
                // Jelöld meg blokkoltnak
                floodBlocked[nick] = now
                privFloodMap[nick] = validMsgs  // Tartsd meg az időbélyegeket
                floodMutex.Unlock()
                
                log.Printf("[IRC FLOOD BLOCKED] Blocking %s for 30 seconds", nick)
                return // ✋ STOP
            }
            
            // Nincs flood
            validMsgs = append(validMsgs, now)
            privFloodMap[nick] = validMsgs
            floodMutex.Unlock()
            
            log.Printf("[IRC PRIVMSG] Allowing private message from %s", nick)
        }
        
        // Plugin-ok hívása (csak ha nem volt flood - már kívül vagyunk a blokkon)
        c.pluginMu.RLock()
        for _, plugin := range c.plugins {
            if p, ok := plugin.(interface {
                OnPrivMsg(nick, target, msg, hostmask string, isPrivate bool)
            }); ok {
                p.OnPrivMsg(msg.Nick, target, message, msg.Sender, isPrivate)
            }
        }
        c.pluginMu.RUnlock()
        
        // OnMessage callback (mindig, függetlenül attól, hogy privát vagy csatorna)
        if c.OnMessage != nil {
            c.OnMessage(*msg)
        }
        
    } else {
        // Nem PRIVMSG command (pl. JOIN, MODE, stb.)
        if c.OnMessage != nil {
            c.OnMessage(*msg)
        }
    }
}
}

func isUserModeChange(line string) bool {
    // Ellenőrizzük, hogy user MODE változtatás-e (nem numeric response)
    if !strings.HasPrefix(line, ":") {
        return false
    }
    
    parts := strings.Fields(line)
    if len(parts) < 3 {
        return false
    }
    
    // Ha a második rész numeric (pl. "324", "005"), akkor az nem user MODE
    if _, err := strconv.Atoi(parts[1]); err == nil {
        // Numeric response
        return false
    }
    
    // Ha "MODE" a második rész, akkor user MODE
    return parts[1] == "MODE"
}
// ─────────────────────── Specific Event Handlers ─────────────────────────

func (c *Client) handleNickInUse() {
	c.mu.Lock()
//	oldNick := c.nick
	newNick := fmt.Sprintf("%s_%d", c.config.NickName, time.Now().Unix()%10000)
	c.nick = newNick
	c.mu.Unlock()
	
	//fmt.Printf("Nick %s foglalt/rezervált, új nick: %s\n", oldNick, newNick)
	c.SendRaw("NICK " + newNick)
}

func (c *Client) handleWelcome() {
    c.mu.Lock()
    c.loggedIn = true
    if c.startedAt.IsZero() {
        c.startedAt = time.Now()
    }
    c.mu.Unlock()

    go func() {
        time.Sleep(3 * time.Second)

        if c.config.Undernet.Enabled {
            c.mu.Lock()
            if !c.undernetLoginSent {
                c.undernetLoginSent = true
                c.mu.Unlock()
                c.UndernetLogin()
            } else {
                c.mu.Unlock()
            }
        } else if c.config.NickServLogin {
            c.IdentifyNickServ()
        }
    }()
}


func (c *Client) handleUndernetResponse(msg Message) {
    // Check if this is a message from X service
    if strings.ToLower(msg.Nick) != "x" {
        return
    }
    
    // Check if it's from the correct X service
    if !strings.Contains(strings.ToLower(msg.Sender), "undernet.org") {
        return
    }
    
    response := strings.ToLower(msg.Text)
    
    //fmt.Printf("📋 Undernet X response: %s\n", msg.Text)
    
    // Handle successful login responses
    if strings.Contains(response, "authentication successful") {
        ////fmt.Println("✅ Undernet X authentication successful")
        c.mu.Lock()
        c.undernetLoggedIn = true
        c.mu.Unlock()
        return
    }
    
			// Handle already logged in
		if strings.Contains(response, "authentication successful") {
			c.mu.Lock()
			c.undernetLoggedIn = true
			c.mu.Unlock()

			if c.OnLoginSuccess != nil {
				c.OnLoginSuccess()
			}
			go c.joinAllChannels()
			return
		}

		if strings.Contains(response, "already authenticated") {
			c.mu.Lock()
			c.undernetLoggedIn = true
			c.mu.Unlock()

			if c.OnLoginSuccess != nil {
				c.OnLoginSuccess()
			}
			go c.joinAllChannels()
			return
		}
}

func (c *Client) handleJoin(line string) {
    // Parse: :nick!user@host JOIN #channel
    parts := strings.SplitN(line, " ", 4)
    
    if len(parts) < 3 {
        return
    }
    
    prefix := strings.TrimPrefix(parts[0], ":")
    nick, hostmask := parseNickHostmask(prefix)
//	if strings.EqualFold(nick, c.nick) {
    // Ez a bot saját JOIN-ja → SENKI nem látja
 //   return
//	}
    channel := strings.TrimPrefix(parts[2], ":")
    
    // Handle colon prefix in channel name
    if len(parts) == 4 && strings.HasPrefix(parts[3], ":") {
        channel = strings.TrimPrefix(parts[3], ":")
    }

    // ========== UNDERNET AZONOSÍTÁS DETEKTÁLÁS ==========

    c.mu.Lock()
    // Create or get channel
    ch := c.getOrCreateChannel(channel)
    ch.AddUser(nick, hostmask, "")

    // If it's us joining, mark the channel as joined
    if nick == c.nick {
        if c.OnChannelJoined != nil {
            c.OnChannelJoined(channel)
        }
    }
    c.mu.Unlock()

    // Call OnJoin callback (AutoMode handler)
    if c.OnJoin != nil {
        c.OnJoin(channel, nick, hostmask)
    }

    // Call plugins with full hostmask
    for _, plugin := range c.plugins {
        if joinPlugin, ok := plugin.(JoinHandler); ok {
            joinPlugin.OnJoin(channel, nick, hostmask)
        }
    }
    
    // A JOIN sor nem PRIVMSG, ezért parseMessage NULL-t ad vissza
    // Ezt a részt EL KELL TÁVOLÍTANI a handleJoin-ből:
    // if msg := parseMessage(line); msg != nil {
    //    ... ez JOIN sor, nem PRIVMSG ...
    // }
    
    // Inkább hozzunk létre egy Message struktúrát a JOIN-hoz:
    joinMsg := &Message{
        Sender:   prefix,
        Nick:     nick,
        Hostmask: hostmask,
        Channel:  channel,
        Text:     "",  // JOIN-nak nincs szövege
        Command:  "JOIN",
        Params:   []string{channel},
        Time:     time.Now(),
    }
    
    // Call OnMessage callback with JOIN message
    if c.OnMessage != nil {
        c.OnMessage(*joinMsg)
    }
}
func (c *Client) joinAllChannels() {
    deadline := time.Now().Add(15 * time.Second)

    for {
        c.mu.RLock()
        undernetEnabled := c.config.Undernet.Enabled
        isAuthenticated := c.undernetLoggedIn
        isLoggedIn := c.loggedIn
        console := strings.TrimSpace(c.config.ConsoleChannel)
        c.mu.RUnlock()

        // ha még 001 sincs meg, várunk
        if !isLoggedIn {
            if time.Now().After(deadline) {
                // 001 nélkül nem joinolunk semmit
                log.Printf("🔒 joinAllChannels: timeout waiting for 001 (loggedIn=false)")
                return
            }
            time.Sleep(500 * time.Millisecond)
            continue
        }

        // UnderNet OFF: mehet minden csatorna
        if !undernetEnabled {
            break
        }

        // UnderNet ON és már auth: mehet minden csatorna
        if isAuthenticated {
            break
        }

        // UnderNet ON, nincs auth, de lejárt a 15mp: csak console
        if time.Now().After(deadline) {
            if console != "" {
                log.Printf("⏳ No X-auth in 15s -> joining ConsoleChannel only: %s", console)
                c.Join(console)
            }
            return
        }

        time.Sleep(500 * time.Millisecond)
    }

    // ide csak akkor jutunk, ha:
    // - UnderNet OFF, vagy
    // - UnderNet ON és auth megvan
    time.Sleep(1 * time.Second)

    // JOIN MINDEN csatorna (és console is)
    joined := make(map[string]bool)

    for i, channel := range c.config.Channels {
        ch := strings.TrimSpace(channel)
        if ch == "" {
            continue
        }
        key := strings.ToLower(ch)
        if joined[key] {
            continue
        }
        joined[key] = true

        log.Printf("📍 Joining channel %d/%d: %s", i+1, len(c.config.Channels), ch)
        c.Join(ch)
        time.Sleep(500 * time.Millisecond)
    }

    // Ha a ConsoleChannel nincs benne a listában, akkor is menjünk be oda.
    if c.config.ConsoleChannel != "" {
        cc := strings.TrimSpace(c.config.ConsoleChannel)
        key := strings.ToLower(cc)
        if cc != "" && !joined[key] {
            log.Printf("📍 Joining ConsoleChannel: %s", cc)
            c.Join(cc)
        }
    }
}
// Handle Chan MODE
func (c *Client) handleChannelModeIs(line string) {
    // Parse: :server 324 nick #channel +modes params
    parts := strings.Fields(line)
    if len(parts) < 4 {
        return
    }
    
    channel := parts[3]
    modes := ""
    if len(parts) > 4 {
        modes = parts[4]
    }
    
    c.mu.Lock()
    key := strings.ToLower(channel)
    if ch, exists := c.channels[key]; exists {
        ch.Modes = strings.TrimPrefix(modes, "+")
        //log.Printf("✅ Channel %s modes updated: %s", channel, ch.Modes)
    }
    c.mu.Unlock()
}

func (c *Client) handlePart(line string) {
	// Parse: :nick!user@host PART #channel :reason
	parts := strings.SplitN(line, " ", 4)
	if len(parts) < 3 {
		return
	}

	prefix := strings.TrimPrefix(parts[0], ":")
	nick, _ := parseNickHostmask(prefix)
	if strings.EqualFold(nick, c.nick) {
    return
	}
	channel := parts[2]
	
	reason := ""
	if len(parts) == 4 {
		reason = strings.TrimPrefix(parts[3], ":")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	key := strings.ToLower(channel)
	if ch, exists := c.channels[key]; exists {
		ch.RemoveUser(nick)
		
		// If it's us parting, remove the channel
		if nick == c.nick {
			delete(c.channels, channel)
		}
	}

	if c.OnPart != nil {
		c.OnPart(channel, nick, reason)
	}
}

func (c *Client) handleQuit(line string) {
    // Parse: :nick!user@host QUIT :reason
    parts := strings.SplitN(line, " ", 3)
    if len(parts) < 2 {
        return
    }

    prefix := strings.TrimPrefix(parts[0], ":")
    nick, _ := parseNickHostmask(prefix) // MÓDOSÍTOTT: hostmask helyett _
    
    reason := ""
    if len(parts) == 3 {
        reason = strings.TrimPrefix(parts[2], ":")
    }

    // ========== UNDERNET AZONOSÍTÁS DETEKTÁLÁS ==========
    // Ha a bot QUIT-el és az Undernet szerverről jön, valószínűleg azonosítás
    if nick == c.nick && strings.Contains(reason, "Registered") {
        log.Println("🔑 Undernet azonosítás folyamatban - QUIT érzékelve")
        // Ne csinálj semmit, várjuk az új JOIN-t az azonosított hostmask-szal
        return
    }

    c.mu.Lock()
    defer c.mu.Unlock()

    // Remove user from all channels
    for _, ch := range c.channels {
        ch.RemoveUser(nick)
    }

    if c.OnQuit != nil {
        c.OnQuit(nick, reason)
    }
}

func (c *Client) handleKick(line string) {
	// Parse: :kicker!user@host KICK #channel kicked :reason
	parts := strings.SplitN(line, " ", 5)
	if len(parts) < 4 {
		return
	}

	prefix := strings.TrimPrefix(parts[0], ":")
	kicker, _ := parseNickHostmask(prefix)
	channel := parts[2]
	kicked := parts[3]
	
	reason := ""
	if len(parts) == 5 {
		reason = strings.TrimPrefix(parts[4], ":")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	key := strings.ToLower(channel)
	if ch, exists := c.channels[key]; exists {
		ch.RemoveUser(kicked)
		if kicked == c.nick {
			delete(c.channels, key) // kisbetűs kulccsal törlünk
		}
	}
	if kicked == c.nick {
		// Újra belépünk a csatornába, ha minket rúgtak ki
		go func() {
			// Kis késleltetés, hogy ne túl gyorsan próbálkozzon
			time.Sleep(1 * time.Second)
			c.Join(channel)
		}()
	}
	if c.OnKick != nil {
		c.OnKick(channel, kicked, kicker, reason)
	}
}

func (c *Client) handleNickChange(line string) {
	// Parse: :oldnick!user@host NICK :newnick
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 3 {
		return
	}

	prefix := strings.TrimPrefix(parts[0], ":")
	oldNick, _ := parseNickHostmask(prefix)
	newNick := strings.TrimPrefix(parts[2], ":")

	c.mu.Lock()
	defer c.mu.Unlock()

	// Update nick in all channels
	for _, ch := range c.channels {
		if user, exists := ch.GetUser(oldNick); exists {
			ch.RemoveUser(oldNick)
			ch.AddUser(newNick, user.Hostmask, user.Modes)
		}
	}

	// If it's our nick change
	if oldNick == c.nick {
		c.nick = newNick
	}

	if c.OnNickChange != nil {
		c.OnNickChange(oldNick, newNick)
	}
}

func (c *Client) handleMode(line string) {
    
    parts := strings.Fields(line)
    if len(parts) < 4 {
        return
    }
    
    prefix := strings.TrimPrefix(parts[0], ":")
    channel := parts[2]
    modes := parts[3]
    args := ""
    if len(parts) > 4 {
        args = strings.Join(parts[4:], " ")
    }
    
    if c.IsChannel(channel) {
        c.mu.Lock()
        key := strings.ToLower(channel)
        if ch, exists := c.channels[key]; exists {
            c.updateChannelModes(ch, modes, args)
        }
        c.mu.Unlock()
    }

    nick, _ := parseNickHostmask(prefix)
    modeMsg := &Message{
        Sender:     prefix,
        Nick:       nick,
        Hostmask:   prefix,
        Channel:    channel,
        Text:       fmt.Sprintf("%s %s", modes, args),
        Command:    "MODE",
        Params:     []string{channel, modes, args},
        Time:       time.Now(),
    }

    if c.OnMode != nil {
        c.OnMode(channel, modes, args, prefix)
    }
    
    if c.OnMessage != nil {
        c.OnMessage(*modeMsg)
    }
    c.callModePlugins(channel, modes, args, prefix, modeMsg)
}
func (c *Client) callModePlugins(channel, modes, args, prefix string, modeMsg *Message) {
    c.pluginMu.RLock()
    defer c.pluginMu.RUnlock()
    
    for _, plugin := range c.plugins {
        if p, ok := plugin.(interface {
            OnMessage(msg Message)
        }); ok {
            p.OnMessage(*modeMsg)
        }
    }
}
func (c *Client) handleTopicChange(line string) {
	// Parse: :setter!user@host TOPIC #channel :new topic
	parts := strings.SplitN(line, " ", 4)
	if len(parts) < 4 {
		return
	}

	prefix := strings.TrimPrefix(parts[0], ":")
	setBy, _ := parseNickHostmask(prefix)
	channel := parts[2]
	topic := strings.TrimPrefix(parts[3], ":")

	c.mu.Lock()
	key := strings.ToLower(channel)
	if ch, exists := c.channels[key]; exists {
		ch.SetTopic(topic, setBy)
	}
	c.mu.Unlock()

	if c.OnTopic != nil {
		c.OnTopic(channel, topic, setBy)
	}
}

func (c *Client) handlePong(line string) {
	// Parse: :server PONG server :pongID
	parts := strings.Fields(line)
	if len(parts) >= 3 {
		pongID := strings.TrimPrefix(parts[len(parts)-1], ":")
		if c.OnPong != nil {
			c.OnPong(pongID)
		}
	}
}

func (c *Client) handleNamesReply(line string) {
    // Parse: :server 353 nick = #channel :nick1 @nick2 +nick3
    parts := strings.SplitN(line, " ", 6)
    if len(parts) < 6 {
        return
    }

    channel := parts[4]
    names := strings.TrimPrefix(parts[5], ":")

    c.mu.Lock()
    defer c.mu.Unlock()

    ch := c.getOrCreateChannel(channel)
    
    // Parse user list with modes
    for _, name := range strings.Fields(names) {
        nick := name
        modes := ""
        
        // Extract modes from nick prefixes
        for len(nick) > 0 {
            switch nick[0] {
            case '@':
                modes += "o"
                nick = nick[1:]
            case '+':
                modes += "v"
                nick = nick[1:]
            case '%':
                modes += "h"
                nick = nick[1:]
            case '&':
                modes += "a"
                nick = nick[1:]
            case '~':
                modes += "q"
                nick = nick[1:]
            default:
                goto done
            }
        }
        done:
        
        if nick != "" {
            ch.AddUser(nick, "", modes)
        }
    }
    
    if c.OnNames != nil {
        namesList := strings.Fields(names)
        c.OnNames(channel, namesList)
    }
}

func (c *Client) handleEndOfNames(line string) {
	// Parse: :server 366 nick #channel :End of NAMES list
	parts := strings.Fields(line)
	if len(parts) >= 4 {
		//channel := parts[3]
		//fmt.Printf("📋 NAMES lista befejezve: %s\n", channel)
	}
}

func (c *Client) handleTopicReply(line string) {
	// Parse: :server 332 nick #channel :topic
	// Or: :server 331 nick #channel :No topic is set
	parts := strings.SplitN(line, " ", 5)
	if len(parts) < 4 {
		return
	}

	channel := parts[3]
	topic := ""
	
	if strings.Contains(line, " 332 ") && len(parts) == 5 {
		topic = strings.TrimPrefix(parts[4], ":")
	}

	c.mu.Lock()
	key := strings.ToLower(channel)
	if ch, exists := c.channels[key]; exists {
		ch.SetTopic(topic, "")
	}
	c.mu.Unlock()
}
func (c *Client) SendWho(channel string) {
    c.SendRaw(fmt.Sprintf("WHO %s", channel))
}
func (c *Client) handleWhoResponse(line string) {
    parts := strings.Split(line, " ")
    if len(parts) < 10 {
        return
    }
    
    channel := parts[3]
    username := parts[4]
    hostname := parts[5]
    nick := parts[7]
 
    hostmask := fmt.Sprintf("%s!%s@%s", nick, username, hostname)
    
    //log.Printf("[DEBUG] WHO: %s -> %s (flags: %s)", nick, channel, flags)
    flags := parts[8]
	if strings.EqualFold(nick, c.nick) {
		prefix := ""
		if strings.Contains(flags, "@") { prefix = "@" } else if strings.Contains(flags, "%") { prefix = "%" } else if strings.Contains(flags, "+") { prefix = "+" }

		key := strings.ToLower(strings.TrimSpace(channel))
		c.whoBotPrefixMu.Lock()
		if c.whoBotPrefix == nil { c.whoBotPrefix = make(map[string]string) }
		c.whoBotPrefix[key] = prefix
		c.whoBotPrefixMu.Unlock()
	}
    if c.OnWho != nil {
        c.OnWho(channel, nick, hostmask)
    }
}
func (c *Client) handleWhoisUser(line string) {
    parts := strings.SplitN(line, " ", 8)
    if len(parts) < 8 {
        return
    }

    nick := parts[3]
    nickKey := strings.ToLower(nick)

    username := parts[4]
    hostname := parts[5]
    realname := strings.TrimPrefix(parts[7], ":")

    c.whoisMutex.Lock()
    defer c.whoisMutex.Unlock()

    if _, exists := c.whoisData[nickKey]; !exists {
        c.whoisData[nickKey] = &WhoisData{
            Nick:     nick, // eredeti case
            Username: username,
            Hostname: hostname,
            Realname: realname,
            Hostmask: fmt.Sprintf("%s!%s@%s", nick, username, hostname),
            Channels: make([]string, 0),
            Extra:    make(map[string]string),
            Raw:      make([]string, 0),
        }
    }

    c.whoisData[nickKey].Raw = append(c.whoisData[nickKey].Raw, line)
}

func (c *Client) handleWhoisServer(line string) {
    // :server 312 mynick nick server :server info
    parts := strings.SplitN(line, " ", 6)
    if len(parts) < 6 {
        return
    }

    nick := parts[3]
    nickKey := strings.ToLower(nick)

    server := parts[4]
    serverInfo := strings.TrimPrefix(parts[5], ":")

    c.whoisMutex.Lock()
    defer c.whoisMutex.Unlock()

    if data, exists := c.whoisData[nickKey]; exists {
        data.Server = server
        data.ServerInfo = serverInfo
        data.Raw = append(data.Raw, line)
    }
}

func (c *Client) handleWhoisChannels(line string) {
	// Parse: :server 319 mynick nick :@#chan1 +#chan2 #chan3
	parts := strings.SplitN(line, " ", 5)
	if len(parts) < 5 {
		return
	}

	nick := parts[3]
	nickKey := strings.ToLower(nick)

	channelsStr := strings.TrimPrefix(parts[4], ":")

	c.whoisMutex.Lock()
	defer c.whoisMutex.Unlock()

	if data, exists := c.whoisData[nickKey]; exists {
		for _, ch := range strings.Fields(channelsStr) {
			cleanChan := strings.TrimLeft(ch, "@+%&~")
			data.Channels = append(data.Channels, cleanChan)
		}
		data.Raw = append(data.Raw, line)
	}
}


func (c *Client) handleWhoisAccount(line string) {
    parts := strings.Fields(line)
    if len(parts) < 5 {
        return
    }

    nickKey := strings.ToLower(parts[3])
    account := parts[4]

    c.whoisMutex.Lock()
    defer c.whoisMutex.Unlock()

    if data, exists := c.whoisData[nickKey]; exists {
        data.Account = account
        data.IsLoggedIn = true
        data.Raw = append(data.Raw, line)
    }
}

func (c *Client) handleWhoisSecure(line string) {
    parts := strings.Fields(line)
    if len(parts) < 4 {
        return
    }

    nickKey := strings.ToLower(parts[3])

    c.whoisMutex.Lock()
    defer c.whoisMutex.Unlock()

    if data, exists := c.whoisData[nickKey]; exists {
        data.IsSecure = true
        data.Raw = append(data.Raw, line)
    }
}
func (c *Client) handleWhoisIdle(line string) {
    parts := strings.Fields(line)
    if len(parts) < 6 {
        return
    }

    nickKey := strings.ToLower(parts[3])

    idle, err1 := strconv.Atoi(parts[4])
    signon, err2 := strconv.ParseInt(parts[5], 10, 64)
    if err1 != nil || err2 != nil {
        return
    }

    c.whoisMutex.Lock()
    defer c.whoisMutex.Unlock()

    if data, exists := c.whoisData[nickKey]; exists {
        data.Idle = idle
        data.SignonTime = signon
        data.Raw = append(data.Raw, line)
    }
}

func (c *Client) handleWhoisOperator(line string) {
    parts := strings.Fields(line)
    if len(parts) < 4 {
        return
    }

    nickKey := strings.ToLower(parts[3])

    c.whoisMutex.Lock()
    defer c.whoisMutex.Unlock()

    if data, exists := c.whoisData[nickKey]; exists {
        data.IsOper = true
        data.Raw = append(data.Raw, line)
    }
}

func (c *Client) handleEndOfWho(line string) {
    // :server 315 mynick #channel :End of WHO list
    parts := strings.Fields(line)
    if len(parts) < 5 {
        return
    }
    channel := parts[3]
	//log.Printf("[WHO] END (315) channel=%s", channel)
    if c.OnEndOfWho != nil {
        c.OnEndOfWho(channel)
    }
}

func (c *Client) handleEndOfWhois(line string) {
    line = stripIRCTags(line)

    parts := strings.Fields(line)
    if len(parts) < 4 {
        return
    }

    nickKey := strings.ToLower(strings.TrimSpace(parts[3]))

    c.whoisMutex.Lock()
    data := c.whoisData[nickKey]
    chans := c.whoisChannels[nickKey]
    delete(c.whoisData, nickKey)
    delete(c.whoisChannels, nickKey)
    c.whoisMutex.Unlock()

    if data == nil {
        log.Printf("[WHOIS] Missing 311 data for %s, line=%q", nickKey, line)
        data = &WhoisData{Nick: parts[3]}
    }

    for _, ch := range chans {
        select {
        case ch <- data:
        default:
        }
        close(ch)
    }
}

func (c *Client) handleSASL(line string) bool {
	if !c.useSASL {
		return false
	}
	
	// Handle CAP ACK for SASL
	if strings.Contains(line, "CAP * ACK") && strings.Contains(line, "sasl") {
		//fmt.Println("🔑 SASL capability acknowledged, starting authentication...")
		c.SendRaw("AUTHENTICATE PLAIN")
		return true
	}
	
	// Handle AUTHENTICATE + response
	if strings.Contains(line, "AUTHENTICATE +") {
		//fmt.Println("🔑 Sending SASL credentials...")
		auth := fmt.Sprintf("%s\x00%s\x00%s", c.saslUser, c.saslUser, c.saslPass)
		encoded := base64.StdEncoding.EncodeToString([]byte(auth))
		c.SendRaw("AUTHENTICATE " + encoded)
		return true
	}
	
	// Handle SASL success (903)
	if strings.Contains(line, " 903 ") {
		//log.Println("✔️ SASL autentikáció sikeres")
		c.SendRaw("CAP END")
		
		c.SendRaw(fmt.Sprintf("NICK %s", c.config.NickName))
		c.SendRaw(fmt.Sprintf("USER %s 0 * :%s", c.config.UserName, c.config.RealName))
		
		c.mu.Lock()
		c.loggedIn = true
		c.mu.Unlock()
		
		//log.Println("🎯 Meghívom az OnLoginSuccess callbacket...")
		if c.OnLoginSuccess != nil {
			//log.Println("✅ OnLoginSuccess callback létezik, meghívom...")
			c.OnLoginSuccess()
			//log.Println("✅ OnLoginSuccess callback meghívva")
		} else {
			//log.Println("⚠️⚠️⚠️ OnLoginSuccess callback NIL!")
		}
		return true
	}
		
	// SASL failure
	if strings.Contains(line, " 904 ") || strings.Contains(line, " 905 ") || strings.Contains(line, " 906 ") {
		log.Println("❌ SASL autentikáció sikertelen")
		
		c.mu.Lock()
		c.loggedIn = false
		// c.saslFailed = true // 👈 Nem kell flag
		c.mu.Unlock()
		
		c.SendRaw("CAP END")
		
		// 👇 KÜLDJÜK AZ ÜZENETET KÉSŐBB, AMIKOR MÁR CSATORNÁBAN VAGYUNK
		go func() {
			// Várjunk, hogy a bot belépjen a console channel-be
			time.Sleep(8 * time.Second) // 8 másodperc várás
			if c.config.ConsoleChannel != "" {
				c.SendMessage(c.config.ConsoleChannel, "❌ Sikertelen bejelentkezés: SASL hitelesítés nem sikerült")
				c.SendMessage(c.config.ConsoleChannel, "⚠️ Fallback: NICK/USER regisztráció használatban")
			}
		}()
		
		if c.OnLoginFailed != nil {
			c.OnLoginFailed("SASL authentication failed")  
		}
		
		// Fallback to normal registration
		c.SendRaw(fmt.Sprintf("NICK %s", c.config.NickName))
		c.SendRaw(fmt.Sprintf("USER %s 0 * :%s", c.config.UserName, c.config.RealName))
		return true
	}
	
	return false
}

func (c *Client) handleNickServ(line string) bool {
	// Check if message is from NickServ
	if !strings.HasPrefix(line, ":") {
		return false
	}

	parts := strings.SplitN(line, " ", 4)
	if len(parts) < 4 {
		return false
	}

	sender := strings.TrimPrefix(parts[0], ":")
	nick, _ := parseNickHostmask(sender)
	
	// ⚠️ JAVÍTÁS: Case-insensitive összehasonlítás
	if strings.ToLower(nick) != strings.ToLower(c.config.NickservBotnick) {
		return false
	}

	message := strings.TrimPrefix(parts[3], ":")
	
	// ⚠️ DEBUG: Nézd meg, mit mond a NickServ
	//log.Printf("🔍 NickServ üzenet: %s", message)

	// NickServ successful identification
	// ⚠️ JAVÍTÁS: Több mintát keresünk (case-insensitive)
	messageLower := strings.ToLower(message)
	if strings.Contains(messageLower, "you are now identified") ||
		strings.Contains(messageLower, "password accepted") ||
		strings.Contains(messageLower, "recognized") ||
		strings.Contains(messageLower, "logged in") {
		//log.Println("✔️ NickServ azonosítás sikeres")
		
		c.mu.Lock()
		c.loggedIn = true
		c.mu.Unlock()
		
		if c.OnLoginSuccess != nil {
			c.OnLoginSuccess()
		}
		return true
	}

	// NickServ failure
	if strings.Contains(messageLower, "failed") ||
		strings.Contains(messageLower, "access denied") ||
		strings.Contains(messageLower, "Invalid account") ||	
		strings.Contains(messageLower, "incorrect password") {
		//log.Println("❌ NickServ azonosítás sikertelen")
		
		c.mu.Lock()
		c.loggedIn = false
		c.mu.Unlock()
		
		if c.OnLoginFailed != nil {
			c.OnLoginFailed("NickServ authentication failed")
		}
		return true
	}

	return false
}


func (c *Client) handleNickServNotice(line string) bool {
	// Parse: :NickServ!services@host NOTICE YnM-Beta :You are now identified...
	parts := strings.SplitN(line, " ", 5)
	if len(parts) < 5 {
		return false
	}

	sender := strings.TrimPrefix(parts[0], ":")
	nick, _ := parseNickHostmask(sender)
	
	if strings.ToLower(nick) != strings.ToLower(c.config.NickservBotnick) {
		return false
	}

	message := strings.TrimPrefix(parts[4], ":")
	
	//log.Printf("🔍 NickServ NOTICE: %s", message)

	messageLower := strings.ToLower(message)
	if strings.Contains(messageLower, "you are now identified") ||
		strings.Contains(messageLower, "password accepted") ||
		strings.Contains(messageLower, "recognized") ||
		strings.Contains(messageLower, "logged in") {
		//log.Println("✔️ NickServ azonosítás sikeres (NOTICE)")
		
		c.mu.Lock()
		c.loggedIn = true
		c.mu.Unlock()
		
		if c.OnLoginSuccess != nil {
			c.OnLoginSuccess()
		}
		return true
	}

	return false
}
//Én 

// ─────────────────────── Helper Functions ─────────────────────────

// parseMessage függvény bővítése:
func parseMessage(line string) *Message {
    if !strings.HasPrefix(line, ":") || len(line) < 3 {
        return nil
    }

    // Split the line
    parts := strings.Fields(line)
    if len(parts) < 3 {
        return nil
    }
    
    sender := strings.TrimPrefix(parts[0], ":")
    command := parts[1]
    
    nick, hostmask := parseNickHostmask(sender)
    
    // Külön kezelés különböző parancsokhoz
    switch command {
    case "PRIVMSG":
        if len(parts) < 4 {
            return nil
        }
        target := parts[2]
        raw := strings.Join(parts[3:], " ")
		text := strings.TrimPrefix(raw, ":")
        
        return &Message{
            Sender:   sender,
            Nick:     nick,
            Hostmask: hostmask,
            Channel:  target,
            Text:     text,
            Command:  command,
            Params:   []string{target, text},
            Time:     time.Now(),
        }
        
    case "MODE":
        if len(parts) < 4 {
            return nil
        }
        target := parts[2]
        modes := parts[3]
        args := ""
        if len(parts) > 4 {
            rawArgs := strings.Join(parts[4:], " ")
			args = strings.TrimPrefix(rawArgs, ":")
        }
        
        return &Message{
            Sender:   sender,
            Nick:     nick,
            Hostmask: hostmask,
            Channel:  target,
            Text:     fmt.Sprintf("%s %s", modes, args),
            Command:  command,
            Params:   []string{target, modes, args},
            Time:     time.Now(),
        }
        
    // További parancsok...
    default:
        return nil
    }
}

func parseNickHostmask(prefix string) (nick, hostmask string) {
	if strings.Contains(prefix, "!") {
		parts := strings.SplitN(prefix, "!", 2)
		return parts[0], prefix
	}
	return prefix, prefix
}
func (c *Client) isWhoisMessage(line string) bool {
	// WHOIS válasz kódok
	whoisCodes := []string{
		" 311 ", // RPL_WHOISUSER
		" 312 ", // RPL_WHOISSERVER  
		" 313 ", // RPL_WHOISOPERATOR
		" 317 ", // RPL_WHOISIDLE
		" 318 ", // RPL_ENDOFWHOIS
		" 319 ", // RPL_WHOISCHANNELS
		" 330 ", // RPL_WHOISLOGGEDIN (services)
		" 335 ", // RPL_WHOISBOT
		" 378 ", // RPL_WHOISHOST
		" 379 ", // RPL_WHOISMODES
		" 671 ", // RPL_WHOISSECURE (secure connection)
		" 301 ", // RPL_AWAY
		" 401 ", // ERR_NOSUCHNICK
	}
	
	// Check for WHOIS response codes
	for _, code := range whoisCodes {
		if strings.Contains(line, code) {
			return true
		}
	}
	
	return false
}
func (c *Client) isWhoisRequest(message string) bool {
	return strings.HasPrefix(strings.ToUpper(message), "WHOIS ")
}

func (c *Client) updateChannelModes(ch *Channel, modes, args string) {
	if len(modes) == 0 {
		return
	}

	add := true
	argIndex := 0
	argList := strings.Fields(args)

	for _, mode := range modes {
		switch mode {
		case '+':
			add = true
		case '-':
			add = false
		case 'o', 'v', 'h', 'a', 'q': // User modes
			if argIndex < len(argList) {
				nick := argList[argIndex]
				modeStr := string(mode)
				ch.UpdateUserModes(nick, modeStr, add)
				argIndex++
			}
		case 'k': // Channel key
			if add && argIndex < len(argList) {
				ch.Key = argList[argIndex]
				argIndex++
			} else if !add {
				ch.Key = ""
			}
		case 'l': // User limit
			if add && argIndex < len(argList) {
				if limit, err := strconv.Atoi(argList[argIndex]); err == nil {
					ch.Limit = limit
				}
				argIndex++
			} else if !add {
				ch.Limit = 0
			}
		default:
			// Channel modes like +t, +n, etc.
			if add {
				if !strings.Contains(ch.Modes, string(mode)) {
					ch.Modes += string(mode)
				}
			} else {
				ch.Modes = strings.ReplaceAll(ch.Modes, string(mode), "")
			}
		}
	}
}
func (c *Client) GetMyPrefix(channel string) string {
    key := strings.ToLower(strings.TrimSpace(channel))
    c.whoBotPrefixMu.RLock()
    defer c.whoBotPrefixMu.RUnlock()
    if c.whoBotPrefix == nil {
        return ""
    }
    return c.whoBotPrefix[key]
}