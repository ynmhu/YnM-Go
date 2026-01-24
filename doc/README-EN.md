# YnM Go Bot

A powerful, modular IRC bot written in Go with a plugin system. Designed for self-hosted environments, low resource usage, and maximum customization.

---

## 🌍 Support & Official Links

### 🇭🇺 Hungarian
- **Forum:** https://forum.ynm.hu/c/ynm-go/13
- **Development:** https://git.ynm.hu/Markus/YnM-Go

### 🇷🇴 Romanian
- **Forum:** https://forum.ynm.hu/c/ynm-go/13
- **Development:** https://git.ynm.hu/Markus/YnM-Go

### 🇬🇧 English
- **Forum:** https://forum.ynm.hu/c/ynm-go/13
- **Development:** https://git.ynm.hu/Markus/YnM-Go

> **Note:** The project is now maintained by Markus. Support is provided exclusively on the official forum.

---

## 📦 Docker Deployment

### System Requirements
- **API port:** 2525 (default)
- **Port mapping:** `8585:2525` (host:container)

### Docker Images

```bash
# GitHub Container Registry
ghcr.io/ynmhu/ynm-go:latest

# YnM Git Registry
git.ynm.hu/markus/ynm-go:latest
```

> **Tip:** For production, use version tags (e.g., `:2026-01-07`). For testing, `:latest` is fine.

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

## ⚙️ Installation & Build

### Clone and Build

```bash
# Clone the repository
git clone https://git.ynm.hu/Markus/YnM-Go.git
cd YnM-Go

# Download dependencies
go mod tidy

# Build the bot
go build -o YnM-Go
```

### Configuration

```bash
# Navigate to config directory
cd YnMConfig

# Copy default configuration
cp example-ynm.yaml ynm.yaml

# Edit configuration
nano ynm.yaml
```

#### Important Configuration Points

| Section | Description |
|---------|-------------|
| **IRC Connection** | Server, Port, NickName, UserName, RealName |
| **NickServ** | NickservBotnick, NickservNick, NickservPass |
| **SASL/TLS** | SASL, SASLUser, SASLPass, TLS, TLSCert, TLSKey |
| **Console** | Console (log channel), ConsoleKey (password) |
| **Plugins** | `true/false` options to enable/disable features |
| **Databases** | LogDir, data_dir, seen_db, SmsDBPath |
| **Scheduling** | Ping, Joke, Nevnap, topic_update_interval |

### Start the Bot

```bash
./YnM-Go
```

---

## 🎯 Available Plugins

 ## IRC Bot – Active Plugins List

* 🛡️ Admin Commands – permission management, automatic VOICE/OP
* 📺 Media Recommendations – shows latest uploaded movies/series
* ⏫ Media Upload Monitor – detects new Jellyfin uploads via webhook
* ✏️ Media Requests – users can request movies/series (“kell” and “keresek”)
* ✅ Upload Confirmation – marks requests as completed
* 🗑️ Request Deletion – admin command to delete a media request
* 🔎 Media Info – search and detailed info on media content
* 🤖 Tamagotchi Game – raise a digital pet via IRC (!tama)
* ❌⭕ Tic-Tac-Toe Game – interactive IRC-based XO implementation
* 🤣 Daily Joke – fetches a daily or random joke
* 👀 Seen Plugin – tracks when users were last seen on IRC
* 📰 RSS Reader – monitors custom feeds like HunTorrent
* 🔍 IMDB Lookup – fetches info about movies/series by title (!imdb)
* 🎬 TMDb Search – movie/series/actor info via TMDb API (in development)
* 🍿 Random Movie Recommender – picks movies by popularity or region
* ✉️ Mail Reader – access mailbox through IRC (!mail)
* ⛅ Weather Info – fetches weather via OpenWeatherMap or wttr.in
* 🎂 Nameday Notifier – posts today’s namedays every morning
* ⌨️ Shell Commands – safe predefined commands (!ssh, !nmap, !dns, !ip)
* 🖥️ Resource Monitor – CPU, memory, load tracking, saved to SQLite
* 🔴 Push Notifications – alerts for service events (e.g., Jellyfin down)
* 🛠️ Service Uptime Checker – port monitoring and online status reports
* 📡 Ping Plugin – simple !ping and host reachability check
* 🧠 ChatGPT Integration – smart AI replies in IRC chat
* ⚡ XP System – user level-up system based on activity
* 📅 Time Plugin – returns current time (!ora)
* 🧠 Debug Plugin – used for test and debug logging
* 📦 Learn Plugin – allows learning custom replies via !learn
* 🌍 DNS / IP Info – domain or IP-based info lookup (!dns, !ip)
* 🦠 BruteForce Monitor – monitors `/var/log/auth.log` for brute force attacks
* 🔁 IRC Relay – mirrors messages between two IRC networks
* 📬 Info/Help – responds to !help and !info with usage instructions
* 📹 YouTube Info – extracts title, duration, likes from YouTube links
* 🔐 WebAuth – enables web authentication for IRC commands
* 🔄 Autotopic – automatic topic updates on the console channel
* 🛎️ Service Manager – service management commands (!service)
* 🛡️ Fail2Ban integration – automatic IP banning based on log files
* 🔁 Cycle plugin – channel cycling behavior when no @ is present on the nick
* 📊 Media activity tracking – monitoring usage statistics of media files
* 🗓️ Media upload date tracking – tracking based on sent_dates.json
* 🔑 ConsoleKey – console channel protected with a password for commands
* ✉️ SMTP sending – email sending / SMTP integration (mail plugin)
* 🗂️ Git API interaction – fetching commit and repository information via IRC
* 🌐 Undernet / X-service integration – connecting to other IRC networks and X-service management
* 🔐 SASL authentication – IRC SASL login support
* 🧪 Experimental (X) plugin – enables experimental / in-development features

