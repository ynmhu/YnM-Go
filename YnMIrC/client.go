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
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"
	"log"


	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMDebug"
)

// ─────────────────────── Konstruktor ─────────────────────────

func NewClient(cfg *YnMConfig.Config) *Client {
	c := &Client{
		config:         cfg,
		disconnectChan: make(chan struct{}, 1),
		useSASL:        cfg.UseSASL,
		saslUser:       cfg.SASLUser,
		saslPass:       cfg.SASLPass,
		channels:       make(map[string]*Channel),
		nick:           cfg.NickName,
		sendQueue:      make(chan string, 100),
		sendDone:       make(chan struct{}),
		whoisData:      make(map[string]*WhoisData),
		whoisChannels:	make(map[string][]chan *WhoisData), 
		whoBotPrefix:   make(map[string]string),
		//messageDelay:   time.Duration(client.messageDelay) * time.Millisecond,
		messageDelay:  1 * time.Second,
		
	}
	
	
	// Start send queue handler
	go c.sendQueueHandler()
	
	// Start reconnect loop if enabled
	if cfg.ReconnectOnDisconnect > 0 {
		go c.reconnectLoop()
	}
	return c
}

// ─────────────────────── Configuration and State ─────────────────────────

func (c *Client) Config() *YnMConfig.Config {
	return c.config
}
func normalizeChannel(name string) string {
	return strings.ToLower(name)
}

func (c *Client) GetConfig() *YnMConfig.Config {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    // Make a copy to prevent external modifications
    cfgCopy := *c.config
    
    // Set default version if empty
    if cfgCopy.Version == "" {
        cfgCopy.Version = "1.0.0" // Your default version here
    }
    
    return &cfgCopy
}

func (c *Client) GetNick() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nick
}
func (c *Client) GetLoggedUsers() map[string]bool {
    // Return a map of logged users if you track them
    // For now, return empty map or implement user tracking
    return make(map[string]bool)
}

func (c *Client) GetJoinedChannels() map[string]*Channel {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.channels
}

func (c *Client) IsLoggedIn() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loggedIn
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *Client) GetStartedAt() time.Time {
    return c.startedAt
}


func (c *Client) IsChannel(name string) bool {
	return len(name) > 0 && (name[0] == '#' || name[0] == '&' || name[0] == '+' || name[0] == '!')
}

func (c *Client) Channels() map[string]*Channel {
    return c.channels
}

func (c *Client) IsTLS() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.conn == nil {
		return false
	}
	_, ok := c.conn.(*tls.Conn)
	return ok
}
func (c *Client) GetDisconnectChan() <-chan struct{} {
    return c.disconnectChan
}

// ─────────────────────── Channel Management ─────────────────────────

func (c *Client) GetChannels() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	channels := make([]string, 0, len(c.channels))
	for name := range c.channels {
		channels = append(channels, name)
	}
	return channels
}

func (c *Client) GetChannelUsers(channel string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := strings.ToLower(channel)
	if ch, exists := c.channels[key]; exists {
		return ch.GetUserList()
	}
	return nil
}

func (c *Client) GetUserModes(channel, nick string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
		key := strings.ToLower(channel)
	if ch, exists := c.channels[key]; exists {
		if user, userExists := ch.GetUser(nick); userExists {
			return user.Modes
		}
	}
	return ""
}

// YnMIrC/client.go - add hozzá:

func (c *Client) GetChannelUserHostmask(channel, nick string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
		key := strings.ToLower(channel)
	if ch, exists := c.channels[key]; exists {
		if user, userExists := ch.GetUser(nick); userExists {
			if user.Hostmask != "" {
				return user.Hostmask
			}
		}
	}
	return nick + "!*@*" // fallback
}

func (c *Client) getOrCreateChannel(name string) *Channel {
	key := strings.ToLower(name)
	if ch, exists := c.channels[key]; exists {
		return ch
	}
	ch := NewChannel(name)
	ch.Name = name // eredeti, esetérzékeny név tárolása
	c.channels[key] = ch
	return ch
}

// ─────────────────────── Connection Management ─────────────────────────

