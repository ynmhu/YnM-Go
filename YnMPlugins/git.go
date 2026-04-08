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

package ynm

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMModuls/discord"
	"gopkg.in/yaml.v3"
)

type GitConfig struct {
	GitPlugin struct {
		Channel []string `yaml:"channel"`
		ApiURL  string   `yaml:"apiURL"`
	} `yaml:"GitPlugin"`
}

func LoadGitConfig(path string) (*GitConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg GitConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

type GitUser struct {
	ID               int    `json:"id"`
	Login            string `json:"login"`
	LoginName        string `json:"login_name"`
	FullName         string `json:"full_name"`
	Email            string `json:"email"`
	AvatarURL        string `json:"avatar_url"`
	HTMLURL          string `json:"html_url"`
	Language         string `json:"language"`
	IsAdmin          bool   `json:"is_admin"`
	LastLogin        string `json:"last_login"`
	Created          string `json:"created"`
	Location         string `json:"location"`
	Pronouns         string `json:"pronouns"`
	Website          string `json:"website"`
	Description      string `json:"description"`
	Visibility       string `json:"visibility"`
	FollowersCount   int    `json:"followers_count"`
	FollowingCount   int    `json:"following_count"`
	StarredReposCount int   `json:"starred_repos_count"`
	Username         string `json:"username"`
}

type Commit struct {
	URL      string `json:"url"`
	SHA      string `json:"sha"`
	Created  string `json:"created"`
	HTMLURL  string `json:"html_url"`
	Commit   struct {
		URL       string `json:"url"`
		Author    struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"author"`
		Committer struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"committer"`
		Message      string `json:"message"`
		Tree         struct {
			URL     string `json:"url"`
			SHA     string `json:"sha"`
			Created string `json:"created"`
		} `json:"tree"`
		Verification struct {
			Verified  bool        `json:"verified"`
			Reason    string      `json:"reason"`
			Signature string      `json:"signature"`
			Signer    interface{} `json:"signer"`
			Payload   string      `json:"payload"`
		} `json:"verification"`
	} `json:"commit"`
	Author    GitUser `json:"author"`
	Committer GitUser `json:"committer"`
}

type GitPlugin struct {
	bot             *YnMIrC.Client
	discord         *discord.DiscordAdapter
	apiURL          string
	lastSeen        string
	ticker          *time.Ticker
	stopChan        chan bool
	channels        []string
	discordChannels []string
	lastSeenPath    string
	firstRun        bool
}

func NewGitPluginWithDiscord(bot *YnMIrC.Client, gitConfig struct {
	Channel []string `yaml:"channel"`
	ApiURL  string   `yaml:"apiURL"`
}, discordAdapter *discord.DiscordAdapter) (*GitPlugin, error) {

	var discordChannels []string
	var ircChannels []string

	//log.Printf("🔍 Git csatornák feldolgozása...")
	
	for _, channel := range gitConfig.Channel {
		if isDiscordChannelGit(channel) {
			discordChannels = append(discordChannels, channel)
			//log.Printf("  🎮 Discord csatorna: %s", channel)
		} else {
			ircChannels = append(ircChannels, channel)
			//log.Printf("  📡 IRC csatorna: %s", channel)
		}
	}

	lastSeenPath := "./data/git_last_seen.txt"
	lastSeen := readLastSeen(lastSeenPath)
	
	// JAVÍTÁS: Ha nincs lastSeen, ez az első futás
	firstRun := (lastSeen == "")
	if firstRun {
		log.Printf("🆕 Git plugin első indítás - csak új commitokat fog küldeni")
	}

	return &GitPlugin{
		bot:             bot,
		discord:         discordAdapter,
		apiURL:          gitConfig.ApiURL,
		stopChan:        make(chan bool),
		channels:        ircChannels,
		discordChannels: discordChannels,
		lastSeenPath:    lastSeenPath,
		lastSeen:        lastSeen,
		firstRun:        firstRun,
	}, nil
}

// isDiscordChannelGit ellenőrzi, hogy a channel ID csak számokat tartalmaz-e
func isDiscordChannelGit(channel string) bool {
	if len(channel) == 0 {
		return false
	}
	for _, char := range channel {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func readLastSeen(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		// Nem hiba, ha nem létezik (első futás)
		return ""
	}
	return strings.TrimSpace(string(data))
}

func writeLastSeen(path, sha string) {
	// Biztosítjuk, hogy a data könyvtár létezik
	err := os.MkdirAll("./data", 0755)
	if err != nil {
		log.Printf("❌ Git: Nem sikerült létrehozni a data könyvtárat: %v", err)
		return
	}
	
	err = os.WriteFile(path, []byte(sha), 0644)
	if err != nil {
		log.Printf("❌ Git: Nem sikerült menteni a lastSeen SHA-t: %v", err)
	}
}

func (p *GitPlugin) Start() {
	p.ticker = time.NewTicker(30 * time.Minute)

	//log.Printf("🔧 Git plugin elindult. API: %s", p.apiURL)
	if len(p.channels) > 0 {
		//log.Printf("📡 Git IRC csatornák: %v", p.channels)
	}
	if len(p.discordChannels) > 0 && p.discord != nil {
		//log.Printf("🎮 Git Discord csatornák: %v", p.discordChannels)
	}

	go func() {
		for {
			select {
			case <-p.ticker.C:
				p.checkCommits()
			case <-p.stopChan:
				p.ticker.Stop()
				return
			}
		}
	}()
}

func (p *GitPlugin) Stop() {
	close(p.stopChan)
}

// Helper function to send messages to all channels
func (p *GitPlugin) sendToAllChannels(message string) {
	// Küldés IRC csatornákra
	for _, channel := range p.channels {
		p.bot.SendMessage(channel, message)
	}

	// Küldés Discord csatornákra
	if p.discord != nil {
		for _, channel := range p.discordChannels {
			err := p.discord.SendMessage(channel, message)
			if err != nil {
				log.Printf("❌ Git Discord hiba (%s): %v", channel, err)
			}
		}
	}
}

func (p *GitPlugin) checkCommits() {
	commits, err := fetchCommits(p.apiURL)
	if err != nil {
		log.Printf("❌ Git: Hiba commit lekéréskor: %v", err)
		return
	}
	if len(commits) == 0 {
		return
	}

	newest := commits[0].SHA

	// JAVÍTÁS: Első futáskor csak mentjük a SHA-t, nem küldünk értesítést
	if p.firstRun {
		p.lastSeen = newest
		writeLastSeen(p.lastSeenPath, newest)
		p.firstRun = false
		log.Printf("✅ Git: Inicializálva, legfrissebb commit: %s", newest[:7])
		return
	}

	if newest != p.lastSeen && p.lastSeen != "" {
		p.lastSeen = newest
		writeLastSeen(p.lastSeenPath, newest)
		c := commits[0]

		// Clean up commit message
		message := strings.TrimSpace(c.Commit.Message)
		if message == "" {
			message = "No commit message"
		}

		// Get verification status
		verificationEmoji := "❌"
		verificationText := "unsigned"
		if c.Commit.Verification.Verified {
			verificationEmoji = "✅"
			verificationText = "signed"
		}

		// Format timestamp
		timeStr := c.Commit.Author.Date.Format("15:04:05")
		dateStr := c.Commit.Author.Date.Format("Jan 02")

		// Short SHA
		shortSHA := c.SHA[:7]

		// Location info (if available)
		locationInfo := ""
		if c.Author.Location != "" {
			locationInfo = fmt.Sprintf(" 📍 %s", c.Author.Location)
		}

		// Professional formatted messages with rich data
		headerMsg := fmt.Sprintf("🔧 [YnM-Go] New commit by %s (%s)%s",
			c.Author.FullName,
			c.Author.Login,
			locationInfo)

		detailsMsg := fmt.Sprintf("📝 \"%s\" | ⏰ %s %s | 🔗 %s %s (%s)",
			message,
			dateStr,
			timeStr,
			shortSHA,
			verificationEmoji,
			verificationText)

		urlMsg := fmt.Sprintf("🌐 %s", c.HTMLURL)

		// Send messages to all channels
		p.sendToAllChannels(headerMsg)
		p.sendToAllChannels(detailsMsg)
		p.sendToAllChannels(urlMsg)

		log.Printf("✅ Git commit értesítés elküldve: %s", message)
	}
}

func fetchCommits(apiURL string) ([]Commit, error) {
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	var commits []Commit
	err = json.Unmarshal(body, &commits)
	if err != nil {
		return nil, err
	}
	return commits, nil
}

func (p *GitPlugin) OnTick() []YnMIrC.Message {
	return nil
}

func (p *GitPlugin) HandleMessage(msg YnMIrC.Message) string {
	switch msg.Text {
	case "!checkgit":
		p.checkCommits()
		return "✅ Git lekérdezés elindítva."
	}
	return ""
}

func (p *GitPlugin) Name() string {
	return "Git Commit Monitor"
}