package plugins

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
	"github.com/ynmhu/YnM-Go/config"
	"github.com/ynmhu/YnM-Go/irc"
)

// TamagotchiAllapot a kisállat aktuális életállapotát reprezentálja
type TamagotchiAllapot int

const (
	AllapotTojas TamagotchiAllapot = iota
	AllapotBaba
	AllapotGyermek
	AllapotFelnott
	AllapotIdos
	AllapotHalott
)

func (a TamagotchiAllapot) String() string {
	switch a {
	case AllapotTojas:
		return "🥚 Tojás"
	case AllapotBaba:
		return "🐣 Baba"
	case AllapotGyermek:
		return "🐤 Gyermek"
	case AllapotFelnott:
		return "🐔 Felnőtt"
	case AllapotIdos:
		return "🦅 Idős"
	case AllapotHalott:
		return "💀 Halott"
	default:
		return "❓ Ismeretlen"
	}
}

// Tamagotchi egy virtuális kisállatot reprezentál
type Tamagotchi struct {
	Nev         string            `json:"nev"`
	Allapot     TamagotchiAllapot `json:"allapot"`
	Kor         int               `json:"kor"`         // órákban
	Ehseg       int               `json:"ehseg"`       // 0-100 (0 = éhező, 100 = jóllakott)
	Boldogsag   int               `json:"boldogsag"`   // 0-100
	Egeszseg    int               `json:"egeszseg"`    // 0-100
	Tisztasag   int               `json:"tisztasag"`   // 0-100
	UtoljaraEtrek    time.Time     `json:"utoljara_etrek"`
	UtoljaraJatszott time.Time     `json:"utoljara_jatszott"`
	UtoljaraTisztitva time.Time    `json:"utoljara_tisztitva"`
	SzuletesiIdo time.Time         `json:"szuletesi_ido"`
	HalalIdeje   *time.Time        `json:"halal_ideje,omitempty"`
	Tulajdonos  string            `json:"tulajdonos"`
}

// UjTamagotchi létrehoz egy új Tamagotchi kisállatot
func UjTamagotchi(nev, tulajdonos string) *Tamagotchi {
	most := time.Now()
	return &Tamagotchi{
		Nev:         nev,
		Allapot:     AllapotTojas,
		Kor:         0,
		Ehseg:       70,
		Boldogsag:   80,
		Egeszseg:    100,
		Tisztasag:   100,
		UtoljaraEtrek:    most,
		UtoljaraJatszott: most,
		UtoljaraTisztitva: most,
		SzuletesiIdo: most,
		Tulajdonos:  tulajdonos,
	}
}

// Emoji visszaadja a kisállat aktuális állapotához tartozó emojit
func (t *Tamagotchi) Emoji() string {
	if t.Allapot == AllapotHalott {
		return "💀"
	}
	
	if t.Egeszseg < 20 {
		return "🤒" // beteg
	}
	
	if t.Ehseg < 20 {
		return "😫" // éhes
	}
	
	if t.Boldogsag < 20 {
		return "😢" // szomorú
	}
	
	if t.Tisztasag < 20 {
		return "🤢" // piszkos
	}
	
	switch t.Allapot {
	case AllapotTojas:
		return "🥚"
	case AllapotBaba:
		return "🐣"
	case AllapotGyermek:
		return "🐤"
	case AllapotFelnott:
		return "🐔"
	case AllapotIdos:
		return "🦅"
	default:
		return "❓"
	}
}