func (c *Client) Connect() error {
    var conn net.Conn
    var err error
    var tlsConfig *tls.Config

    server := c.config.Server
    port := c.config.Port
    if c.config.UseTLS && c.config.TLSPort != "" {
        port = c.config.TLSPort
    }

    addr := server + ":" + port

    if c.config.UseTLS {
        tlsConfig = &tls.Config{
            InsecureSkipVerify: true,
            ServerName:         server,
        }

        if c.config.TLSCert != "" && c.config.TLSKey != "" {
            cert, certErr := tls.LoadX509KeyPair(c.config.TLSCert, c.config.TLSKey)
            if certErr == nil {
                tlsConfig.Certificates = []tls.Certificate{cert}
            } else {
                fmt.Printf("⚠️ TLS cert/key betöltési hiba: %v\n", certErr)
            }
        }

        conn, err = tls.Dial("tcp", addr, tlsConfig)
    } else {
        conn, err = net.Dial("tcp", addr)
    }
    if err != nil {
        return fmt.Errorf("kapcsolódási hiba: %v", err)
    }

    c.mu.Lock()
    c.conn = conn
    c.connected = true
    c.loggedIn = false
    c.reconnecting = false
    c.mu.Unlock()

    go c.readLoop()

    if c.useSASL {
        c.SendRaw("CAP REQ :sasl")
    } else {
        c.SendRaw(fmt.Sprintf("NICK %s", c.config.NickName))
        c.SendRaw(fmt.Sprintf("USER %s 0 * :%s", c.config.UserName, c.config.RealName))
    }

    if c.OnConnect != nil {
        c.OnConnect()
    }
    return nil
}

func (c *Client) Disconnect(manual bool) {
    c.mu.Lock()
    c.manualDisconnect = manual

    if c.conn != nil {
        c.conn.Write([]byte("QUIT :Client disconnecting\r\n"))
        c.conn.Close()
        c.conn = nil
    }
    c.connected = false
    c.loggedIn = false
    c.channels = make(map[string]*Channel)
    c.mu.Unlock()

    // csak akkor jelezzük az auto reconnectet, ha nem manuális
    if !manual {
        select {
        case c.disconnectChan <- struct{}{}:
        default:
        }
    }
}
func (c *Client) reconnectLoop() {
    for range c.disconnectChan {
        c.mu.Lock()
        if c.manualDisconnect {
            c.manualDisconnect = false
            c.reconnecting = false
            c.mu.Unlock()
            continue
        }
        if c.reconnecting {
            c.mu.Unlock()
            continue
        }
        c.reconnecting = true
        c.mu.Unlock()

        log.Println("🔄 Újracsatlakozás...")
        time.Sleep(c.config.ReconnectOnDisconnect)

        for {
            if err := c.Connect(); err == nil {
                log.Println("✔️ Újracsatlakozás sikeres")
                c.mu.Lock()
                c.reconnecting = false
                c.mu.Unlock()
                break
            }
            time.Sleep(c.config.ReconnectOnDisconnect)
        }
    }
}
// ─────────────────────── Message Sending ─────────────────────────

func (c *Client) sendQueueHandler() {
	for {
		select {
		case msg, ok := <-c.sendQueue:
			if !ok {
				return
			}
			c.sendRawDirect(msg)
			// Rate limiting
			time.Sleep(c.messageDelay)
		case <-c.sendDone:
			return
		}
	}
}

func (c *Client) sendRawDirect(msg string) error {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return fmt.Errorf("not connected")
	}

	_, err := conn.Write([]byte(msg + "\r\n"))
	if err != nil {
		log.Printf("❌ Write hiba: %v", err)
		go c.Disconnect(false)
		return err
	}

	return nil
}

func (c *Client) SendRaw(msg string) error {
    return c.sendRawDirect(msg)
}

func (c *Client) SendMessage(target, text string) {
    c.messageMutex.Lock()
    defer c.messageMutex.Unlock()
    
    // Várakozás, ha túl gyors
    if !c.lastMessageTime.IsZero() {
        elapsed := time.Since(c.lastMessageTime)
        if elapsed < c.messageDelay {
            time.Sleep(c.messageDelay - elapsed)
        }
    }
    
    const maxLength = 500
    
    if len(text) <= maxLength {
        c.SendRaw(fmt.Sprintf("PRIVMSG %s :%s", target, text))
        c.lastMessageTime = time.Now()
        return
    }
    
    // Split the message
    for len(text) > 0 {
        end := maxLength
        if end > len(text) {
            end = len(text)
        }
        
        if end < len(text) {
            lastSpace := strings.LastIndex(text[:end], " ")
            if lastSpace > end/2 {
                end = lastSpace
            }
        }
        
        c.SendRaw(fmt.Sprintf("PRIVMSG %s :%s", target, text[:end]))
        text = strings.TrimSpace(text[end:])
        
        // Splitelt részek között is frissítsük
        c.lastMessageTime = time.Now()
        
        if len(text) > 0 {
            time.Sleep(c.messageDelay)
        }
    }
}
// ─────────────────────── Channel Operations ─────────────────────────

