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
package YnMModule

import (
	"crypto/rand"
	"math/big"
)

const passwordChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GenerateRandomPassword létrehoz egy véletlenszerű jelszót adott hosszban
func GenerateRandomPassword(length int) string {
	password := make([]byte, length)
	charLen := big.NewInt(int64(len(passwordChars)))

	for i := 0; i < length; i++ {
		num, _ := rand.Int(rand.Reader, charLen)
		password[i] = passwordChars[num.Int64()]
	}

	return string(password)
}
