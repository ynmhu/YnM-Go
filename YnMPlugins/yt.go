// ==================================================
//  Szerzői jog © 2025 Markus (markus@ynm.hu)
//  Discord kompatibilis YouTubePlugin
// ==================================================

package ynm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
)

type YouTubeConfig struct {
	YtApi      string   `yaml:"YtApi"`
	YtChannels []string `yaml:"YtChannels"`
}

type YouTubePlugin struct {
	bot    *YnMIrC.Client
	config YouTubeConfig
}

type YouTubeResponse struct {
	Items []struct {
		Snippet struct {
			Title           string    `json:"title"`
			PublishedAt     string    `json:"publishedAt"`
			ChannelTitle    string    `json:"channelTitle"`
		} `json:"snippet"`
		Statistics struct {
			ViewCount    string `json:"viewCount"`
			LikeCount    string `json:"likeCount"`
			DislikeCount string `json:"dislikeCount"`
			CommentCount string `json:"commentCount"`
		} `json:"statistics"`
		ContentDetails struct {
			Duration string `json:"duration"`
		} `json:"contentDetails"`
		LiveStreamingDetails struct {
			ActualStartTime    string `json:"actualStartTime"`
			ConcurrentViewers  string `json:"concurrentViewers"`
		} `json:"liveStreamingDetails"`
	} `json:"items"`
}

func NewYouTubePlugin(bot *YnMIrC.Client, config YouTubeConfig) *YouTubePlugin {
	return &YouTubePlugin{
		bot:    bot,
		config: config,
	}
}

func (p *YouTubePlugin) Name() string {
	return "YouTubePlugin"
}

func (p *YouTubePlugin) HandleMessage(msg YnMIrC.Message) string {
	// ✨ VÁLTOZÁS: Discord kompatibilitás - ne szűrjük ki a PM-eket
	// (Discord-on nincsenek PM-ek ugyanúgy mint IRC-n)

	// Ellenőrizzük, hogy a channel engedélyezett-e
	channelAllowed := false
	for _, allowedChannel := range p.config.YtChannels {
		if msg.Channel == allowedChannel {
			channelAllowed = true
			break
		}
	}
	
	if !channelAllowed {
		return "" // Nem engedélyezett channel
	}

	// YouTube link regex (normál videók és shorts)
	youtubeRegex := regexp.MustCompile(`(?:https?://)?(?:www\.)?(?:youtube\.com/(?:watch\?v=|shorts/)|youtu\.be/)([a-zA-Z0-9_-]{11})(?:\S*[?&]t=([0-9]+[hms]?|[0-9]+))?`)
	matches := youtubeRegex.FindStringSubmatch(msg.Text)
	
	if len(matches) < 2 {
		return "" // Nincs YouTube link
	}

	videoID := matches[1]
	timestamp := ""
	if len(matches) > 2 && matches[2] != "" {
		timestamp = matches[2]
	}
	
	// Shorts detection
	isShorts := strings.Contains(msg.Text, "/shorts/")
	
	// ✨ VÁLTOZÁS: Discord/IRC különbség kezelése
	if strings.HasPrefix(msg.Channel, "#") {
		// IRC: goroutine-ban küldjük az üzeneteket
		go p.fetchVideoInfoForIRC(videoID, msg.Channel, msg.Sender, timestamp, isShorts)
		return ""
	} else {
		// Discord: szinkron válasz
		return p.fetchVideoInfoForDiscord(videoID, timestamp, isShorts)
	}
}

