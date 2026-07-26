package media

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"strconv"
	"time"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
	"git.ynm.hu/markus/YnM-Go/YnMDb"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	_ "github.com/mattn/go-sqlite3"
)

// ============================================
// JELLYFIN ADATSTRUKTÚRÁK
// ============================================

type JellyfinUser struct {
	ID               string `json:"Id"`
	Name             string `json:"Name"`
	LastActivityDate string `json:"LastActivityDate"`
	Policy           struct {
		IsHidden bool `json:"IsHidden"`
	} `json:"Policy"`
}

// ============================================
// FIGYELMEZTETÉSI SZINTEK
// ============================================
const (
	StageNone      = 0
	StageWarn1     = 1 // 72 óra
	StageWarn2     = 2 // 48 óra
	StageWarn3     = 3 // 24 óra
	StageSuspended = 99
)

// warningStage - a napok alapján eldönti, melyik szinten kellene állnia a usernek
// Használható normál (van dátum) esetekre. (A "soha nem jelentkezett" esetet
// a SendJellyfishReport külön kezeli, hogy sorban kapja a figyelmeztetéseket.)
func warningStage(days int) int {
	switch {
	case days > 35:
		return StageSuspended
	case days > 28:
		return StageWarn3
	case days > 21:
		return StageWarn2
	case days > 14:
		return StageWarn1
	default:
		return StageNone
	}
}

// ============================================
// WARNING TRACKER (állapot mentés userenként)
// ============================================

type WarningTracker struct {
	mu       sync.Mutex
	warnings map[string]int // user -> jelenlegi stage
	dataFile string
}

func NewWarningTracker(dataDir string) *WarningTracker {
	tracker := &WarningTracker{
		warnings: make(map[string]int),
		dataFile: strings.TrimRight(dataDir, "/") + "/jellyfin_warnings.json",
	}
	fmt.Printf("[WarningTracker] adatfájl: %s\n", tracker.dataFile)
	tracker.load()
	fmt.Printf("[WarningTracker] betöltött állapotok: %v\n", tracker.warnings)
	return tracker
}

func (t *WarningTracker) load() {
	t.mu.Lock()
	defer t.mu.Unlock()
	data, err := os.ReadFile(t.dataFile)
	if err != nil {
		// nincs fájl -> üres map
		return
	}
	var warnings map[string]int
	if err := json.Unmarshal(data, &warnings); err != nil {
		fmt.Printf("[WarningTracker] load: unmarhsal hiba: %v\n", err)
		return
	}
	t.warnings = warnings
}

func (t *WarningTracker) save() {
	t.mu.Lock()
	defer t.mu.Unlock()
	data, err := json.MarshalIndent(t.warnings, "", "  ")
	if err != nil {
		fmt.Printf("[WarningTracker] marshal hiba: %v\n", err)
		return
	}
	if err := os.WriteFile(t.dataFile, data, 0644); err != nil {
		fmt.Printf("[WarningTracker] fájl írási hiba (%s): %v\n", t.dataFile, err)
	}
}

// GetStage - visszaadja a user jelenleg eltárolt figyelmeztetési szintjét
func (t *WarningTracker) GetStage(user string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.warnings[user]
}

// SetStage - beállítja a user figyelmeztetési szintjét
func (t *WarningTracker) SetStage(user string, stage int) {
	t.mu.Lock()
	t.warnings[user] = stage
	t.mu.Unlock()
	t.save()
}

// ResetWarnings - törli a user figyelmeztetéseit
func (t *WarningTracker) ResetWarnings(user string) {
	t.mu.Lock()
	delete(t.warnings, user)
	t.mu.Unlock()
	t.save()
}

// SuspendUser - végleg felfüggeszti a usert (lokálisan)
func (t *WarningTracker) SuspendUser(user string) {
	t.mu.Lock()
	t.warnings[user] = StageSuspended
	t.mu.Unlock()
	t.save()
}

// IsSuspended - true, ha a user már fel van függesztve
func (t *WarningTracker) IsSuspended(user string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.warnings[user] >= StageSuspended
}

