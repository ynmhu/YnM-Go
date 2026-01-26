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
	"log"
	"time"
	"strings"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
)

// ChannelMode represents a saved channel mode configuration
type ChannelMode struct {
	ID          int       `json:"id"`
	Channel     string    `json:"channel"`
	Modes       string    `json:"modes"`
	ModeParams  string    `json:"mode_params"`  
	SetBy       string    `json:"set_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Active      bool      `json:"active"`
}

func (db *AdminDB) SaveChannelModes(channel, modes, modeParams, setBy string) error {
	channel = normalizeChannelName(channel)
	
	var existingID int
	err := db.db.QueryRow("SELECT id FROM channel_modes WHERE channel = ?", channel).Scan(&existingID)
	if err == sql.ErrNoRows {
		query := `INSERT INTO channel_modes (channel, modes, mode_params, set_by, created_at, updated_at, enabled, active)
				  VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1, 1)`
		_, err = db.db.Exec(query, channel, modes, modeParams, setBy)
		if err != nil {
			return fmt.Errorf("failed to insert channel modes: %v", err)
		}
		//log.Printf("[DB] Saved new channel modes for %s: %s %s (set by %s)", channel, modes, modeParams, setBy)
	} else if err != nil {
		return fmt.Errorf("failed to check existing channel modes: %v", err)
	} else {
		query := `UPDATE channel_modes 
				  SET modes = ?, mode_params = ?, set_by = ?, updated_at = CURRENT_TIMESTAMP, enabled = 1, active = 1
				  WHERE channel = ?`
		_, err = db.db.Exec(query, modes, modeParams, setBy, channel)
		if err != nil {
			return fmt.Errorf("failed to update channel modes: %v", err)
		}
		//log.Printf("[DB] Updated channel modes for %s: %s %s (set by %s)", channel, modes, modeParams, setBy)
	}
	return nil
}


// Get channel modes from database
func (db *AdminDB) GetChannelModes(channel string) (*ChannelMode, error) {
	var cm ChannelMode
	query := `SELECT id, channel, modes, mode_params, set_by, created_at, updated_at, active 
			  FROM channel_modes WHERE channel = ? AND active = 1`

	err := db.db.QueryRow(query, channel).Scan(
		&cm.ID, &cm.Channel, &cm.Modes, &cm.ModeParams, &cm.SetBy,
		&cm.CreatedAt, &cm.UpdatedAt, &cm.Active)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get channel modes: %v", err)
	}

	return &cm, nil
}

// Get all saved channel modes
func (db *AdminDB) GetAllChannelModes() ([]*ChannelMode, error) {
	query := `SELECT id, channel, modes, set_by, created_at, updated_at, active 
			  FROM channel_modes WHERE active = 1 ORDER BY channel`
	
	rows, err := db.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query channel modes: %v", err)
	}
	defer rows.Close()
	
	var modes []*ChannelMode
	for rows.Next() {
		var cm ChannelMode
		err := rows.Scan(&cm.ID, &cm.Channel, &cm.Modes, &cm.SetBy, 
						&cm.CreatedAt, &cm.UpdatedAt, &cm.Active)
		if err != nil {
			return nil, fmt.Errorf("failed to scan channel mode: %v", err)
		}
		modes = append(modes, &cm)
	}
	
	return modes, nil
}

// Delete channel modes (set as inactive)
func (db *AdminDB) DeleteChannelModes(channel string) error {
	query := `UPDATE channel_modes SET active = 0, updated_at = CURRENT_TIMESTAMP WHERE channel = ?`
	_, err := db.db.Exec(query, channel)
	if err != nil {
		return fmt.Errorf("failed to delete channel modes: %v", err)
	}
	
	// log.Printf("[DB] Deleted channel modes for %s", channel)
	return nil
}

// Get channel modes history (including inactive ones)
func (a *AdminDB) GetChannelModeHistory(channel string, limit int) ([]ChannelModeHistory, error) {
	rows, err := a.db.Query(`
		SELECT id, channel, modes, set_by, created_at
		FROM channel_mode_history
		WHERE channel = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, channel, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []ChannelModeHistory
	for rows.Next() {
		var cmh ChannelModeHistory
		if err := rows.Scan(&cmh.ID, &cmh.Channel, &cmh.Modes, &cmh.SetBy, &cmh.CreatedAt); err != nil {
			return nil, err
		}
		history = append(history, cmh)
	}
	return history, nil
}



