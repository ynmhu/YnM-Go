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
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"github.com/ynmhu/YnM-Go/irc"

)

type NameDayPlugin struct {
    AnnounceChannels []string
    userRequestTimes map[string][]time.Time   // felhasználó lekérései időpontjai
    userBanUntil    map[string]time.Time      // felhasználói tiltások lejárati ideje
    userBanNotified map[string]bool            // jelezte-e már a tiltást
	lastMorningAnnounce string
	lastEveningAnnounce string
	NevnapReggel time.Time
    NevnapEste   time.Time
	mu              sync.Mutex
}

var nameDays = map[string][]string{
	
    "01-01": {"Fruzsina", "Aladár"},
    "01-02": {"Ábel", "Vazul"},
    "01-03": {"Genovéva", "Benjámin"},
    "01-04": {"Titusz", "Leona"},
    "01-05": {"Simon", "Emília"},
    "01-06": {"Boldizsár"},
    "01-07": {"Attila", "Ramóna"},
    "01-08": {"Gyöngyvér", "Erhard"},
    "01-09": {"Marcell"},
    "01-10": {"Melánia"},
    "01-11": {"Ágota", "Honoráta"},
    "01-12": {"Ernő"},
    "01-13": {"Veronika"},
    "01-14": {"Bódog"},
    "01-15": {"Lóránt", "Loránd"},
    "01-16": {"Gusztáv"},
    "01-17": {"Antal", "Antónia"},
    "01-18": {"Piroska"},
    "01-19": {"Sára", "Márió"},
    "01-20": {"Fábián", "Sebestyén"},
    "01-21": {"Ágnes"},
    "01-22": {"Vince", "Artúr"},
    "01-23": {"Zelma", "Rajmund"},
    "01-24": {"Timót"},
    "01-25": {"Pál"},
    "01-26": {"Vanda", "Paula"},
    "01-27": {"Angelika"},
    "01-28": {"Károly", "Karola"},
    "01-29": {"Adél"},
    "01-30": {"Martina", "Gerda"},
    "01-31": {"Marcella"},
    "02-01": {"Ignác"},
    "02-02": {"Karolina", "Aida"},
    "02-03": {"Balázs"},
    "02-04": {"Ráhel", "Csenge"},
    "02-05": {"Ágota", "Ingrid"},
    "02-06": {"Dorottya", "Dóra"},
    "02-07": {"Tódor", "Richárd"},
    "02-08": {"Aranka"},
    "02-09": {"Abigél", "Alex"},
    "02-10": {"Elvira"},
    "02-11": {"Bertold", "Marietta"},
    "02-12": {"Lívia", "Lídia"},
    "02-13": {"Ella", "Linda"},
    "02-14": {"Bálint", "Valentin"},
    "02-15": {"Kolos", "Georgina"},
    "02-16": {"Julianna", "Lilla"},
    "02-17": {"Donát"},
    "02-18": {"Bernadett"},
    "02-19": {"Zsuzsanna"},
    "02-20": {"Aladár", "Álmos"},
    "02-21": {"Eleonóra"},
    "02-22": {"Gerzson"},
    "02-23": {"Alfréd"},
    "02-24": {"Mátyás"},
    "02-25": {"Géza"},
    "02-26": {"Edina"},
    "02-27": {"Ákos", "Bátor"},
    "02-28": {"Elemér"},
    "02-29": {"Elemér"},  
    "03-01": {"Albin"},
    "03-02": {"Lujza"},
    "03-03": {"Kornélia"},
    "03-04": {"Kázmér"},
    "03-05": {"Adorján", "Adrián"},
    "03-06": {"Leonóra", "Inez"},
    "03-07": {"Tamás"},
    "03-08": {"János", "Zoltán"},
    "03-09": {"Franciska", "Fanni"},
    "03-10": {"Ildikó"},
    "03-11": {"Szilárd"},
    "03-12": {"Gergely"},
    "03-13": {"Krisztián", "Ajtony"},
    "03-14": {"Matild"},
    "03-15": {"Kristóf"},
    "03-16": {"Henrietta"},
    "03-17": {"Gertrúd", "Patrik"},
    "03-18": {"Sándor", "Ede"},
    "03-19": {"József", "Bánk"},
    "03-20": {"Klaudia"},
    "03-21": {"Benedek"},
    "03-22": {"Beáta", "Izolda"},
    "03-23": {"Emőke"},
    "03-24": {"Gábor", "Karina"},
    "03-25": {"Irén", "Írisz"},
    "03-26": {"Emánuel"},
    "03-27": {"Hajnalka"},
    "03-28": {"Gedeon"},
    "03-29": {"Auguszta"},
    "03-30": {"Zalán"},
    "03-31": {"Árpád"},
    "04-01": {"Hugó"},
    "04-02": {"Áron"},
    "04-03": {"Buda", "Richárd"},
    "04-04": {"Izidor"},
    "04-05": {"Vince"},
    "04-06": {"Vilmos", "Bíborka"},
    "04-07": {"Herman"},
    "04-08": {"Dénes"},
    "04-09": {"Erhard"},
    "04-10": {"Zsolt"},
    "04-11": {"Leó", "Szaniszló"},
    "04-12": {"Gyula"},
    "04-13": {"Ida"},
    "04-14": {"Tibor"},
    "04-15": {"Anasztázia", "Tas"},
    "04-16": {"Csongor"},
    "04-17": {"Rudolf"},
    "04-18": {"Andrea", "Ilma"},
    "04-19": {"Emma"},
    "04-20": {"Tivadar"},
    "04-21": {"Konrád"},
    "04-22": {"Csilla", "Noémi"},
    "04-23": {"Béla"},
    "04-24": {"György"},
    "04-25": {"Márk"},
    "04-26": {"Ervin"},
    "04-27": {"Zita"},
    "04-28": {"Valéria"},
    "04-29": {"Péter"},
    "04-30": {"Katalin", "Kitti"},
    "05-01": {"Fülöp", "Jakab"},
    "05-02": {"Zsigmond"},
    "05-03": {"Irma"},
    "05-04": {"Mónika", "Flórián"},
    "05-05": {"Györgyi"},
    "05-06": {"Ivett", "Frida"},
    "05-07": {"Gizella"},
    "05-08": {"Mihály"},
    "05-09": {"Gergely"},
    "05-10": {"Ármin", "Pálma"},
    "05-11": {"Ferenc"},
    "05-12": {"Pongrác"},
    "05-13": {"Szervác"},
    "05-14": {"Bonifác"},
    "05-15": {"Zsófia", "Szonja"},
    "05-16": {"Mózes", "Botond"},
    "05-17": {"Paszkál"},
    "05-18": {"Erik", "Alexandra"},
    "05-19": {"Ivó", "Milán"},
    "05-20": {"Bernát", "Felícia"},
    "05-21": {"Konstantin"},
    "05-22": {"Júlia", "Rita"},
    "05-23": {"Dezső"},
    "05-24": {"Eszter", "Eliza"},
    "05-25": {"Orbán"},
    "05-26": {"Fülöp", "Evelin"},
    "05-27": {"Hella"},
    "05-28": {"Emil", "Csanád"},
    "05-29": {"Magdolna"},
    "05-30": {"Janka", "Zsanett"},
    "05-31": {"Angéla", "Petronella"},
    "06-01": {"Tünde"},
    "06-02": {"Kármen", "Anita"},
    "06-03": {"Klotild"},
    "06-04": {"Bulcsú"},
    "06-05": {"Fatime"},
    "06-06": {"Norbert"},
    "06-07": {"Róbert"},
    "06-08": {"Medárd"},
    "06-09": {"Félix"},
    "06-10": {"Margit", "Gréta"},
    "06-11": {"Barnabás"},
    "06-12": {"Villő"},
    "06-13": {"Antal", "Anett"},
    "06-14": {"Vazul"},
    "06-15": {"Jolán", "Vid"},
    "06-16": {"Jusztin"},
    "06-17": {"Laura", "Alida"},
    "06-18": {"Arnold", "Levente"},
    "06-19": {"Gyárfás"},
    "06-20": {"Rafael"},
    "06-21": {"Alajos"},
    "06-22": {"Paulina"},
    "06-23": {"Zoltán"},
    "06-24": {"Iván"},
    "06-25": {"Vilmos"},
    "06-26": {"János", "Pál"},
    "06-27": {"László"},
    "06-28": {"Levente", "Irén"},
    "06-29": {"Péter", "Pál"},
    "06-30": {"Pál"},
    "07-01": {"Tihamér", "Annamária"},
    "07-02": {"Ottó"},
    "07-03": {"Kornél", "Soma"},
    "07-04": {"Ulrik"},
    "07-05": {"Emese", "Sarolta"},
    "07-06": {"Csaba"},
    "07-07": {"Appolónia"},
    "07-08": {"Ellák"},
    "07-09": {"Lukrécia"},
    "07-10": {"Amália"},
    "07-11": {"Nóra", "Lili"},
    "07-12": {"Izabella", "Dalma"},
    "07-13": {"Jenő"},
    "07-14": {"Örs", "Stella"},
    "07-15": {"Henrik", "Roland"},
    "07-16": {"Valter"},
    "07-17": {"Endre", "Elek"},
    "07-18": {"Frigyes"},
    "07-19": {"Emília"},
    "07-20": {"Illés"},
    "07-21": {"Dániel", "Daniella"},
    "07-22": {"Magdolna"},
    "07-23": {"Lenke"},
    "07-24": {"Kinga", "Kincső"},
    "07-25": {"Kristóf", "Jakab"},
    "07-26": {"Anna", "Anikó"},
    "07-27": {"Liliána", "Olga"},
    "07-28": {"Szabolcs"},
    "07-29": {"Márta", "Flóra"},
    "07-30": {"Judit", "Xénia"},
    "07-31": {"Oszkár"},
    "08-01": {"Boglárka"},
    "08-02": {"Lehel"},
    "08-03": {"Hermina"},
    "08-04": {"Domonkos", "Dominika"},
    "08-05": {"Krisztina"},
    "08-06": {"Berta", "Bettina"},
    "08-07": {"Ibolya"},
    "08-08": {"László"},
    "08-09": {"Emőd"},
    "08-10": {"Lőrinc"},
    "08-11": {"Zsuzsanna", "Tiborc"},
    "08-12": {"Klára"},
    "08-13": {"Ipoly"},
    "08-14": {"Marcell"},
    "08-15": {"Mária"},
    "08-16": {"Ábrahám"},
    "08-17": {"Jácint"},
    "08-18": {"Ilona"},
    "08-19": {"Huba"},
    "08-20": {"István"},
    "08-21": {"Sámuel", "Hajna"},
    "08-22": {"Menyhért", "Mirjam"},
    "08-23": {"Bence"},
    "08-24": {"Bertalan"},
    "08-25": {"Lajos", "Patrícia"},
    "08-26": {"Izsó"},
    "08-27": {"Gáspár"},
    "08-28": {"Ágoston"},
    "08-29": {"Beatrix", "Erna"},
    "08-30": {"Rózsa"},
    "08-31": {"Erika", "Bella"},
    "09-01": {"Egyed"},
    "09-02": {"Rebeka", "Dorina"},
    "09-03": {"Hilda"},
    "09-04": {"Rozália"},
    "09-05": {"Viktor", "Lőrinc"},
    "09-06": {"Zakariás"},
    "09-07": {"Regina"},
    "09-08": {"Mária", "Adrienn"},
    "09-09": {"Ádám"},
    "09-10": {"Nikolett", "Hunor"},
    "09-11": {"Teodóra"},
    "09-12": {"Mária"},
    "09-13": {"Kornél"},
    "09-14": {"Szeréna", "Roxána"},
    "09-15": {"Enikő", "Melitta"},
    "09-16": {"Edit"},
    "09-17": {"Zsófia"},
    "09-18": {"Diána"},
    "09-19": {"Vilhelmina"},
    "09-20": {"Friderika"},
    "09-21": {"Máté", "Mirella"},
    "09-22": {"Móric"},
    "09-23": {"Tekla"},
    "09-24": {"Gellért", "Mercédesz"},
    "09-25": {"Eufrozina", "Kende"},
    "09-26": {"Jusztina"},
    "09-27": {"Adalbert"},
    "09-28": {"Vencel"},
    "09-29": {"Mihály"},
    "09-30": {"Jeromos"},
    "10-01": {"Malvin"},
    "10-02": {"Petra"},
    "10-03": {"Helga"},
    "10-04": {"Ferenc"},
    "10-05": {"Aurél"},
    "10-06": {"Brúnó", "Renáta"},
    "10-07": {"Amália"},
    "10-08": {"Koppány"},
    "10-09": {"Dénes"},
    "10-10": {"Gedeon"},
    "10-11": {"Brigitta"},
    "10-12": {"Miksa"},
    "10-13": {"Kálmán", "Ede"},
    "10-14": {"Helén"},
    "10-15": {"Teréz"},
    "10-16": {"Gál"},
    "10-17": {"Hedvig"},
    "10-18": {"Lukács"},
    "10-19": {"Nándor"},
    "10-20": {"Vendel"},
    "10-21": {"Orsolya"},
    "10-22": {"Előd"},
    "10-23": {"Gyöngyi"},
    "10-24": {"Salamon"},
    "10-25": {"Blanka", "Bianka"},
    "10-26": {"Dömötör"},
    "10-27": {"Szabina"},
    "10-28": {"Simon", "Júdás"},
    "10-29": {"Nárcisz"},
    "10-30": {"Alfonz"},
    "10-31": {"Farkas"},
    "11-01": {"Marianna"},
    "11-02": {"Achilles"},
    "11-03": {"Győző"},
    "11-04": {"Károly"},
    "11-05": {"Imre"},
    "11-06": {"Lénárd"},
    "11-07": {"Rezső"},
    "11-08": {"Zsombor"},
    "11-09": {"Tivadar"},
    "11-10": {"Réka"},
    "11-11": {"Márton"},
    "11-12": {"Jónás", "Renátó"},
    "11-13": {"Szilvia"},
    "11-14": {"Aliz"},
    "11-15": {"Albert", "Lipót"},
    "11-16": {"Ödön"},
    "11-17": {"Hortenzia", "Gergő"},
    "11-18": {"Jenő"},
    "11-19": {"Erzsébet"},
    "11-20": {"Jolán"},
    "11-21": {"Olivér"},
    "11-22": {"Cecília"},
    "11-23": {"Kelemen", "Klementina"},
    "11-24": {"Emma"},
    "11-25": {"Katalin"},
    "11-26": {"Virág"},
    "11-27": {"Virgil"},
    "11-28": {"Stefánia"},
    "11-29": {"Taksony"},
    "11-30": {"András", "Andor"},
    "12-01": {"Elza"},
    "12-02": {"Melinda", "Vivien"},
    "12-03": {"Ferenc", "Olívia"},
    "12-04": {"Borbála", "Barbara"},
    "12-05": {"Vilma"},
    "12-06": {"Miklós"},
    "12-07": {"Ambrus"},
    "12-08": {"Mária"},
    "12-09": {"Natália"},
    "12-10": {"Judit"},
    "12-11": {"Árpád"},
    "12-12": {"Gabriella"},
    "12-13": {"Luca", "Otília"},
    "12-14": {"Szilárda"},
    "12-15": {"Valér"},
    "12-16": {"Etelka", "Aletta"},
    "12-17": {"Lázár", "Olimpia"},
    "12-18": {"Auguszta"},
    "12-19": {"Viola"},
    "12-20": {"Teofil"},
    "12-21": {"Tamás"},
    "12-22": {"Zénó"},
    "12-23": {"Viktória"},
    "12-24": {"Ádám", "Éva"},
    "12-25": {"Eugénia"},
    "12-26": {"István"},
    "12-27": {"János"},
    "12-28": {"Kamilla"},
    "12-29": {"Tamás"},
    "12-30": {"Dávid"},
    "12-31": {"Szilveszter"},
}