// ============================================
// FELHASZNÁLÓK LEKÉRÉSE API-RÓL
// ============================================

func GetJellyfinUsers(jellyfinURL, jellyfinToken string) ([]JellyfinUser, error) {
	url := strings.TrimRight(jellyfinURL, "/") + "/Users"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Emby-Token", jellyfinToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("hibás válasz: %s - %s", resp.Status, string(b))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var users []JellyfinUser
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, err
	}

	return users, nil
}

// ============================================
// IDŐ FORMÁZÁS
// ============================================

// getDaysSinceLastActivity - visszaadja, hány nap telt el az utolsó aktivitás óta
// Ha a dátum üres vagy nem parse-olható, visszatér ""-vel és days=0, így
// a hívó eldönti, hogyan kezelje (éppen ez kell ahhoz, hogy a "soha" esetet
// ne felfüggesztésre küldje azonnal).
func getDaysSinceLastActivity(lastActivity string) (days int, isEmpty bool) {
	if strings.TrimSpace(lastActivity) == "" {
		return 0, true // jelzés: soha nem jelentkezett
	}

	t, err := time.Parse(time.RFC3339, lastActivity)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05.0000000Z", lastActivity)
		if err != nil {
			return 0, true // ha nem parse-olható, kezeljük mint "soha"
		}
	}

	return int(time.Since(t).Hours() / 24), false
}

// ============================================
// JELENTÉS KÜLDÉS & FIGYELMEZTETÉSEK (ÁTDOLGOZVA)
// ============================================
// package-szintű debounce-változók a duplikált reportok elkerülésére
var recentReportsMu sync.Mutex
var recentReports = make(map[string]time.Time)
var reportDebounce = 5 * time.Second // ha 5s-on belül új report jön ugyanarra a target-re, elnyomjuk

