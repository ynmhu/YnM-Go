# YnM Go Bot

A powerful, modular IRC bot written in Go with plugin system.

## 💪 Why YnM-Go? Comparison with Traditional IRC Bots

| Feature | Eggdrop | Sopel | **YnM-Go** |
|---------|:-------:|:-----:|:----------:|
| **Modern WebUI** | ❌ | ❌ | ✅ **PROFESSIONAL** |
| **REST API** | ❌ | ⚠️ Basic | ✅ **COMPLETE** |
| **Docker Support** | ⚠️ Hacky | ✅ | ✅ **NATIVE** |
| **Real-time Updates** | ❌ | ⚠️ | ✅ **INSTANT** |
| **Role-based Access** | ⚠️ TCL Scripts | ⚠️ Python Scripts | ✅ **BUILT-IN** |
| **Audit Logging** | ❌ | ❌ | ✅ **FULL TRACKING** |
| **Easy Installation** | ❌ Complex | ⚠️ Medium | ✅ **ONE COMMAND** |
| **Channel Management** | ⚠️ Commands only | ⚠️ Commands only | ✅ **WebUI + API** |
| **User Management** | ⚠️ Text files | ⚠️ Config files | ✅ **Database + UI** |
| **Performance** | ⚠️ Single-threaded | ⚠️ Python | ✅ **Go Concurrency** |
| **Active Development** | ⚠️ Legacy | ✅ | ✅ **Modern Stack** |
| **Security** | ⚠️ TCL risks | ⚠️ | ✅ **Enterprise-grade** |

### 🎯 Key Advantages

#### 🚀 **Modern Technology Stack**
- Written in **Go** for blazing-fast performance
- **SQLite** database for reliable data persistence
- **RESTful API** for seamless integrations
- **Responsive WebUI** for easy management

#### 🎨 **Professional Web Interface**
- Manage channels, users, and permissions from your browser
- Real-time status updates
- Intuitive dashboard with statistics
- Mobile-friendly responsive design


---

## 📚 Documentation Languages
## Permission System
For a detailed description of the bot's permissions, see: [PERMISSIONS](doc/PERMISSIONS.md)

Please select your preferred language:

### 🇬🇧 English
For English documentation, please refer to:
**[README-EN.md](doc/README-EN.md)**

### 🇭🇺 Hungarian (Magyar)
Magyar dokumentáció:
**[README-HU.md](doc/README-HU.md)**

### 🇷🇴 Romanian (Română)
Documentația română:
**[README-RO.md](doc/README-RO.md)**

---

## 🚀 Quick Start

### Option 1: Docker Run (Quick Test)

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

### Option 2: Docker Compose (Recommended)

Create a new folder and a `docker-compose.yaml` file:

```bash
mkdir ynm-go-bot
cd ynm-go-bot
```

Then create `docker-compose.yaml`:

```yaml
services:
  ynm-bot:
    image: ghcr.io/ynmhu/ynm-go:latest
    container_name: YnM-Go
    restart: unless-stopped
    ports:
      - "8585:2525"  # Host:8585 -> Container:2525 (API)
    volumes:
      - ./YnMConfig:/app/YnMConfig
      - ./data:/app/data
      - ./logs:/app/logs
    environment:
      - TZ=Europe/Budapest
```

Start the bot:

```bash
# First, test the configuration to see any errors
docker-compose up

# If everything looks good (no errors), stop it with Ctrl+C
# Then run it in the background
docker-compose up -d
```

Check logs:

```bash
docker-compose logs -f
```

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
| **Plugins** | `true/false*` options to enable/disable features |
| **Databases** | LogDir, data_dir, seen_db, SmsDBPath |
| **Scheduling** | Ping, Joke, Nevnap, topic_update_interval |

### Start the Bot

```bash
./YnM-Go
```

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

## ℹ️ About

**IRC BoT Go Lang YnM-Go:** Modular, Go-based IRC bot with plugins, automation, external service integrations, API access, webhooks, and a web-based interface.

### Keywords & Tags

IRC Bot • Go Language • YnM • Golang • Plugin System • YnM Bot • Chat Bot • YnM-Go •Automation • Jellyfin Integration • Media Management • ChatGPT • Discord Alternative • Self-hosted • Open Source • Real-time Monitoring • Network Tools • RSS Reader • Web API

### Features

✅ Plugin Architecture  
✅ Low Resource Usage  
✅ Self-hosted Capable  
✅ Jellyfin Integration  
✅ Media Management  
✅ AI Integration (ChatGPT)  
✅ Webhooks & API Access  
✅ Network Monitoring  
✅ User Management  
✅ Multi-language Support  

---
## 📝 Developer Information

**Developed by:** Markus (YnM.hu)  
📧 **Email:** [markus@ynm.hu](mailto:markus@ynm.hu)  
🌐 **Website:** https://ynm.hu  
📋 **Copyright:** 2012-2025 – All rights reserved.