func (c *Client) Join(channel string) {
	if !c.IsChannel(channel) {
		return
	}
	
	c.mu.RLock()
	_, alreadyIn := c.channels[strings.ToLower(channel)]
	c.mu.RUnlock()
	
	if !alreadyIn {
		c.SendRaw("JOIN " + channel)
	}
}

func (c *Client) Part(channel string, reason string) {
	if !c.IsChannel(channel) {
		return
	}
	
	c.mu.Lock()
	key := strings.ToLower(channel)
	delete(c.channels, key)
	c.mu.Unlock()
	
	if reason != "" {
		c.SendRaw(fmt.Sprintf("PART %s :%s", channel, reason))
	} else {
		c.SendRaw("PART " + channel)
	}
	
	if c.OnChannelParted != nil {
		c.OnChannelParted(channel)
	}
}

// ─────────────────────── WHOIS Handling ─────────────────────────

func (c *Client) GetWhoisData(nick string) *WhoisData {
    nickKey := strings.ToLower(strings.TrimSpace(nick))
    respChan := make(chan *WhoisData, 1)

    c.whoisMutex.Lock()
    if c.whoisChannels == nil {
        c.whoisChannels = make(map[string][]chan *WhoisData)
    }
    c.whoisChannels[nickKey] = append(c.whoisChannels[nickKey], respChan)
    c.whoisMutex.Unlock()

    // WHOIS küldése a lock UTÁN
    c.SendRawSilent(fmt.Sprintf("WHOIS %s", nickKey))

    select {
    case data := <-respChan:
        return data
    case <-time.After(5 * time.Second):
        // cleanup
        c.whoisMutex.Lock()
        if chans, ok := c.whoisChannels[nickKey]; ok {
            newChans := make([]chan *WhoisData, 0, len(chans))
            for _, ch := range chans {
                if ch != respChan {
                    newChans = append(newChans, ch)
                }
            }
            if len(newChans) == 0 {
                delete(c.whoisChannels, nickKey)
            } else {
                c.whoisChannels[nickKey] = newChans
            }
        }
        c.whoisMutex.Unlock()

        close(respChan)
        return nil
    }
}

func (c *Client) GetWhoisChannel(nick string) chan *WhoisData {
	nickKey := strings.ToLower(strings.TrimSpace(nick))

	c.whoisMutex.Lock()
	defer c.whoisMutex.Unlock()

	if c.whoisChannels == nil {
		c.whoisChannels = make(map[string][]chan *WhoisData)
	}

	respChan := make(chan *WhoisData, 1)
	c.whoisChannels[nickKey] = append(c.whoisChannels[nickKey], respChan)

	return respChan
}

func (c *Client) CleanupWhoisChannel(nick string) {
    nickKey := strings.ToLower(strings.TrimSpace(nick))

    c.whoisMutex.Lock()
    defer c.whoisMutex.Unlock()

    if c.whoisChannels != nil {
        delete(c.whoisChannels, nickKey)
    }
}

func (c *Client) RequestWhois(nick string) {
    nickKey := strings.ToLower(strings.TrimSpace(nick))
    c.SendRawSilent("WHOIS " + nickKey)
}

func (c *Client) HandleWhoisData(whois *WhoisData) {
    nickKey := strings.ToLower(strings.TrimSpace(whois.Nick))

    c.whoisMutex.Lock()
    chans, ok := c.whoisChannels[nickKey]
    if ok {
        delete(c.whoisChannels, nickKey)
    }
    c.whoisMutex.Unlock()

    if ok {
        for _, ch := range chans {
            select {
            case ch <- whois:
            default:
            }
            close(ch)
        }
    }
}
// ─────────────────────── NickServ Authentication ─────────────────────────

func (c *Client) IdentifyNickServ() error {
	if !c.config.NickServLogin {  // ← JAVÍTÁS
		return nil
	}
	if c.config.NickservBotnick == "" || c.config.NickservNick == "" || c.config.NickservPass == "" {
		return fmt.Errorf("NickServ adatok hiányoznak a konfigurációból")
	}
	// Non-blocking authentication
	go func() {
		time.Sleep(3 * time.Second)
		// Change to registered nick if needed
		if c.GetNick() != c.config.NickservNick {
			nickChangeCmd := fmt.Sprintf("NICK %s", c.config.NickservNick)
			if err := c.SendRaw(nickChangeCmd); err != nil {
				fmt.Printf("❌ Nem sikerült nicket váltani: %v\n", err)
			}
		}
		// Identify
		identifyCmd := fmt.Sprintf("PRIVMSG %s :IDENTIFY %s %s", 
			c.config.NickservBotnick, c.config.NickservNick, c.config.NickservPass)
		if err := c.SendRaw(identifyCmd); err != nil {
			fmt.Printf("❌ Nem sikerült azonosítani: %v\n", err)
			if c.OnLoginFailed != nil {
				c.OnLoginFailed("Nem sikerült azonosítani: " + err.Error())
			}
		}
	}()
	return nil
}