func SendJellyfishReport(cfg *YnMConfig.MediaActivityConfig, sendFunc func(target, msg string), tracker *WarningTracker) {
	if cfg == nil {
		return
	}

	// --- határozzuk meg a targetet előre, szükséges a debounce-hoz ---
	target := cfg.ReportChannel
	if target == "" {
		target = cfg.IRCChannel
	}
	// Debounce: ha az utolsó küldés ehhez a targethez túl közel volt, elnyomjuk a mostanit
	recentReportsMu.Lock()
	last, ok := recentReports[target]
	if ok && time.Since(last) < reportDebounce {
		// logoljuk, hogy miért nem küldünk duplikátumot — segít debugolni
		fmt.Printf("[SendJellyfishReport] suppressed duplicate report for %s (last: %v)\n", target, last)
		recentReportsMu.Unlock()
		return
	}
	recentReports[target] = time.Now()
	recentReportsMu.Unlock()

	// --- API lekérés ---
	users, err := GetJellyfinUsers(cfg.JellyfinURL, cfg.JellyfinToken)
	if err != nil {
		sendFunc(target, fmt.Sprintf("❌ Jellyfin hiba: %v", err))
		return
	}

	// --- Szűrés: minden felhasználó (rejtetteket is), BOT/admin kihagyva ---
	var activeUsers []JellyfinUser
	for _, u := range users {
		n := strings.TrimSpace(u.Name)
		if n == "" {
			continue
		}
		low := strings.ToLower(n)
		if low == "bot" || low == "ynm-bot" || low == "admin" {
			continue
		}
		activeUsers = append(activeUsers, u)
	}

	// alapértelmezett értékek
	mult := 3   
	off := 0    

	if cfg != nil {
		if cfg.ReportMultiplier > 0 {
			mult = cfg.ReportMultiplier
		}
		// engedi, hogy negatív legyen, de általában >=0 a jó
		off = cfg.ReportOffset
	}

	displayCount := len(activeUsers)*mult + off
	sendFunc(target, fmt.Sprintf("📊 Összesen: %d felhasználó", displayCount))
	
	// mapping stage -> hátralévő órák
	durations := map[int]int{
		StageWarn1: 72,
		StageWarn2: 48,
		StageWarn3: 24,
	}

	// figyelmeztetések küldése (csak ha új szintre lépett)
	for _, u := range activeUsers {
		days, wasEmpty := getDaysSinceLastActivity(u.LastActivityDate)

		// Ha már fel van függesztve lokálisan, nem foglalkozunk vele (admin dolga)
		if tracker.IsSuspended(u.Name) {
			continue
		}

		currentStage := tracker.GetStage(u.Name)
		var targetStage int

		if wasEmpty {
			// Nincs utolsó aktivitás — felhasználót sorban figyelmeztetjük
			if currentStage == StageNone {
				targetStage = StageWarn1
			} else if currentStage >= StageWarn1 && currentStage < StageWarn3 {
				targetStage = currentStage + 1
			} else if currentStage == StageWarn3 {
				targetStage = StageSuspended
			} else {
				targetStage = StageWarn1
			}
		} else {
			// Normál eset: ha <=14 nap -> nincs teendő (reset korábban kezelve)
			if days <= 14 {
				targetStage = StageNone
			} else {
				if currentStage == StageNone {
					targetStage = StageWarn1
				} else if currentStage >= StageWarn1 && currentStage < StageWarn3 {
					targetStage = currentStage + 1
				} else if currentStage == StageWarn3 {
					targetStage = StageSuspended
				} else {
					targetStage = StageWarn1
				}
			}
		}

		// Visszatért aktívnak számító állapotba -> reseteljük a figyelmeztetéseit
		if !wasEmpty && targetStage == StageNone {
			if currentStage != StageNone {
				tracker.ResetWarnings(u.Name)
			}
			continue
		}

		// Csak akkor küldünk üzenetet, ha új szintre lépett (nem spammelünk minden tick-nél)
		if targetStage <= currentStage {
			continue
		}

		// Üzenet küldése és (ha szükséges) felfüggesztés az API-n keresztül
		if targetStage >= StageSuspended {
			sendFunc(target, fmt.Sprintf("🛑 @%s %d napja nem voltál online! A fiókod felfüggesztésre kerül.", u.Name, days))

			if cfg.JellyfinURL != "" && cfg.JellyfinToken != "" {
				err := SuspendJellyfinUser(cfg.JellyfinURL, cfg.JellyfinToken, u.ID)
				if err != nil {
					sendFunc(target, fmt.Sprintf("❌ Hiba a felfüggesztéskor @%s: %v", u.Name, err))
					continue
				}
				tracker.SuspendUser(u.Name)
				sendFunc(target, fmt.Sprintf("✅ @%s fiókja felfüggesztve lett a Jellyfin-en.", u.Name))
			} else {
				sendFunc(target, "⚠️ Nincs Jellyfin URL/Token beállítva — nem történt tényleges felfüggesztés.")
				tracker.SuspendUser(u.Name)
			}
		} else {
			hrs := durations[targetStage]
			if wasEmpty {
				sendFunc(target, fmt.Sprintf("⚠️ @%s soha nem jelentkeztél be! Soron következő figyelmeztetés: %d órád van bejelentkezni.", u.Name, hrs))
			} else {
				sendFunc(target, fmt.Sprintf("⚠️ @%s %d napja nem voltál online! %d órád van bejelentkezni.", u.Name, days, hrs))
			}
			tracker.SetStage(u.Name, targetStage)
		}
	}
}

// ============================================
// JELLYFIN USER POLICY FRISSÍTÉS (FELFÜGGESZTÉS)
// ============================================