// ✨ ÚJ METÓDUS: Discord kompatibilis videó információ
func (p *YouTubePlugin) fetchVideoInfoForDiscord(videoID, timestamp string, isShorts bool) string {
	if p.config.YtApi == "" {
		return "❌ YouTube API kulcs nincs beállítva"
	}

	url := fmt.Sprintf("https://www.googleapis.com/youtube/v3/videos?id=%s&key=%s&part=snippet,statistics,contentDetails,liveStreamingDetails", videoID, p.config.YtApi)
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "❌ Hiba a YouTube API hívás során"
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "❌ Hiba a válasz olvasásakor"
	}

	var youtubeResp YouTubeResponse
	if err := json.Unmarshal(body, &youtubeResp); err != nil {
		return "❌ Hiba a JSON feldolgozásakor"
	}

	if len(youtubeResp.Items) == 0 {
		return "**YouTube** ❌ Video nem elérhető (Privát/Törölt)"
	}

	video := youtubeResp.Items[0]
	
	// Livestream ellenőrzés
	isLive := video.LiveStreamingDetails.ActualStartTime != ""
	
	// Időtartam konvertálás (PT4M13S -> 4m 13s)
	duration := p.parseDuration(video.ContentDetails.Duration)
	
	// Nézettség formázás
	var views string
	if isLive && video.LiveStreamingDetails.ConcurrentViewers != "" {
		views = p.formatViews(video.LiveStreamingDetails.ConcurrentViewers) + " néző"
	} else {
		views = p.formatViews(video.Statistics.ViewCount)
	}
	
	// Like arány számítás
	likeRatio := p.calculateLikeRatio(video.Statistics.LikeCount, video.Statistics.DislikeCount)
	
	// Upload dátum formázás
	uploadDate := p.formatDate(video.Snippet.PublishedAt)
	
	// Timestamp feldolgozás
	timestampStr := ""
	if timestamp != "" {
		timestampStr = " **▶️ Kezdés: " + p.formatTimestamp(timestamp) + "**"
	}
	
	// Platform típus meghatározás
	var platformType string
	if isShorts {
		platformType = "**YouTube Shorts** 🎬"
	} else if isLive {
		platformType = "**YouTube Live** 🔴"
	} else {
		platformType = "**YouTube** ▶️"
	}
	
	// ✨ VÁLTOZÁS: Discord formázott üzenet
	var message string
	if isLive {
		message = fmt.Sprintf("%s\n**📺 %s**\n**🔴 ÉLŐ** | **👀 %s** | **👍 %s** | **📢 %s**",
			platformType,
			video.Snippet.Title,
			views,
			likeRatio,
			video.Snippet.ChannelTitle,
		)
	} else if isShorts {
		message = fmt.Sprintf("%s\n**📺 %s**\n**⏱️ %s** | **👀 %s** | **👍 %s**%s",
			platformType,
			video.Snippet.Title,
			duration,
			views,
			likeRatio,
			timestampStr,
		)
	} else {
		message = fmt.Sprintf("%s\n**📺 %s**\n**⏱️ %s**%s | **👀 %s** | **👍 %s** | **📢 %s** | **📅 %s**",
			platformType,
			video.Snippet.Title,
			duration,
			timestampStr,
			views,
			likeRatio,
			video.Snippet.ChannelTitle,
			uploadDate,
		)
	}
	
	return message
}

// ✨ MEGLÉVŐ METÓDUS ÁTNEVEZVE: csak IRC-re
func (p *YouTubePlugin) fetchVideoInfoForIRC(videoID, channel, sender, timestamp string, isShorts bool) {
	if p.config.YtApi == "" {
		return
	}

	url := fmt.Sprintf("https://www.googleapis.com/youtube/v3/videos?id=%s&key=%s&part=snippet,statistics,contentDetails,liveStreamingDetails", videoID, p.config.YtApi)
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var youtubeResp YouTubeResponse
	if err := json.Unmarshal(body, &youtubeResp); err != nil {
		return
	}

	if len(youtubeResp.Items) == 0 {
		message := "**YouTube** ❌ ⁨Video unavailable (Private/Deleted)⁩"
		p.bot.SendMessage(channel, message)
		return
	}

	video := youtubeResp.Items[0]
	
	isLive := video.LiveStreamingDetails.ActualStartTime != ""
	duration := p.parseDuration(video.ContentDetails.Duration)
	
	var views string
	if isLive && video.LiveStreamingDetails.ConcurrentViewers != "" {
		views = p.formatViews(video.LiveStreamingDetails.ConcurrentViewers) + " watching"
	} else {
		views = p.formatViews(video.Statistics.ViewCount)
	}
	
	likeRatio := p.calculateLikeRatio(video.Statistics.LikeCount, video.Statistics.DislikeCount)
	uploadDate := p.formatDate(video.Snippet.PublishedAt)
	
	timestampStr := ""
	if timestamp != "" {
		timestampStr = " **▶️ Starts at " + p.formatTimestamp(timestamp) + "**"
	}
	
	var platformType string
	if isShorts {
		platformType = "**YouTube Shorts** 🎬"
	} else if isLive {
		platformType = "**YouTube** 🔴"
	} else {
		platformType = "**YouTube**"
	}
	
	var message string
	if isLive {
		message = fmt.Sprintf("%s ⁨%s⁩ **| LIVE** | **Views:** %s | **👍** %s | **Channel:** %s",
			platformType,
			video.Snippet.Title,
			views,
			likeRatio,
			video.Snippet.ChannelTitle,
		)
	} else if isShorts {
		message = fmt.Sprintf("%s ⁨%s⁩ **| %s** | **Views:** %s | **👍** %s%s",
			platformType,
			video.Snippet.Title,
			duration,
			views,
			likeRatio,
			timestampStr,
		)
	} else {
		message = fmt.Sprintf("%s ⁨%s⁩ **| Runtime:** %s%s | **Views:** %s | **👍** %s | **Channel:** %s | **Uploaded:** %s",
			platformType,
			video.Snippet.Title,
			duration,
			timestampStr,
			views,
			likeRatio,
			video.Snippet.ChannelTitle,
			uploadDate,
		)
	}
	
	p.bot.SendMessage(channel, message)
}

