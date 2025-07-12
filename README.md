# YnM Go Bot

Egy erőteljes, moduláris IRC-bot Go nyelven, Sopel-szerű pluginrendszerrel. Kifejezetten önálló szerverre és saját rendszerekre lett tervezve, alacsony erőforrásigénnyel és maximális testreszabhatósággal.

## Főbb jellemzők

✅ Modularitás pluginokkal  
✅ Könnyen bővíthető új parancsokkal  
✅ Gyors, stabil Go-alapú IRC kapcsolat  
✅ Naplózás, adatbázis, JSON és statikus fájltámogatás  
✅ Beépített **médiaajánló és fájlfigyelő rendszer** Jellyfin integrációval

## Alapból elérhető funkciók

* 📊 **Resource monitor** – CPU, memória, load  
* 🧍 **Névnap értesítő** – napi névnapok  
* ☁️ **Időjárás** – aktuális állapot (API-val vagy `wttr.in`)  
* 🔍 **Google keresés** – `!google valami`  
* 📽️ **Film ajánló** – random, országos vagy népszerű film  
* 🎬 **Médiaajánló** – legfrissebb feltöltött film/sorozat ajánlása  
* ⬆️ **Médiafeltöltés figyelő** – Jellyfin adatbázisból automatikusan kiküldi az új tartalmakat  
* 📝 **Média kérés** – felhasználók által kért filmek nyilvántartása  
* 😂 **Vicc plugin** – napi poén vagy random  
* 📆 **Seen plugin** – utoljára látott idő IRC-n  
* 📡 **RSS olvasó** – hírek, saját feed-ekből  
* 💬 **Info / help** – használati utasítások  
* 💻 **Shell parancsok** – biztonságosan előre definiált parancsok  
* 🎮 **XP rendszer** – felhasználók aktivitásalapú szintlépése  
* 🔔 **Push értesítések** – szolgáltatások állapota (pl. Jellyfin down)  
* 🔧 **Szolgáltatásfigyelés** – portok, szolgáltatások uptime-ja

## Telepítés

```bash
git clone https://github.com/ynmhu/YnM-Go.git
cd YnM-Go
go mod tidy
go build -o YnM-Go
./YnM-Go
```

## Könyvtárszerkezet

```
YnM-Go/

├── config
│   ├── config.go
│   ├── config.yaml
│   └── example-config.yaml
├── data
│   ├── admins.json
│   ├── joke_status.json
│   ├── movies.db
│   ├── owners.json
│   ├── sent_dates.json
│   └── vips.json
├── go.mod
├── go.sum
├── irc
│   └── client.go
├── logs
├── main.go
├── plugins
│   ├── admin.go
│   ├── admin_store.go
│   ├── manager.go
│   ├── media_ajanlo.go
│   ├── media_del.go
│   ├── media_kell.go
│   ├── media_keresek.go
│   ├── media_ok.go
│   ├── media_upload.go
│   ├── models.go
│   ├── Napi_vicc.go
│   ├── nevnap.go
│   ├── ping.go
│   ├── status.go
│   ├── szekelyhon.go
│   ├── teszt.go
│   ├── utils.go
│   └── vicc.go
├── README.md
└── YnM-Go


```

## Konfiguráció

A `config/config.yaml` fájl tartalmazza az IRC-szerver, nicknév és csatornák beállításait:

```
# ─── SSL/TLS kapcsolat (opcionális) ─────────────────────────────────
TLS: true               # ha true, akkor TLS (SSL) kapcsolaton csatlakozik
TLSCert: "/home/bot/ssl.cert"   # kliens tanúsítvány (opcionális, ha a szerver igényli)
TLSKey: "/home/bot/ssl.key"    # kliens privát kulcs (opcionális)

# ─── SASL kapcsolat (opcionális) ─────────────────────────────────
SASL: true       # Kapcsold be a SASL-t
SASLUser: "YnM-Go"        # Ez a regisztrált nick
SASLPass: "11111"      # A jelszó (tárolás titkosítva javasolt)

# ─── Alap IRC ‑kapcsolat ─────────────────────────────────────────────
Server: "192.168.0.150"       # csak cím vagy domain név, port nélkül
Port: "6667"                  # sima TCP port
TLSPort: "6697"              # TLS/SSL port

NickName: "YnM-Go"                 # ideiglenes / végleges nick (NickServ védett)
UserName: "YnM"               # USER parancs adatai
RealName: "Markus Lajos"

# ─── Rendszer‑/­konzolcsatorna ───────────────────────────────────────
Console: "#YnM"        # kötelező! ide kerül minden belső log, hiba, státusz

# ─── Automatikus csatlakozás további szobákhoz ───────────────────────
Channels:
  - "#Help"
  - "#Magyar"
  

# ─── Naplók, reconnect, parancs‑cooldown ─────────────────────────────
LogDir: "./logs"              # helyi mappa a naplófájloknak
ReconOnDiscon: "60s" # automatikus újracsatlakozás 60 mp után


# ─── NickServ azonosítás és viselkedés ──────────────────────────────
NickservBotnick:    "NickServ"   # NickServ bot neve a hálózaton
NickservNick:          "YnM-Go"        # a regisztrált fiók nickje
NickservPass:          "1111"      # jelszó (tárold biztonságosan!)

Autologin: true          # ha false, nem próbál bejelentkezni NickServ-hez
AutoJoinWithoutLogin: false # ha true, akkor login nélkül is belép a channels listában lévő szobákba


#───────── NévNap Plugin Időzitök ──────────── 
NevnapReggel:    "07:30"
NevnapEste:         "21:30"
NevnapChannels:
  - "#Magyar"
  
#───────── Ping Plugin Időzitök ──────────── 
Ping: "30s"   # felhasználói !ping parancs várakozási ideje

#───────── Székelyhon Hírek Plugin ──────────── 
SzekelyhonChannels:
  - "#Magyar"
SzekelyhonInterval: 120m       # minden 30 percben
SzekelyhonStartHour: 7        # reggel 7-től
SzekelyhonEndHour: 22         # este 22-ig

#───────── Viccek Plugin ──────────── 
JokeChannels:
  - "#Magyar"

JokeSendTime: "08:00"   # Óra:perc formátumban, 24 órás


# Movie plugin configuration
jellyfin_db_path: "/var/lib/jellyfin/data/library.db"
movie_db_path: "./data/movies.db"
movie_requests_channel: "#Magyar"

# Optional: Movie plugin settings (with defaults)
movie_plugin:
  post_time: "20:00"
  post_chan: "#Magyar"
  post_nick: "ML"
  
# Media Ajanlo 
media_ajanlat:
  channel: "#Magyar"
  time: "21:35"
  
media_upload:
  enabled: true
  channels: ["#Magyar"]
  interval_minutes: 1
  jellyfin_db: "/var/lib/jellyfin/data/library.db"
  sent_dates_file: "./data/sent_dates.json"


```

---

Fejlesztette: **Markus (YnM.hu)**
📧 [markus@ynm.hu](mailto:markus@ynm.hu)
Szerzői jog: 2012-2025 – Minden jog fenntartva.