func SuspendJellyfinUser(jellyfinURL, jellyfinToken, userID string) error {
	if jellyfinURL == "" || jellyfinToken == "" || userID == "" {
		return fmt.Errorf("hiányzó paraméter (URL/Token/userID)")
	}

	policyURL := strings.TrimRight(jellyfinURL, "/") + "/Users/" + userID + "/Policy"
	client := &http.Client{Timeout: 15 * time.Second}

	// Próbáljuk meg lekérni közvetlenül a /Policy végpontot
	reqGet, err := http.NewRequest("GET", policyURL, nil)
	if err != nil {
		return err
	}
	reqGet.Header.Set("X-Emby-Token", jellyfinToken)
	reqGet.Header.Set("Accept", "application/json")

	resp, err := client.Do(reqGet)
	if err != nil {
		return fmt.Errorf("policy lekérés hiba: %w", err)
	}
	defer resp.Body.Close()

	var policyObj map[string]interface{}
	if resp.StatusCode == http.StatusOK {
		// sikeres GET /Users/{id}/Policy
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("policy response olvasási hiba: %w", err)
		}
		if err := json.Unmarshal(body, &policyObj); err != nil {
			return fmt.Errorf("policy JSON parse hiba: %w (raw: %s)", err, string(body))
		}
	} else if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotFound {
		// A szerver nem támogatja GET /Users/{id}/Policy — fallback: GET /Users/{id} és onnan vegyük a Policy mezőt
		userURL := strings.TrimRight(jellyfinURL, "/") + "/Users/" + userID
		reqUser, err := http.NewRequest("GET", userURL, nil)
		if err != nil {
			return err
		}
		reqUser.Header.Set("X-Emby-Token", jellyfinToken)
		reqUser.Header.Set("Accept", "application/json")

		resp2, err := client.Do(reqUser)
		if err != nil {
			return fmt.Errorf("user lekérés hiba (fallback): %w", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp2.Body)
			return fmt.Errorf("user lekérés sikertelen (fallback): %s - %s", resp2.Status, string(b))
		}

		body, err := io.ReadAll(resp2.Body)
		if err != nil {
			return fmt.Errorf("user response olvasási hiba: %w", err)
		}
		var userObj map[string]interface{}
		if err := json.Unmarshal(body, &userObj); err != nil {
			return fmt.Errorf("user JSON parse hiba: %w (raw: %s)", err, string(body))
		}
		// kinyerjük a Policy mezőt
		if p, ok := userObj["Policy"]; ok {
			if pm, ok := p.(map[string]interface{}); ok {
				policyObj = pm
			} else {
				return fmt.Errorf("Policy mező nem megfelelő formátumban érkezett")
			}
		} else {
			return fmt.Errorf("Policy mező nem található a Users/{id} válaszában")
		}
	} else {
		// egyéb hibakód
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("policy lekérés sikertelen: %s - %s", resp.Status, string(b))
	}

	// Módosítjuk a policy-t: IsDisabled = true
	policyObj["IsDisabled"] = true

	payload, err := json.Marshal(policyObj)
	if err != nil {
		return fmt.Errorf("policy marshal hiba: %w", err)
	}

	reqPost, err := http.NewRequest("POST", policyURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	reqPost.Header.Set("X-Emby-Token", jellyfinToken)
	reqPost.Header.Set("Content-Type", "application/json")

	resp3, err := client.Do(reqPost)
	if err != nil {
		return fmt.Errorf("policy frissítés hiba: %w", err)
	}
	defer resp3.Body.Close()

	if resp3.StatusCode < 200 || resp3.StatusCode >= 300 {
		b, _ := io.ReadAll(resp3.Body)
		return fmt.Errorf("hibás válasz: %s - %s", resp3.Status, string(b))
	}

	return nil
}
// ============================================
// ⏰ IDŐZÍTŐS INDÍTÁS (24 óránként) - egyszeri indítás biztosítása
// ============================================

var reportersMu sync.Mutex
var startedReporters = map[string]bool{}

