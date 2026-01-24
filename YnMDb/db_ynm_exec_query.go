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
)


func (db *AdminDB) Exec(query string, args ...interface{}) (sql.Result, error) {
    if db.SQL == nil {
        return nil, fmt.Errorf("database not initialized")
    }
    return db.SQL.Exec(query, args...)
}
func (db *AdminDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
    if db.SQL == nil {
        return nil, fmt.Errorf("database not initialized")
    }
    return db.SQL.Query(query, args...)
}
func (db *AdminDB) QueryRow(query string, args ...interface{}) *sql.Row {
    if db.SQL == nil {
        return nil
    }
    return db.SQL.QueryRow(query, args...)
}