func NewNameDayPlugin(channels []string, reggelStr, esteStr string) (*NameDayPlugin, error) {
    loc := time.Now().Location()
    reggel, err := time.ParseInLocation("15:04", reggelStr, loc)
    if err != nil {
        return nil, err
    }
    este, err := time.ParseInLocation("15:04", esteStr, loc)
    if err != nil {
        return nil, err
    }

    return &NameDayPlugin{
        AnnounceChannels: channels,
        NevnapReggel:     reggel,
        NevnapEste:       este,
        userRequestTimes: make(map[string][]time.Time),
        userBanUntil:     make(map[string]time.Time),
        userBanNotified:  make(map[string]bool),
    }, nil
}

func (p *NameDayPlugin) HandleMessage(msg irc.Message) string {
    cmd := strings.TrimSpace(strings.ToLower(msg.Text))
    
    if !strings.HasPrefix(cmd, "!nevnap") {
        return ""
    }

    p.mu.Lock()
    defer p.mu.Unlock()

    now := time.Now()
    user := msg.Sender

    // Tiltás ellenőrzése
    banUntil, banned := p.userBanUntil[user]
    if banned && now.Before(banUntil) {
        if !p.userBanNotified[user] {
            remaining := banUntil.Sub(now).Round(time.Second)
            p.userBanNotified[user] = true
            return fmt.Sprintf("Túl sok !nevnap parancs, kérlek várj még %s-ig!", remaining)
        }
        return ""  // már jelezte, nem válaszol többször
    } else if banned && now.After(banUntil) {
        // Tiltás lejárt, töröljük
        delete(p.userBanUntil, user)
        delete(p.userBanNotified, user)
        p.userRequestTimes[user] = nil
    }

    // Kitisztítjuk az 5 percnél régebbi hívásokat
    times := p.userRequestTimes[user]
    validTimes := []time.Time{}
    for _, t := range times {
        if now.Sub(t) <= 5*time.Minute {
            validTimes = append(validTimes, t)
        }
    }
    p.userRequestTimes[user] = validTimes

    // Ha 3 vagy több hívás van 5 perc alatt, tiltjuk 24 órára
    if len(validTimes) >= 3 {
        p.userBanUntil[user] = now.Add(24 * time.Hour)
        p.userBanNotified[user] = true  // azonnal jelezzük
        p.userRequestTimes[user] = nil
        return "Túl sok !nevnap parancs, ezért 24 órára le vagy tiltva erről a parancsról."
    }

    // Hozzáadjuk az aktuális hívást
    p.userRequestTimes[user] = append(p.userRequestTimes[user], now)

    // Eredeti parancs feldolgozása (eredeti kódod, itt van vágva pl.)
    args := strings.TrimSpace(cmd[len("!nevnap"):])
    
    if args == "" {
        return p.getTodayTomorrow()
    }
    if day, month, ok := p.parseDate(args); ok {
        return p.getNameDayByDate(month, day)
    }
    return p.searchNameDay(args)
}