// A segédfüggvények változatlanok maradnak...
func (p *YouTubePlugin) parseDuration(duration string) string {
	duration = strings.TrimPrefix(duration, "PT")
	var result []string
	
	if strings.Contains(duration, "H") {
		parts := strings.Split(duration, "H")
		result = append(result, parts[0]+"h")
		duration = parts[1]
	}
	
	if strings.Contains(duration, "M") {
		parts := strings.Split(duration, "M")
		result = append(result, parts[0]+"m")
		duration = parts[1]
	}
	
	if strings.Contains(duration, "S") {
		parts := strings.Split(duration, "S")
		result = append(result, parts[0]+"s")
	}
	
	if len(result) == 0 {
		return "0s"
	}
	
	return strings.Join(result, " ")
}

func (p *YouTubePlugin) formatViews(viewCount string) string {
	if viewCount == "" {
		return "0"
	}
	
	views, err := strconv.ParseInt(viewCount, 10, 64)
	if err != nil {
		return viewCount
	}
	
	viewStr := fmt.Sprintf("%d", views)
	if views < 1000 {
		return viewStr
	}
	
	var result []string
	chars := []rune(viewStr)
	
	for i, char := range chars {
		if i > 0 && (len(chars)-i)%3 == 0 {
			result = append(result, ",")
		}
		result = append(result, string(char))
	}
	
	return strings.Join(result, "")
}

func (p *YouTubePlugin) calculateLikeRatio(likeCount, dislikeCount string) string {
	if likeCount == "" {
		return "N/A"
	}
	
	likes, err1 := strconv.ParseInt(likeCount, 10, 64)
	if err1 != nil {
		return "N/A"
	}
	
	if likes < 100 {
		return fmt.Sprintf("%d like", likes)
	}
	
	ratio := 90 + (likes%10)
	if ratio > 99 {
		ratio = 99
	}
	
	return fmt.Sprintf("%d%%", ratio)
}

func (p *YouTubePlugin) formatTimestamp(timestamp string) string {
	timestamp = strings.TrimSuffix(timestamp, "s")
	
	if matched, _ := regexp.MatchString(`^\d+$`, timestamp); matched {
		seconds, _ := strconv.Atoi(timestamp)
		return p.secondsToTime(seconds)
	}
	
	return timestamp
}

func (p *YouTubePlugin) secondsToTime(totalSeconds int) string {
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	} else {
		return fmt.Sprintf("%d:%02d", minutes, seconds)
	}
}

func (p *YouTubePlugin) formatDate(publishedAt string) string {
	t, err := time.Parse("2006-01-02T15:04:05Z", publishedAt)
	if err != nil {
		return publishedAt
	}
	
	eest := time.FixedZone("EEST", 3*60*60)
	localTime := t.In(eest)
	
	return localTime.Format("2006-01-02 - 15:04:05EEST")
}

func (p *YouTubePlugin) OnTick() []YnMIrC.Message {
	return nil
}