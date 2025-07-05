package irc

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"YnM-Go/config"
)

type Message struct {
	Sender  string
	Channel string
	Text    string
}

type Client struct {
	conn      net.Conn
	config    *config.Config
	OnConnect func()
	OnMessage func(Message)
}

func NewClient(cfg *config.Config) *Client {
	return &Client{config: cfg}
}

func (c *Client) Connect() error {
	conn, err := net.Dial("tcp", c.config.Server)
	if err != nil {
		return err
	}
	c.conn = conn

	// Bejelentkezés
	fmt.Fprintf(c.conn, "NICK %s\r\n", c.config.Nick)
	fmt.Fprintf(c.conn, "USER %s\r\n", c.config.User)

	// Üzenetek olvasása
	go c.readLoop()

	// OnConnect esemény
	if c.OnConnect != nil {
		c.OnConnect()
	}

	return nil
}

func (c *Client) Disconnect() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) Join(channel string) {
	fmt.Fprintf(c.conn, "JOIN %s\r\n", channel)
}

func (c *Client) SendMessage(target, text string) {
	fmt.Fprintf(c.conn, "PRIVMSG %s :%s\r\n", target, text)
}

func (c *Client) readLoop() {
	reader := bufio.NewReader(c.conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// Hibakezelés
			break
		}

		line = strings.TrimSpace(line)
		fmt.Println("<<", line)

		// PING válasz
		if strings.HasPrefix(line, "PING ") {
			pong := strings.TrimPrefix(line, "PING ")
			fmt.Fprintf(c.conn, "PONG %s\r\n", pong)
			continue
		}

		// Üzenetek parse-olása
		if c.OnMessage != nil {
			if msg := parseMessage(line); msg != nil {
				c.OnMessage(*msg)
			}
		}
	}
}

func parseMessage(line string) *Message {
	parts := strings.Split(line, " ")
	if len(parts) < 4 || parts[1] != "PRIVMSG" {
		return nil
	}

	sender := strings.Split(parts[0], "!")[0][1:]
	channel := parts[2]
	text := strings.Join(parts[3:], " ")[1:]

	return &Message{
		Sender:  sender,
		Channel: channel,
		Text:    text,
	}
}
