// ==================================================
//  Szerzői jog © 2025 Markus (markus@ynm.hu)
//  https://ynm.hu   – főoldal
//  https://forum.ynm.hu   – hivatalos fórum
//  https://bot.ynm.hu     – bot oldala és dokumentáció
//
//  Minden jog fenntartva. A kód Markus tulajdona, tilos terjeszteni vagy
//  módosítani a szerző írásos engedélye nélkül.
//
//  Ez a fájl a YnM-Go IRC-bot rendszerének része.
// ==================================================
package main

import (
	"log"
//	"net/http"
	"git.ynm.hu/markus/YnM-Go/YnM"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	_ "git.ynm.hu/markus/YnM-Go/YnMLang"
)

func main() {
	configPath := "YnMConfig/ynm.yaml"
	
	// Konfiguráció betöltése
	cfg, err := YnMConfig.Load(configPath)
	if err != nil {
		log.Fatalf("Config betöltési hiba: %v", err)
	}
	
	// ✨ MONITORING PLUGIN INICIALIZÁLÁS ✨
	monitor := owner.NewBotMonitor(configPath)
	monitor.StartBackground()

//	go func() {
//    	http.HandleFunc("/tools", YnM.HandleGetTools)
 //   	http.HandleFunc("/tools/get_data", YnM.HandleToolCall)
  //  	log.Println("HTTP Tools server running on :5556")
  //  	log.Fatal(http.ListenAndServe(":5556", nil))
//	}()



	
	// Alkalmazás létrehozása
	application := YnM.New(cfg)
	
	// Graceful shutdown a monitor számára
	defer monitor.Shutdown()
	
	// Alkalmazás indítása (IRC és többi, beleértve a Discord plugint is)
	if err := application.Run(); err != nil {
		log.Fatalf("Alkalmazás hiba: %v", err)
	}
}
