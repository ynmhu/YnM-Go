# YnM Go Bot

Un bot IRC puternic și modular scris în Go cu sistem de plugin. Conceput pentru medii auto-găzduite, consum redus de resurse și personalizare maximă.

---

## 🌍 Suport & Linkuri Oficiale

### 🇭🇺 Maghiară
- **Forum:** https://forum.ynm.hu/c/ynm-go/13
- **Dezvoltare:** https://git.ynm.hu/Markus/YnM-Go

### 🇷🇴 Română
- **Forum:** https://forum.ynm.hu/c/ynm-go/13
- **Dezvoltare:** https://git.ynm.hu/Markus/YnM-Go

### 🇬🇧 Engleză
- **Forum:** https://forum.ynm.hu/c/ynm-go/13
- **Dezvoltare:** https://git.ynm.hu/Markus/YnM-Go

> **Notă:** Proiectul este acum menținut de Markus. Suportul este oferit exclusiv pe forum oficial.

---

## 📦 Implementare Docker

### Cerințe de Sistem
- **Port API:** 2525 (implicit)
- **Mapare port:** `8585:2525` (gazdă:container)

### Imagini Docker

```bash
# GitHub Container Registry
ghcr.io/ynmhu/ynm-go:latest

# YnM Git Registry
git.ynm.hu/markus/ynm-go:latest
```

> **Sfat:** Pentru producție, utilizează tag-uri de versiune (ex: `:2026-01-07`). Pentru testare, `:latest` este adecvat.

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

## ⚙️ Instalare & Compilare

### Clonare și Compilare

```bash
# Clonează repository-ul
git clone https://git.ynm.hu/Markus/YnM-Go.git
cd YnM-Go

# Descarcă dependențele
go mod tidy

# Compilează botul
go build -o YnM-Go
```

### Configurare

```bash
# Navighează în directorul de configurare
cd YnMConfig

# Copiază configurația implicită
cp example-ynm.yaml ynm.yaml

# Editează configurația
nano ynm.yaml
```

#### Puncte Importante de Configurare

| Secțiune | Descriere |
|----------|-----------|
| **Conexiune IRC** | Server, Port, NickName, UserName, RealName |
| **NickServ** | NickservBotnick, NickservNick, NickservPass |
| **SASL/TLS** | SASL, SASLUser, SASLPass, TLS, TLSCert, TLSKey |
| **Consolă** | Console (canal jurnal), ConsoleKey (parolă) |
| **Plugin-uri** | Opțiuni `true/false` pentru a activa/dezactiva funcții |
| **Baze de date** | LogDir, data_dir, seen_db, SmsDBPath |
| **Programare** | Ping, Joke, Nevnap, topic_update_interval |

### Pornește Botul

```bash
./YnM-Go
```

---

## 🎯 Plugin-uri Disponibile

### 🛡️ Admin & Gestionare Permisiuni
- Comenzi admin – gestionare permisiuni, VOICE/OP autometic
- Atribuire modul automată

### 📺 Plugin-uri Media
- **Recomandări Media** – sugerează filme/seriale încărcate recent
- **Monitor Încărcare Media** – detectează încărcări noi via webhook-uri Jellyfin
- **Cereri Media** – utilizatorii pot solicita filme/seriale ("kell" și "keresek")
- **Confirmare Încărcare** – marchează cererile ca finalizate
- **Ștergere Cerere** – comandă admin pentru eliminarea cererilor
- **Info Media** – căutare și informații detaliate despre conținut
- **Urmărire Activitate Media** – statistici și monitorizare utilizare

### 🎮 Jocuri
- **Tamagotchi** – crește un animal digital via IRC (!tama)
- **Tic-Tac-Toe** – joc interactiv bazat pe IRC

### 📰 Informații & Căutare
- **Căutare IMDB** – informații despre filme/seriale (!imdb)
- **Căutare TMDb** – date detaliate via API
- **Info YouTube** – extragere date video
- **Recomandator Film Aleatoriu** – după țară sau popularitate
- **Cititor RSS** – monitorizare HunTorrent și alte feed-uri

### ⚡ Funcții Utilizator
- **Plugin Seen** – urmărește ultimul mesaj al utilizatorului
- **Sistem XP** – niveluri bazate pe activitate
- **Cititor Mail** – acces poștă via IRC
- **Glume Zilei** – umor aleatoriu sau zilei
- **Notificator Zile Onomastice** – anunțuri zilei
- **Tamagotchi** – simulare îngrijire animal

### 🌐 Rețea & Sistem
- **Comenzi Shell** – !ssh, !nmap, !dns, !ip
- **Plugin Ping** – verificare accesibilitate gazdă
- **Monitor Brute Force** – monitorizare `/var/log/auth.log`
- **Integrare Fail2Ban** – blocare IP automată
- **Monitor Servicii** – urmărire disponibilitate port și servicii
- **Monitor Resurse** – monitorizare CPU, memorie, load

