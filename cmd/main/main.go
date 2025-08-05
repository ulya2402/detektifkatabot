package main

import (
	"log"
	"os"

	"detektif-kata-bot/internal/bot"
	"detektif-kata-bot/internal/config"
	"detektif-kata-bot/internal/db"
	"detektif-kata-bot/internal/i18n"
)

func main() {
	log.Println("Starting bot...")

	cfg := config.Load()

	localizer := i18n.New(os.DirFS("locales"))

	dbClient := db.NewClient(cfg)

	b := bot.New(cfg, localizer, dbClient)
	b.Start()
}