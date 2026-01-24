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
package owner

import (
	"fmt"
	"strings"
	"time"
	"log"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

const (
	MAX_MODES_PER_COMMAND = 6
	MODE_DELAY            = 100 * time.Millisecond
	QUEUE_RETRY_DELAY     = 500 * time.Millisecond
)

// Role hierarchy levels
const (
	ROLE_owner = 5
	ROLE_ADMIN = 4
	ROLE_MOD   = 3
	ROLE_VIP   = 2
	ROLE_USER  = 1
	ROLE_NONE  = 0
)

type UserModeConfig struct {
	Op     bool
	Voice  bool
	HalfOp bool
}

type UserModeInfo struct {
	Nick       string
	Hostmask   string
	ModeConfig UserModeConfig
}

func (p *YnmAdminPlugin) getAutoModesFromDB(nick, hostmask, channel string) (bool, bool, bool) {
    simpleHost := YnMModule.SimplifyHostmask(hostmask)
    
    var autoOp, autoVoice, autoHalfop bool   
    query := `SELECT auto_op, auto_voice, auto_halfop 
              FROM channel_users 
              WHERE hostmask = ? AND LOWER(channel) = LOWER(?) 
              LIMIT 1`   
    
    err := p.Db.QueryRow(query, simpleHost, channel).Scan(&autoOp, &autoVoice, &autoHalfop)
    
    if err != nil {
        return false, false, false
    }
    
    return autoOp, autoVoice, autoHalfop
}

func (p *YnmAdminPlugin) getCombinedModeConfig(globalRole, localRole string, 
    autoOp, autoVoice, autoHalfop bool) UserModeConfig {
    return UserModeConfig{
        Op:     autoOp,
        HalfOp: autoHalfop,
        Voice:  autoVoice,
    }
}

func (p *YnmAdminPlugin) AutoApplyUserModes(nick, hostmask, channel string) {
    // user-specifikus (channel_users)
    uOp, uVoice, uHalfop := p.getAutoModesFromDB(nick, hostmask, channel)

    // csatorna default (channels)
    dOp, dVoice, dHalfop := p.getChannelDefaultModesFromDB(channel)

    // ✅ FORCED default: minden belépőre érvényes (OR)
    autoOp := uOp || dOp
    autoVoice := uVoice || dVoice
    autoHalfop := uHalfop || dHalfop

    if !autoOp && !autoVoice && !autoHalfop {
        return
    }

    modeConfig := UserModeConfig{
        Op:     autoOp,
        HalfOp: autoHalfop,
        Voice:  autoVoice,
    }

    time.Sleep(MODE_DELAY)
    p.applyUserModes(nick, channel, modeConfig)
}

func (p *YnmAdminPlugin) ApplyModesToChannel(channel string) {
    log.Printf("[INFO] Jogok alkalmazása mindenkire: %s", channel) 
    p.Bot.SendRaw(fmt.Sprintf("WHO %s", channel))

}


// getUserModeInfo retrieves mode information for a user based on global and local roles
func (p *YnmAdminPlugin) getUserModeInfo(nick, channel string) *UserModeInfo {
	userInfo, err := p.Db.GetUserInfoByNick(nick)
	if err != nil || userInfo == nil {
		// Fallback: Try direct database lookup
		return p.checkUserByDirectDatabaseLookup(nick, channel)
	}
	
	hostmask := userInfo.Hostmask
	return p.checkUserByKnownPatternsForModeInfo(nick, hostmask, channel)
}

func (p *YnmAdminPlugin) checkUserByDirectDatabaseLookup(nick, channel string) *UserModeInfo {
	// Common patterns based on nick
	commonPatterns := []string{
		fmt.Sprintf("*!*@%s", strings.ToLower(nick)), 
		fmt.Sprintf("*@%s", strings.ToLower(nick)),
	}
	
	for _, pattern := range commonPatterns {
		// Get global role
		globalRole := ""
		userInfo, err := p.Db.GetUserInfoByHost(pattern)
		if err == nil && userInfo != nil {
			globalRole = userInfo.Role  // ← userInfo pointer, .Role string
		}
		
		// Get local role
		localRole := ""
		channelUserInfo, err := p.Db.GetUserChannelRole(pattern, channel)
		if err == nil && channelUserInfo != nil {
			localRole = channelUserInfo.Role  // ← channelUserInfo pointer, .Role string
		}
		
		// Combine both roles
		modeConfig := p.getCombinedModeConfig(globalRole, localRole, false, false, false)
		
		if modeConfig.Op || modeConfig.HalfOp || modeConfig.Voice {
			return &UserModeInfo{
				Nick:       nick,
				Hostmask:   pattern,
				ModeConfig: modeConfig,
			}
		}
	}
	return nil
}
func (p *YnmAdminPlugin) getChannelDefaultModesFromDB(channel string) (bool, bool, bool) {
    var autoOp, autoVoice, autoHalfop bool

    err := p.Db.QueryRow(`
        SELECT auto_op, auto_voice, auto_halfop
        FROM channels
        WHERE LOWER(name) = LOWER(?)
        LIMIT 1
    `, channel).Scan(&autoOp, &autoVoice, &autoHalfop)

    if err != nil {
        return false, false, false
    }
    return autoOp, autoVoice, autoHalfop
}

func (p *YnmAdminPlugin) checkUserByKnownPatternsForModeInfo(nick, hostmask, channel string) *UserModeInfo {
	simpleHost := YnMModule.SimplifyHostmask(hostmask)
	if simpleHost == "" {
		return nil
	}

	// Extract domain from hostmask
	domain := extractDomain(hostmask)
	if domain == "" {
		return nil
	}

	// Domain-based patterns
	patterns := []string{
		fmt.Sprintf("*!*@%s", domain),
		fmt.Sprintf("*@%s", domain),
		fmt.Sprintf("~*@%s", domain),
		fmt.Sprintf("*!~*@%s", domain),
		fmt.Sprintf("~o@%s", domain),
		fmt.Sprintf("*!*@%s", strings.ToLower(domain)),
	}
	
	for _, hostmaskPattern := range patterns {
		// Get global role
		globalUserInfo, err := p.Db.GetUserInfoByHost(hostmaskPattern)
		if err != nil || globalUserInfo == nil {
			continue
		}
		
		// Get local role (channel-specific)
		localRole := ""
		localChannelInfo, err := p.Db.GetUserChannelRole(hostmaskPattern, channel)
		if err == nil && localChannelInfo != nil {
			localRole = localChannelInfo.Role
		}
		
		// Combine both roles (global + local)
		// JAVÍTVA: globalUserInfo.Role (nem globalRole)
		modeConfig := p.getCombinedModeConfig(globalUserInfo.Role, localRole, false, false, false)
		
		if modeConfig.Op || modeConfig.HalfOp || modeConfig.Voice {
			return &UserModeInfo{
				Nick:       nick,
				Hostmask:   hostmaskPattern,
				ModeConfig: modeConfig,
			}
		}
	}

	return nil
}

// extractDomain extracts domain part from hostmask
func extractDomain(hostmask string) string {
	parts := strings.Split(hostmask, "@")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
}

// applyUserModes applies modes to a user with batch optimization
func (p *YnmAdminPlugin) applyUserModes(nick, channel string, config UserModeConfig) {
	modes, args := p.buildModeArguments(nick, config)
	
	if modes != "" {
		cmd := fmt.Sprintf("MODE %s +%s %s", channel, modes, strings.Join(args, " "))
		if err := p.safeSendRaw(cmd); err != nil {
			p.applyModesIndividually(nick, channel, config)
		}
	}
}

// buildModeArguments constructs mode strings and arguments
func (p *YnmAdminPlugin) buildModeArguments(nick string, config UserModeConfig) (string, []string) {
	var modes strings.Builder
	var args []string

	if config.Op {
		modes.WriteString("o")
		args = append(args, nick)
	}
	if config.HalfOp {
		modes.WriteString("h")
		args = append(args, nick)
	}
	if config.Voice {
		modes.WriteString("v")
		args = append(args, nick)
	}

	return modes.String(), args
}

// applyModesIndividually applies modes one by one as fallback
func (p *YnmAdminPlugin) applyModesIndividually(nick, channel string, config UserModeConfig) {
	if config.Op {
		p.safeSendRaw(fmt.Sprintf("MODE %s +o %s", channel, nick))
		time.Sleep(MODE_DELAY * 2)
	}
	if config.HalfOp {
		p.safeSendRaw(fmt.Sprintf("MODE %s +h %s", channel, nick))
		time.Sleep(MODE_DELAY * 2)
	}
	if config.Voice {
		p.safeSendRaw(fmt.Sprintf("MODE %s +v %s", channel, nick))
		time.Sleep(MODE_DELAY * 2)
	}
}

// applyBatchModes applies modes to multiple users efficiently
func (p *YnmAdminPlugin) applyBatchModes(channel string, users []UserModeInfo) {
	opUsers, halfOpUsers, voiceUsers := p.groupUsersByMode(users)

	time.Sleep(MODE_DELAY * 2)

	p.sendBatchModes(channel, "o", opUsers)
	p.sendBatchModes(channel, "h", halfOpUsers)
	p.sendBatchModes(channel, "v", voiceUsers)
}

// groupUsersByMode categorizes users by their required modes
func (p *YnmAdminPlugin) groupUsersByMode(users []UserModeInfo) ([]string, []string, []string) {
	var opUsers, halfOpUsers, voiceUsers []string

	for _, user := range users {
		if user.ModeConfig.Op {
			opUsers = append(opUsers, user.Nick)
		}
		if user.ModeConfig.HalfOp {
			halfOpUsers = append(halfOpUsers, user.Nick)
		}
		if user.ModeConfig.Voice {
			voiceUsers = append(voiceUsers, user.Nick)
		}
	}

	return opUsers, halfOpUsers, voiceUsers
}

// sendBatchModes sends mode commands in batches
func (p *YnmAdminPlugin) sendBatchModes(channel, mode string, users []string) {
	if len(users) == 0 {
		return
	}

	for i := 0; i < len(users); i += MAX_MODES_PER_COMMAND {
		end := min(i+MAX_MODES_PER_COMMAND, len(users))
		batch := users[i:end]
		
		modeChanges := strings.Repeat(mode, len(batch))
		cmd := fmt.Sprintf("MODE %s +%s %s", channel, modeChanges, strings.Join(batch, " "))
		
		if err := p.safeSendRaw(cmd); err != nil {
			p.applyModesToBatchIndividually(channel, mode, batch)
		}

		if end < len(users) {
			delay := time.Duration(float64(MODE_DELAY) * 1.5 / float64(len(batch)))
			time.Sleep(delay)
		}
	}
}

// applyModesToBatchIndividually applies modes individually as fallback for a batch
func (p *YnmAdminPlugin) applyModesToBatchIndividually(channel, mode string, users []string) {
	for _, nick := range users {
		p.safeSendRaw(fmt.Sprintf("MODE %s +%s %s", channel, mode, nick))
		time.Sleep(MODE_DELAY)
	}
}

// safeSendRaw safely sends raw commands with retry logic
func (p *YnmAdminPlugin) safeSendRaw(cmd string) error {
	for {
		err := p.Bot.SendRaw(cmd)
		if err != nil && strings.Contains(err.Error(), "send queue full") {
			time.Sleep(QUEUE_RETRY_DELAY)
			continue
		}
		return err
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}