// Frissit frissíti a kisállat állapotát az eltelt idő alapján
func (t *Tamagotchi) Frissit() {
	if t.Allapot == AllapotHalott {
		return
	}
	
	most := time.Now()
	
	// Kor növelése
	t.Kor = int(most.Sub(t.SzuletesiIdo).Hours())
	
	// Állapot frissítése kor alapján
	switch {
	case t.Kor < 1:
		t.Allapot = AllapotTojas
	case t.Kor < 24:
		t.Allapot = AllapotBaba
	case t.Kor < 72:
		t.Allapot = AllapotGyermek
	case t.Kor < 168:
		t.Allapot = AllapotFelnott
	default:
		t.Allapot = AllapotIdos
	}
	
	// Statisztikák romlása idővel
	oraEtelNelkul := most.Sub(t.UtoljaraEtrek).Hours()
	oraJatekNelkul := most.Sub(t.UtoljaraJatszott).Hours()
	oraTisztitasNelkul := most.Sub(t.UtoljaraTisztitva).Hours()
	
	// Éhség növekszik idővel
	t.Ehseg -= int(oraEtelNelkul * 5)
	if t.Ehseg < 0 {
		t.Ehseg = 0
	}
	
	// Boldogság csökken idővel
	t.Boldogsag -= int(oraJatekNelkul * 3)
	if t.Boldogsag < 0 {
		t.Boldogsag = 0
	}
	
	// Tisztaság csökken idővel
	t.Tisztasag -= int(oraTisztitasNelkul * 2)
	if t.Tisztasag < 0 {
		t.Tisztasag = 0
	}
	
	// Egészséget befolyásolják a többi stat
	if t.Ehseg < 20 || t.Boldogsag < 20 || t.Tisztasag < 20 {
		t.Egeszseg -= 2
	} else if t.Ehseg > 80 && t.Boldogsag > 80 && t.Tisztasag > 80 {
		t.Egeszseg += 1
	}
	
	// Egészség korlátozása
	if t.Egeszseg < 0 {
		t.Egeszseg = 0
	}
	if t.Egeszseg > 100 {
		t.Egeszseg = 100
	}
	
	// Halál ellenőrzése
	if t.Egeszseg <= 0 {
		t.Allapot = AllapotHalott
		most := time.Now()
		t.HalalIdeje = &most
	}
}

// Etet megéteti a kisállatot
func (t *Tamagotchi) Etet() string {
	if t.Allapot == AllapotHalott {
		return "💀 Nem etethetsz egy halott kisállatot!"
	}
	
	if t.Ehseg >= 90 {
		return fmt.Sprintf("%s %s már jól lakott!", t.Emoji(), t.Nev)
	}
	
	t.Ehseg += 30
	if t.Ehseg > 100 {
		t.Ehseg = 100
	}
	t.UtoljaraEtrek = time.Now()
	
	return fmt.Sprintf("🍎 Megetetted %s-t! Éhség: %d/100", t.Nev, t.Ehseg)
}

// Jatszik játszik a kisállattal
func (t *Tamagotchi) Jatszik() string {
	if t.Allapot == AllapotHalott {
		return "💀 Nem játszhatsz egy halott kisállattal!"
	}
	
	if t.Boldogsag >= 90 {
		return fmt.Sprintf("%s %s már nagyon boldog!", t.Emoji(), t.Nev)
	}
	
	t.Boldogsag += 25
	if t.Boldogsag > 100 {
		t.Boldogsag = 100
	}
	t.UtoljaraJatszott = time.Now()
	
	jatekok := []string{"labdázás", "bújócska", "frizbi", "futócska", "kutyusozás"}
	jatek := jatekok[rand.Intn(len(jatekok))]
	
	return fmt.Sprintf("🎮 Játszottál %s-t %s-val! Boldogság: %d/100", jatek, t.Nev, t.Boldogsag)
}

// Tisztit megtisztítja a kisállatot
func (t *Tamagotchi) Tisztit() string {
	if t.Allapot == AllapotHalott {
		return "💀 Nem tisztíthatsz egy halott kisállatot!"
	}
	
	if t.Tisztasag >= 90 {
		return fmt.Sprintf("%s %s már tiszta!", t.Emoji(), t.Nev)
	}
	
	t.Tisztasag += 40
	if t.Tisztasag > 100 {
		t.Tisztasag = 100
	}
	t.UtoljaraTisztitva = time.Now()
	
	return fmt.Sprintf("🧼 Megtisztítottad %s-t! Tisztaság: %d/100", t.Nev, t.Tisztasag)
}

