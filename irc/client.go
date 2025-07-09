// ============================================================================
//  Szerzői jog © 2025 Markus (markus@ynm.hu)
//  https://ynm.hu   – főoldal
//  https://forum.ynm.hu   – hivatalos fórum
//  https://bot.ynm.hu     – bot oldala és dokumentáció
//
//  Minden jog fenntartva. A kód Markus tulajdona, tilos terjeszteni vagy
//  módosítani a szerző írásos engedélye nélkül.
//
//  Ez a fájl a YnM-Go IRC-bot rendszerének része.
// ============================================================================

package irc

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
	"crypto/tls"
	"encoding/base64"

	"github.com/ynmhu/YnM-Go/config"
)

// ──────────────────────── Típusok ────────────────────────────

// bejövő PRIVMSG
type Message struct {
	Sender  string
	Channel string
	Text    string
}

// fő kliens‑struktúra
type Client struct {
	conn            net.Conn
	config          *config.Config
	OnConnect       func()
	OnMessage       func(Message)
	OnPong          func(pongID string)
	OnLoginFailed   func(reason string)
	OnLoginSuccess  func()
	mu              sync.RWMutex
	connected       bool
	disconnectChan  chan struct{}
	reconnecting    bool
	loggedIn        bool
	
	// felhasználók és csatornák követése
	loggedUsers    map[string]struct{}
	joinedChannels map[string]struct{}
	nick           string
	
	// SASL beállítások
	useSASL  bool
	saslUser string
	saslPass string
	
	// üzenet küldés queue (optimalizálás)
	sendQueue chan string
	sendDone  chan struct{}
}

// ─────────────────────── Konstruktor ─────────────────────────

func NewClient(cfg *config.Config) *Client {
	c := &Client{
		config:         cfg,
		disconnectChan: make(chan struct{}, 1),
		useSASL:        cfg.UseSASL,
		saslUser:       cfg.SASLUser,
		saslPass:       cfg.SASLPass,
		loggedUsers:    make(map[string]struct{}),
		joinedChannels: make(map[string]struct{}),
		nick:           cfg.NickName,
		sendQueue:      make(chan string, 100),
		sendDone:       make(chan struct{}),
	}
	
	// indítjuk a send queue kezelőt
	go c.sendQueueHandler()
	
	// reconnect figyelő goroutine
	if cfg.ReconnectOnDisconnect > 0 {
		go c.reconnectLoop()
	}
	return c
}

// ─────────────────────── Getter metódusok ─────────────────────────

func (c *Client) GetLoggedUsers() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	users := make([]string, 0, len(c.loggedUsers))
	for u := range c.loggedUsers {
		users = append(users, u)
	}
	return users
}

func (c *Client) GetJoinedChannels() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	channels := make([]string, 0, len(c.joinedChannels))
	for ch := range c.joinedChannels {
		channels = append(channels, ch)
	}
	return channels
}

func (c *Client) GetNick() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nick
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

// ─────────────────────── Kapcsolódás ─────────────────────────

func (c *Client) IsTLS() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.conn == nil {
		return false
	}
	_, ok := c.conn.(*tls.Conn)
	return ok
}

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

	// Kezdeti parancsok küldése
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

// ─────────────────────── Leválasztás ─────────────────────────

func (c *Client) Disconnect() {
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connected = false
	c.loggedIn = false
	c.mu.Unlock()

	// jelezzük a reconnect‑ciklusnak
	select {
	case c.disconnectChan <- struct{}{}:
	default:
	}
}

// ──────────────────── Üzenet küldés optimalizálva ─────────────────

func (c *Client) sendQueueHandler() {
	for {
		select {
		case msg, ok := <-c.sendQueue:
			if !ok {
				return
			}
			c.sendRawDirect(msg)
			// kis késleltetés az IRC szerver túlterhelésének elkerülése miatt
			time.Sleep(100 * time.Millisecond)
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
	if err == nil {
		fmt.Println(">>", msg)
	}
	return err
}

// ──────────────────── Csatorna / üzenet küldése ─────────────────

func (c *Client) Join(channel string) {
	c.SendRaw("JOIN " + channel)
}

func (c *Client) SendMessage(target, text string) {
	c.SendRaw(fmt.Sprintf("PRIVMSG %s :%s", target, text))
}

func (c *Client) SendRaw(msg string) error {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		return fmt.Errorf("not connected")
	}

	select {
	case c.sendQueue <- msg:
		return nil
	default:
		return fmt.Errorf("send queue full")
	}
}

// ─────────────────────── Olvasó‑ciklus ───────────────────────

func (c *Client) readLoop() {
	defer func() {
		c.Disconnect()
	}()

	reader := bufio.NewReader(c.conn)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Olvasási hiba: %v\n", err)
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		fmt.Println("<<", line)

		// PING → PONG response
		if strings.HasPrefix(line, "PING ") {
			c.SendRaw(strings.Replace(line, "PING", "PONG", 1))
			continue
		}

		// Nick foglalt/rezervált kezelése (433)
		if strings.Contains(line, " 433 ") || strings.Contains(line, "NICKNAME_RESERVED") || strings.Contains(line, "Nickname is reserved") {
			c.handleNickInUse()
			continue
		}

		// JOIN események kezelése
		if strings.Contains(line, " JOIN ") {
			c.handleJoin(line)
			continue
		}

		// NAMES lista kezelése (353)
		if strings.Contains(line, " 353 ") {
			c.handleNames(line)
			continue
		}

		// SASL autentikáció kezelése
		if c.handleSASL(line) {
			continue
		}

		// NickServ kezelése
		if c.handleNickServ(line) {
			continue
		}

		// Sikeres kapcsolódás (001) - javított logika
		if strings.Contains(line, " 001 ") {
			c.handleWelcome()
			continue
		}

		// PONG callback
		if strings.Contains(line, " PONG ") {
			c.handlePong(line)
			continue
		}

		// PRIVMSG feldolgozás
		if msg := parseMessage(line); msg != nil && c.OnMessage != nil {
			c.OnMessage(*msg)
		}
	}
}