// ─────────────────────── Undernet Authentication ─────────────────────────
func (c *Client) UndernetLogin() {
    cfg := c.config.Undernet
    //YnMDebug.Logf("🔍 DEBUG Undernet: cfg.Enabled = %v", cfg.Enabled)

    if !cfg.Enabled {
        //fmt.Println("🔒 Undernet login disabled")
        return
    }

    // 1️⃣ MODE beállítás mindig fusson, LOGIN előtt, login flag nélkül
    if cfg.Modes != "" {
        go func() {
            //YnMDebug.Logf("🔍 DEBUG: Will set modes '%s' after delay", cfg.Modes)
            time.Sleep(3 * time.Second)

            currentNick := c.GetNick()
            modeCmd := fmt.Sprintf("MODE %s %s", currentNick, cfg.Modes)
            if err := c.SendRaw(modeCmd); err != nil {
                //YnMDebug.Logf("❌ Failed to set Undernet modes: %v", err)
            } else {
                //YnMDebug.Logf("🎭 Set Undernet modes %s for %s", cfg.Modes, currentNick)
            }
        }()
    } else {
        YnMDebug.Log("🔍 DEBUG: No modes to set")
    }

    // 2️⃣ LOGIN X csak akkor, ha van user/pass
    if cfg.Username == "" || cfg.Password == "" {
        YnMDebug.Log("🔍 DEBUG: No login credentials, skipping X login")
        return
    }

    c.mu.Lock()
    if c.undernetLoggedIn {
        c.mu.Unlock()
        YnMDebug.Log("✅ Already logged in to Undernet X")
        return
    }
    c.mu.Unlock()

    loginCmd := fmt.Sprintf(
        "PRIVMSG X@channels.undernet.org :LOGIN %s %s",
        cfg.Username, cfg.Password,
    )

    if err := c.SendRaw(loginCmd); err != nil {
        //YnMDebug.Logf("❌ Failed to send Undernet login command: %v", err)
        return
    }

    YnMDebug.Log("📤 Undernet X login command sent")
}

func (c *Client) handleNotice(prefix, target, message string) {
    // Csak az X szolgáltatás üzenetei
    if strings.Contains(prefix, "cservice@undernet.org") {
        // Sikeres login
        if strings.Contains(message, "*SUCCESS*") {
            c.SetUndernetAuthenticated(true)
            YnMDebug.Log("✅ Undernet X login successful (flag set)")
        }

        // Sikertelen login
        if strings.Contains(message, "Invalid password") || strings.Contains(message, "*FAILED*") {
            c.SetUndernetAuthenticated(false)
            YnMDebug.Log("❌ Undernet X login failed (flag set)")
        }
    

        // Sikertelen login
        if strings.Contains(message, "Invalid password") || strings.Contains(message, "*FAILED*") {
            c.SetUndernetAuthenticated(false)
            YnMDebug.Log("❌ Undernet X login failed (flag set)")
        }
    }
}


func (c *Client) IsUndernetAuthenticated() bool {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.undernetLoggedIn
}

func (c *Client) SetUndernetAuthenticated(status bool) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.undernetLoggedIn = status
}
// Opcionális: Getter az eredeti undernetLoggedIn mezőhöz
func (c *Client) GetUndernetLoggedIn() bool {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.undernetLoggedIn
}

// ─────────────────────── Event Handlers ─────────────────────────

func (c *Client) AddHandler(event string, handler func(*Client, Message)) {
	existingHandler := c.OnMessage
	
	c.OnMessage = func(msg Message) {
		if existingHandler != nil {
			existingHandler(msg)
		}
		
		if msg.Command == event {
			handler(c, msg)
		}
	}
}


// ─────────────────────── Cleanup ─────────────────────────
func (c *Client) SendRawSilent(message string) error {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return fmt.Errorf("not connected")
	}
	
	// Send directly without going through sendQueue and without any logging
	_, err := conn.Write([]byte(message + "\r\n"))
	return err
}
func (c *Client) Close() {
	c.Disconnect(true)
	close(c.sendDone)
	close(c.sendQueue)
}