// feltételezett importok: "strconv", "time", "strings", "fmt"
func StartJellyfishReporter(cfg *YnMConfig.MediaActivityConfig, sendFunc func(target, msg string)) {
	if cfg == nil || !cfg.Enabled {
		return
	}
	key := cfg.BaseDataDir + "|" + cfg.ReportChannel
	reportersMu.Lock()
	if startedReporters[key] { reportersMu.Unlock(); return }
	startedReporters[key] = true
	reportersMu.Unlock()

	tracker := NewWarningTracker(cfg.BaseDataDir)

	// opcionális: azonnali futtatás induláskor (konfig szerint)
	if cfg.RunOnStart {
		if cfg.ReportChannel != "" {
			go SendJellyfishReport(cfg, sendFunc, tracker)
		}
	}

	// Ha van ReportTime, naponta azon az időponton fusson
	if strings.TrimSpace(cfg.ReportTime) != "" {
		parts := strings.Split(cfg.ReportTime, ":")
		if len(parts) != 2 { fmt.Printf("[StartJellyfishReporter] érvénytelen ReportTime: %s\n", cfg.ReportTime); return }
		hour, err1 := strconv.Atoi(parts[0]); min, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil || hour<0 || hour>23 || min<0 || min>59 {
			fmt.Printf("[StartJellyfishReporter] érvénytelen ReportTime érték: %s\n", cfg.ReportTime); return
		}
		go func() {
			loc := time.Now().Location()
			for {
				now := time.Now().In(loc)
				next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, loc)
				if !next.After(now) {
					next = next.Add(24 * time.Hour)
				}
				sleep := time.Until(next)
				fmt.Printf("[StartJellyfishReporter] következő futás: %v (in %v)\n", next, sleep)
				time.Sleep(sleep)

				if cfg.ReportChannel != "" {
					SendJellyfishReport(cfg, sendFunc, tracker)
				}

				// ezt követően minden 24 órában
				time.Sleep(24 * time.Hour)
			}
		}()
		return
	}

	// fallback: egyszerű 24 órás ticker az indítástól számítva
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		for range ticker.C {
			if cfg.ReportChannel != "" {
				SendJellyfishReport(cfg, sendFunc, tracker)
			}
		}
	}()
}
// ============================================
// PARANCSKEZELŐ (!jf, !jf status, !jf reset ...)
// ============================================

// segédfüggvény: user ID keresése név alapján (case-sensitive)
func findUserIDByName(users []JellyfinUser, name string) string {
	for _, u := range users {
		if u.Name == name {
			return u.ID
		}
	}
	return ""
}

// UnsuspendJellyfinUser: ugyanaz a logika, mint a Suspend, de IsDisabled=false
func UnsuspendJellyfinUser(jellyfinURL, jellyfinToken, userID string) error {
	if jellyfinURL == "" || jellyfinToken == "" || userID == "" {
		return fmt.Errorf("hiányzó paraméter (URL/Token/userID)")
	}

	policyURL := strings.TrimRight(jellyfinURL, "/") + "/Users/" + userID + "/Policy"
	client := &http.Client{Timeout: 15 * time.Second}

	// Próbáljuk meg lekérni közvetlenül a /Policy végpontot
	reqGet, err := http.NewRequest("GET", policyURL, nil)
	if err != nil {
		return err
	}
	reqGet.Header.Set("X-Emby-Token", jellyfinToken)
	reqGet.Header.Set("Accept", "application/json")

	resp, err := client.Do(reqGet)
	if err != nil {
		return fmt.Errorf("policy lekérés hiba: %w", err)
	}
	defer resp.Body.Close()

	var policyObj map[string]interface{}
	if resp.StatusCode == http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("policy response olvasási hiba: %w", err)
		}
		if err := json.Unmarshal(body, &policyObj); err != nil {
			return fmt.Errorf("policy JSON parse hiba: %w (raw: %s)", err, string(body))
		}
	} else if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotFound {
		// fallback a /Users/{id} végpontra
		userURL := strings.TrimRight(jellyfinURL, "/") + "/Users/" + userID
		reqUser, err := http.NewRequest("GET", userURL, nil)
		if err != nil {
			return err
		}
		reqUser.Header.Set("X-Emby-Token", jellyfinToken)
		reqUser.Header.Set("Accept", "application/json")

		resp2, err := client.Do(reqUser)
		if err != nil {
			return fmt.Errorf("user lekérés hiba (fallback): %w", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp2.Body)
			return fmt.Errorf("user lekérés sikertelen (fallback): %s - %s", resp2.Status, string(b))
		}

		body, err := io.ReadAll(resp2.Body)
		if err != nil {
			return fmt.Errorf("user response olvasási hiba: %w", err)
		}
		var userObj map[string]interface{}
		if err := json.Unmarshal(body, &userObj); err != nil {
			return fmt.Errorf("user JSON parse hiba: %w (raw: %s)", err, string(body))
		}
		if p, ok := userObj["Policy"]; ok {
			if pm, ok := p.(map[string]interface{}); ok {
				policyObj = pm
			} else {
				return fmt.Errorf("Policy mező nem megfelelő formátumban érkezett")
			}
		} else {
			return fmt.Errorf("Policy mező nem található a Users/{id} válaszában")
		}
	} else {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("policy lekérés sikertelen: %s - %s", resp.Status, string(b))
	}

	// beállítjuk IsDisabled = false
	policyObj["IsDisabled"] = false

	payload, err := json.Marshal(policyObj)
	if err != nil {
		return fmt.Errorf("policy marshal hiba: %w", err)
	}

	reqPost, err := http.NewRequest("POST", policyURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	reqPost.Header.Set("X-Emby-Token", jellyfinToken)
	reqPost.Header.Set("Content-Type", "application/json")

	resp3, err := client.Do(reqPost)
	if err != nil {
		return fmt.Errorf("policy frissítés hiba: %w", err)
	}
	defer resp3.Body.Close()

	if resp3.StatusCode < 200 || resp3.StatusCode >= 300 {
		b, _ := io.ReadAll(resp3.Body)
		return fmt.Errorf("hibás válasz: %s - %s", resp3.Status, string(b))
	}

	return nil
}