// ─────────────────────── Segéd metódusok ─────────────────────────

func (c *Client) handleNickInUse() {
	c.mu.Lock()
	oldNick := c.nick
	newNick := fmt.Sprintf("%s_%d", c.config.NickName, time.Now().Unix()%10000)
	c.nick = newNick
	c.mu.Unlock()
	
	fmt.Printf("Nick %s foglalt/rezervált, új nick: %s\n", oldNick, newNick)
	c.SendRaw("NICK " + newNick)
}

func (c *Client) handleJoin(line string) {
	parts := strings.SplitN(line, " ", 4)
	if len(parts) < 3 {
		return
	}

	prefix := parts[0]
	var channel string
	if len(parts) >= 4 {
		channel = strings.TrimPrefix(parts[3], ":")
	} else {
		channel = parts[2]
	}

	if strings.HasPrefix(prefix, ":") {
		nickParts := strings.SplitN(strings.TrimPrefix(prefix, ":"), "!", 2)
		if len(nickParts) > 0 {
			nick := nickParts[0]

			c.mu.Lock()
			if nick == c.nick {
				c.joinedChannels[channel] = struct{}{}
			}
			c.loggedUsers[nick] = struct{}{}
			c.mu.Unlock()
		}
	}
}

func (c *Client) handleNames(line string) {
	parts := strings.SplitN(line, " :", 2)
	if len(parts) < 2 {
		return
	}

	usersPart := parts[1]
	lineParts := strings.Split(line, " ")
	if len(lineParts) < 5 {
		return
	}
	channel := lineParts[4]

	c.mu.Lock()
	c.joinedChannels[channel] = struct{}{}
	nicks := strings.Fields(usersPart)
	for _, nick := range nicks {
		// Csatorna operator/voice előtagok eltávolítása
		nick = strings.TrimLeft(nick, "+%@&~!")
		if nick != "" {
			c.loggedUsers[nick] = struct{}{}
		}
	}
	c.mu.Unlock()
}

func (c *Client) handleSASL(line string) bool {
	if !c.useSASL {
		return false
	}

	// CAP ACK :sasl → send AUTHENTICATE PLAIN
	if strings.Contains(line, "CAP") && strings.Contains(line, "ACK") && strings.Contains(line, "sasl") {
		c.SendRaw("AUTHENTICATE PLAIN")
		return true
	}

	// AUTHENTICATE + → send auth data in base64
	if strings.HasPrefix(line, "AUTHENTICATE +") {
		authStr := "\x00" + c.saslUser + "\x00" + c.saslPass
		encoded := base64.StdEncoding.EncodeToString([]byte(authStr))
		maxLen := 400

		if len(encoded) == 0 {
			c.SendRaw("AUTHENTICATE +")
		} else {
			for i := 0; i < len(encoded); i += maxLen {
				end := i + maxLen
				if end > len(encoded) {
					end = len(encoded)
				}
				c.SendRaw("AUTHENTICATE " + encoded[i:end])
			}
			if len(encoded)%maxLen == 0 {
				c.SendRaw("AUTHENTICATE +")
			}
		}
		return true
	}

	// SASL sikeres (903)
	if strings.Contains(line, " 903 ") {
		fmt.Println("✔️ SASL autentikáció sikeres")
		c.mu.Lock()
		c.loggedIn = true
		c.mu.Unlock()

		c.SendRaw("CAP END")
		c.SendRaw(fmt.Sprintf("NICK %s", c.config.NickName))
		c.SendRaw(fmt.Sprintf("USER %s 0 * :%s", c.config.UserName, c.config.RealName))

		if c.OnLoginSuccess != nil {
			c.OnLoginSuccess()
		}
		return true
	}

	// SASL sikertelen (904 vagy 905)
	if strings.Contains(line, " 904 ") || strings.Contains(line, " 905 ") {
		fmt.Println("❌ SASL autentikáció sikertelen")
		c.SendRaw("CAP END")

		if c.OnLoginFailed != nil {
			c.OnLoginFailed("SASL autentikáció sikertelen")
		}
		return true
	}

	return false
}

