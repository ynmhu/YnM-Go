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
    "crypto/rand"
    "encoding/hex"
	"golang.org/x/crypto/bcrypt"
	"fmt"
    _ "github.com/mattn/go-sqlite3"
)

// generateSecretKey - biztonságos random token generálás (session token-hez)
func generateSecretKey() string {
    key := make([]byte, 32) // 256 bit
    rand.Read(key) // crypto/rand - kriptográfiailag biztonságos
    return hex.EncodeToString(key)
}

// hashPassword - jelszó hash-elése SHA256-al
func (p *YnMApiPlugin) checkPasswordHash(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    if err == nil {
        return true
    }
    
    // Debug log sikertelen ellenőrzésnél
    if err != bcrypt.ErrMismatchedHashAndPassword {
        fmt.Printf("[YnMApI] bcrypt error: %v\n", err)
    }
    
    return false
}
func (p *YnMApiPlugin) hashPassword(password string) string {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        fmt.Printf("[YnMApI] Failed to hash password: %v\n", err)
        return ""
    }
    return string(hash)
}