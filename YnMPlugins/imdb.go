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
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

type IMDBConfig struct {
	APIKey       string `yaml:"api_key"`        // TMDb API key
	Trigger      string `yaml:"trigger"`
	DefaultLang  string `yaml:"default_lang"`   // "hu" for Hungarian
	NumList      int    `yaml:"num_list"`
	RatingSymbol string `yaml:"rating_symbol"`
}

type IMDBPlugin struct {
	bot         *YnMIrC.Client
	config      IMDBConfig
	adminPlugin *owner.YnmAdminPlugin
	lastCmd     struct {
		nick  string
		text  string
		time  time.Time
		mutex sync.Mutex
	}
}

func NewIMDBPluginFromConfig(bot *YnMIrC.Client, config IMDBConfig, adminPlugin *owner.YnmAdminPlugin) *IMDBPlugin {
	return &IMDBPlugin{
		bot:         bot,
		config:      config,
		adminPlugin: adminPlugin,
	}
}

func LoadIMDBConfig(path string) (IMDBConfig, error) {
	var config struct {
		IMDB IMDBConfig `yaml:"imdb"`
	}

	file, err := os.Open(path)
	if err != nil {
		return IMDBConfig{}, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	if err := yaml.NewDecoder(file).Decode(&config); err != nil {
		return IMDBConfig{}, fmt.Errorf("failed to parse config: %w", err)
	}

	return config.IMDB, nil
}

func (p *IMDBPlugin) Name() string {
	return "IMDb Plugin"
}

func (p *IMDBPlugin) Start() {
	p.bot.AddHandler("PRIVMSG", func(c *YnMIrC.Client, msg YnMIrC.Message) {
		response := p.HandleMessage(msg)
		if response == "" {
			return
		}

		// Check for duplicates ONLY when we have a response
		p.lastCmd.mutex.Lock()
		now := time.Now()
		if msg.Nick == p.lastCmd.nick && 
		   msg.Text == p.lastCmd.text && 
		   now.Sub(p.lastCmd.time) < 2*time.Second {
			p.lastCmd.mutex.Unlock()
			return
		}

		// Update last command info
		p.lastCmd.nick = msg.Nick
		p.lastCmd.text = msg.Text
		p.lastCmd.time = now
		p.lastCmd.mutex.Unlock()

		// Split response into logical parts
		parts := strings.SplitN(response, "| Történet: ", 2) 
		if len(parts) == 2 {
			p.bot.SendMessage(msg.Params[0], parts[0])
			time.Sleep(300 * time.Millisecond)
			p.bot.SendMessage(msg.Params[0], "Történet: "+parts[1])
		} else {
			p.bot.SendMessage(msg.Params[0], response)
		}
	})
}

func (p *IMDBPlugin) HandleMessage(msg YnMIrC.Message) string {
	
	hostmask := YnMModule.SimplifyHostmask(msg.Sender)
    prefix := p.adminPlugin.GetPrefixForHost(hostmask)
    nick, hostmask := YnMModule.GetEffectiveNickAndHost(p.adminPlugin, msg.Sender)
    minLevel := 1

    if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, msg.Channel, minLevel) {
        return ""
    }

	text := strings.TrimSpace(msg.Text)
    if !strings.HasPrefix(strings.ToLower(text), strings.ToLower(prefix+"imdb")) {
        return ""
    }



	args := strings.Fields(msg.Text)
	if len(args) < 2 {
		return "Használat: " + p.config.Trigger + "imdb <cím|tmdbid>"
	}

	query := strings.Join(args[1:], " ")
	// Check if it's a TMDb ID (numeric)
	if id, err := strconv.Atoi(query); err == nil {
		return p.searchByID(id)
	}
	return p.searchByTitle(query)
}

func (p *IMDBPlugin) searchByID(id int) string {
	apiURL := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d?api_key=%s&language=%s",
		id, p.config.APIKey, p.config.DefaultLang)
	return p.searchAPI(apiURL)
}

func (p *IMDBPlugin) searchByTitle(title string) string {
	// First search for the movie
	searchURL := fmt.Sprintf("https://api.themoviedb.org/3/search/movie?api_key=%s&language=%s&query=%s",
		p.config.APIKey, p.config.DefaultLang, url.QueryEscape(title))
	
	resp, err := http.Get(searchURL)
	if err != nil {
		return "Hiba a TMDb API elérésekor"
	}
	defer resp.Body.Close()

	var searchData TMDbSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchData); err != nil {
		return "Hiba a keresési válasz feldolgozásakor"
	}

	if len(searchData.Results) == 0 {
		return "Nincs találat a megadott címre: " + title
	}

	// Get detailed info for the first result
	return p.searchByID(searchData.Results[0].ID)
}

func (p *IMDBPlugin) searchAPI(apiURL string) string {
	resp, err := http.Get(apiURL)
	if err != nil {
		return "Hiba a TMDb API elérésekor"
	}
	defer resp.Body.Close()

	var data TMDbMovieResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "Hiba a válasz feldolgozásakor"
	}

	return p.formatInfo(&data)
}

func (p *IMDBPlugin) formatInfo(data *TMDbMovieResponse) string {
	rating := data.VoteAverage
	stars := strings.Repeat(p.config.RatingSymbol, int(rating+0.5))

	// Shorten plot if too long
	plot := data.Overview
	if len(plot) > 200 {
		plot = plot[:200] + "..."
	}

	// Handle missing data
	if plot == "" {
		plot = "Nincs elérhető leírás"
	}

	// Get runtime
	runtime := "N/A"
	if data.Runtime > 0 {
		runtime = fmt.Sprintf("%d perc", data.Runtime)
	}

	// Genres
	genres := ""
	if len(data.Genres) > 0 {
		genreNames := make([]string, len(data.Genres))
		for i, genre := range data.Genres {
			genreNames[i] = genre.Name
		}
		genres = strings.Join(genreNames, ", ")
	} else {
		genres = "N/A"
	}

	// IMDb link
	imdbLink := ""
	if data.ImdbID != "" {
		imdbLink = data.ImdbID
	} else {
		imdbLink = fmt.Sprintf("TMDb:%d", data.ID)
	}

	// Release year – biztonságos slice ellenőrzéssel
	releaseYear := "N/A"
	if len(data.ReleaseDate) >= 4 {
		releaseYear = data.ReleaseDate[:4]
	}

	return fmt.Sprintf(
		"[TMDb] %s (%s) | Értékelés: %s %.1f/10 | %s | %s | %s | Népszerűség: %.1f | Történet: %s",
		data.Title,
		releaseYear,
		stars,
		rating,
		genres,
		runtime,
		imdbLink,
		data.Popularity,
		plot,
	)
}

// TMDb API Response Structures
type TMDbSearchResponse struct {
	Results []struct {
		ID          int    `json:"id"`
		Title       string `json:"title"`
		ReleaseDate string `json:"release_date"`
	} `json:"results"`
}

type TMDbMovieResponse struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Overview    string  `json:"overview"`
	ReleaseDate string  `json:"release_date"`
	VoteAverage float64 `json:"vote_average"`
	Popularity  float64 `json:"popularity"`
	Runtime     int     `json:"runtime"`
	ImdbID      string  `json:"imdb_id"`
	Genres      []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
}

func (p *IMDBPlugin) OnTick() []YnMIrC.Message {
	return nil
}