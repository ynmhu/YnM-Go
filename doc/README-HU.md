# YnM Go Bot

Egy erőteljes, moduláris IRC bot Go-ban, plugin rendszerrel. Önhosztolt környezetekhez tervezve, alacsony erőforrás-felhasználással és maximális testreszabási lehetőséggel.

---

## 🌍 Támogatás & Hivatalos Linkek

### 🇭🇺 Magyar
- **Fórum:** https://forum.ynm.hu/c/ynm-go/13
- **Fejlesztés:** https://git.ynm.hu/Markus/YnM-Go

### 🇷🇴 Română
- **Forum:** https://forum.ynm.hu/c/ynm-go/13
- **Dezvoltare:** https://git.ynm.hu/Markus/YnM-Go

### 🇬🇧 English
- **Forum:** https://forum.ynm.hu/c/ynm-go/13
- **Development:** https://git.ynm.hu/Markus/YnM-Go

> **Megjegyzés:** A projektet innentől kezdve Markus fejleszti. Segítségért kizárólag a hivatalos fórumon nyújtunk támogatást.

---

## 📦 Docker Futtatás

### Rendszerkövetelmények
- **API port:** 2525 (alapértelmezés)
- **Port mapping:** `8585:2525` (host:container)

### Docker Image-ek

```bash
# GitHub Container Registry
ghcr.io/ynmhu/ynm-go:latest

# YnM Git Registry
git.ynm.hu/markus/ynm-go:latest
```

> **Tipp:** Produkció esetén verzió tag-et használj (pl. `:2026-01-07`), teszt esetén a `:latest` megfelelő.

### Docker Run

```bash
docker run -d --name YnM-Go \
  --restart unless-stopped \
  -p 8585:2525 \
  -v "$(pwd)/YnMConfig:/app/YnMConfig" \
  -v "$(pwd)/data:/app/data" \
  -v "$(pwd)/logs:/app/logs" \
  -e TZ=Europe/Budapest \
  ghcr.io/ynmhu/ynm-go:latest
```

### Docker Compose

```yaml
services:
  ynm-bot:
    image: git.ynm.hu/markus/ynm-go:latest
    container_name: YnM-Go
    restart: unless-stopped
    ports:
      - "8585:2525"
    volumes:
      - ./YnMConfig:/app/YnMConfig
      - ./data:/app/data
      - ./logs:/app/logs
    environment:
      - TZ=Europe/Budapest
```

---

## ⚙️ Telepítés & Build

### Klónozás és Felépítés

```bash
# Repository klónozása
git clone https://git.ynm.hu/Markus/YnM-Go.git
cd YnM-Go

# Függőségek letöltése
go mod tidy

# Bot fordítása
go build -o YnM-Go
```

### Konfigurálás

```bash
# Konfigurációs mappa megnyitása
cd YnMConfig

# Alapértelmezés másolása
cp example-ynm.yaml ynm.yaml

# Szerkesztés
nano ynm.yaml
```

#### Wichtig Konfigurációs Pontok

| Szekció | Leírás |
|---------|--------|
| **IRC Kapcsolat** | Server, Port, NickName, UserName, RealName |
| **NickServ** | NickservBotnick, NickservNick, NickservPass |
| **SASL/TLS** | SASL, SASLUser, SASLPass, TLS, TLSCert, TLSKey |
| **Console** | Console (log csatorna), ConsoleKey (jelszó) |
| **Pluginok** | `true/false` opciók az egyes funkciók kikapcsolásához |
| **Adatbázisok** | LogDir, data_dir, seen_db, SmsDBPath |
| **Ütemezés** | Ping, Joke, Nevnap, topic_update_interval |

### Bot Indítása

```bash
./YnM-Go
```

---

## 🎯 Elérhető Pluginek

### 🛡️ Admin & Jogosultságkezelés
- Admin parancsok – jogosultságkezelés, automatikus VOICE/OP
- Automatikus módok beállítása

## IRC bot – Aktív pluginek listája