### 🤖 Inteligență Artificială & Altele
- **Integrare ChatGPT** – răspunsuri AI via IRC
- **Vreme** – integrare OpenWeatherMap sau wttr.in
- **Horoscop** – horoscop zilei
- **Plugin Learn** – predă răspunsuri personalizate (!learn)
- **Git API** – informații repository și commit
- **Suport Webhook** – notificări push
- **WebAuth** – autentificare web pentru comenzi IRC
- **Autotopic** – actualizări topic automată
- **Releu IRC** – reflectă mesaje pe alte rețele
- **SASL & TLS** – conexiuni securizate

---

## 📁 Structura Directoarelor

```
YnM-Go/
├── data/              # Baze de date și date
│   ├── admins.json
│   ├── movies.db
│   ├── xp.db
│   ├── seen.db
│   ├── sent_dates.json
│   └── ...
├── logs/              # Fișiere jurnal
├── YnMConfig/         # Fișiere configurare
│   ├── ynm.yaml       # Configurație principală
│   ├── media.yaml
│   ├── xp.yaml
│   └── ...
├── YnM/               # Module bot nucleu
├── YnMIrC/            # Protocol IRC
├── YnMPlugins/        # Plugin-uri
├── YnMModuls/         # Module
└── main.go            # Punct de intrare
```

---

## 🔧 Exemplu de Configurare

```yaml
# ════════════════════════════════════════
# Setări Bază IRC
# ════════════════════════════════════════

Server: "192.168.0.150"
Port: "6667"
TLSPort: "6697"
TLS: true

NickName: "YnM-Go"
UserName: "YnM"
RealName: "Markus Lajos"

# ════════════════════════════════════════
# Autentificare NickServ
# ════════════════════════════════════════

SASL: true
SASLUser: "YnM-Go"
SASLPass: "parola"

NickservBotnick: "NickServ"
NickservNick: "YnM-Go"
NickservPass: "parola"

Autologin: true
AutoJoinWithoutLogin: false

# ════════════════════════════════════════
# Canale & Sistem
# ════════════════════════════════════════

Console: "#YnM"
Channels:
  - "#Help"
  - "#Magyar"

LogDir: "./logs"
data_dir: "./data"
ReconOnDiscon: "60s"

# ════════════════════════════════════════
# Activare Plugin-uri (true/false)
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
# Integrare Jellyfin
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
# Programare
# ════════════════════════════════════════

NevnapReggel: "07:30"
JokeSendTime: "08:00"
SzekelyhonInterval: 120m

media_ajanlat:
  channel: "#Magyar"
  time: "21:35"

# ════════════════════════════════════════
# API-uri & Chei
# ════════════════════════════════════════

openai:
  api_key: "sk-..."

weather:
  weatherAPIKey: "..."
  defaultLocation: "Budapest"
```

---

## 📊 Comenzi Personalizate

| Comandă | Descriere |
|---------|-----------|
| `!help` | Afișează mesajul de ajutor |
| `!whoami` | Arată nivelul permisiuni utilizator |
| `!uptime` | Timp funcționare bot |
| `!status` | Status sistem |
| `!chatgpt <text>` | Răspuns AI |
| `!imdb <film>` | Informații film |
| `!yt <link>` | Info YouTube |
| `!tama` | Joc Tamagotchi |
| `!vicc` | Glumă zilei |
| `!ssh <gazdă> <port>` | Verificare port SSH |
| `!nmap <gazdă>` | Scanare rețea |
| `!ip <gazdă>` | Informații IP |
| `!dns <domeniu>` | Căutare DNS |
| `!ora` | Oră curentă |

---
## 🔗 Linkuri Rapide

| Resursa | Link |
|---------|------|
| 📋 **Forum Oficial** | https://forum.ynm.hu/c/ynm-go/13 |
| 💻 **Dezvoltare** | https://git.ynm.hu/Markus/YnM-Go |
| ⚙️ **Interfață Admin Web** | https://ynm-go.ynm.hu |
| 📊 **Monitorul de Disponibilitate Boti** | http://uptime.ynm.hu |
| 🔌 **Boti și Plugin-uri** | https://bot.ynm.hu |

---

### Navigare Rapidă

- **Abia începi?** → [Forum Oficial](https://forum.ynm.hu/c/ynm-go/13)
- **Vrei să contribui la cod?** → [Depozit Dezvoltare](https://git.ynm.hu/Markus/YnM-Go)
- **Vrei să gestionezi boti?** → [Panou de Control Admin](https://ynm-go.ynm.hu)
- **Vrei să verifici starea?** → [Stare Disponibilitate](http://uptime.ynm.hu)

---
## 📝 Informații Dezvoltator

**Dezvoltat de:** Markus (YnM.hu)  
📧 **Email:** [markus@ynm.hu](mailto:markus@ynm.hu)  
🌐 **Website:** https://ynm.hu  
📋 **Copyright:** 2012-2025 – Toate drepturile rezervate.
