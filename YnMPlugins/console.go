package ynm

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMDb"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
)

// GlobalLogBroadcaster - Globális log továbbító
var globalLogBroadcaster *LogBroadcaster

type LogBroadcaster struct {
	mu        sync.RWMutex
	listeners []chan string
	buffer    *RingBuffer
}

func init() {
	globalLogBroadcaster = &LogBroadcaster{
		listeners: make([]chan string, 0),
		buffer:    NewRingBuffer(1000),
	}
	
	// Átirányítjuk az összes log output-ot
	log.SetOutput(io.MultiWriter(os.Stdout, globalLogBroadcaster))
}

func (lb *LogBroadcaster) Write(p []byte) (n int, err error) {
	line := strings.TrimSpace(string(p))
	if line == "" {
		return len(p), nil
	}
	
	lb.buffer.Add(line)
	
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	
	for _, listener := range lb.listeners {
		select {
		case listener <- line:
		default:
			// Skip if channel is full
		}
	}
	
	return len(p), nil
}

func (lb *LogBroadcaster) Subscribe() chan string {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	
	ch := make(chan string, 100)
	lb.listeners = append(lb.listeners, ch)
	return ch
}

func (lb *LogBroadcaster) Unsubscribe(ch chan string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	
	for i, listener := range lb.listeners {
		if listener == ch {
			lb.listeners = append(lb.listeners[:i], lb.listeners[i+1:]...)
			close(ch)
			break
		}
	}
}

// Partyline Console
type PartylineConsole struct {
	bot         *YnMIrC.Client
	adminPlugin *owner.YnmAdminPlugin
	Db          *YnMDb.AdminDB
	
	listener    net.Listener
	port        int
	password    string
	bindAddr    string
	
	mu          sync.Mutex
	sessions    map[string]*ConsoleSession
	logBuffer   *RingBuffer
	broadcast   chan string
	
	startTime   time.Time
}

type ConsoleSession struct {
	conn       net.Conn
	nick       string
	authed     bool
	channels   map[string]bool
	writer     *bufio.Writer
	logStream  chan string
	stopStream chan struct{}
}

type RingBuffer struct {
	mu    sync.Mutex
	lines []string
	size  int
	pos   int
}

func NewPartylineConsole(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin, db *YnMDb.AdminDB, config YnMConfig.ConsoleConfig) *PartylineConsole {
	if config.Port == 0 {
		config.Port = 3333
	}
	if config.BindAddr == "" {
		config.BindAddr = "127.0.0.1"
	}
	if config.Password == "" {
		config.Password = "changeme"
		log.Println("⚠️  FIGYELEM: Alapértelmezett console jelszó!")
	}
	
	pc := &PartylineConsole{
		bot:         bot,
		adminPlugin: adminPlugin,
		Db:          db,
		port:        config.Port,
		password:    config.Password,
		bindAddr:    config.BindAddr,
		sessions:    make(map[string]*ConsoleSession),
		logBuffer:   NewRingBuffer(1000),
		broadcast:   make(chan string, 100),
		startTime:   time.Now(),
	}
	
	if config.Enabled {
		go pc.startServer()
		go pc.broadcastHandler()
		log.Printf("✅ Partyline console enabled on %s:%d", config.BindAddr, config.Port)
	} else {
		log.Println("ℹ️  Partyline console disabled")
	}
	
	return pc
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		lines: make([]string, size),
		size:  size,
	}
}

func (rb *RingBuffer) Add(line string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.lines[rb.pos] = line
	rb.pos = (rb.pos + 1) % rb.size
}

func (rb *RingBuffer) GetLast(n int) []string {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	
	if n > rb.size {
		n = rb.size
	}
	
	result := make([]string, 0, n)
	for i := 0; i < n; i++ {
		idx := (rb.pos - n + i + rb.size) % rb.size
		if rb.lines[idx] != "" {
			result = append(result, rb.lines[idx])
		}
	}
	return result
}

func (pc *PartylineConsole) startServer() {
	var err error
	addr := fmt.Sprintf("%s:%d", pc.bindAddr, pc.port)
	pc.listener, err = net.Listen("tcp", addr)
	if err != nil {
		log.Printf("❌ Console listener error: %v", err)
		log.Printf("💡 Try different port in ynm.yaml (console.port)")
		return
	}
	
	log.Printf("🎮 Partyline console started: telnet %s %d", pc.bindAddr, pc.port)
	
	for {
		conn, err := pc.listener.Accept()
		if err != nil {
			continue
		}
		go pc.handleConnection(conn)
	}
}

