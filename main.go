package main

import (
	"log"

	"github.com/ynmhu/YnM-Go/app"
	"github.com/ynmhu/YnM-Go/config"
)

func main() {
	// Konfiguráció betöltése
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatalf("Config betöltési hiba: %v", err)
	}

	// Alkalmazás létrehozása és indítása
	application := app.New(cfg)
	if err := application.Run(); err != nil {
		log.Fatalf("Alkalmazás hiba: %v", err)
	}
}