func (db *AdminDB) ApplySavedChannelModes(bot *YnMIrC.Client, channel string) {
    channel = normalizeChannelName(channel)
    
    // 1. MENTETT MODE-OK LEKÉRDEZÉSE
    rows, err := db.db.Query("SELECT modes, mode_params FROM channel_modes WHERE channel = ? AND active = 1 AND enabled = 1", channel)
    if err != nil {
        return
    }
    defer rows.Close()
    
    var savedModes string
    var savedParams string
    foundModes := false
    
    for rows.Next() {
        var modes, params string
        if err := rows.Scan(&modes, &params); err != nil {
            continue
        }
        foundModes = true
        savedModes = modes
        savedParams = params
        break
    }
    
    if !foundModes {
        bot.SendRaw(fmt.Sprintf("MODE %s +nt", channel))
        return
    }
    
    // 2. MENTETT MODE-OK FELDOLGOZÁSA
    desiredModes := make(map[rune]bool)
    adding := true
    
    for _, ch := range savedModes {
        if ch == '+' {
            adding = true
        } else if ch == '-' {
            adding = false
        } else {
            desiredModes[ch] = adding
        }
    }
    
    // 3. JELENLEGI CSATORNA MODE LEKÉRÉSE A SZERVERRŐL
    // Létrehozunk egy channel-t a MODE válasz fogadására
    modeReceived := make(chan string, 1)
    currentModeString := ""
    
    // Indítunk egy goroutine-t ami figyeli a MODE választ
    go func() {
        // Várunk maximum 2 másodpercet a MODE válaszra
        timeout := time.After(2 * time.Second)
        startTime := time.Now()
        
        for {
            select {
            case <-timeout:
                log.Printf("⚠️ Timeout: MODE válasz nem érkezett meg 2 másodpercen belül")
                modeReceived <- ""
                return
            default:
                // Ellenőrizzük a channels map-et
                channels := bot.Channels()
                key := strings.ToLower(channel)
                if ch, exists := channels[key]; exists && ch.Modes != "" {
                    modeReceived <- ch.Modes
                    return
                }
                
                // Ha már több mint 1.5 másodperc telt el és nincs válasz
                if time.Since(startTime) > 1500*time.Millisecond {
                    modeReceived <- ""
                    return
                }
                
                time.Sleep(100 * time.Millisecond)
            }
        }
    }()
    
    // MODE lekérés küldése
    bot.SendRaw(fmt.Sprintf("MODE %s", channel))
    
    // Várunk a MODE válaszra
    currentModeString = <-modeReceived
    
    //log.Printf("🔍 DEBUG: Aktuális mode string a szerverről: '%s'", currentModeString)
    
    // 4. JELENLEGI MODE-OK FELDOLGOZÁSA
    currentModes := make(map[rune]bool)
    
    for _, mode := range currentModeString {
        if mode != '+' && mode != '-' {
            currentModes[mode] = true
        }
    }
    
    // 5. KÜLÖNBSÉGEK MEGHATÁROZÁSA
    var toAdd string
    var toRemove string
    
    allStandardModes := "mnstiprkpl"
    
    for _, mode := range allStandardModes {
        currentState := currentModes[mode]
        desiredState := desiredModes[mode]
        
        if desiredState && !currentState {
            // Szeretnénk rajta, de jelenleg nincs -> hozzáadás
            toAdd += string(mode)
        } else if !desiredState && currentState {
            // Nem szeretnénk rajta, de jelenleg rajta van -> eltávolítás
            toRemove += string(mode)
        }
    }
    
    // 6. MODE PARANCS ÖSSZEÁLLÍTÁSA ÉS KÜLDÉSE
    if toRemove == "" && toAdd == "" {
        //log.Printf("✅ Nincs változtatás szükséges a(z) %s csatornán (current: %s, desired: %s)", channel, currentModeString, savedModes)
        return
    }
    
    var cmd string
    if toRemove != "" && toAdd != "" {
        if savedParams != "" {
            cmd = fmt.Sprintf("MODE %s -%s+%s %s", channel, toRemove, toAdd, savedParams)
        } else {
            cmd = fmt.Sprintf("MODE %s -%s+%s", channel, toRemove, toAdd)
        }
    } else if toRemove != "" {
        cmd = fmt.Sprintf("MODE %s -%s", channel, toRemove)
    } else if toAdd != "" {
        if savedParams != "" {
            cmd = fmt.Sprintf("MODE %s +%s %s", channel, toAdd, savedParams)
        } else {
            cmd = fmt.Sprintf("MODE %s +%s", channel, toAdd)
        }
    }
    
   // log.Printf("📤 MODE parancs küldése: %s (current: %s, desired: %s, toAdd: %s, toRemove: %s)",  cmd, currentModeString, savedModes, toAdd, toRemove)
    bot.SendRaw(cmd)
}
func (db *AdminDB) ClearChannelModes(channel string) error {
	channel = normalizeChannelName(channel)
	
	// Üres modes-ra állítjuk, de megtartjuk a rekordot
	_, err := db.db.Exec(`
		UPDATE channel_modes 
		SET modes = '', mode_params = '', active = 0, enabled = 0, updated_at = CURRENT_TIMESTAMP
		WHERE channel = ?
	`, channel)
	if err != nil {
		return fmt.Errorf("failed to clear channel modes: %w", err)
	}
	
	//log.Printf("✅ Cleared modes for channel: %s", channel)
	return nil
}

