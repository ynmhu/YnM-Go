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
	
	_ "github.com/mattn/go-sqlite3"
)

func (a *AdminDB) EnsurePlugin(name, description string) error {
	if description == "" {
		description = fmt.Sprintf("%s plugin", name)
	}
	
	_, err := a.db.Exec(`INSERT OR IGNORE INTO plugins (name, description) VALUES (?, ?)`, 
		name, description)
	return err
}
// Új metódusok a plugin állapotkezeléshez
func (a *AdminDB) SetPluginState(pluginName, channel string, isActive bool) error {
	_, err := a.db.Exec(`
		INSERT OR REPLACE INTO plugin_states (plugin_name, channel, is_active, updated_at) 
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
		pluginName, channel, isActive)
	return err
}

// GetPluginState gets the state of a plugin for a specific channel
func (a *AdminDB) GetPluginState(pluginName, channel string) (bool, error) {
	var isActive bool
	err := a.db.QueryRow(`
		SELECT is_active FROM plugin_states 
		WHERE plugin_name = ? AND channel = ?`,
		pluginName, channel).Scan(&isActive)
	
	if err == sql.ErrNoRows {
		return false, nil // Default to false if no state found
	}
	return isActive, err
}
func (a *AdminDB) GetPluginStatesForChannel(channel string) ([]PluginState, error) {
	rows, err := a.db.Query(`
		SELECT id, plugin_name, channel, is_active, updated_at 
		FROM plugin_states 
		WHERE channel = ? 
		ORDER BY plugin_name`, channel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []PluginState
	for rows.Next() {
		var s PluginState
		err := rows.Scan(&s.ID, &s.PluginName, &s.Channel, &s.IsActive, &s.UpdatedAt)
		if err != nil {
			return nil, err
		}
		states = append(states, s)
	}
	return states, nil
}

// GetAllPlugins gets all plugins from the database
func (a *AdminDB) GetAllPlugins() ([]PluginInfo, error) {
	rows, err := a.db.Query(`SELECT id, name, description, created_at FROM plugins ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plugins []PluginInfo
	for rows.Next() {
		var p PluginInfo
		err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt)
		if err != nil {
			return nil, err
		}
		plugins = append(plugins, p)
	}
	return plugins, nil
}
func (a *AdminDB) GetActivePluginsForChannel(channel string) ([]string, error) {
	rows, err := a.db.Query(`SELECT plugin_name FROM plugin_states 
		WHERE channel = ? AND is_active = 1`, channel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plugins []string
	for rows.Next() {
		var plugin string
		if err := rows.Scan(&plugin); err != nil {
			return nil, err
		}
		plugins = append(plugins, plugin)
	}
	return plugins, nil
}

func (a *AdminDB) GetActiveChannelsForPlugin(pluginName string) ([]string, error) {
	rows, err := a.db.Query(`SELECT channel FROM plugin_states 
		WHERE plugin_name = ? AND is_active = 1`, pluginName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []string
	for rows.Next() {
		var channel string
		if err := rows.Scan(&channel); err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	return channels, nil
}

func (a *AdminDB) GetUserChannelRole(hostmask, channel string) (*UserChannelRole, error) {
	var ucr UserChannelRole
	err := a.db.QueryRow(`
		SELECT cu.id, cu.nick, cu.hostmask, cu.channel, cu.channel as channel_id, 
		       cu.role, cu.auto_op, cu.auto_voice, cu.auto_halfop, 
		       CASE WHEN cu.auto_op THEN 'op' 
		            WHEN cu.auto_voice THEN 'voice' 
		            WHEN cu.auto_halfop THEN 'halfop' 
		            ELSE cu.role END as automode,
		       c.owner, c.owner_hostmask, cu.created_at
		FROM channel_users cu
		LEFT JOIN channels c ON cu.channel = c.name
		WHERE cu.hostmask = ? AND cu.channel = ?`, hostmask, channel).Scan(
		&ucr.UserID, &ucr.Nick, &ucr.Hostmask, &ucr.Channel, &ucr.ChannelID,
		&ucr.Role, &ucr.AutoOp, &ucr.AutoVoice, &ucr.AutoHalfOp,
		&ucr.Automode, &ucr.owner, &ucr.ownerHostmask, &ucr.CreatedAt)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ucr, nil
}

