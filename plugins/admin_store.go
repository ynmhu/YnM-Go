// ============================================================================
//  Szerzői jog © 2024 Markus (markus@ynm.hu)
//  https://ynm.hu   – főoldal
//  https://forum.ynm.hu   – hivatalos fórum
//  https://bot.ynm.hu     – bot oldala és dokumentáció
//
//  Minden jog fenntartva. A kód Markus tulajdona, tilos terjeszteni vagy
//  módosítani a szerző írásos engedélye nélkül.
//
//  Ez a fájl a YnM-Go IRC-bot rendszerének része.
// ============================================================================

package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)


// AdminInfo stores admin information with privilege levels
type AdminInfo struct {
	Nick     string    `json:"nick"`
	Hostmask string    `json:"hostmask"`
	Level    int       `json:"level"`
	AddedBy  string    `json:"added_by"`
	AddedAt  time.Time `json:"added_at"`
}

// AdminStore manages admin data persistence
type AdminStore struct {
	mu     sync.RWMutex
	path   string
	Admins map[string]AdminInfo `json:"admins"` // Key format: "nick:host"
}


func NewAdminStore(path string) *AdminStore {
	return &AdminStore{
		path:   path,
		Admins: make(map[string]AdminInfo),
	}
}

func (s *AdminStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Create directory if it doesn't exist
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}
	
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet (first run)
		}
		return err
	}
	
	// Try to load new format first
	var store struct {
		Admins map[string]AdminInfo `json:"admins"`
	}
	
	if err := json.Unmarshal(data, &store); err == nil && store.Admins != nil {
		s.Admins = store.Admins
		return nil
	}
	
	// Fallback to old format for backward compatibility
	var oldAdmins map[string]AdminInfo
	if err := json.Unmarshal(data, &oldAdmins); err == nil {
		s.Admins = make(map[string]AdminInfo)
		// Migrate old format - set default levels for existing admins
		for _, info := range oldAdmins {
			// Convert old AdminLevel enum to numeric
			numericLevel := AdminLevelAdmin // Default to admin (2) for existing users
			if info.Level == 0 {
				numericLevel = AdminLevelVIP // If it was 0, make it VIP (1)
			} else if info.Level == 3 {
				numericLevel = AdminLevelOwner // If it was 3, make it Owner (3)
			}
			
			// Extract host from hostmask for new key format
			host := s.extractHost(info.Hostmask)
			newKey := fmt.Sprintf("%s:%s", info.Nick, host)
			
			info.Level = numericLevel
			if info.AddedBy == "" {
				info.AddedBy = "migration"
			}
			if info.AddedAt.IsZero() {
				info.AddedAt = time.Now()
			}
			
			s.Admins[newKey] = info
		}
		// Save in new format
		return s.Save()
	}
	
	return fmt.Errorf("failed to parse admin data")
}

func (s *AdminStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	store := struct {
		Admins map[string]AdminInfo `json:"admins"`
	}{
		Admins: s.Admins,
	}
	
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(s.path, data, 0600)
}

// extractHost extracts the host part from a full hostmask
func (s *AdminStore) extractHost(hostmask string) string {
    // Handle different hostmask formats:
    // 1. Full format: nick!user@host
    // 2. User@host format: user@host
    // 3. Just host: host
    
    // Find the last @ symbol
    atPos := strings.LastIndex(hostmask, "@")
    if atPos == -1 {
        // No @ found, could be just a hostname or malformed
        // If it contains ! it's malformed, return *
        if strings.Contains(hostmask, "!") {
            return "*"
        }
        // Otherwise assume it's just a hostname
        return hostmask
    }
    
    // Extract everything after the last @
    host := hostmask[atPos+1:]
    
    // Validate that we got a meaningful host
    if host == "" {
        return "*"
    }
    
    return host
}

// createHostmask creates a hostmask in format *!*@host
func (s *AdminStore) createHostmask(host string) string {
	return fmt.Sprintf("*!*@%s", host)
}

// createKey creates a storage key in format nick:host
func (s *AdminStore) createKey(nick, hostmask string) string {
	host := s.extractHost(hostmask)
	return fmt.Sprintf("%s:%s", nick, host)
}

// AddAdminWithCurrentHostmask adds an admin using their current full hostmask
func (s *AdminStore) AddAdminWithCurrentHostmask(nick, currentHostmask string, level int, addedBy string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Extract the actual hostname from the current connection
	host := s.extractHost(currentHostmask)
	
	// If we couldn't extract a valid host, fall back to wildcard
	if host == "*" || host == "" {
		// Create wildcard hostmask
		finalHostmask := fmt.Sprintf("%s!*@*", nick)
		key := fmt.Sprintf("%s:*", nick)
		
		s.Admins[key] = AdminInfo{
			Nick:     nick,
			Hostmask: finalHostmask,
			Level:    level,
			AddedBy:  addedBy,
			AddedAt:  time.Now(),
		}
		return
	}
	
	// Create proper hostmask format: nick!*@actualhost
	finalHostmask := fmt.Sprintf("%s!*@%s", nick, host)
	
	// Create storage key
	key := fmt.Sprintf("%s:%s", nick, host)
	
	s.Admins[key] = AdminInfo{
		Nick:     nick,
		Hostmask: finalHostmask,
		Level:    level,
		AddedBy:  addedBy,
		AddedAt:  time.Now(),
	}
}