// parancs: !jfsuspend <user>
func CommandJellyfishSuspend(cfg *YnMConfig.MediaActivityConfig, sendFunc func(target, msg string), user string) {
	if cfg == nil || !cfg.Enabled {
		sendFunc(cfg.IRCChannel, "❌ Media aktivitás plugin ki van kapcsolva")
		return
	}
	if user == "" {
		target := cfg.ReportChannel
		if target == "" {
			target = cfg.IRCChannel
		}
		sendFunc(target, "❓ Használat: !jfsuspend <felhasználónév>")
		return
	}

	target := cfg.ReportChannel
	if target == "" {
		target = cfg.IRCChannel
	}

	// lekérjük az ID-t
	users, err := GetJellyfinUsers(cfg.JellyfinURL, cfg.JellyfinToken)
	if err != nil {
		sendFunc(target, fmt.Sprintf("❌ Jellyfin hiba: %v", err))
		return
	}
	id := findUserIDByName(users, user)
	if id == "" {
		sendFunc(target, fmt.Sprintf("❌ Nem találom a felhasználót: %s", user))
		return
	}

	sendFunc(target, fmt.Sprintf("🛑 @%s felfüggesztés indítása...", user))
	if err := SuspendJellyfinUser(cfg.JellyfinURL, cfg.JellyfinToken, id); err != nil {
		sendFunc(target, fmt.Sprintf("❌ Hiba a felfüggesztéskor @%s: %v", user, err))
		return
	}

	// lokális tracker frissítése
	tracker := NewWarningTracker(cfg.BaseDataDir)
	tracker.SuspendUser(user)
	sendFunc(target, fmt.Sprintf("✅ @%s fiókja felfüggesztve lett a Jellyfin-en.", user))
}

