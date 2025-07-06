// ============================================================================
//  Szerzői jog © 2024 Markus (markus@ynm.hu)
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
	OnLoginFailed func(reason string)
	OnLoginSuccess func()
	mu              sync.Mutex
	connected       bool
	disconnectChan  chan struct{}
	reconnecting    bool
	loggedIn        bool
}


// ─────────────────────── Konstruktor ─────────────────────────

func NewClient(cfg *config.Config) *Client {
	c := &Client{
		config:         cfg,
		disconnectChan: make(chan struct{}, 1),
	}
	// reconnect‑figyelő goroutine (ha be van állítva)
	if cfg.ReconnectOnDisconnect > 0 {
		go c.reconnectLoop()
	}
	return c
}

// ─────────────────────── Kapcsolódás ─────────────────────────

func (c *Client) Connect() error {
	conn, err := net.Dial("tcp", c.config.Server)
	if err != nil {
		return fmt.Errorf("cannot connect to server: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.loggedIn = false  // Itt reseteljük, hogy újra loginoljon a bot
	c.mu.Unlock()

	// IRC‑handshake
	c.SendRaw(fmt.Sprintf("NICK %s", c.config.Nick+"_"))
	c.SendRaw(fmt.Sprintf("USER %s YnM-Go :%s", c.config.User, c.config.User))

	// olvasó‑ciklus
	go c.readLoop()

	// callback az első sikeres csatlakozáskor
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
	c.mu.Unlock()

	// jelezzük a reconnect‑ciklusnak
	select { case c.disconnectChan <- struct{}{}: default: }
}

// ──────────────────── Csatorna / üzi küldése ─────────────────

func (c *Client) Join(channel string) {
	c.SendRaw("JOIN " + channel)
}

func (c *Client) SendMessage(target, text string) {
	c.SendRaw(fmt.Sprintf("PRIVMSG %s :%s", target, text))
}

// szálbiztos nyers küldés
func (c *Client) SendRaw(msg string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		return fmt.Errorf("not connected")
	}
	_, err := c.conn.Write([]byte(msg + "\r\n"))
	if err == nil {
		fmt.Println(">>", msg)
	}
	return err
}

// ─────────────────────── Olvasó‑ciklus ───────────────────────

func (c *Client) readLoop() {
	reader := bufio.NewReader(c.conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// bontás → reconnect jelzés
			c.Disconnect()
			return
		}

		line = strings.TrimSpace(line)
		fmt.Println("<<", line)

		// PING → PONG
		if strings.HasPrefix(line, "PING ") {
			c.SendRaw(strings.Replace(line, "PING", "PONG", 1))
			continue
		}
		// Auth Error
		if strings.Contains(line, "NickServ") && strings.Contains(line, "Authentication failed") {
			if c.OnLoginFailed != nil {
				c.OnLoginFailed("NickServ Authentication failed: Hibás jelszó vagy nincs regisztrált fiók")
			}
		}
		
if strings.HasPrefix(line, ":NickServ!NickServ@") && strings.Contains(line, "NOTICE") && strings.Contains(line, "You're now logged in") {
    c.mu.Lock()
    if !c.loggedIn {
        c.loggedIn = true
        c.mu.Unlock()
        if c.OnLoginSuccess != nil {
            c.OnLoginSuccess()
        }
    } else {
        c.mu.Unlock()
    }
}



		// PONG callback
		if strings.Contains(line, " PONG ") {
			parts := strings.Split(line, " ")
			if len(parts) >= 4 && c.OnPong != nil {
				c.OnPong(strings.TrimPrefix(parts[3], ":"))
			}
			continue
		}

		// PRIVMSG feldolgozás
		if msg := parseMessage(line); msg != nil && c.OnMessage != nil {
			c.OnMessage(*msg)
		}
	}
}

// ────────────────────── Reconnect‑ciklus ─────────────────────

func (c *Client) reconnectLoop() {
	for range c.disconnectChan {
		if c.reconnecting {
			continue
		}
		c.reconnecting = true
		time.Sleep(c.config.ReconnectOnDisconnect)

		for {
			if err := c.Connect(); err == nil {
				c.reconnecting = false
				break
			}
			// újrapróba ugyanennyi idő múlva
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

	sender := strings.Split(parts[0][1:], "!")[0]
	channel := parts[2]
	text := strings.TrimPrefix(strings.Join(parts[3:], " "), ":")

	return &Message{Sender: sender, Channel: channel, Text: text}
}

