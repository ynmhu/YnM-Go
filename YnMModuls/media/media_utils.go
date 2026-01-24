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
package media

import "fmt"

// TicksToTime converts Jellyfin ticks to HH:MM:SS format
func TicksToTime(ticks int64) string {
	if ticks == 0 {
		return "Ismeretlen"
	}
	
	seconds := ticks / 10_000_000
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	sec := seconds % 60
	
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, sec)
}