func (pc *PartylineConsole) handleConnection(conn net.Conn) {
	defer conn.Close()
	
	session := &ConsoleSession{
		conn:       conn,
		authed:     false,
		channels:   make(map[string]bool),
		writer:     bufio.NewWriter(conn),
		stopStream: make(chan struct{}),
	}
	
	pc.writeLine(session, "\r\n╔═══════════════════════════════════════╗")
	pc.writeLine(session, "║     YnM-Go Partyline Console v2.0     ║")
	pc.writeLine(session, "╚═══════════════════════════════════════╝\r\n")
	pc.writeLine(session, "Nick: ")
	
	scanner := bufio.NewScanner(conn)
	
	if scanner.Scan() {
		session.nick = strings.TrimSpace(scanner.Text())
	} else {
		return
	}
	
	pc.writeLine(session, "Password: ")
	if scanner.Scan() {
		password := strings.TrimSpace(scanner.Text())
		if password != pc.password {
			pc.writeLine(session, "❌ Invalid password\r\n")
			return
		}
	} else {
		return
	}
	
	session.authed = true
	
	pc.mu.Lock()
	pc.sessions[session.nick] = session
	pc.mu.Unlock()
	
	defer func() {
		close(session.stopStream)
		pc.mu.Lock()
		delete(pc.sessions, session.nick)
		pc.mu.Unlock()
		pc.broadcast <- fmt.Sprintf("*** %s left the partyline", session.nick)
	}()
	
	pc.writeLine(session, fmt.Sprintf("✅ Welcome %s!\r\n", session.nick))
	pc.writeLine(session, "Type .help for commands\r\n\r\n")
	
	// Utolsó 50 log sor mutatása
	logs := globalLogBroadcaster.buffer.GetLast(50)
	if len(logs) > 0 {
		pc.writeLine(session, "═══ Last 50 log entries ═══\r\n")
		for _, log := range logs {
			pc.writeLine(session, log+"\r\n")
		}
		pc.writeLine(session, "═══════════════════════════\r\n\r\n")
	}
	
	pc.broadcast <- fmt.Sprintf("*** %s joined the partyline", session.nick)
	
	// Élő log streaming indítása
	session.logStream = globalLogBroadcaster.Subscribe()
	go pc.streamLogs(session)
	
	// Parancs kezelés
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		
		pc.handleCommand(session, line)
	}
}

func (pc *PartylineConsole) streamLogs(session *ConsoleSession) {
	for {
		select {
		case <-session.stopStream:
			globalLogBroadcaster.Unsubscribe(session.logStream)
			return
		case logLine := <-session.logStream:
			pc.writeLine(session, logLine+"\r\n")
		}
	}
}

