package YnMCmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

// UnifiedUpdatePlugin - handles all update functionality:
// - Manual commands: !update, !upgrade, !version
// - Automatic update checking
// - Auto-upgrade capability
// - Notifications
type UnifiedUpdatePlugin struct {
	bot               *YnMIrC.Client
	adminPlugin       *owner.YnmAdminPlugin
	lastCheck         time.Time
	lastNotify        time.Time
	lastVersion       map[string]time.Time  // For !version command rate limiting
	hasUpdate         bool
	latestTag         string
	checkInterval     time.Duration
	notifyInterval    time.Duration
	enabled           bool
	autoUpgrade       bool
	upgrading         bool
	failedAttempts    int
	maxFailedAttempts int
	cooldownPeriod    time.Duration
	lastFailedTime    time.Time
}

func NewUnifiedUpdatePlugin(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin, cfg *YnMConfig.Config) *UnifiedUpdatePlugin {
	// Set default values
	checkInterval := cfg.UpdateCheck.CheckInterval
	if checkInterval <= 0 {
		checkInterval = 30 * time.Minute
	}

	notifyInterval := cfg.UpdateCheck.NotifyInterval
	if notifyInterval <= 0 {
		notifyInterval = 6 * time.Hour
	}

	// Get auto-upgrade setting
	autoUpgrade := false
	if cfg.UpdateCheck.AutoUpgrade != nil {
		autoUpgrade = *cfg.UpdateCheck.AutoUpgrade
	}

	plugin := &UnifiedUpdatePlugin{
		bot:               bot,
		adminPlugin:       adminPlugin,
		checkInterval:     checkInterval,
		notifyInterval:    notifyInterval,
		enabled:           cfg.UpdateCheck.Enabled,
		autoUpgrade:       autoUpgrade,
		upgrading:         false,
		failedAttempts:    0,
		maxFailedAttempts: 3,
		cooldownPeriod:    1 * time.Hour,
		lastVersion:       make(map[string]time.Time),
	}

//	log.Printf("[UnifiedUpdate] Plugin initialized - enabled: %v, auto_upgrade: %v, check_interval: %v", 
//		plugin.enabled, plugin.autoUpgrade, plugin.checkInterval)

	return plugin
}

// HandleMessage - processes manual commands: !update, !upgrade, !version
func (p *UnifiedUpdatePlugin) HandleMessage(msg YnMIrC.Message) string {
	text := strings.ToLower(msg.Text)
	if text != "!update" && text != "!upgrade" && text != "!version" {
		return ""
	}

	if strings.ToLower(msg.Channel) != strings.ToLower(p.adminPlugin.Cfg.ConsoleChannel) {
		return ""
	}

	nick, hostmask := YnMModule.GetEffectiveNickAndHost(p.adminPlugin, msg.Sender)
	role := YnMModule.GetUserGlobalRoleWithDB(p.adminPlugin.Db, nick, hostmask)
	if role != "owner" {
		return ""
	}



	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Printf("Error getting working directory: %v", err)
		dir = "."
	}

	switch text {
	case "!version":
		return p.handleVersionCommand(msg, nick)

	case "!update":
		return p.handleUpdateCommand(msg, dir)

	case "!upgrade":
		return p.handleUpgradeCommand(msg, dir)
	}

	return ""
}

func (p *UnifiedUpdatePlugin) handleVersionCommand(msg YnMIrC.Message, nick string) string {
	// Rate limiting for !version command
	if t, ok := p.lastVersion[nick]; ok {
		if time.Since(t) < 1*time.Second {
			return "" // Too fast, ignore
		}
	}

	currentVersion := owner.YnMVersion
	resp := "ℹ️ Bot current version: " + currentVersion
	p.bot.SendMessage(msg.Channel, resp)
	p.lastVersion[nick] = time.Now()
	return ""
}

func (p *UnifiedUpdatePlugin) handleUpdateCommand(msg YnMIrC.Message, dir string) string {
	currentVersion := owner.YnMVersion

	// Check for updates
	hasUpdate, latestTag, err := p.checkForUpdateSync(dir)
	if err != nil {
		p.bot.SendMessage(msg.Channel, "🔴 Update check failed: "+err.Error())
		return ""
	}

	if hasUpdate {
		comparison := p.compareVersions(currentVersion, latestTag)
		if comparison < 0 {
			p.bot.SendMessage(msg.Channel, "ℹ️ New version available: "+latestTag+" | Use: !upgrade")
		} else if comparison > 0 {
			p.bot.SendMessage(msg.Channel, "ℹ️ Local version newer than remote: "+currentVersion+" > "+latestTag)
		} else {
			p.bot.SendMessage(msg.Channel, "✅ Already running latest version ("+currentVersion+")")
		}
	} else {
		p.bot.SendMessage(msg.Channel, "✅ Already running latest version ("+currentVersion+")")
	}

	return ""
}