// parancs: !jfunsuspend <user>
func CommandJellyfishUnsuspend(cfg *YnMConfig.MediaActivityConfig, sendFunc func(target, msg string), user string) {
	if cfg == nil || !cfg.Enabled {
		sendFunc(cfg.IRCChannel, "❌ Media aktivitás plugin ki van kapcsolva")
		return
	}
	if user == "" {
		target := cfg.ReportChannel
		if target == "" {
			target = cfg.IRCChannel
		}
		sendFunc(target, "❓ Használat: !jfunsuspend <felhasználónév>")
		return
	}

	target := cfg.ReportChannel
	if target == "" {
		target = cfg.IRCChannel
	}

	users, err := GetJellyfinUsers(cfg.JellyfinURL, cfg.JellyfinToken)
	if err != nil {
		sendFunc(target, fmt.Sprintf("❌ Jellyfin hiba: %v", err))
		return
	}
	id := findUserIDByName(users, user)
	if id == "" {
		sendFunc(target, fmt.Sprintf("❌ Nem találom a felhasználót: %s", user))
		return
	}

	sendFunc(target, fmt.Sprintf("🔓 @%s felfüggesztés visszavonása indítása...", user))
	if err := UnsuspendJellyfinUser(cfg.JellyfinURL, cfg.JellyfinToken, id); err != nil {
		sendFunc(target, fmt.Sprintf("❌ Hiba a visszaengedéskor @%s: %v", user, err))
		return
	}

	// tracker reset
	tracker := NewWarningTracker(cfg.BaseDataDir)
	tracker.ResetWarnings(user)
	sendFunc(target, fmt.Sprintf("✅ @%s fiókja újra engedélyezve lett a Jellyfin-en és figyelmeztetések törölve.", user))
}

// isInvokerAdmin — DB-alapú ellenőrzés: ha adminPlugin és db elérhető, használjuk azt.
// minLevel=1 (owner/admin). Ha adminPlugin vagy db nil, visszaad false (biztonságos).
func isInvokerAdmin(adminPlugin *owner.YnmAdminPlugin, db *YnMDb.AdminDB, invokerNick, invokerHost, channel string) bool {
	if adminPlugin == nil || db == nil {
		return false
	}
	return YnMModule.HasMinAdminLevelWithDB(adminPlugin.Db, invokerNick, invokerHost, channel, 4)
}

// FRISSÍTETT HandleJellyfishCommand: most invoker adatokkal (nick, host, channel)
// és adminPlugin + db pointerekkel, hogy DB-alapú jogosultságellenőrzést tudjunk végezni.
func HandleJellyfishCommand(cfg *YnMConfig.MediaActivityConfig, sendFunc func(target, msg string),
	rawMsg, invokerNick, invokerHost, channel string, adminPlugin *owner.YnmAdminPlugin, db *YnMDb.AdminDB) bool {

	fmt.Printf("[HandleJellyfishCommand] nyers üzenet: %q (invoker=%s host=%s channel=%s)\n", rawMsg, invokerNick, invokerHost, channel)

	fields := strings.Fields(rawMsg)
	if len(fields) == 0 {
		return false
	}

	// target csatorna
	target := cfg.ReportChannel
	if target == "" {
		target = cfg.IRCChannel
	}

	switch fields[0] {
	case "!jf":
		if len(fields) > 1 {
			switch strings.ToLower(fields[1]) {
			case "reset":
				// admin-only
				if !isInvokerAdmin(adminPlugin, db, invokerNick, invokerHost, channel) {
					//sendFunc(target, fmt.Sprintf("⛔ @%s: nincs jogosultságod a !jf reset használatához.", invokerNick))
					return true
				}
				arg := ""
				if len(fields) > 2 {
					arg = fields[2]
				}
				CommandJellyfishReset(cfg, sendFunc, arg)
				return true
			case "status":
				CommandJellyfishStatus(cfg, sendFunc)
				return true
			default:
				CommandJellyfish(cfg, sendFunc)
				return true
			}
		}
		CommandJellyfish(cfg, sendFunc)
		return true

	case "!jfstatus":
		CommandJellyfishStatus(cfg, sendFunc)
		return true

	case "!jfreset":
		// admin-only
		if !isInvokerAdmin(adminPlugin, db, invokerNick, invokerHost, channel) {
			//sendFunc(target, fmt.Sprintf("⛔ @%s: nincs jogosultságod a !jfreset parancshoz.", invokerNick))
			return true
		}
		arg := ""
		if len(fields) > 1 {
			arg = fields[1]
		}
		CommandJellyfishReset(cfg, sendFunc, arg)
		return true

	case "!jfsuspend":
		// admin-only
		if !isInvokerAdmin(adminPlugin, db, invokerNick, invokerHost, channel) {
			//sendFunc(target, fmt.Sprintf("⛔ @%s: nincs jogosultságod a !jfsuspend parancshoz.", invokerNick))
			return true
		}
		arg := ""
		if len(fields) > 1 {
			arg = fields[1]
		}
		CommandJellyfishSuspend(cfg, sendFunc, arg)
		return true

	case "!jfunsuspend":
		// admin-only
		if !isInvokerAdmin(adminPlugin, db, invokerNick, invokerHost, channel) {
			//sendFunc(target, fmt.Sprintf("⛔ @%s: nincs jogosultságod a !jfunsuspend parancshoz.", invokerNick))
			return true
		}
		arg := ""
		if len(fields) > 1 {
			arg = fields[1]
		}
		CommandJellyfishUnsuspend(cfg, sendFunc, arg)
		return true
	}

	return false
}


