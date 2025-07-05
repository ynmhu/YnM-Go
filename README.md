# YnM Go Bot

Egy erőteljes, moduláris IRC-bot Go nyelven, Sopel-szerű pluginrendszerrel. Kifejezetten önálló szerverre és saját rendszerekre lett tervezve, alacsony erőforrásigénnyel és maximális testreszabhatósággal.

## Főbb jellemzők

✅ Modularitás pluginokkal
✅ Könnyen bővíthető új parancsokkal
✅ Gyors, stabil Go-alapú IRC kapcsolat
✅ Naplózás, adatbázis, JSON és statikus fájltámogatás

## Alapból elérhető funkciók

* 📊 **Resource monitor** – CPU, memória, load
* 🧍 **Névnap értesítő** – napi névnapok
* ☁️ **Időjárás** – aktuális állapot (API-val vagy `wttr.in`)
* 🔍 **Google keresés** – `!google valami`
* 📽️ **Film ajánló** – random, országos vagy népszerű film
* 😂 **Vicc plugin** – napi poén vagy random
* 🎬 **Film kérés** – felhasználói filmkérés kezelése
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
│   └── config.yaml
├── go.mod
├── go.sum
├── irc
│   └── client.go
├── logs
│   ├── #YnM_2025-07-05.log
│   └── #YnM_2025-07-06.log
├── main.go
├── plugins
│   ├── manager.go
│   ├── nevnap.go
│   └── ping.go
└── YnM-Go

```

## Konfiguráció

A `config/config.yaml` fájl tartalmazza az IRC-szerver, nicknév és csatornák beállításait:

```
server: "192.168.0.150:6667"
nick: "YnM-Go"
user: "gobot 0 * :Go IRC Bot"
channels:
  - "#YnM"
log_dir: "./logs"

```

## Plugin hozzáadása

1. Hozz létre egy új fájlt a `plugins/` mappában (pl. `jokes.go`)
2. Írj egy `Plugin` implementációt, és regisztráld `init()`-ben
3. Parancsnevet adj a `Commands()` metódusban (`[]string{"vicc"}`)
4. A `Handle()` függvényben válaszold meg a hívást

## Feltöltés GitHubra

```bash
git add .
git commit -m "Új plugin: vicc"
git push origin main
```

---

Fejlesztette: **Markus (YnM.hu)**
📧 [markus@ynm.hu](mailto:markus@ynm.hu)
Szerzői jog: 2024 – Minden jog fenntartva.