func (p *NameDayPlugin) OnTick() []irc.Message {
	p.mu.Lock()
    defer p.mu.Unlock()
	now := time.Now()
	todayKey := now.Format("2006-01-02")
	var messages []irc.Message

	todayNames := p.getTodaysNameDay()
	tomorrowNames := p.getTomorrowsNameDay()


	// Reggeli bejelentés (pontos idő az configból, ±1 perc tolerancia)
	if now.Hour() == p.NevnapReggel.Hour() && now.Minute() <= p.NevnapReggel.Minute()+1 && p.lastMorningAnnounce != todayKey && todayNames != "" {
		for _, channel := range p.AnnounceChannels {
			messages = append(messages, irc.Message{
				Channel: channel,
				Text:    fmt.Sprintf("Ma *%s* névnapja van! Boldog névnapot! 🎉", todayNames),
			})
			fmt.Printf("[Névnap] Küldés reggel %s csatornára\n", channel)
		}
		p.lastMorningAnnounce = todayKey
	}

	// Esti bejelentés (pontos idő az configból, ±1 perc tolerancia)
	if now.Hour() == p.NevnapEste.Hour() && now.Minute() == p.NevnapEste.Minute() && p.lastEveningAnnounce != todayKey {
		for _, channel := range p.AnnounceChannels {
			if todayNames != "" {
				messages = append(messages, irc.Message{
					Channel: channel,
					Text:    fmt.Sprintf("Ma *%s* névnapja volt.", todayNames),
				})
				fmt.Printf("[Névnap] Küldés este (ma) %s csatornára\n", channel)
			}
			if tomorrowNames != "" {
				messages = append(messages, irc.Message{
					Channel: channel,
					Text:    fmt.Sprintf("Holnap *%s* névnapja lesz.", tomorrowNames),
				})
				fmt.Printf("[Névnap] Küldés este (holnap) %s csatornára\n", channel)
			}
		}
		p.lastEveningAnnounce = todayKey
	}

	return messages
}