func CommandJellyfish(cfg *YnMConfig.MediaActivityConfig, sendFunc func(target, msg string)) {
	if cfg == nil || !cfg.Enabled {
		sendFunc(cfg.IRCChannel, "❌ Media aktivitás plugin ki van kapcsolva")
		return
	}
	tracker := NewWarningTracker(cfg.BaseDataDir)
	SendJellyfishReport(cfg, sendFunc, tracker)
}

// ============================================
// !jfreset parancs
// ============================================

func CommandJellyfishReset(cfg *YnMConfig.MediaActivityConfig, sendFunc func(target, msg string), arg string) {
	if cfg == nil || !cfg.Enabled {
		sendFunc(cfg.IRCChannel, "❌ Media aktivitás plugin ki van kapcsolva")
		return
	}

	target := cfg.ReportChannel
	if target == "" {
		target = cfg.IRCChannel
	}

	if arg == "" {
		sendFunc(target, "❓ Használat: !jfreset <felhasználónév> vagy !jfreset all")
		return
	}

	tracker := NewWarningTracker(cfg.BaseDataDir)

	if arg == "all" {
		tracker.mu.Lock()
		tracker.warnings = make(map[string]int)
		tracker.mu.Unlock()
		tracker.save()
		sendFunc(target, "✅ Minden felhasználó figyelmeztetése/felfüggesztése törölve lett.")
		return
	}

	if tracker.GetStage(arg) == StageNone {
		sendFunc(target, fmt.Sprintf("ℹ️ @%s-nak nincs eltárolt figyelmeztetése.", arg))
		return
	}

	tracker.ResetWarnings(arg)
	sendFunc(target, fmt.Sprintf("✅ @%s figyelmeztetése/felfüggesztése törölve lett.", arg))
}

// ============================================
// !jfstatus parancs
// ============================================

func CommandJellyfishStatus(cfg *YnMConfig.MediaActivityConfig, sendFunc func(target, msg string)) {
	if cfg == nil || !cfg.Enabled {
		sendFunc(cfg.IRCChannel, "❌ Media aktivitás plugin ki van kapcsolva")
		return
	}

	target := cfg.ReportChannel
	if target == "" {
		target = cfg.IRCChannel
	}

	tracker := NewWarningTracker(cfg.BaseDataDir)

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	if len(tracker.warnings) == 0 {
		sendFunc(target, "ℹ️ Jelenleg senkinek sincs eltárolt figyelmeztetése.")
		return
	}

	for user, stage := range tracker.warnings {
		var label string
		switch {
		case stage >= StageSuspended:
			label = "felfüggesztve"
		case stage == StageWarn3:
			label = "3. figyelmeztetés (24 óra)"
		case stage == StageWarn2:
			label = "2. figyelmeztetés (48 óra)"
		case stage == StageWarn1:
			label = "1. figyelmeztetés (72 óra)"
		default:
			label = fmt.Sprintf("ismeretlen (%d)", stage)
		}
		sendFunc(target, fmt.Sprintf("• @%s → %s", user, label))
	}
}