func (s *AdminStore) RemoveAdmin(nick string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Find admin by nick (search through all keys)
	var keyToDelete string
	for key, info := range s.Admins {
		if info.Nick == nick {
			keyToDelete = key
			break
		}
	}
	
	if keyToDelete != "" {
		delete(s.Admins, keyToDelete)
		return true
	}
	return false
}

func (s *AdminStore) GetAdmin(nick string) (AdminInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Find admin by nick (search through all keys)
	for _, info := range s.Admins {
		if info.Nick == nick {
			return info, true
		}
	}
	return AdminInfo{}, false
}

func (s *AdminStore) IsAdmin(nick, hostmask string) bool {
	return s.GetAdminLevel(nick, hostmask) > AdminLevelNone
}

func (s *AdminStore) GetAdminLevel(nick, hostmask string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Extract host from the provided hostmask
	host := s.extractHost(hostmask)
	
	// Try exact match first: nick:host
	key := fmt.Sprintf("%s:%s", nick, host)
	if info, exists := s.Admins[key]; exists {
		return info.Level
	}
	
	// Try to find by nick and check if hostmask matches
	for _, info := range s.Admins {
		if info.Nick == nick && s.hostmaskMatches(info.Hostmask, hostmask) {
			return info.Level
		}
	}
	
	return AdminLevelNone
}

func (s *AdminStore) hostmaskMatches(pattern, hostmask string) bool {
	// pattern is in format nick!*@host or *!*@host
	// hostmask is in format nick!user@host
	
	if pattern == "*!*@*" {
		return true
	}
	
	if pattern == hostmask {
		return true
	}
	
	// Extract components
	patternNick := ""
	patternHost := s.extractHost(pattern)
	hostmaskNick := ""
	hostmaskHost := s.extractHost(hostmask)
	
	// Extract nick from pattern
	if exclamPos := strings.Index(pattern, "!"); exclamPos != -1 {
		patternNick = pattern[:exclamPos]
	}
	
	// Extract nick from hostmask
	if exclamPos := strings.Index(hostmask, "!"); exclamPos != -1 {
		hostmaskNick = hostmask[:exclamPos]
	}
	
	// Check nick match (if pattern specifies a nick)
	if patternNick != "*" && patternNick != "" && patternNick != hostmaskNick {
		return false
	}
	
	// Check host match
	if patternHost == "*" {
		return true
	}
	
	return patternHost == hostmaskHost
}

func (s *AdminStore) ListAdmins() []AdminInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var admins []AdminInfo
	for _, info := range s.Admins {
		admins = append(admins, info)
	}
	return admins
}

// GetAdminByNickAndHost gets admin info by nick and host
func (s *AdminStore) GetAdminByNickAndHost(nick, host string) (AdminInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	key := fmt.Sprintf("%s:%s", nick, host)
	info, exists := s.Admins[key]
	return info, exists
}

// UpdateAdminLevel updates an admin's level
func (s *AdminStore) UpdateAdminLevel(nick string, newLevel int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Find admin by nick
	for key, info := range s.Admins {
		if info.Nick == nick {
			info.Level = newLevel
			s.Admins[key] = info
			return true
		}
	}
	return false
}

// GetAdminsByLevel returns all admins with a specific level
func (s *AdminStore) GetAdminsByLevel(level int) []AdminInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var admins []AdminInfo
	for _, info := range s.Admins {
		if info.Level == level {
			admins = append(admins, info)
		}
	}
	return admins
}
func (s *AdminStore) AddAdmin(nick, hostmask string, level int, addedBy string) {
    s.mu.Lock()
    defer s.mu.Unlock()

    host := s.extractHost(hostmask)
    key  := fmt.Sprintf("%s:%s", nick, host)

    // Ha a hívó már teljes hostmaskot ad (pl. *!*@host), tartsuk meg.
    finalHostmask := hostmask
    if !strings.Contains(hostmask, "!") {                 // csak "host" érkezett
        finalHostmask = fmt.Sprintf("%s!*@%s", nick, host) // régi viselkedés
    }

    s.Admins[key] = AdminInfo{
        Nick:     nick,
        Hostmask: finalHostmask,
        Level:    level,
        AddedBy:  addedBy,
        AddedAt:  time.Now(),
    }
}