func (a *AdminDB) UpdatePasswordByHost(hostmask, hashed string) error {
	_, err := a.db.Exec("UPDATE users SET pass = ? WHERE hostmask = ?", hashed, hostmask)
	return err
}

func (a *AdminDB) UpdateMyCharByHost(hostmask, newChar string) error {
    if newChar == "" {
        newChar = "!" 
    }
    return a.UpdateUserFieldByHost(hostmask, "mychar", newChar)
}

func (a *AdminDB) HasAnyowner() bool {
	var count int
	err := a.db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'owner'").Scan(&count)
	return err == nil && count > 0
}
// Database method to get user info by nick

func (db *AdminDB) GetUserInfoByNick(nick string) (*UserInfo, error) {
    var info UserInfo
    var myChar, welcome, pass, email, discordID, telegramID, facebook sql.NullString
    var avatarURL sql.NullString  // ÚJ
    var lastLogin, updatedAt sql.NullTime  // ÚJ
    
    query := `SELECT nick, hostmask, role, added_by, lang, mychar, welcome, pass, email,
                     discord_id, telegram_id, facebook, invites,
                     avatar_url, avatar_type, last_login, updated_at, created_at
              FROM users WHERE nick = ? COLLATE NOCASE`
    
    row := db.db.QueryRow(query, nick)
    err := row.Scan(
        &info.Nick, &info.Hostmask, &info.Role, &info.AddedBy, &info.Lang,
        &myChar, &welcome, &pass, &email, &discordID, &telegramID, &facebook,
        &info.Invites,
        &avatarURL, &info.AvatarType, &lastLogin, &updatedAt, &info.CreatedAt,
    )
    
    if err != nil {
        return nil, err
    }
    
    // Convert null values to pointers
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

// Database method to add a new user
func (db *AdminDB) AddUser(nick, hostmask, role, addedBy string) error {
    query := `INSERT INTO users (nick, hostmask, role, added_by, lang, created_at) 
              VALUES (?, ?, ?, ?, 'En', datetime('now'))`
    
    _, err := db.db.Exec(query, nick, hostmask, role, addedBy)
    return err
}

// Database method to update user role in channel
func (db *AdminDB) UpdateUserRoleInChannel(nick, hostmask, channel, role string) error {
    normalizedHost := normalizeHostmask(hostmask)
    
    fmt.Printf("[DEBUG UpdateUserRoleInChannel] nick=%s, hostmask=%s, channel=%s, role=%s\n", 
        nick, normalizedHost, channel, role)
    
    // Először pontos egyezéssel próbáljuk
    query := `UPDATE channel_users SET role = ? WHERE LOWER(nick) = LOWER(?) AND hostmask = ? AND LOWER(channel) = LOWER(?)`
    result, err := db.db.Exec(query, role, nick, normalizedHost, channel)
    
    if err != nil {
        return err
    }
    
    rowsAffected, _ := result.RowsAffected()
    
    // Ha nem talált semmit, próbáljuk LIKE-al (többszörös hostmask támogatás)
    if rowsAffected == 0 {
        query = `UPDATE channel_users 
                 SET role = ? 
                 WHERE LOWER(nick) = LOWER(?) 
                   AND LOWER(channel) = LOWER(?)
                   AND (hostmask LIKE ? OR hostmask LIKE ? OR hostmask LIKE ?)`
        result, err = db.db.Exec(query, role, nick, channel, 
            normalizedHost+",%", "%,"+normalizedHost, "%,"+normalizedHost+",%")
        
        if err != nil {
            return err
        }
        rowsAffected, _ = result.RowsAffected()
    }
    
    fmt.Printf("[DEBUG UpdateUserRoleInChannel] Updated %d row(s)\n", rowsAffected)
    
    return nil
}

func (db *AdminDB) GetSavedModes(channel string) (string, error) {
	cm, err := db.GetChannelModes(channel)
	if err != nil {
		return "", err
	}
	if cm == nil || !cm.Active {
		return "", nil
	}
	return cm.Modes, nil
}
func (a *AdminDB) SetUserAutomode(nick, hostmask, channel string, autoOp, autoVoice, autoHalfOp bool, isAdmin bool) error {
    res, err := a.db.Exec(`
        UPDATE channel_users
        SET auto_op = ?, auto_voice = ?, auto_halfop = ?
        WHERE LOWER(nick) = LOWER(?) AND hostmask = ? AND LOWER(channel) = LOWER(?)`,
        autoOp, autoVoice, autoHalfOp, nick, hostmask, channel)
    if err != nil {
        return err
    }
    rowsAffected, err := res.RowsAffected()
    if err != nil {
        return err
    }
    if rowsAffected == 0 {
        if !isAdmin {
            return fmt.Errorf("Nincs ilyen user/csatorna kombináció az adatbázisban")
        }
        // Ha admin, akkor beszúrjuk újként
		
        _, err = a.db.Exec(`
            INSERT INTO channel_users (nick, hostmask, channel, role, auto_op, auto_voice, auto_halfop, created_at)
            VALUES (?, ?, ?, 'user', ?, ?, ?, CURRENT_TIMESTAMP)
        `, nick, hostmask, channel, autoOp, autoVoice, autoHalfOp)
        if err != nil {
            return err
        }
    }
    return nil
}

func (a *AdminDB) UpdateChannelAutoModes(channelName string, permissions string) error {
	autoOp := 0
	autoVoice := 0
	autoHalfOp := 0

	if strings.Contains(permissions, "o") {
		autoOp = 1
	}
	if strings.Contains(permissions, "v") {
		autoVoice = 1
	}
	if strings.Contains(permissions, "h") {
		autoHalfOp = 1
	}
	res, err := a.db.Exec(`
		UPDATE channels
		SET auto_op = ?, auto_voice = ?, auto_halfop = ?
		WHERE LOWER(name) = LOWER(?)
	`, autoOp, autoVoice, autoHalfOp, strings.ToLower(channelName))

	if err != nil {
		// log.Printf("[UpdateChannelAutoModes] SQL hiba: %v", err)
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		// log.Printf("[UpdateChannelAutoModes] Nem tudtam lekérdezni a módosított sorok számát: %v", err)
	} else if rowsAffected == 0 {
		// log.Printf("[UpdateChannelAutoModes] Figyelem: nincs ilyen csatorna: %s", channelName)
	}

	return nil
}
func (db *AdminDB) ChannelExists(channel string) bool {
	var count int
	err := db.db.QueryRow("SELECT COUNT(*) FROM channels WHERE LOWER(name) = LOWER(?)", channel).Scan(&count)
	return err == nil && count > 0
}
func (a *AdminDB) IsUserOp(nick, hostmask, channel string) (bool, error) {
	query := `
		SELECT COUNT(*) 
		FROM channel_users 
		WHERE LOWER(nick) = LOWER(?) AND hostmask = ? AND LOWER(channel) = LOWER(?) AND auto_op = 1
	`
	
	var count int
	err := a.db.QueryRow(query, nick, hostmask, channel).Scan(&count)
	if err != nil {
		return false, err
	}
	
	return count > 0, nil
}

// IsChannelowner ellenőrzi, hogy a felhasználó owner-e a channelen
func (a *AdminDB) IsChannelowner(nick, channel string) (bool, error) {
	query := `
		SELECT role 
		FROM channel_users
		WHERE LOWER(nick) = LOWER(?) AND LOWER(channel) = LOWER(?)
	`
	
	rows, err := a.db.Query(query, nick, channel)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return false, err
		}
		
		roleLower := strings.ToLower(role)
		// owner vagy admin is elegendő jogosultság
		if roleLower == "owner" || roleLower == "admin" {
			return true, nil
		}
	}
	
	return false, rows.Err()
}
//aded ellenörzés