// AllapotJelentes visszaadja a kisállat aktuális állapotjelentését
func (t *Tamagotchi) AllapotJelentes() string {
	if t.Allapot == AllapotHalott {
		return fmt.Sprintf("💀 %s meghalt %d órás korában. Nyugodjék békében 😢", t.Nev, t.Kor)
	}
	
	jelentes := fmt.Sprintf("%s **%s** (%s) - Kor: %d óra\n", t.Emoji(), t.Nev, t.Allapot, t.Kor)
	jelentes += fmt.Sprintf("❤️ Egészség: %d/100 | � Éhség: %d/100 | 😊 Boldogság: %d/100 | 🧼 Tisztaság: %d/100\n", 
		t.Egeszseg, t.Ehseg, t.Boldogsag, t.Tisztasag)
	
	// Állapotüzenetek hozzáadása
	var uzenetek []string
	if t.Egeszseg < 30 {
		uzenetek = append(uzenetek, "🤒 Betegnek érzi magát")
	}
	if t.Ehseg < 30 {
		uzenetek = append(uzenetek, "😫 Nagyon éhes")
	}
	if t.Boldogsag < 30 {
		uzenetek = append(uzenetek, "😢 Szomorú")
	}
	if t.Tisztasag < 30 {
		uzenetek = append(uzenetek, "🤢 Nagyon piszkos")
	}
	
	if len(uzenetek) > 0 {
		jelentes += "Állapot: " + strings.Join(uzenetek, ", ")
	} else {
		jelentes += "Állapot: 😊 Nagyon jól érzi magát!"
	}
	
	return jelentes
}

// TamagotchiPlugin kezeli a virtuális kisállatokat IRC felhasználók számára
type TamagotchiPlugin struct {
	aktiv       bool
	kisallatok  map[string]*Tamagotchi // csatorna -> kisállat leképezés
	adatKonyvtar string
	utolsoFrissites time.Time
	bot         *irc.Client
}

func NewTamagotchiPlugin(adatKonyvtar string, bot *irc.Client) *TamagotchiPlugin {
    return &TamagotchiPlugin{
        aktiv:      true,
        kisallatok: make(map[string]*Tamagotchi),
        adatKonyvtar: adatKonyvtar,
        utolsoFrissites: time.Now(),
        bot:        bot, // Bot referencia hozzáadva
    }
}

func (p *TamagotchiPlugin) Nev() string {
	return "TamagotchiPlugin"
}

func (p *TamagotchiPlugin) Initialize(bot *irc.Client, config *config.Config) error {
	// Adatkönyvtár létrehozása, ha nem létezik
	if err := os.MkdirAll(p.adatKonyvtar, 0755); err != nil {
		return err
	}
	
	// Meglévő kisállatok betöltése
	return p.kisallatokBetoltese()
}