* 🛡️ Admin parancsok – jogosultságkezelés, automatikus VOICE/OP
* 📺 Médiaajánló – legfrissebb film/sorozat ajánlása
* ⏫ Médiafeltöltés figyelő – Jellyfin webhook alapján
* ✏️ Média kérés – felhasználói igények kezelése, „kell” és „keresek”
* ✅ Feltöltés visszaigazolás – kérések teljesítése
* 🗑️ Média kérés törlése – admin parancs
* 🔎 Média információ – keresés és részletes adatok
* 🤖 Tamagotchi játék – saját IRC kisállat gondozása (!tama)
* ❌⭕ X és 0 játék – fejlett interaktív játék IRC-n
* 🤣 Napi vicc – véletlenszerű vagy napi poénok
* 👀 Seen plugin – felhasználók utolsó üzenetének nyilvántartása
* 📰 RSS olvasó – HunTorrent vagy más feedek automatikus figyelése
* 🔍 IMDB kereső – film/sorozat információk címből (!imdb)
* 🎬 TMDb kereső – részletes film/sorozat/színész API integráció (fejlesztés alatt)
* 🍿 Random film ajánló – ország vagy népszerűség alapján válogatva
* ✉️ Mail olvasó – IRC-n keresztül hozzáférhető e-mail doboz
* ⛅ Időjárás – OpenWeatherMap vagy wttr.in integráció
* 🎂 Névnap értesítő – napi névnapok küldése reggelente
* ⌨️ Shell parancsok – előre meghatározott biztonságos parancsok (!ssh, !nmap, !dns, !ip)
* 🖥️ Resource monitor – CPU, memória, load figyelése és SQLite-ba mentés
* 🔴 Push értesítések – webhook események fogadása (pl. Jellyfin down)
* 🛠️ Szolgáltatásfigyelés – portok és szolgáltatások online állapotának követése
* 📡 Ping plugin – !ping / !pong parancsok és host elérhetőség
* 🧠 ChatGPT – mesterséges intelligencia válaszok IRC-n keresztül
* ⚡ XP rendszer – aktivitás alapú szintlépés és motivációs rendszer
* 📅 Óra / idő – pontos idő küldése (!ora)
* 🧠 Debug plugin – teszteléshez és hibafigyeléshez használható
* 📦 Tanuló plugin – !learn parancs egyedi válaszok tanítására
* 🌍 DNS / IP info – domain vagy IP cím alapján információ (!dns, !ip)
* 🦠 BruteForce figyelő – `/var/log/auth.log` valós idejű brute force ellenőrzés
* 🔁 IRC relay – üzenetek tükrözése másik IRC hálózatba
* 📬 Info/help – !help és !info parancsok válaszai
* 📹 YouTube információ – link alapján: cím, hossz, like szám, stb.
* 🔐 WebAuth – webes hitelesítés engedélyezése IRC parancsokhoz
* 🔄 Autotopic – automatikus topic frissítés a console csatornán
* 🛎️ Service Manager – szolgáltatás menedzsment parancsok (!service)
* 🛡️ Fail2Ban integráció – automatikus IP-tiltás logok alapján
* 🔁 Cycle plugin – csatorna cycle viselkedés ha nincs @ a nicknél
* 📊 Média aktivitás követés – médiafájlok használati statisztikáinak figyelése
* 🗓️ Média feltöltés dátumkövetés – sent_dates.json alapú követés
* 🔑 ConsoleKey – konzolcsatorna jelszóval védett parancsok
* ✉️ SMTP küldés – e-mail küldés/SMTP integráció (mail plugin)
* 🗂️ Git API interakció – commitok és repo információk lekérése IRC-n keresztül
* 🌐 Undernet / X-service integráció – más IRC hálózatokhoz való kapcsolódás és X-service kezelés
* 🔐 SASL autentikáció – IRC SASL login támogatás
* 🧪 Experimental (X) plugin – kísérleti/fejlesztés alatt álló funkciók engedélyezése
---

## 📁 Könyvtárszerkezet

```
YnM-Go/
├── data/              # Adatbázisok és adatok
│   ├── admins.json
│   ├── movies.db
│   ├── xp.db
│   ├── seen.db
│   ├── sent_dates.json
│   └── ...
├── logs/              # Naplófájlok
├── YnMConfig/         # Konfigurációs fájlok
│   ├── ynm.yaml       # Fő konfiguráció
│   ├── media.yaml
│   ├── xp.yaml
│   └── ...
├── YnM/               # Fő bot modulok
├── YnMIrC/            # IRC protokoll
├── YnMPlugins/        # Plugin-ek
├── YnMModuls/         # Modul-ok
└── main.go            # Belépési pont
```

---

## 🔧 Konfigurációs Példa

