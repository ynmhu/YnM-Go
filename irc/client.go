package irc

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	//"time"

	"github.com/ynmhu/YnM-Go/config"
)

type Message struct {
	Sender  string
	Channel string
	Text    string
}

type Client struct {
	conn      net.Conn
	OnPong func(pongID string)
	config    *config.Config
	OnConnect func()
	OnMessage func(Message)
	mu        sync.Mutex // Írási szinkronizációhoz
	connected bool
}

// Konstruktor
func NewClient(cfg *config.Config) *Client {
	return &Client{config: cfg}
}

// Kapcsolódás az IRC szerverhez
func (c *Client) Connect() error {
	conn, err := net.Dial("tcp", c.config.Server)
	if err != nil {
		return fmt.Errorf("cannot connect to server: %w", err)
	}
	c.conn = conn
	c.connected = true

	// Bejelentkezés
	c.SendRaw(fmt.Sprintf("NICK %s", c.config.Nick))
	c.SendRaw(fmt.Sprintf("USER %s 8 * :%s", c.config.User, c.config.User))

	// Üzenetek olvasása külön goroutine-ban
	go c.readLoop()

	// OnConnect callback hívása, ha van
	if c.OnConnect != nil {
		c.OnConnect()
	}

	return nil
}

// Kapcsolat bontása
func (c *Client) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
		c.connected = false
	}
}

// Csatlakozás csatornához
func (c *Client) Join(channel string) {
	c.SendRaw(fmt.Sprintf("JOIN %s", channel))
}

// Privát üzenet küldése
func (c *Client) SendMessage(target, text string) {
	c.SendRaw(fmt.Sprintf("PRIVMSG %s :%s", target, text))
}

// Alacsony szintű üzenetküldés - szálbiztos
func (c *Client) SendRaw(message string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		return fmt.Errorf("not connected")
	}

	_, err := c.conn.Write([]byte(message + "\r\n"))
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	fmt.Println(">>", message) // Debug log küldött üzenetekhez
	return nil
}

// Folyamatos olvasás a szervertől
func (c *Client) readLoop() {
	reader := bufio.NewReader(c.conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// Hiba vagy kapcsolat bontás
			c.Disconnect()
			break
		}

		line = strings.TrimSpace(line)
		fmt.Println("<<", line) // Debug log érkező üzenetekhez

		// PING üzenetre válasz PONG-gal
		if strings.Contains(line, " PONG ") {
			parts := strings.Split(line, " ")
			if len(parts) >= 4 {
				pongID := strings.TrimPrefix(parts[3], ":")
				if c.OnPong != nil {
					c.OnPong(pongID)
				}
			}
			continue
		}


		// Privát üzenet parse és callback hívás
		if c.OnMessage != nil {
			if msg := parseMessage(line); msg != nil {
				c.OnMessage(*msg)
			}
		}
	}
}

// Egyszerű IRC PRIVMSG üzenet feldolgozó
func parseMessage(line string) *Message {
	parts := strings.Split(line, " ")
	if len(parts) < 4 || parts[1] != "PRIVMSG" {
		return nil
	}

	sender := ""
	if idx := strings.Index(parts[0], "!"); idx > 0 {
		sender = parts[0][1:idx]
	} else if len(parts[0]) > 1 {
		sender = parts[0][1:]
	}

	channel := parts[2]
	text := strings.Join(parts[3:], " ")
	if len(text) > 0 {
		text = text[1:] // Első karakter ':' eltávolítása
	}

	return &Message{
		Sender:  sender,
		Channel: channel,
		Text:    text,
	}
}