func (p *TamagotchiPlugin) HandleMessage(uzenet irc.Message) string {
    if !p.aktiv {
        return ""
    }
    
    szoveg := strings.TrimSpace(uzenet.Text)
    reszek := strings.Fields(szoveg)
    
    if len(reszek) == 0 {
        return ""
    }
    
    parancs := strings.ToLower(reszek[0])
    
    switch parancs {
    case "!kisallat", "!tamagotchi":
        if len(reszek) < 2 {
            return p.segitoSzoveg()
        }
        
        alparancs := strings.ToLower(reszek[1])
        switch alparancs {
        case "uj", "letrehoz":
            if len(reszek) < 3 {
                return "Használat: !kisallat uj <név>"
            }
            nev := strings.Join(reszek[2:], " ")
            return p.kisallatLetrehozasValasz(uzenet.Channel, nev, uzenet.Nick)
            
        case "allapot", "status":
            return p.allapotValasz(uzenet.Channel)
            
        case "etet":
            return p.etetValasz(uzenet.Channel, uzenet.Nick)
            
        case "jatszik":
            return p.jatszikValasz(uzenet.Channel, uzenet.Nick)
            
        case "tisztit":
            return p.tisztitValasz(uzenet.Channel, uzenet.Nick)
            
        case "segitség":
            return p.segitoSzoveg()
            
        default:
            return p.segitoSzoveg()
        }
    }
    
    return ""
}
func (p *TamagotchiPlugin) kisallatLetrehozasValasz(csatorna, nev, tulajdonos string) string {
	//fmt.Printf("[DEBUG] kisallatLetrehozasValasz: tulajdonos = %q\n", tulajdonos)
	if _, letezik := p.kisallatok[csatorna]; letezik {
		return fmt.Sprintf("Már van kisállat ebben a csatornában! Használd a '!kisallat allapot' parancsot, hogy megnézd %s állapotát", p.kisallatok[csatorna].Nev)
	}
	
	if len(nev) > 20 {
		return "A kisállat neve túl hosszú! Maximum 20 karakter lehet."
	}
	
	kisallat := UjTamagotchi(nev, tulajdonos)
	p.kisallatok[csatorna] = kisallat
	p.kisallatokMentese()
	
	return fmt.Sprintf("🥚 %s létrehozott egy új kisállatot %s néven! Hamarosan kikel... Használd a '!kisallat segitség' parancsot a lehetőségek megtekintéséhez.", tulajdonos, nev)
}

func (p *TamagotchiPlugin) allapotValasz(csatorna string) string {
    kisallat, letezik := p.kisallatok[csatorna]
    if !letezik {
        return "Nincs kisállat ebben a csatornában! Használd: !kisallat uj <név>"
    }
    
    // Egyszerűbb formátum
    return fmt.Sprintf("%s %s (%s) | ❤️%d 🍎%d 😊%d 🧼%d | %s",
        kisallat.Emoji(),
        kisallat.Nev,
        kisallat.Allapot,
        kisallat.Egeszseg,
        kisallat.Ehseg,
        kisallat.Boldogsag,
        kisallat.Tisztasag,
        "Használd: !kisallat etet/jatszik/tisztit")
}

func (p *TamagotchiPlugin) etetValasz(csatorna, kuldo string) string {
	kisallat, letezik := p.kisallatok[csatorna]
	if !letezik {
		return "Nincs kisállat ebben a csatornában! Használd a '!kisallat uj <név>' parancsot egy létrehozásához."
	}
	
	valasz := kisallat.Etet()
	p.kisallatokMentese()
	return valasz
}

func (p *TamagotchiPlugin) jatszikValasz(csatorna, kuldo string) string {
	kisallat, letezik := p.kisallatok[csatorna]
	if !letezik {
		return "Nincs kisállat ebben a csatornában! Használd a '!kisallat uj <név>' parancsot egy létrehozásához."
	}
	
	valasz := kisallat.Jatszik()
	p.kisallatokMentese()
	return valasz
}

func (p *TamagotchiPlugin) tisztitValasz(csatorna, kuldo string) string {
	kisallat, letezik := p.kisallatok[csatorna]
	if !letezik {
		return "Nincs kisállat ebben a csatornában! Használd a '!kisallat uj <név>' parancsot egy létrehozásához."
	}
	
	valasz := kisallat.Tisztit()
	p.kisallatokMentese()
	return valasz
}

func (p *TamagotchiPlugin) segitoSzoveg() string {
	return "🐣 Tamagotchi Kisállat Parancsok !kisallat uj <név> - Új kisállat létrehozása !kisallat allapot - Kisállat állapotának ellenőrzése !kisallat etet - Kisállat etetése !kisallat jatszik - Játék a kisállattal !kisallat tisztit - Kisállat tisztítása !kisallat segitség - Segítség megjelenítése Tartsd a kisállatod boldognak, jól lakottnak és tisztán! 🎮"
}