func (c *Client) handleNickServ(line string) bool {
	// NickServ autentikáció sikertelen
	if strings.Contains(line, "NickServ") && strings.Contains(line, "Authentication failed") {
		if c.OnLoginFailed != nil {
			c.OnLoginFailed("NickServ autentikáció sikertelen: Hibás jelszó vagy nem regisztrált fiók")
		}
		return true
	}

	// Sikeres NickServ bejelentkezés
	if strings.HasPrefix(line, ":NickServ!NickServ@") && strings.Contains(line, "NOTICE") &&
		(strings.Contains(line, "You're now logged in") || strings.Contains(line, "You are now identified")) {
		c.mu.Lock()
		wasLoggedIn := c.loggedIn
		c.loggedIn = true
		c.mu.Unlock()

		fmt.Println("✔️ NickServ autentikáció sikeres")
		if !wasLoggedIn && c.OnLoginSuccess != nil {
			c.OnLoginSuccess()
		}
		return true
	}

	return false
}

func (c *Client) handleWelcome() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Javított logika a config alapján
	if c.config.UseSASL {
		// SASL esetén már történt az autentikáció
		return
	}

	if c.config.AutoLogin {
		// AutoLogin be van kapcsolva, várjuk a NickServ választ
		// A loggedIn flag-et a NickServ válasz fogja beállítani
		return
	}

	// AutoLogin kikapcsolva
	if c.config.AutoJoinWithoutLogin {
		// Engedélyezett a csatlakozás login nélkül
		c.loggedIn = true
		if c.OnLoginSuccess != nil {
			go c.OnLoginSuccess() // goroutine-ban, hogy ne blokkoljuk a readLoop-ot
		}
	}
	// Ha AutoJoinWithoutLogin is false, akkor nem csinálunk semmit
}

func (c *Client) handlePong(line string) {
	parts := strings.Split(line, " ")
	if len(parts) >= 4 && c.OnPong != nil {
		c.OnPong(strings.TrimPrefix(parts[3], ":"))
	}
}

// ────────────────────── Reconnect‑ciklus ─────────────────────

func (c *Client) reconnectLoop() {
	for range c.disconnectChan {
		c.mu.Lock()
		if c.reconnecting {
			c.mu.Unlock()
			continue
		}
		c.reconnecting = true
		c.mu.Unlock()

		fmt.Println("🔄 Újracsatlakozás...")
		time.Sleep(c.config.ReconnectOnDisconnect)

		for {
			if err := c.Connect(); err == nil {
				fmt.Println("✔️ Újracsatlakozás sikeres")
				break
			} else {
				fmt.Printf("❌ Újracsatlakozás sikertelen: %v\n", err)
			}
			time.Sleep(c.config.ReconnectOnDisconnect)
		}
	}
}

// ───────────────────── PRIVMSG parser ───────────────────────

func parseMessage(line string) *Message {
	parts := strings.Split(line, " ")
	if len(parts) < 4 || parts[1] != "PRIVMSG" {
		return nil
	}

	// sender legyen az egész hostmask: nick!user@host
	sender := parts[0]
	if strings.HasPrefix(sender, ":") {
		sender = sender[1:]
	}

	channel := parts[2]
	text := strings.TrimPrefix(strings.Join(parts[3:], " "), ":")

	return &Message{
		Sender:  sender,
		Channel: channel,
		Text:    text,
	}
}

// ───────────────────── NickServ azonosítás ───────────────────────

func (c *Client) IdentifyNickServ() error {
	if !c.config.AutoLogin {
		return nil
	}

	if c.config.NickservBotnick == "" || c.config.NickservNick == "" || c.config.NickservPass == "" {
		return fmt.Errorf("NickServ adatok hiányoznak a konfigurációból")
	}

	// Nem blokkoló várakozás
	go func() {
		time.Sleep(3 * time.Second)

		// Nick váltás a regisztrált nickre
		if c.GetNick() != c.config.NickservNick {
			nickChangeCmd := fmt.Sprintf("NICK %s", c.config.NickservNick)
			if err := c.SendRaw(nickChangeCmd); err != nil {
				fmt.Printf("❌ Nem sikerült nicket váltani: %v\n", err)
			}
		}

		// Azonosítás
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

// ───────────────────── Cleanup ───────────────────────

func (c *Client) Close() {
	c.Disconnect()
	close(c.sendDone)
	close(c.sendQueue)
}