func (p *UnifiedUpdatePlugin) handleUpgradeCommand(msg YnMIrC.Message, dir string) string {
	if p.upgrading {
		p.bot.SendMessage(msg.Channel, "⚠️ Upgrade already in progress")
		return ""
	}

	currentVersion := owner.YnMVersion

	// Check if upgrade is actually needed
	hasUpdate, latestTag, err := p.checkForUpdateSync(dir)
	if err == nil && !hasUpdate {
		p.bot.SendMessage(msg.Channel, "✅ Already running latest version ("+currentVersion+"), upgrade not needed.")
		return ""
	}

	// Start manual upgrade
	go func() {
		if err := p.performUpgrade(latestTag, true); err != nil {
			p.bot.SendMessage(msg.Channel, "🔴 Manual upgrade failed: "+err.Error())
		}
	}()

	return ""
}

// OnTick - handles automatic update checking and auto-upgrade
func (p *UnifiedUpdatePlugin) OnTick() []YnMIrC.Message {
	if !p.enabled {
		return nil
	}

	now := time.Now()
	var messages []YnMIrC.Message

	// Periodic update checking
	if now.Sub(p.lastCheck) >= p.checkInterval {
		p.lastCheck = now

		hasUpdate, tag, err := p.checkForUpdate()
		if err != nil {
			log.Printf("[UnifiedUpdate] Update check failed: %v", err)
			return nil
		}

		// New update detected
		if hasUpdate && tag != p.latestTag {
			p.hasUpdate = true
			p.latestTag = tag
			p.lastNotify = time.Time{} // Reset to trigger immediate notification
			log.Printf("[UnifiedUpdate] New version detected: %s", tag)

			if p.autoUpgrade && !p.upgrading {
				// Check cooldown period
				if p.failedAttempts >= p.maxFailedAttempts && 
				   time.Since(p.lastFailedTime) < p.cooldownPeriod {
					log.Printf("[UnifiedUpdate] In cooldown period, skipping auto-upgrade")
					messages = append(messages, YnMIrC.Message{
						Channel: p.adminPlugin.Cfg.ConsoleChannel,
						Text: fmt.Sprintf("⏳ Auto-upgrade in cooldown after %d failed attempts. Next attempt in %v", 
							p.failedAttempts, p.cooldownPeriod-time.Since(p.lastFailedTime)),
					})
				} else {
					log.Println("[UnifiedUpdate] Starting auto-upgrade...")
					go func(targetTag string) {
						if err := p.performUpgrade(targetTag, false); err != nil {
							log.Printf("[UnifiedUpdate] Auto-upgrade failed: %v", err)
							p.bot.SendMessage(p.adminPlugin.Cfg.ConsoleChannel, 
								fmt.Sprintf("❌ Auto-upgrade failed: %v", err))
							
							if p.failedAttempts < p.maxFailedAttempts {
								p.bot.SendMessage(p.adminPlugin.Cfg.ConsoleChannel, 
									fmt.Sprintf("🔄 Will retry on next check (%d/%d attempts)", 
										p.failedAttempts, p.maxFailedAttempts))
							} else {
								p.bot.SendMessage(p.adminPlugin.Cfg.ConsoleChannel, 
									fmt.Sprintf("🚫 Auto-upgrade disabled for %v after %d failed attempts", 
										p.cooldownPeriod, p.maxFailedAttempts))
							}
						}
					}(p.latestTag)
				}
			}
		}
	}

	// Send notifications (only if auto-upgrade is disabled)
	if p.hasUpdate && !p.autoUpgrade && now.Sub(p.lastNotify) >= p.notifyInterval {
		p.lastNotify = now
		messages = append(messages, YnMIrC.Message{
			Channel: p.adminPlugin.Cfg.ConsoleChannel,
			Text: fmt.Sprintf("ℹ️ New version available: %s (current: %s) | Use: !upgrade", 
				p.latestTag, owner.YnMVersion),
		})
	}

	return messages
}

