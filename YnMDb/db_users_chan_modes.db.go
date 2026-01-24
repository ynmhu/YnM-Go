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

package YnMDb



import (
	"database/sql"
	"fmt"
//	"log"
	"strings"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
	"golang.org/x/crypto/bcrypt"

)

// UpdateUserAutomodeByHost updates channel-level automode settings
func (a *AdminDB) UpdateUserAutomodeByHost(hostmask, channel, automode string) error {
    channel = strings.ToLower(channel)
    
    // Extract just the host part for comparison (user@host)
    userHost := strings.Split(hostmask, "@")
    if len(userHost) < 2 {
        return fmt.Errorf("invalid hostmask format")
    }
    compareHost := "*!*@" + userHost[1]  

    var isowner bool
    err := a.db.QueryRow(`
        SELECT 1 FROM channel_users 
        WHERE LOWER(name) = ? AND LOWER(owner_hostmask) = LOWER(?)
    `, channel, compareHost).Scan(&isowner)
    
    if err != nil {
        if err == sql.ErrNoRows {
            return fmt.Errorf("you must be channel owner to modify automode")
        }
        return fmt.Errorf("database error: %v", err)
    }

    // Reset all automode flags first
    _, err = a.db.Exec(`
        UPDATE channel_users 
        SET auto_op = 0, auto_voice = 0, auto_halfop = 0
        WHERE LOWER(name) = ?
    `, channel)
    if err != nil {
        return fmt.Errorf("failed to reset automodes: %v", err)
    }

    // Set the specific automode if requested
    if automode != "" && automode != "off" && automode != "none" {
        var column string
        switch automode {
        case "+o":
            column = "auto_op"
        case "+h":
            column = "auto_halfop"
        case "+v":
            column = "auto_voice"
        default:
            return fmt.Errorf("invalid automode: %s", automode)
        }

        _, err = a.db.Exec(fmt.Sprintf(`
            UPDATE channel_users 
            SET %s = 1 
            WHERE LOWER(name) = ?
        `, column), channel)
        if err != nil {
            return fmt.Errorf("failed to set automode: %v", err)
        }
    }

    return nil
}
func (a *AdminDB) GetUserAutomode(userID, channelID int) (string, error) {
	var (
		autoOp     bool
		autoVoice  bool
		autoHalfOp bool
	)

	err := a.db.QueryRow(`
		SELECT auto_op, auto_voice, auto_halfop 
		FROM channel_users 
		WHERE user_id = ? AND channel_id = ?`,
		userID, channelID).Scan(&autoOp, &autoVoice, &autoHalfOp)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}

	switch {
	case autoOp:
		return "+o", nil
	case autoHalfOp:
		return "+h", nil
	case autoVoice:
		return "+v", nil
	default:
		return "", nil
	}
}
func (a *AdminDB) UpdateLastLogin(nick string) error {
	_, err := a.db.Exec(`
		UPDATE users 
		SET last_login = CURRENT_TIMESTAMP 
		WHERE nick = ? COLLATE NOCASE
	`, nick)
	return err
}
// Jelszó és egyéb mezők frissítése hostmask alapján
func (a *AdminDB) UpdateUserFieldByHost(hostmask, field, value string) error {
	// Jelszó hash-elés (ha pass mezőt módosítunk)
	if field == "pass" {
		if len(value) < 4 || len(value) > 20 {
			return fmt.Errorf("password must be between 4 and 20 characters")
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(value), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		value = string(hashed)
	}
	
	// Engedélyezett mezők listája
	validFields := map[string]bool{
		"mychar":       true,
		"welcome":      true,
		"pass":         true,
		"lang":         true,
		"nick":         true,
		"email":        true,
		"discord_id":   true,
		"telegram_id":  true,
		"facebook":     true,
		"avatar_url":   true,  // ÚJ
		"avatar_type":  true,  // ÚJ
	}
	
	if !validFields[field] {
		return fmt.Errorf("invalid field: %s", field)
	}
	
	// UPDATE query - updated_at is frissül
	query := fmt.Sprintf("UPDATE users SET %s = ?, updated_at = CURRENT_TIMESTAMP WHERE hostmask = ?", field)
	result, err := a.db.Exec(query, value, hostmask)
	if err != nil {
		return err
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("no user found with hostmask: %s", hostmask)
	}
	
	return nil
}
// Felhasználó lekérése vagy létrehozása hostmask alapján
func (a *AdminDB) GetOrCreateUserByHost(nick, hostmask string) (*UserInfo, error) {
	info, err := a.GetUserInfoByHost(hostmask)
	if err == nil {
		if info.Nick != nick {
			_ = a.UpdateUserFieldByHost(hostmask, "nick", nick)
			info.Nick = nick
		}
		return info, nil
	}
	if err == sql.ErrNoRows {
		if err := a.CreateUserByHost(nick, hostmask); err != nil {
			return nil, fmt.Errorf("error creating user: %v", err)
		}
		return a.GetUserInfoByHost(hostmask)
	}
	return nil, err
}



func (a *AdminDB) GetUserInfoByHost(hostmask string) (*UserInfo, error) {
	normalizedHost := normalizeHostmask(hostmask)
	
	query := `
		SELECT nick, hostmask, role, added_by, lang, mychar, welcome, pass, email, 
		       discord_id, telegram_id, facebook, invites, 
		       avatar_url, avatar_type, last_login, updated_at, created_at
		FROM users
		WHERE hostmask = ? 
		   OR hostmask LIKE ? 
		   OR hostmask LIKE ?
		LIMIT 1
	`
	
	// Keresési minták: pontos egyezés, kezdődik vele, vagy tartalmazza
	pattern1 := normalizedHost + ",%"  // Első helyen van
	pattern2 := "%," + normalizedHost + "%"  // Középen vagy végén van
	
	row := a.db.QueryRow(query, normalizedHost, pattern1, pattern2)
	
	var info UserInfo
	var myChar, welcome, pass, email, discordID, telegramID, facebook sql.NullString
	var avatarURL sql.NullString  // ÚJ
	var lastLogin, updatedAt sql.NullTime  // ÚJ
	
	err := row.Scan(
		&info.Nick, &info.Hostmask, &info.Role, &info.AddedBy, &info.Lang,
		&myChar, &welcome, &pass, &email, &discordID, &telegramID, &facebook,
		&info.Invites, 
		&avatarURL, &info.AvatarType, &lastLogin, &updatedAt, &info.CreatedAt,  // ÚJ
	)
	if err != nil {
		return nil, err
	}
	
	// Convert null strings to pointers
	if myChar.Valid {
		info.MyChar = &myChar.String
	}
	if welcome.Valid {
		info.Welcome = &welcome.String
	}
	if pass.Valid {
		info.Pass = &pass.String
	}
	if email.Valid {
		info.Email = &email.String
	}
	if discordID.Valid {
		info.DiscordID = &discordID.String
	}
	if telegramID.Valid {
		info.TelegramID = &telegramID.String
	}
	if facebook.Valid {
		info.Facebook = &facebook.String
	}
	if avatarURL.Valid {  // ÚJ
		info.AvatarURL = &avatarURL.String
	}
	if lastLogin.Valid {  // ÚJ
		info.LastLogin = &lastLogin.Time
	}
	if updatedAt.Valid {  // ÚJ
		info.UpdatedAt = &updatedAt.Time
	}
	
	return &info, nil
}

func (a *AdminDB) GetUserByDiscordID(discordID string) (*UserInfo, error) {
	var ui UserInfo
	var mychar, welcome, pass, email, discID, telegramID, facebook sql.NullString
	var avatarURL sql.NullString  // ÚJ
	var lastLogin, updatedAt sql.NullTime  // ÚJ
	
	err := a.db.QueryRow(`
		SELECT 
			nick, hostmask, role, added_by, lang,
			mychar, welcome, pass, email,
			discord_id, telegram_id, facebook, invites,
			avatar_url, avatar_type, last_login, updated_at,
			created_at
		FROM users 
		WHERE discord_id = ? COLLATE NOCASE
	`, discordID).Scan(
		&ui.Nick, &ui.Hostmask, &ui.Role, &ui.AddedBy, &ui.Lang,
		&mychar, &welcome, &pass, &email,
		&discID, &telegramID, &facebook, &ui.Invites,
		&avatarURL, &ui.AvatarType, &lastLogin, &updatedAt,
		&ui.CreatedAt,
	)
	
	if err != nil {
		return nil, err
	}
	
	// NULL kezelések
	if mychar.Valid { ui.MyChar = &mychar.String }
	if welcome.Valid { ui.Welcome = &welcome.String }
	if pass.Valid { ui.Pass = &pass.String }
	if email.Valid { ui.Email = &email.String }
	if discID.Valid { ui.DiscordID = &discID.String }
	if telegramID.Valid { ui.TelegramID = &telegramID.String }
	if facebook.Valid { ui.Facebook = &facebook.String }
	if avatarURL.Valid { ui.AvatarURL = &avatarURL.String }
	if lastLogin.Valid { ui.LastLogin = &lastLogin.Time }
	if updatedAt.Valid { ui.UpdatedAt = &updatedAt.Time }
	
	return &ui, nil
}

func (a *AdminDB) GetUserByTelegramID(telegramID string) (*UserInfo, error) {
	var ui UserInfo
	var mychar, welcome, pass, email, discordID, telID, facebook sql.NullString
	var avatarURL sql.NullString  // ÚJ
	var lastLogin, updatedAt sql.NullTime  // ÚJ
	
	err := a.db.QueryRow(`
		SELECT 
			nick, hostmask, role, added_by, lang,
			mychar, welcome, pass, email,
			discord_id, telegram_id, facebook, invites,
			avatar_url, avatar_type, last_login, updated_at,
			created_at
		FROM users 
		WHERE telegram_id = ? COLLATE NOCASE
	`, telegramID).Scan(
		&ui.Nick, &ui.Hostmask, &ui.Role, &ui.AddedBy, &ui.Lang,
		&mychar, &welcome, &pass, &email,
		&discordID, &telID, &facebook, &ui.Invites,
		&avatarURL, &ui.AvatarType, &lastLogin, &updatedAt,
		&ui.CreatedAt,
	)
	
	if err != nil {
		return nil, err
	}
	
	// NULL kezelések
	if mychar.Valid { ui.MyChar = &mychar.String }
	if welcome.Valid { ui.Welcome = &welcome.String }
	if pass.Valid { ui.Pass = &pass.String }
	if email.Valid { ui.Email = &email.String }
	if discordID.Valid { ui.DiscordID = &discordID.String }
	if telID.Valid { ui.TelegramID = &telID.String }
	if facebook.Valid { ui.Facebook = &facebook.String }
	if avatarURL.Valid { ui.AvatarURL = &avatarURL.String }
	if lastLogin.Valid { ui.LastLogin = &lastLogin.Time }
	if updatedAt.Valid { ui.UpdatedAt = &updatedAt.Time }
	
	return &ui, nil
}

func (a *AdminDB) GetUserChannelsRoles(hostmask string) (map[string]string, error) {
    result := make(map[string]string)
    
    // 1. ELŐSZÖR: Csatorna tulajdonosok (a legfontosabb jog)
    rows, err := a.db.Query(`
        SELECT name, 'owner' as role 
        FROM channels 
        WHERE owner_hostmask = ? OR owner = ?
    `, hostmask, strings.Split(hostmask, "!")[0])
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    for rows.Next() {
        var channel, role string
        if err := rows.Scan(&channel, &role); err != nil {
            return nil, err
        }
        result[channel] = role
    }
    
    // 2. UTÁNA: Channel_users tábla
    rows2, err := a.db.Query(`
        SELECT channel, role
        FROM channel_users
        WHERE hostmask = ?
    `, hostmask)
    if err != nil {
        return result, nil // Már van eredmény a tulajdonosi csatornákból
    }
    defer rows2.Close()
    
    for rows2.Next() {
        var channel, role string
        if err := rows2.Scan(&channel, &role); err != nil {
            return result, nil
        }
        // Ne írjuk felül az "owner" jogot, ha már az van
        if _, exists := result[channel]; !exists {
            result[channel] = role
        }
    }
    
    return result, nil
}
// Szerepkör ellenőrzése hostmask alapján
func (a *AdminDB) HasRoleByHost(hostmask, role string) bool {
	var count int
	err := a.db.QueryRow(
		"SELECT COUNT(*) FROM users WHERE hostmask = ? AND role = ?",
		hostmask, role,
	).Scan(&count)
	return err == nil && count > 0
}

func (a *AdminDB) CreateUserByHost(nick, hostmask string) error {
	_, err := a.db.Exec(`
        INSERT INTO users (nick, hostmask, role, added_by, lang, mychar, email, discord_id, telegram_id, facebook, invites)
        VALUES (?, ?, 'user', 'YnM-Go', 'en', '!', NULL, NULL, NULL, NULL, 0)
    `, nick, hostmask)
	return err
}

func (a *AdminDB) SetUserChannelRole(userID, channelID int, role string, automodes []string) error {

	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Reset all automodes first
	_, err = tx.Exec(`
		INSERT OR REPLACE INTO channel_users 
		(user_id, channel_id, role, auto_op, auto_voice, auto_halfop)
		VALUES (?, ?, ?, 0, 0, 0)`,
		userID, channelID, role)
	if err != nil {
		return err
	}

	// Set specified automodes
	for _, mode := range automodes {
		var field string
		switch mode {
		case "+o":
			field = "auto_op"
		case "+v":
			field = "auto_voice"
		case "+h":
			field = "auto_halfop"
		default:
			continue
		}

		_, err = tx.Exec(fmt.Sprintf(`
			UPDATE channel_users  
			SET %s = 1 
			WHERE user_id = ? AND channel_id = ?`,
			field), userID, channelID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
func (a *AdminDB) GetUserIDByHost(hostmask string) (int, error) {
    var userID int
    err := a.db.QueryRow("SELECT id FROM users WHERE hostmask = ?", hostmask).Scan(&userID)
    if err != nil {
        return 0, err
    }
    return userID, nil
}

func (a *AdminDB) AddUserWithRole(nick, hostmask, role string, cfg *YnMConfig.Config, addedBy, addedByHost string) error {
    // 1. Beszúrás a users táblába
    _, err := a.db.Exec(`
        INSERT INTO users (nick, hostmask, role, added_by, lang, mychar, email, discord_id, telegram_id, facebook, invites)
        VALUES (?, ?, ?, ?, 'En', '!', NULL, NULL, NULL, NULL, 0)
    `, nick, hostmask, role, addedBy)
    if err != nil {
        return err
    }

    if strings.ToLower(role) == "owner" && cfg.ConsoleChannel != "" {
        normalizedChannel := strings.ToLower(cfg.ConsoleChannel)
        
        // PRÓBÁLJUK MINDENKÉPPEN FRISSÍTENI/ LÉTREHOZNI
        // 1. Próbáljuk frissíteni a channels táblát
        _, updateErr := a.db.Exec(
            `UPDATE channels SET owner = ?, owner_hostmask = ? WHERE LOWER(name) = LOWER(?)`, 
            nick, hostmask, normalizedChannel)
        
        // 2. Ha nem található (0 sor frissült), akkor hozzuk létre
        if updateErr != nil {
            // VAGY: ellenőrizd, hogy tényleg "nem található" hiba-e
            _ = a.AddChannel(normalizedChannel, nick, hostmask, addedByHost)
        }
        
        // 3. Mindenképpen biztosítsuk a channel_users-ban
        _, err = a.db.Exec(`
            INSERT INTO channel_users 
            (nick, hostmask, channel, role, auto_op, auto_voice, auto_halfop, added_by, added_by_host, created_at)
            VALUES (?, ?, ?, 'owner', 1, 0, 0, ?, ?, CURRENT_TIMESTAMP)
            ON CONFLICT(nick, channel) DO UPDATE SET
                role = 'owner',
                hostmask = excluded.hostmask,
                auto_op = 1,
                auto_voice = 0,
                auto_halfop = 0,
                added_by = excluded.added_by,
                added_by_host = excluded.added_by_host
        `, nick, hostmask, normalizedChannel, addedBy, addedByHost)
        if err != nil {
            return fmt.Errorf("nem sikerült beállítani az auto_op-t: %v", err)
        }
    }

    return nil
}
func (a *AdminDB) GetUserAutoModes(hostmask, channel string) (autoOp, autoVoice, autoHalfOp bool, err error) {
	simpleHostmask := YnMModule.SimplifyHostmask(hostmask)
	normalizedHost := normalizeHostmask(simpleHostmask)
	
	// Pattern-ek a többszörös host kereséshez
	pattern1 := normalizedHost + ",%"        // Első helyen van
	pattern2 := "%," + normalizedHost + "%"  // Középen vagy végén van
	
	// 1. Ellenőrizzük a channel_users táblában (JAVÍTOTT verzió)
	query1 := `
		SELECT 
			CASE WHEN COUNT(CASE WHEN auto_op = 1 THEN 1 END) > 0 THEN 1 ELSE 0 END as has_auto_op,
			CASE WHEN COUNT(CASE WHEN auto_voice = 1 THEN 1 END) > 0 THEN 1 ELSE 0 END as has_auto_voice,
			CASE WHEN COUNT(CASE WHEN auto_halfop = 1 THEN 1 END) > 0 THEN 1 ELSE 0 END as has_auto_halfop
		FROM channel_users
		WHERE LOWER(channel) = LOWER(?)
		  AND (hostmask = ? OR hostmask = ? OR hostmask LIKE ? OR hostmask LIKE ?)`
	
	var cuAutoOp, cuAutoVoice, cuAutoHalfOp bool
	err1 := a.db.QueryRow(query1, channel, simpleHostmask, normalizedHost, pattern1, pattern2).Scan(&cuAutoOp, &cuAutoVoice, &cuAutoHalfOp)
	if err1 != nil && err1 != sql.ErrNoRows {
		return false, false, false, err1
	}
	
	// 2. Ellenőrizzük a csatorna szintű auto-módokat a channels táblából
	query2 := `
		SELECT auto_op, auto_voice, auto_halfop
		FROM channels
		WHERE LOWER(name) = LOWER(?) 
		LIMIT 1`
	
	var chAutoOp, chAutoVoice, chAutoHalfOp bool
	err2 := a.db.QueryRow(query2, channel).Scan(&chAutoOp, &chAutoVoice, &chAutoHalfOp)
	if err2 != nil && err2 != sql.ErrNoRows {
		return false, false, false, err2
	}
	
	// 3. Összevonjuk a csatorna és felhasználói auto-módokat (OR logika)
	autoOp = cuAutoOp || chAutoOp
	autoVoice = cuAutoVoice || chAutoVoice
	autoHalfOp = cuAutoHalfOp || chAutoHalfOp
	
	return autoOp, autoVoice, autoHalfOp, nil
}
// SetChannelModes beállítja a channel mode-okat
func (a *AdminDB) SetChannelModes(channel, modes, setBy string) error {
	// Elmentjük a history-ba is
	_, err := a.db.Exec(`
		INSERT INTO channel_mode_history (channel, modes, set_by)
		VALUES (?, ?, ?)
	`, channel, modes, setBy)
	if err != nil {
		return err
	}
	
	// Frissítjük az aktuális mode-ot
	_, err = a.db.Exec(`
		UPDATE channels SET 
			current_modes = ?,
			modes_set_by = ?,
			modes_set_at = CURRENT_TIMESTAMP
		WHERE name = ?
	`, modes, setBy, channel)
	return err
}