// Helper functions
func (p *NameDayPlugin) getTodayTomorrow() string {
	todayNames := p.getTodaysNameDay()
	tomorrowNames := p.getTomorrowsNameDay()
	
	if todayNames == "" && tomorrowNames == "" {
		return "Ma és holnap sincs névnap."
	}
	
	msg := ""
	if todayNames != "" {
		msg += fmt.Sprintf("Névnap: Ma (%s): %s", time.Now().Format("2006.1. 2"), todayNames)
	}
	if tomorrowNames != "" {
		if msg != "" {
			msg += ", "
		}
		msg += fmt.Sprintf("holnap: %s", tomorrowNames)
	}
	
	return msg
}

func (p *NameDayPlugin) getNameDayByDate(month, day int) string {
	key := fmt.Sprintf("%02d-%02d", month, day)
	if names, ok := nameDays[key]; ok {
		return fmt.Sprintf("névnap: %d.%d: %s", month, day, strings.Join(names, ", "))
	}
	return fmt.Sprintf("Nincs névnap %d.%d.-án.", month, day)
}

func (p *NameDayPlugin) searchNameDay(name string) string {
    var results []string
    normalizedSearch := normalizeString(strings.TrimSpace(name))
    
    for date, names := range nameDays {
        for _, n := range names {
            normalizedName := normalizeString(n)
            if normalizedName == normalizedSearch {
                parts := strings.Split(date, "-")
                month, _ := strconv.Atoi(parts[0])
                day, _ := strconv.Atoi(parts[1])
                results = append(results, fmt.Sprintf("%s%d.", getHungarianMonth(month), day))
            }
        }
    }
    
    if len(results) > 0 {
        return fmt.Sprintf("Névnap: Mikor van %s nap: %s", name, strings.Join(results, " "))
    }
    return fmt.Sprintf("Névnap: Ilyen nevű névnap nincs (%s).", name)
}

