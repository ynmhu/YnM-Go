// ============================================================================
//  YnM‑Go – több­fájlos admin tároló (Owner / Admin / VIP)
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

// ────────────────────── Típusok ──────────────────────────────

type AdminInfo struct {
	Nick     string    `json:"nick"`
	Hostmask string    `json:"hostmask"`
	Level    int       `json:"level"`
	AddedBy  string    `json:"added_by"`
	AddedAt  time.Time `json:"added_at"`
}

// egyszerű fájl‑alapú store (egy szint / fájl)
type fileStore struct {
	mu       sync.RWMutex
	filePath string
	Admins   map[string]AdminInfo // key = nick
}

func newFileStore(path string) *fileStore {
	return &fileStore{
		filePath: path,
		Admins:   make(map[string]AdminInfo),
	}
}

func (f *fileStore) load() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(f.filePath), 0755); err != nil {
		return err
	}
	data, err := os.ReadFile(f.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // első futás
		}
		return err
	}
	return json.Unmarshal(data, &f.Admins)
}

func (f *fileStore) save() error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	data, err := json.MarshalIndent(f.Admins, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(f.filePath, data, 0600)
}

// ───────────────── Multi‑store (Owner / Admin / VIP) ─────────

type MultiAdminStore struct {
	owners *fileStore
	admins *fileStore
	vips   *fileStore
}

func NewMultiAdminStore() *MultiAdminStore {
	return &MultiAdminStore{
		owners: newFileStore("data/owners.json"),
		admins: newFileStore("data/admins.json"),
		vips:   newFileStore("data/vips.json"),
	}
}

// betölt minden fájlt
func (m *MultiAdminStore) Load() error {
	if err := m.owners.load(); err != nil {
		return err
	}
	if err := m.admins.load(); err != nil {
		return err
	}
	return m.vips.load()
}

// ment minden fájlt
func (m *MultiAdminStore) Save() error {
	if err := m.owners.save(); err != nil {
		return err
	}
	if err := m.admins.save(); err != nil {
		return err
	}
	return m.vips.save()
}

// van‑e már owner?
func (m *MultiAdminStore) HasOwner() bool {
	m.owners.mu.RLock()
	defer m.owners.mu.RUnlock()
	return len(m.owners.Admins) > 0
}


// hozzáadás szint alapján
func (m *MultiAdminStore) AddAdmin(info AdminInfo) error {
	switch info.Level {
	case AdminLevelOwner:
		if m.HasOwner() {
			return fmt.Errorf("owner already exists")
		}
		m.owners.mu.Lock()
		m.owners.Admins[info.Nick] = info
		m.owners.mu.Unlock()
	case AdminLevelAdmin:
		m.admins.mu.Lock()
		m.admins.Admins[info.Nick] = info
		m.admins.mu.Unlock()
	case AdminLevelVIP:
		m.vips.mu.Lock()
		m.vips.Admins[info.Nick] = info
		m.vips.mu.Unlock()
	default:
		return fmt.Errorf("invalid level")
	}
	return m.Save()
}

// törlés (nick alapján, bármely szint)
func (m *MultiAdminStore) RemoveAdmin(nick string) bool {
	removed := false
	for _, fs := range []*fileStore{m.owners, m.admins, m.vips} {
		fs.mu.Lock()
		if _, ok := fs.Admins[nick]; ok {
			delete(fs.Admins, nick)
			removed = true
		}
		fs.mu.Unlock()
	}
	if removed {
		_ = m.Save()
	}
	return removed
}

// lekér egy admin info‑t
func (m *MultiAdminStore) GetAdmin(nick string) (AdminInfo, bool) {
	for _, fs := range []*fileStore{m.owners, m.admins, m.vips} {
		fs.mu.RLock()
		if info, ok := fs.Admins[nick]; ok {
			fs.mu.RUnlock()
			return info, true
		}
		fs.mu.RUnlock()
	}
	return AdminInfo{}, false
}

// szint meghatározása hostmask alapján
func (m *MultiAdminStore) GetAdminLevel(nick, hostmask string) int {
	if info, ok := m.GetAdmin(nick); ok {
		// egyszerű hostmask‑match: csak host részt nézzük
		host := strings.Split(hostmask, "@")
		if len(host) == 2 && strings.HasSuffix(info.Hostmask, host[1]) {
			return info.Level
		}
	}
	return AdminLevelNone
}

// listázás szintenként
func (m *MultiAdminStore) ListAll() []AdminInfo {
	var res []AdminInfo
	for _, fs := range []*fileStore{m.owners, m.admins, m.vips} {
		fs.mu.RLock()
		for _, a := range fs.Admins {
			res = append(res, a)
		}
		fs.mu.RUnlock()
	}
	return res
}