func (pc *PartylineConsole) handleCommand(session *ConsoleSession, cmd string) {
	if strings.HasPrefix(cmd, ".") {
		parts := strings.Fields(cmd)
		command := parts[0][1:]
		
		switch command {
		case "help":
			pc.writeLine(session, "╔═══════════════════════════════════════╗\r\n")
			pc.writeLine(session, "║          Console Commands             ║\r\n")
			pc.writeLine(session, "╚═══════════════════════════════════════╝\r\n")
			pc.writeLine(session, ".help           - This help\r\n")
			pc.writeLine(session, ".who            - Online users\r\n")
			pc.writeLine(session, ".quit           - Disconnect\r\n")
			pc.writeLine(session, ".status         - Bot status\r\n")
			pc.writeLine(session, ".uptime         - Bot uptime\r\n")
			pc.writeLine(session, ".restart        - Restart bot (owner only)\r\n")
			pc.writeLine(session, ".die            - Shutdown bot (owner only)\r\n")
			pc.writeLine(session, ".channels       - List channels\r\n")
			pc.writeLine(session, ".console <ch>   - Follow channel\r\n")
			pc.writeLine(session, ".msg <t> <msg>  - Send message\r\n")
			pc.writeLine(session, ".raw <cmd>      - Raw IRC command\r\n")
			pc.writeLine(session, ".logs <n>       - Show last N logs\r\n")
			
		case "who":
			pc.mu.Lock()
			pc.writeLine(session, fmt.Sprintf("Online users (%d):\r\n", len(pc.sessions)))
			for nick := range pc.sessions {
				pc.writeLine(session, fmt.Sprintf("  • %s\r\n", nick))
			}
			pc.mu.Unlock()
			
		case "quit":
			pc.writeLine(session, "Goodbye!\r\n")
			session.conn.Close()
			
		case "status":
			pc.writeLine(session, fmt.Sprintf("Bot: %s\r\n", pc.bot.GetNick()))
			pc.writeLine(session, fmt.Sprintf("Connected: yes\r\n"))
			pc.writeLine(session, fmt.Sprintf("Uptime: %s\r\n", time.Since(pc.startTime).Round(time.Second)))
			pc.writeLine(session, fmt.Sprintf("Console users: %d\r\n", len(pc.sessions)))
			
		case "uptime":
			uptime := time.Since(pc.startTime)
			days := int(uptime.Hours() / 24)
			hours := int(uptime.Hours()) % 24
			minutes := int(uptime.Minutes()) % 60
			pc.writeLine(session, fmt.Sprintf("Uptime: %dd %dh %dm\r\n", days, hours, minutes))
			
		case "restart":
			if !pc.isOwner(session.nick) {
				pc.writeLine(session, "❌ Owner only command\r\n")
				return
			}
			pc.writeLine(session, "🔄 Restarting bot...\r\n")
			pc.broadcast <- "*** Bot restarting..."
			go func() {
				time.Sleep(1 * time.Second)
				pc.bot.SendRaw("QUIT :Restarting...")
				time.Sleep(500 * time.Millisecond)
				pc.restartBot()
			}()
			
		case "die":
			if !pc.isOwner(session.nick) {
				pc.writeLine(session, "❌ Owner only command\r\n")
				return
			}
			pc.writeLine(session, "💀 Shutting down bot...\r\n")
			pc.broadcast <- "*** Bot shutting down..."
			go func() {
				time.Sleep(1 * time.Second)
				pc.bot.SendRaw("QUIT :Shutdown from console")
				time.Sleep(500 * time.Millisecond)
				os.Exit(0)
			}()
			
		case "channels":
			pc.writeLine(session, "Active channels:\r\n")
			pc.writeLine(session, "  #YnM\r\n")
			
		case "console":
			if len(parts) < 2 {
				pc.writeLine(session, "Usage: .console <channel>\r\n")
				return
			}
			channel := parts[1]
			session.channels[channel] = true
			pc.writeLine(session, fmt.Sprintf("Now listening to: %s\r\n", channel))
			
		case "msg":
			if len(parts) < 3 {
				pc.writeLine(session, "Usage: .msg <target> <message>\r\n")
				return
			}
			target := parts[1]
			message := strings.Join(parts[2:], " ")
			pc.sendMessage(target, message)
			pc.writeLine(session, fmt.Sprintf("-> %s: %s\r\n", target, message))
			
		case "raw":
			if len(parts) < 2 {
				pc.writeLine(session, "Usage: .raw <IRC command>\r\n")
				return
			}
			rawCmd := strings.Join(parts[1:], " ")
			pc.bot.SendRaw(rawCmd)
			pc.writeLine(session, fmt.Sprintf("Sent: %s\r\n", rawCmd))
			
		case "logs":
			n := 20
			if len(parts) > 1 {
				fmt.Sscanf(parts[1], "%d", &n)
			}
			logs := globalLogBroadcaster.buffer.GetLast(n)
			pc.writeLine(session, fmt.Sprintf("═══ Last %d logs ═══\r\n", n))
			for _, log := range logs {
				pc.writeLine(session, log+"\r\n")
			}
			pc.writeLine(session, "════════════════════\r\n")
			
		default:
			pc.writeLine(session, fmt.Sprintf("Unknown command: .%s\r\n", command))
		}
	} else {
		// Partyline chat
		pc.broadcast <- fmt.Sprintf("[%s] %s", session.nick, cmd)
	}
}

func (pc *PartylineConsole) isOwner(nick string) bool {
	// Ellenőrzés az admin DB-ben
	return true // TODO: implementáld az owner check-et
}

func (pc *PartylineConsole) restartBot() {
	executable, err := os.Executable()
	if err != nil {
		log.Printf("❌ Cannot get executable path: %v", err)
		return
	}
	
	cmd := exec.Command(executable, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	
	if err := cmd.Start(); err != nil {
		log.Printf("❌ Restart failed: %v", err)
		return
	}
	
	// Jelenlegi process leállítása
	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
}

func (pc *PartylineConsole) writeLine(session *ConsoleSession, text string) {
	session.writer.WriteString(text)
	session.writer.Flush()
}

func (pc *PartylineConsole) broadcastHandler() {
	for msg := range pc.broadcast {
		pc.mu.Lock()
		for _, session := range pc.sessions {
			pc.writeLine(session, msg+"\r\n")
		}
		pc.mu.Unlock()
		
		pc.logBuffer.Add(msg)
	}
}

func (pc *PartylineConsole) HandleMessage(msg YnMIrC.Message) string {
	logLine := fmt.Sprintf("[%s] <%s> %s", msg.Channel, msg.Nick, msg.Text)
	pc.logBuffer.Add(logLine)
	
	pc.mu.Lock()
	for _, session := range pc.sessions {
		if session.channels[msg.Channel] || session.channels["*"] {
			pc.writeLine(session, "📨 "+logLine+"\r\n")
		}
	}
	pc.mu.Unlock()
	
	return ""
}

func (pc *PartylineConsole) OnTick() []YnMIrC.Message {
	return nil
}

func (pc *PartylineConsole) Cleanup() {
	if pc.listener != nil {
		pc.listener.Close()
	}
	close(pc.broadcast)
}

func (pc *PartylineConsole) sendMessage(target, message string) {
	pc.bot.SendMessage(target, message)
}