// ==================================================
//  Szerzői jog © 2025 Markus (markus@ynm.hu)
// ==================================================

package ynm

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
)

const cacheDuration = time.Hour
var cacheFile = filepath.Join("data", "weather_cache.json")

type WeatherPlugin struct {
	bot    *YnMIrC.Client
	config YnMConfig.WeatherConfig
}

func NewWeatherPlugin(bot *YnMIrC.Client, cfg YnMConfig.WeatherConfig) *WeatherPlugin {
	return &WeatherPlugin{
		bot:    bot,
		config: cfg,
	}
}

// ✅ JAVÍTOTT: HandleMessage visszaadja a választ
func (p *WeatherPlugin) HandleMessage(msg YnMIrC.Message) string {
	if strings.HasPrefix(msg.Text, "!ido ") {
		location := strings.TrimSpace(strings.TrimPrefix(msg.Text, "!ido "))
		if location == "" {
			location = p.config.DefaultLocation
		}
		
		// ✅ SZINKRON hívás, nem goroutine!
		return p.getWeatherResponse(location)
	}
	return ""
}

func (p *WeatherPlugin) OnTick() []YnMIrC.Message {
	return []YnMIrC.Message{}
}

// ✅ ÚJ: Visszaadja a választ string-ként
func (p *WeatherPlugin) getWeatherResponse(location string) string {
	apiKey := p.config.APIKey
	if apiKey == "" {
		return "⚠️ Nincs megadva OpenWeatherMap API kulcs."
	}

	weather, err := getWeather(apiKey, location, p.config.Units, p.config.Language)
	if err != nil {
		return fmt.Sprintf("🌐 Hiba: %v", err)
	}

	if weather == nil || weather.Main.Temp == 0 {
		return fmt.Sprintf("🌫️ Nem található időjárás: %s", location)
	}

	unit := map[string]string{"metric": "°C", "imperial": "°F"}[p.config.Units]
	windUnit := map[string]string{"metric": "m/s", "imperial": "mph"}[p.config.Units]

	reply := fmt.Sprintf("🌦️ %s, %s: %s, %.1f%s (érzet: %.1f%s), páratartalom: %d%%, szél: %.1f %s",
		weather.Name,
		weather.Sys.Country,
		strings.Title(weather.Weather[0].Description),
		weather.Main.Temp,
		unit,
		weather.Main.FeelsLike,
		unit,
		weather.Main.Humidity,
		weather.Wind.Speed,
		windUnit,
	)

	return reply
}

// ❌ RÉGI FÜGGVÉNY - már nem használjuk
func (p *WeatherPlugin) replyWeather(channel, location string) {
	// Ez már nem kell, de meghagyom kompatibilitás miatt
	response := p.getWeatherResponse(location)
	if response != "" {
		p.bot.SendMessage(channel, response)
	}
}

type weatherData struct {
	Name    string `json:"name"`
	Sys     struct{ Country string } `json:"sys"`
	Weather []struct {
		Description string `json:"description"`
	} `json:"weather"`
	Main struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		Humidity  int     `json:"humidity"`
	} `json:"main"`
	Wind struct {
		Speed float64 `json:"speed"`
	} `json:"wind"`
}

func getWeather(apiKey, location, units, lang string) (*weatherData, error) {
	cache := loadCache()

	key := fmt.Sprintf("%s_%s", location, lang)
	if entry, ok := cache[key]; ok && time.Since(entry.Timestamp) < cacheDuration {
		return &entry.Data, nil
	}

	url := fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=%s&lang=%s",
		location, apiKey, units, lang)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, _ := ioutil.ReadAll(resp.Body)
	var data weatherData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	cache[key] = cacheEntry{Data: data, Timestamp: time.Now()}
	saveCache(cache)

	return &data, nil
}

type cacheEntry struct {
	Data      weatherData `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

func loadCache() map[string]cacheEntry {
	data := make(map[string]cacheEntry)
	file, err := os.ReadFile(cacheFile)
	if err == nil {
		_ = json.Unmarshal(file, &data)
	}
	return data
}

func saveCache(cache map[string]cacheEntry) {
	data, _ := json.MarshalIndent(cache, "", "  ")
	_ = os.MkdirAll(filepath.Dir(cacheFile), 0755)
	_ = os.WriteFile(cacheFile, data, 0644)
}