func (a *AdminDB) GetAddedByForUserInChannel(nick string, channel string) (string, error) {
    var addedBy string
    // Használj LOWER() függvényt a case-insensitive kereséshez
    query := `SELECT added_by FROM channel_users WHERE nick = ? AND LOWER(channel) = LOWER(?)`
    err := a.db.QueryRow(query, nick, channel).Scan(&addedBy)
    if err != nil {
        return "", err
    }
    return addedBy, nil
}
func (a *AdminDB) GetAddedInfoForUserInChannel(nick, channel string) (addedBy, addedByHost string, err error) {
    query := `SELECT added_by, added_by_host FROM channel_users WHERE LOWER(nick) = LOWER(?) AND LOWER(channel) = LOWER(?)`
    err = a.db.QueryRow(query, nick, channel).Scan(&addedBy, &addedByHost)
    return
}
func (a *AdminDB) GetModifiedInfoForUserInChannel(nick, channel string) (modifiedBy, modifiedByHost string, err error) {
    query := `SELECT COALESCE(modified_by, ''), COALESCE(modified_by_host, '') 
              FROM channel_users 
              WHERE LOWER(nick) = LOWER(?) AND LOWER(channel) = LOWER(?)`
    err = a.db.QueryRow(query, nick, channel).Scan(&modifiedBy, &modifiedByHost)
    return
}
func (a *AdminDB) UpdateUserRoleInChannelWithModifiedBy(nick, hostmask, channel, role, modifiedBy, modifiedByHost string) error {
    query := `UPDATE channel_users 
              SET role = ?, 
                  modified_by = ?, 
                  modified_by_host = ?,
                  changed_at = CURRENT_TIMESTAMP
              WHERE LOWER(nick) = LOWER(?) 
                AND LOWER(channel) = LOWER(?)
                AND hostmask = ?`
    
    _, err := a.db.Exec(query, role, modifiedBy, modifiedByHost, nick, channel, hostmask)
    return err
}
func (db *AdminDB) GetSavedTopic(channel string) (string, error) {
    channel = normalizeChannelName(channel)

    var t sql.NullString
    err := db.db.QueryRow(`
        SELECT current_topic
        FROM channels
        WHERE LOWER(name) = LOWER(?)
    `, channel).Scan(&t)

    if err == sql.ErrNoRows {
        return "", nil
    }
    if err != nil {
        return "", err
    }
    if !t.Valid {
        return "", nil
    }

    topic := strings.TrimSpace(t.String)
    if topic == "" || topic == "0" {
        return "", nil
    }
    return topic, nil
}
func (db *AdminDB) ApplySavedChannelTopicIfEmpty(bot *YnMIrC.Client, channel string) {
    channel = normalizeChannelName(channel)

    // 1) DB topic
    savedTopic, err := db.GetSavedTopic(channel)
    if err != nil {
        return
    }
    // ahol DB-ben üres/0 -> semmit
    if savedTopic == "" {
        return
    }

    // 2) Kérünk TOPIC-ot a szervertől és várjuk meg
    topicReceived := make(chan *string, 1)

    go func() {
        timeout := time.After(2 * time.Second)
        start := time.Now()

        for {
            select {
            case <-timeout:
                topicReceived <- nil
                return
            default:
                channels := bot.Channels()
                key := strings.ToLower(channel)

                // ITT A LÉNYEG:
                // ha van ilyen mező a channel state-ben:
                // - Topic string
                // - vagy HasTopic bool
                // - vagy LastTopicTime, stb.
                //


				if ch, ok := channels[key]; ok {
					t := strings.TrimSpace(ch.Topic) //
					if t == "" || t == "0" {
						empty := ""
						topicReceived <- &empty
					} else {
						topicReceived <- &t
					}
					return
				}

                if time.Since(start) > 1500*time.Millisecond {
                    topicReceived <- nil
                    return
                }
                time.Sleep(100 * time.Millisecond)
            }
        }
    }()

    // TOPIC lekérés (sok hálózaton: "TOPIC #chan" -> 331/332 válasz)
    bot.SendRaw(fmt.Sprintf("TOPIC %s", channel))

	currentTopicPtr := <-topicReceived

	// HA NEM TUDTUK LEKÉRNI (timeout / nincs adat) -> NE állítsunk semmit!
	if currentTopicPtr == nil {
		return
	}

	// 3) Ha már van topic -> nem nyúlunk hozzá
	cur := strings.TrimSpace(*currentTopicPtr)
	if cur != "" && cur != "0" {
		return
	}

	// 4) Biztosan üres topic -> beállítjuk DB-ből
	bot.SendRaw(fmt.Sprintf("TOPIC %s :%s", channel, savedTopic))
}