---

## 📁 Directory Structure

```
YnM-Go/
├── data/              # Databases and data
│   ├── admins.json
│   ├── movies.db
│   ├── xp.db
│   ├── seen.db
│   ├── sent_dates.json
│   └── ...
├── logs/              # Log files
├── YnMConfig/         # Configuration files
│   ├── ynm.yaml       # Main configuration
│   ├── media.yaml
│   ├── xp.yaml
│   └── ...
├── YnM/               # Core bot modules
├── YnMIrC/            # IRC protocol
├── YnMPlugins/        # Plugins
├── YnMModuls/         # Modules
└── main.go            # Entry point
```

---

## 🔧 Configuration Example

```yaml
# ════════════════════════════════════════
# IRC Basic Settings
# ════════════════════════════════════════

Server: "192.168.0.150"
Port: "6667"
TLSPort: "6697"
TLS: true

NickName: "YnM-Go"
UserName: "YnM"
RealName: "Markus Lajos"

# ════════════════════════════════════════
# NickServ Login
# ════════════════════════════════════════

SASL: true
SASLUser: "YnM-Go"
SASLPass: "password"

NickservBotnick: "NickServ"
NickservNick: "YnM-Go"
NickservPass: "password"

Autologin: true
AutoJoinWithoutLogin: false

# ════════════════════════════════════════
# Channels & System
# ════════════════════════════════════════

Console: "#YnM"
Channels:
  - "#Help"
  - "#Magyar"

LogDir: "./logs"
data_dir: "./data"
ReconOnDiscon: "60s"

# ════════════════════════════════════════
# Plugin Activation (true/false)
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
# Jellyfin Integration
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
# Scheduling
# ════════════════════════════════════════

NevnapReggel: "07:30"
JokeSendTime: "08:00"
SzekelyhonInterval: 120m

media_ajanlat:
  channel: "#Magyar"
  time: "21:35"

# ════════════════════════════════════════
# APIs & Keys
# ════════════════════════════════════════

openai:
  api_key: "sk-..."

weather:
  weatherAPIKey: "..."
  defaultLocation: "Budapest"
```

---

## 📊 Custom Commands

| Command | Description |
|---------|-------------|
| `!help` | Display help message |
| `!whoami` | Show user permission level |
| `!uptime` | Bot uptime |
| `!status` | System status |
| `!chatgpt <text>` | AI response |
| `!imdb <movie>` | Movie information |
| `!yt <link>` | YouTube info |
| `!tama` | Tamagotchi game |
| `!vicc` | Daily joke |
| `!ssh <host> <port>` | SSH port check |
| `!nmap <host>` | Network scan |
| `!ip <host>` | IP information |
| `!dns <domain>` | DNS lookup |
| `!ora` | Current time |

---
## 🔗 Quick Links

| Resource | Link |
|----------|------|
| 📋 **Official Forum** | https://forum.ynm.hu/c/ynm-go/13 |
| 💻 **Development** | https://git.ynm.hu/Markus/YnM-Go |
| ⚙️ **Admin Web UI** | https://ynm-go.ynm.hu |
| 📊 **Bot Uptime Monitor** | http://uptime.ynm.hu |
| 🔌 **Bots & Plugins** | https://bot.ynm.hu |

---

### Quick Navigation

- **Getting Started?** → [Official Forum](https://forum.ynm.hu/c/ynm-go/13)
- **Contributing Code?** → [Development Repository](https://git.ynm.hu/Markus/YnM-Go)
- **Managing Bots?** → [Admin Dashboard](https://ynm-go.ynm.hu)
- **Check Status?** → [Uptime Status](http://uptime.ynm.hu)

---
## 📝 Developer Information

**Developed by:** Markus (YnM.hu)  
📧 **Email:** [markus@ynm.hu](mailto:markus@ynm.hu)  
🌐 **Website:** https://ynm.hu  
📋 **Copyright:** 2012-2025 – All rights reserved.