```yaml
# ════════════════════════════════════════
# IRC Alapbeállítások
# ════════════════════════════════════════

Server: "192.168.0.150"
Port: "6667"
TLSPort: "6697"
TLS: true

NickName: "YnM-Go"
UserName: "YnM"
RealName: "Markus Lajos"

# ════════════════════════════════════════
# NickServ Bejelentkezés
# ════════════════════════════════════════

SASL: true
SASLUser: "YnM-Go"
SASLPass: "jelszó"

NickservBotnick: "NickServ"
NickservNick: "YnM-Go"
NickservPass: "jelszó"

Autologin: true
AutoJoinWithoutLogin: false

# ════════════════════════════════════════
# Csatornák & Rendszer
# ════════════════════════════════════════

Console: "#YnM"
Channels:
  - "#Help"
  - "#Magyar"

LogDir: "./logs"
data_dir: "./data"
ReconOnDiscon: "60s"

# ════════════════════════════════════════
# Pluginek Aktiválása (true/false)
# ════════════════════════════════════════

Plugins:
  enable_ping: true
  enable_xp: true
  enable_movie: true
  enable_media_upload: true
  enable_chatgpt: true
  enable_git: true
  enable_ssh: true
  enable_nmap: true
  enable_webstatus: true

# ════════════════════════════════════════
# Jellyfin Integráció
# ════════════════════════════════════════

jellyfin_db_path: "/var/lib/jellyfin/data/library.db"
movie_db_path: "./data/movies.db"
movie_requests_channel: "#Magyar"

media_upload:
  enabled: true
  channels:
    - "#Magyar"
  interval_minutes: 1
  jellyfin_db: "/var/lib/jellyfin/data/library.db"

# ════════════════════════════════════════
# Ütemezés
# ════════════════════════════════════════

NevnapReggel: "07:30"
JokeSendTime: "08:00"
SzekelyhonInterval: 120m

media_ajanlat:
  channel: "#Magyar"
  time: "21:35"

# ════════════════════════════════════════
# API-k & Kulcsok
# ════════════════════════════════════════

openai:
  api_key: "sk-..."

weather:
  weatherAPIKey: "..."
  defaultLocation: "Budapest"
```

---

## 📊 Egyedi Parancsok

| Parancs | Leírás |
|---------|--------|
| `!help` | Súgó megjelenítése |
| `!whoami` | Jogosultság szint kiírása |
| `!uptime` | Bot felüli ideje |
| `!status` | Rendszer állapota |
| `!chatgpt <szöveg>` | AI válasz |
| `!imdb <film>` | Film információ |
| `!yt <link>` | YouTube info |
| `!tama` | Tamagotchi játék |
| `!vicc` | Napi vicc |
| `!ssh <host> <port>` | SSH port ellenőrzés |
| `!nmap <host>` | Hálózati scan |
| `!ip <host>` | IP információ |
| `!dns <domain>` | DNS lookup |
| `!ora` | Pontos idő |

---
## 🔗 Gyorsított Linkek

| Erőforrás | Link |
|----------|------|
| 📋 **Hivatalos Fórum** | https://forum.ynm.hu/c/ynm-go/13 |
| 💻 **Fejlesztés** | https://git.ynm.hu/Markus/YnM-Go |
| ⚙️ **Admin Webes Felület** | https://ynm-go.ynm.hu |
| 📊 **Botok Rendelkezésre Állása** | http://uptime.ynm.hu |
| 🔌 **Botok és Pluginok** | https://bot.ynm.hu |

---

### Gyors Navigáció

- **Most kezdesz?** → [Hivatalos Fórum](https://forum.ynm.hu/c/ynm-go/13)
- **Kódot szeretnél közreműködtetni?** → [Fejlesztési Tár](https://git.ynm.hu/Markus/YnM-Go)
- **Botokat szeretnél kezelni?** → [Admin Irányítópult](https://ynm-go.ynm.hu)
- **Szeretnéd ellenőrizni az állapotot?** → [Uptime Státusz](http://uptime.ynm.hu)

---

## 📝 Fejlesztő Információ

**Fejlesztette:** Markus (YnM.hu)  
📧 **Email:** [markus@ynm.hu](mailto:markus@ynm.hu)  
🌐 **Weboldal:** https://ynm.hu  
📋 **Szerzői jog:** 2012-2026 – Minden jog fenntartva.