func (p *TamagotchiPlugin) veletlenEsemeny(csatorna string, kisallat *Tamagotchi) string {
	if kisallat.Allapot == AllapotHalott {
		return ""
	}
	
	esemenyek := []string{
		fmt.Sprintf("🎵 %s boldogan énekel!", kisallat.Nev),
		fmt.Sprintf("😴 %s szunyál...", kisallat.Nev),
		fmt.Sprintf("🦋 %s pillangót kerget!", kisallat.Nev),
		fmt.Sprintf("🌟 %s örömtől ragyog!", kisallat.Nev),
		fmt.Sprintf("🎭 %s táncol!", kisallat.Nev),
	}
	
	// Negatív események hozzáadása, ha elhanyagolják
	if kisallat.Ehseg < 40 {
		esemenyek = append(esemenyek, fmt.Sprintf("😫 %s ételt keres...", kisallat.Nev))
	}
	if kisallat.Boldogsag < 40 {
		esemenyek = append(esemenyek, fmt.Sprintf("😢 %s magányosnak érzi magát...", kisallat.Nev))
	}
	if kisallat.Tisztasag < 40 {
		esemenyek = append(esemenyek, fmt.Sprintf("🤢 %s koszos lesz...", kisallat.Nev))
	}
	
	if rand.Intn(100) < 20 { // 20% esély véletlen eseményre
		return esemenyek[rand.Intn(len(esemenyek))]
	}
	
	return ""
}

// IdozitettFrissites implementálja a Plugin interfészt az időzített frissítésekhez
func (p *TamagotchiPlugin) IdozitettFrissites() []irc.Message {
	if !p.aktiv {
		return nil
	}
	
	var uzenetek []irc.Message
	
	for csatorna, kisallat := range p.kisallatok {
		kisallat.Frissit()
		
		if kisallat.Allapot == AllapotHalott && kisallat.HalalIdeje != nil && time.Since(*kisallat.HalalIdeje) < time.Minute {
			uzenetek = append(uzenetek, irc.Message{
				Channel: csatorna, // Csatorna -> Channel
				Text:    fmt.Sprintf("💀 Jaj ne! %s meghalt! 😢 Nyugodjék békében...", kisallat.Nev), // Szoveg -> Text
			})
		}
		
		if time.Since(p.utolsoFrissites) >= 30*time.Minute {
			if esemenyUzenet := p.veletlenEsemeny(csatorna, kisallat); esemenyUzenet != "" {
				uzenetek = append(uzenetek, irc.Message{
					Channel: csatorna, // Csatorna -> Channel
					Text:    esemenyUzenet, // Szoveg -> Text
				})
			}
		}
	}
	
	if time.Since(p.utolsoFrissites) >= 30*time.Minute {
		p.utolsoFrissites = time.Now()
		p.kisallatokMentese()
	}
	
	return uzenetek
}

func (p *TamagotchiPlugin) kisallatokMentese() error {
	adatok, err := json.MarshalIndent(p.kisallatok, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(filepath.Join(p.adatKonyvtar, "kisallatok.json"), adatok, 0644)
}

func (p *TamagotchiPlugin) kisallatokBetoltese() error {
	fajlNev := filepath.Join(p.adatKonyvtar, "kisallatok.json")
	adatok, err := os.ReadFile(fajlNev)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Még nincs kisállat fájl, ez rendben van
		}
		return err
	}
	
	return json.Unmarshal(adatok, &p.kisallatok)
}

func (p *TamagotchiPlugin) Shutdown() error {
	p.aktiv = false
	return p.kisallatokMentese()
}
func (p *TamagotchiPlugin) OnTick() []irc.Message {
    return nil
}