// Helper function to normalize strings (remove accents and case)
func normalizeString(s string) string {
    // Replace accented characters with their base forms
    replacements := map[rune]rune{
        'á': 'a', 'é': 'e', 'í': 'i', 'ó': 'o', 'ö': 'o', 'ő': 'o',
        'ú': 'u', 'ü': 'u', 'ű': 'u', 'Á': 'a', 'É': 'e', 'Í': 'i',
        'Ó': 'o', 'Ö': 'o', 'Ő': 'o', 'Ú': 'u', 'Ü': 'u', 'Ű': 'u',
    }
    
    var result []rune
    for _, r := range strings.ToLower(s) {
        if replacement, ok := replacements[r]; ok {
            result = append(result, replacement)
        } else {
            result = append(result, r)
        }
    }
    return string(result)
}

// Helper function to get Hungarian month names
func getHungarianMonth(month int) string {
    months := []string{"", "I.", "II.", "III.", "IV.", "V.", "VI.", 
                      "VII.", "VIII.", "IX.", "X.", "XI.", "XII."}
    if month >= 1 && month <= 12 {
        return months[month]
    }
    return ""
}
func (p *NameDayPlugin) getTodaysNameDay() string {
	today := time.Now().Format("01-02")
	if names, ok := nameDays[today]; ok {
		return strings.Join(names, ", ")
	}
	return ""
}

func (p *NameDayPlugin) getTomorrowsNameDay() string {
	tomorrow := time.Now().Add(24 * time.Hour).Format("01-02")
	if names, ok := nameDays[tomorrow]; ok {
		return strings.Join(names, ", ")
	}
	return ""
}

func (p *NameDayPlugin) parseDate(input string) (day, month int, ok bool) {
	parts := strings.Split(input, ".")
	if len(parts) != 2 {
		return 0, 0, false
	}
	
	month, err1 := strconv.Atoi(parts[0])
	day, err2 := strconv.Atoi(parts[1])
	
	if err1 != nil || err2 != nil || month < 1 || month > 12 || day < 1 || day > 31 {
		return 0, 0, false
	}
	
	return month, day, true
}