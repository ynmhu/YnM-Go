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
package ynmapi

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
    
    "golang.org/x/crypto/bcrypt"
)

// UserPasswordChangeRequest - felhasználói jelszóváltás kérés
type UserPasswordChangeRequest struct {
    Username    string `json:"username"`
    OldPassword string `json:"old_password"`
    NewPassword string `json:"new_password"`
}

// AdminPasswordChangeRequest - admin jelszóváltás kérés (nincs régi jelszó)
type AdminPasswordChangeRequest struct {
    Username    string `json:"username"`
    NewPassword string `json:"new_password"`
}

// hashPasswordBcrypt - bcrypt hash generálása
func hashPasswordBcrypt(password string) (string, error) {
    // bcrypt.DefaultCost = 10, ami biztonságos és gyors
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return "", err
    }
    return string(bytes), nil
}

// verifyPasswordBcrypt - jelszó ellenőrzése bcrypt hash-sel
func verifyPasswordBcrypt(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}

// handleUserPasswordChange - HTTP endpoint: felhasználó saját jelszavát változtatja
// POST /api/user/password/change
func (p *YnMApiPlugin) handleUserPasswordChange(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost && r.Method != http.MethodPut {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Parse request
    var req UserPasswordChangeRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // Validáció
    if req.Username == "" || req.OldPassword == "" || req.NewPassword == "" {
        http.Error(w, "All fields are required", http.StatusBadRequest)
        return
    }

    // Új jelszó hossz ellenőrzése
    if len(req.NewPassword) < 8 {
        http.Error(w, "New password must be at least 8 characters", http.StatusBadRequest)
        return
    }

    // Context-ből a bejelentkezett felhasználó
    requestedBy := "web"
    if ctx := r.Context(); ctx != nil {
        if val := ctx.Value("username"); val != nil {
            if username, ok := val.(string); ok && username != "" {
                requestedBy = username
            }
        }
    }

    // Ellenőrizzük, hogy csak a saját jelszavát változtathatja (vagy owner mindenkiét)
	db, err := p.getDB()
	if err != nil || db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	var currentPasswordHash string
	err = db.QueryRow(`
		SELECT password FROM users WHERE nick = ?
	`, req.Username).Scan(&currentPasswordHash)

	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

    // Ellenőrizzük a régi jelszót
    if !verifyPasswordBcrypt(req.OldPassword, currentPasswordHash) {
        p.logAudit(requestedBy, "❌", "YnM-Go", fmt.Sprintf("Failed password change for %s - wrong old password", req.Username))
        http.Error(w, "Old password is incorrect", http.StatusUnauthorized)
        return
    }

    // Hash-eljük az új jelszót
    newPasswordHash, err := hashPasswordBcrypt(req.NewPassword)
    if err != nil {
        http.Error(w, "Failed to hash password", http.StatusInternalServerError)
        return
    }

    // Frissítjük az adatbázisban
    _, err = db.Exec(`
        UPDATE users 
        SET password = ?, updated_at = ?
        WHERE nick = ?
    `, newPasswordHash, time.Now().Format(time.RFC3339), req.Username)

    if err != nil {
        http.Error(w, "Failed to update password", http.StatusInternalServerError)
        return
    }

    p.logAudit(requestedBy, "🔐", "web", fmt.Sprintf("Password changed for %s", req.Username))

    // Válasz
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "message": "Password changed successfully",
    })
}


func (p *YnMApiPlugin) handleAdminPasswordChange(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost && r.Method != http.MethodPut {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Parse request
    var req AdminPasswordChangeRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // Validáció
    if req.Username == "" || req.NewPassword == "" {
        http.Error(w, "Username and new_password are required", http.StatusBadRequest)
        return
    }

    // Új jelszó hossz ellenőrzése
    if len(req.NewPassword) < 8 {
        http.Error(w, "New password must be at least 8 characters", http.StatusBadRequest)
        return
    }

    // Context-ből az admin user
    requestedBy := "admin"
    if ctx := r.Context(); ctx != nil {
        if val := ctx.Value("username"); val != nil {
            if username, ok := val.(string); ok && username != "" {
                requestedBy = username
            }
        }
    }

	db, err := p.getDB()
	if err != nil || db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

    // Ellenőrizzük, hogy létezik-e a felhasználó
	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM users WHERE nick = ?)
	`, req.Username).Scan(&exists)

    if err != nil || !exists {
        http.Error(w, "User not found", http.StatusNotFound)
        return
    }

    // Hash-eljük az új jelszót
    newPasswordHash, err := hashPasswordBcrypt(req.NewPassword)
    if err != nil {
        http.Error(w, "Failed to hash password", http.StatusInternalServerError)
        return
    }

    // Frissítjük az adatbázisban
    _, err = db.Exec(`
        UPDATE users 
        SET password = ?, updated_at = ?
        WHERE nick = ?
    `, newPasswordHash, time.Now().Format(time.RFC3339), req.Username)

    if err != nil {
        http.Error(w, "Failed to update password", http.StatusInternalServerError)
        return
    }

    p.logAudit(requestedBy, "🔐", "web", fmt.Sprintf("Admin changed password for %s", req.Username))

    // Válasz
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success":  true,
        "message":  "Password changed successfully by admin",
        "username": req.Username,
    })
}

