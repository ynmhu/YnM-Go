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
	"time"
	_ "github.com/mattn/go-sqlite3"
)

func (a *AdminDB) SetUserMail(nick, email string) error {
	_, err := a.db.Exec(`UPDATE users SET email = ? WHERE nick = ?`, email, nick)
	return err
}

// GetUserMail lekéri a felhasználó e-mail címét
func (a *AdminDB) GetUserMail(nick string) (string, error) {
	var email sql.NullString
	err := a.db.QueryRow(`SELECT email FROM users WHERE nick = ?`, nick).Scan(&email)
	if err != nil {
		return "", err
	}
	if email.Valid {
		return email.String, nil
	}
	return "", nil
}
func (a *AdminDB) UserHasPassword(username string) (bool, error) {
    var pass sql.NullString
    var passwordExpires sql.NullString
    

    err := a.db.QueryRow(`
        SELECT pass, password_expires
        FROM users 
        WHERE nick = ?
    `, username).Scan(&pass, &passwordExpires)
    

    if err == sql.ErrNoRows {
        return false, nil
    }
    
    if err != nil {
        return false, err
    }
    if !pass.Valid || pass.String == "" {
        return false, nil
    }
    
    if !passwordExpires.Valid {
        return true, nil
    }
    expiryTime, err := time.Parse("2006-01-02 15:04:05", passwordExpires.String)
    if err != nil {
        return true, nil
    }
    
    yearsUntilExpiry := time.Until(expiryTime).Hours() / 24 / 365
    hasIRCPassword := yearsUntilExpiry > 50 
    return hasIRCPassword, nil
}
func (a *AdminDB) UserHasMail(username string) (bool, error) {
    var email sql.NullString

    err := a.db.QueryRow(`
        SELECT email
        FROM users 
        WHERE nick = ?
    `, username).Scan(&email)

    if err == sql.ErrNoRows {
        return false, nil
    }
    
    if err != nil {
        return false, err
    }
    
    // Ha az email NULL vagy üres string, akkor nincs email
    if !email.Valid || email.String == "" {
        return false, nil
    }
    
    // Van email
    return true, nil
}
func (a *AdminDB) LogForgetPassAttempt(nick string) error {
	_, err := a.db.Exec(`INSERT INTO forget_pass_logs(nick, created_at) VALUES(?, ?)`, nick, time.Now())
	return err
}

func (a *AdminDB) CountForgetPassInLast24Hours(nick string) (int, error) {
	var count int
	err := a.db.QueryRow(`SELECT COUNT(*) FROM forget_pass_logs WHERE nick = ? AND created_at >= ?`,
		nick, time.Now().Add(-24*time.Hour)).Scan(&count)
	return count, err
}