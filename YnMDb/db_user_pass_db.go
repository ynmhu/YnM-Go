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
	"golang.org/x/crypto/bcrypt"
//	"strings"
	"fmt"
	"time"


)
// ==================================================
// Password Management Methods (EGY jelszó mindkét célra)
// ==================================================

// SetPassword beállít egy új jelszót (mind IRC, mind web) - PUBLIKUS
func (a *AdminDB) SetPassword(nick string, passwordHash string, expiresAt *time.Time, maxUses int, passwordType string) error {
    exists, err := a.UserExists(nick)
    if err != nil {
        return fmt.Errorf("error checking user existence: %v", err)
    }
    
    if !exists {
        // Create user with default values
        _, err := a.db.Exec(`
            INSERT INTO users 
            (nick, hostmask, role, pass, password_expires, password_max_uses, password_created) 
            VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
            nick, nick+"!*@*", "user", passwordHash, expiresAt, maxUses,
        )
        if err != nil {
            return fmt.Errorf("error creating user: %v", err)
        }
    } else {
        // Update existing user's password
        _, err := a.db.Exec(`
            UPDATE users 
            SET pass = ?, 
                password_expires = ?,
                password_max_uses = ?,
                password_uses = 0,
                password_created = CURRENT_TIMESTAMP
            WHERE nick = ?`,
            passwordHash, expiresAt, maxUses, nick,
        )
        if err != nil {
            return fmt.Errorf("error updating password: %v", err)
        }
    }
    
    return nil
}
// SetUserPassword sets the password for a user
func (a *AdminDB) SetUserPassword(nick string, passwordHash string) error {
    expiresAt := time.Now().AddDate(100, 0, 0)  // ✅ 100 év múlva jár le
    
    exists, err := a.UserExists(nick)
    if err != nil {
        return fmt.Errorf("error checking user existence: %v", err)
    }
    
    if !exists {
        _, err := a.db.Exec(`
            INSERT INTO users 
            (nick, hostmask, role, pass, password_expires, password_max_uses, password_created) 
            VALUES (?, ?, ?, ?, ?, 0, CURRENT_TIMESTAMP)`,
            nick, nick+"!*@*", "user", passwordHash, expiresAt.Format("2006-01-02 15:04:05"),
        )
        if err != nil {
            return fmt.Errorf("error creating user: %v", err)
        }
    } else {
        _, err := a.db.Exec(`
            UPDATE users 
            SET pass = ?, 
                password_expires = ?,
                password_max_uses = 0,
                password_uses = 0,
                password_created = CURRENT_TIMESTAMP
            WHERE nick = ?`,
            passwordHash, expiresAt.Format("2006-01-02 15:04:05"), nick,
        )
        if err != nil {
            return fmt.Errorf("error updating password: %v", err)
        }
    }
    
    return nil
}
func (a *AdminDB) SetWebPassword(nick string, passwordHash string, expiresAt *time.Time, maxUses int, generatedBy string) error {
    // Most már hívhatjuk a publikus SetPassword-t
    return a.SetPassword(nick, passwordHash, expiresAt, maxUses, generatedBy)
}
// UserExists checks if a user exists in the database
func (a *AdminDB) UserExists(nick string) (bool, error) {
    var count int
    err := a.db.QueryRow(`SELECT COUNT(*) FROM users WHERE nick = ?`, nick).Scan(&count)
    if err != nil {
        return false, fmt.Errorf("error checking user existence: %v", err)
    }
    return count > 0, nil
}
// GetPassword lekér egy érvényes jelszót
func (a *AdminDB) GetPassword(username string) (*PasswordInfo, error) {
	var pi PasswordInfo
	err := a.db.QueryRow(`
		SELECT 
			pass,
			password_expires,
			password_uses,
			password_max_uses,
			password_last_used,
			password_created
		FROM users 
		WHERE nick = ? 
			AND pass IS NOT NULL 
			AND (password_expires IS NULL OR datetime(password_expires) > datetime('now'))
			AND (password_max_uses = 0 OR password_uses < password_max_uses)
	`, username).Scan(&pi.PasswordHash, &pi.ExpiresAt, &pi.Uses, &pi.MaxUses, &pi.LastUsed, &pi.CreatedAt)
	
	if err != nil {
		return nil, err
	}
	pi.Username = username
	return &pi, nil
}

// GetUserPassword gets the password hash for a user
func (a *AdminDB) GetUserPassword(nick string) (string, error) {
    var passwordHash sql.NullString
    err := a.db.QueryRow(`
        SELECT pass 
        FROM users 
        WHERE nick = ? 
            AND pass IS NOT NULL
            AND (password_expires IS NULL OR password_expires > datetime('now'))
            AND (password_max_uses = 0 OR password_uses < password_max_uses)
    `, nick).Scan(&passwordHash)
    
    if err != nil {
        if err == sql.ErrNoRows {
            return "", fmt.Errorf("user not found or password expired")
        }
        return "", fmt.Errorf("error getting password: %v", err)
    }
    
    if !passwordHash.Valid {
        return "", fmt.Errorf("no password set for user")
    }
    
    return passwordHash.String, nil
}

// VerifyPassword verifies a password against the stored hash
func (a *AdminDB) VerifyPassword(nick, password string) (bool, error) {
    storedHash, err := a.GetUserPassword(nick)
    if err != nil {
        return false, err
    }
    
    err = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
    if err != nil {
        if err == bcrypt.ErrMismatchedHashAndPassword {
            return false, nil
        }
        return false, fmt.Errorf("error comparing passwords: %v", err)
    }
    
    // ✅ Növeljük a használati számlálót
    _, _ = a.db.Exec(`
        UPDATE users 
        SET password_uses = password_uses + 1,
            password_last_used = datetime('now')
        WHERE nick = ?
    `, nick)
    
    return true, nil
}
func (a *AdminDB) GetHostmaskByNick(nick string) (string, error) {
    row := a.db.QueryRow("SELECT hostmask FROM users WHERE nick = ?", nick)
    var hostmask string
    err := row.Scan(&hostmask)
    if err != nil {
        return "", err
    }
    return hostmask, nil
}
// IncrementPasswordUse növeli a használati számot
func (a *AdminDB) IncrementPasswordUse(username string) error {
	_, err := a.db.Exec(`
		UPDATE users SET 
			password_uses = password_uses + 1,
			password_last_used = CURRENT_TIMESTAMP
		WHERE nick = ?
	`, username)
	return err
}

// CleanupExpiredPasswords törli a lejárt/elhasznált jelszavakat
func (a *AdminDB) CleanupExpiredPasswords() (int64, error) {
	result, err := a.db.Exec(`
		UPDATE users SET 
			pass = NULL,
			password_expires = NULL,
			password_uses = 0
		WHERE 
			(pass IS NOT NULL) AND (
				(password_expires IS NOT NULL AND datetime(password_expires) < datetime('now'))
				OR (password_max_uses > 0 AND password_uses >= password_max_uses)
			)
	`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetAllPasswords visszaadja az összes jelszót (admin célra)
func (a *AdminDB) GetAllPasswords() ([]PasswordInfo, error) {
	rows, err := a.db.Query(`
		SELECT 
			nick,
			pass,
			password_expires,
			password_uses,
			password_max_uses,
			password_last_used,
			password_created
		FROM users
		WHERE pass IS NOT NULL
		ORDER BY password_created DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var passwords []PasswordInfo
	for rows.Next() {
		var pi PasswordInfo
		if err := rows.Scan(&pi.Username, &pi.PasswordHash, &pi.ExpiresAt, &pi.Uses, &pi.MaxUses, &pi.LastUsed, &pi.CreatedAt); err != nil {
			return nil, err
		}
		passwords = append(passwords, pi)
	}
	return passwords, nil
}

// CheckPasswordExist ellenőrzi, hogy van-e jelszó a felhasználónak
func (a *AdminDB) CheckPasswordExist(username string) (bool, error) {
	var exists bool
	err := a.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM users 
			WHERE nick = ? 
				AND pass IS NOT NULL 
				AND (password_expires IS NULL OR datetime(password_expires) > datetime('now'))
				AND (password_max_uses = 0 OR password_uses < password_max_uses)
		)
	`, username).Scan(&exists)
	return exists, err
}