// checkForUpdate - async version for OnTick
func (p *UnifiedUpdatePlugin) checkForUpdate() (bool, string, error) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		dir = "."
	}
	return p.checkForUpdateSync(dir)
}

// checkForUpdateSync - synchronous version for manual commands
func (p *UnifiedUpdatePlugin) checkForUpdateSync(dir string) (bool, string, error) {
	if !p.isGitRepo(dir) {
		return false, "", fmt.Errorf("not a git repository")
	}

	currentVersion := owner.YnMVersion

	// Get remote tags with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmdTags := exec.CommandContext(ctx, "git", "ls-remote", "--tags", "origin")
	cmdTags.Dir = dir
	outTags, err := cmdTags.Output()
	if err != nil {
		return false, "", fmt.Errorf("git ls-remote error: %v", err)
	}

	tags := p.parseTags(string(outTags))
	if len(tags) == 0 {
		return false, "", fmt.Errorf("no remote tags found")
	}

	sortedTags := p.sortVersionTags(tags)
	if len(sortedTags) == 0 {
		return false, "", fmt.Errorf("no valid semver tags")
	}

	latestTag := sortedTags[len(sortedTags)-1]
	comparison := p.compareVersions(currentVersion, latestTag)

	return comparison < 0, latestTag, nil
}

func (p *UnifiedUpdatePlugin) performUpgrade(targetTag string, isManual bool) error {
	if p.upgrading {
		return fmt.Errorf("upgrade already in progress")
	}

	// Check cooldown for automatic upgrades
	if !isManual && p.failedAttempts >= p.maxFailedAttempts {
		if time.Since(p.lastFailedTime) < p.cooldownPeriod {
			return fmt.Errorf("upgrade in cooldown period")
		}
		p.failedAttempts = 0 // Reset after cooldown
	}

	p.upgrading = true
	defer func() { p.upgrading = false }()

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		dir = "."
	}

	upgradeType := "Auto-upgrade"
	if isManual {
		upgradeType = "Manual upgrade"
	}

	log.Printf("[UnifiedUpdate] Starting %s: %s -> %s", upgradeType, owner.YnMVersion, targetTag)

	// Send initial message
	p.bot.SendMessage(p.adminPlugin.Cfg.ConsoleChannel, 
		fmt.Sprintf("⬇️ %s starting: %s -> %s", upgradeType, owner.YnMVersion, targetTag))

	// Set PATH
	paths := []string{"/usr/local/go/bin", "/usr/bin", "/bin", "/usr/local/sbin", "/usr/sbin", "/sbin"}
	os.Setenv("PATH", strings.Join(paths, ":")+":"+os.Getenv("PATH"))

	// **FIX: Set writable TMPDIR and GOCACHE**
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = dir // Fallback to working directory
	}
	tmpDir := filepath.Join(homeDir, ".cache", "ynm-build-tmp")
	cacheDir := filepath.Join(homeDir, ".cache", "go-build")
	
	// Create directories if they don't exist
	os.MkdirAll(tmpDir, 0755)
	os.MkdirAll(cacheDir, 0755)
	
	os.Setenv("TMPDIR", tmpDir)
	os.Setenv("GOCACHE", cacheDir)
	
	log.Printf("[UnifiedUpdate] Using TMPDIR=%s, GOCACHE=%s", tmpDir, cacheDir)

	// Find Go binary
	goBinary, err := exec.LookPath("go")
	if err != nil {
		if !isManual {
			p.recordFailedAttempt()
		}
		return fmt.Errorf("'go' command not found")
	}

	// 1. Update dependencies
	p.bot.SendMessage(p.adminPlugin.Cfg.ConsoleChannel, "🔄 Updating dependencies...")
	if err := p.updateDependencies(goBinary, dir); err != nil {
		if !isManual {
			p.recordFailedAttempt()
		}
		return fmt.Errorf("dependency update failed: %v", err)
	}

	// 2. Git update
	p.bot.SendMessage(p.adminPlugin.Cfg.ConsoleChannel, "🔄 Updating code from Git...")
	if err := p.updateFromGit(dir); err != nil {
		if !isManual {
			p.recordFailedAttempt()
		}
		return fmt.Errorf("git update failed: %v", err)
	}

	// 3. Clean build cache (now using our custom GOCACHE)
	cleanCmd := exec.Command(goBinary, "clean", "-cache")
	cleanCmd.Dir = dir
	cleanCmd.Run()

	// 4. Build
	p.bot.SendMessage(p.adminPlugin.Cfg.ConsoleChannel, "🔨 Rebuilding binary...")
	if err := p.buildBinary(goBinary, dir); err != nil {
		if !isManual {
			p.recordFailedAttempt()
		}
		return fmt.Errorf("build failed: %v", err)
	}

	// 5. Success - reset failed attempts and restart
	p.failedAttempts = 0
	p.bot.SendMessage(p.adminPlugin.Cfg.ConsoleChannel, "✅ "+upgradeType+" completed, restarting...")
	log.Printf("[UnifiedUpdate] %s successful: %s", upgradeType, targetTag)

	// Restart
	if p.bot.OnQuit != nil {
		p.bot.OnQuit("YnM-Go", "♻️ Bot upgraded, restarting...")
	}
	time.Sleep(1 * time.Second)
	os.Exit(0)

	return nil
}

func (p *UnifiedUpdatePlugin) recordFailedAttempt() {
	p.failedAttempts++
	p.lastFailedTime = time.Now()
	log.Printf("[UnifiedUpdate] Failed attempt %d/%d recorded", p.failedAttempts, p.maxFailedAttempts)
}

// Helper methods (updateDependencies, updateFromGit, buildBinary, compareVersions, etc.)
// ... [Same implementation as in the previous version] ...

func (p *UnifiedUpdatePlugin) updateDependencies(goBinary, dir string) error {
	depsToUpdate := []struct {
		cmd  string
		args []string
	}{
		{"mod", []string{"tidy"}},
		{"get", []string{"github.com/shirou/gopsutil/process"}},
		{"get", []string{"golang.org/x/text/encoding/charmap"}},
		{"get", []string{"golang.org/x/text/transform"}},
		{"get", []string{"gopkg.in/yaml.v3"}},
		{"get", []string{"github.com/mattn/go-sqlite3"}},
		{"get", []string{"golang.org/x/crypto/bcrypt"}},
		{"get", []string{"github.com/PuerkitoBio/goquery"}},
		{"get", []string{"github.com/mmcdole/gofeed"}},
		{"mod", []string{"tidy"}},
	}

	for _, dep := range depsToUpdate {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		cmd := exec.CommandContext(ctx, goBinary, append([]string{dep.cmd}, dep.args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			cancel()
			output := strings.TrimSpace(string(out))
			if output != "" {
				log.Printf("[UnifiedUpdate] %s %s: %s", dep.cmd, strings.Join(dep.args, " "), output)
			}
		}
		cancel()
	}
	return nil
}

func (p *UnifiedUpdatePlugin) updateFromGit(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmdGit := exec.CommandContext(ctx, "git", "pull")
	cmdGit.Dir = dir
	out, err := cmdGit.CombinedOutput()
	if err != nil {
		output := strings.TrimSpace(string(out))
		if strings.Contains(output, "would be overwritten by merge") {
			p.bot.SendMessage(p.adminPlugin.Cfg.ConsoleChannel, "⚠️ Overwriting local changes (git reset --hard)...")
			
			ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel2()
			
			cmdReset := exec.CommandContext(ctx2, "git", "reset", "--hard")
			cmdReset.Dir = dir
			if resetOut, resetErr := cmdReset.CombinedOutput(); resetErr != nil {
				return fmt.Errorf("reset failed: %s", strings.TrimSpace(string(resetOut)))
			}

			ctx3, cancel3 := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel3()
			
			cmdGit2 := exec.CommandContext(ctx3, "git", "pull")
			cmdGit2.Dir = dir
			if out2, err2 := cmdGit2.CombinedOutput(); err2 != nil {
				return fmt.Errorf("git error after reset: %s", strings.TrimSpace(string(out2)))
			}
			p.bot.SendMessage(p.adminPlugin.Cfg.ConsoleChannel, "✅ Git update successful after reset.")
		} else {
			return fmt.Errorf("git error: %s", output)
		}
	}
	return nil
}

func (p *UnifiedUpdatePlugin) buildBinary(goBinary, dir string) error {
	// Régi bináris törlése
	_ = os.Remove(filepath.Join(dir, "YnM-Go"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmdBuild := exec.CommandContext(ctx, goBinary, "build", "-o", "YnM-Go")
	cmdBuild.Dir = dir

	if out, err := cmdBuild.CombinedOutput(); err != nil {
		errorOutput := strings.TrimSpace(string(out))
		if errorOutput != "" {
			return fmt.Errorf("build error: %s", errorOutput)
		}
		return fmt.Errorf("unknown build error")
	}

	return nil
}

func (p *UnifiedUpdatePlugin) compareVersions(v1, v2 string) int {
	clean1 := strings.TrimPrefix(v1, "YnM-v")
	clean2 := strings.TrimPrefix(v2, "YnM-v")

	parts1 := strings.Split(clean1, ".")
	parts2 := strings.Split(clean2, ".")

	nums1 := make([]int, 0)
	nums2 := make([]int, 0)

	for _, p := range parts1 {
		if num, err := strconv.Atoi(p); err == nil {
			nums1 = append(nums1, num)
		} else {
			return 0
		}
	}

	for _, p := range parts2 {
		if num, err := strconv.Atoi(p); err == nil {
			nums2 = append(nums2, num)
		} else {
			return 0
		}
	}

	maxLen := len(nums1)
	if len(nums2) > maxLen {
		maxLen = len(nums2)
	}

	for len(nums1) < maxLen {
		nums1 = append(nums1, 0)
	}
	for len(nums2) < maxLen {
		nums2 = append(nums2, 0)
	}

	for i := 0; i < maxLen; i++ {
		if nums1[i] < nums2[i] {
			return -1
		} else if nums1[i] > nums2[i] {
			return 1
		}
	}

	return 0
}

func (p *UnifiedUpdatePlugin) isGitRepo(dir string) bool {
	gitDir := filepath.Join(dir, ".git")
	if stat, err := os.Stat(gitDir); err == nil {
		return stat.IsDir() || stat.Mode().IsRegular()
	}
	return false
}

func (p *UnifiedUpdatePlugin) parseTags(output string) []string {
	lines := strings.Split(output, "\n")
	var tags []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		ref := parts[1]
		if strings.HasPrefix(ref, "refs/tags/") {
			tag := strings.TrimPrefix(ref, "refs/tags/")
			if !strings.HasSuffix(tag, "^{}") {
				tags = append(tags, tag)
			}
		}
	}

	return tags
}

func (p *UnifiedUpdatePlugin) sortVersionTags(tags []string) []string {
	type ver struct {
		orig  string
		parts []int
	}

	var vers []ver

	for _, t := range tags {
		versionStr := strings.TrimPrefix(t, "YnM-v")
		nums := strings.Split(versionStr, ".")
		var parts []int
		valid := true
		for _, n := range nums {
			i, err := strconv.Atoi(n)
			if err != nil {
				valid = false
				break
			}
			parts = append(parts, i)
		}
		if valid {
			vers = append(vers, ver{orig: t, parts: parts})
		}
	}

	sort.Slice(vers, func(i, j int) bool {
		a, b := vers[i].parts, vers[j].parts
		maxLen := len(a)
		if len(b) > maxLen {
			maxLen = len(b)
		}
		for k := 0; k < maxLen; k++ {
			ai, bi := 0, 0
			if k < len(a) {
				ai = a[k]
			}
			if k < len(b) {
				bi = b[k]
			}
			if ai != bi {
				return ai < bi
			}
		}
		return false
	})

	sorted := make([]string, len(vers))
	for i, v := range vers {
		sorted[i] = v.orig
	}
	return sorted
}

// Public control methods
func (p *UnifiedUpdatePlugin) SetEnabled(enabled bool) {
	p.enabled = enabled
	if !enabled {
		p.hasUpdate = false
	}
	log.Printf("[UnifiedUpdate] Plugin %s", map[bool]string{true: "enabled", false: "disabled"}[enabled])
}

func (p *UnifiedUpdatePlugin) SetAutoUpgrade(enabled bool) {
	p.autoUpgrade = enabled
	log.Printf("[UnifiedUpdate] Auto-upgrade %s", map[bool]string{true: "enabled", false: "disabled"}[enabled])
}

func (p *UnifiedUpdatePlugin) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"enabled":              p.enabled,
		"auto_upgrade":         p.autoUpgrade,
		"upgrading":            p.upgrading,
		"has_update":           p.hasUpdate,
		"latest_tag":           p.latestTag,
		"current_version":      owner.YnMVersion,
		"last_check":           p.lastCheck,
		"last_notify":          p.lastNotify,
		"check_interval":       p.checkInterval.String(),
		"notify_interval":      p.notifyInterval.String(),
		"failed_attempts":      p.failedAttempts,
		"max_failed_attempts":  p.maxFailedAttempts,
		"cooldown_period":      p.cooldownPeriod.String(),
		"last_failed_time":     p.lastFailedTime,
	}
}