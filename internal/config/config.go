package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken string
	SupabaseURL      string
	SupabaseKey      string
	MustJoinChannel  string // TANDA: Baris ini ditambahkan
}

type User struct {
	ID        int64
	FirstName string
	Username  string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment variables")
	}

	return &Config{
		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", true),
		SupabaseURL:      getEnv("SUPABASE_URL", true),
		SupabaseKey:      getEnv("SUPABASE_KEY", true),
		MustJoinChannel:  getEnv("MUST_JOIN_CHANNEL", false), // TANDA: Baris ini ditambahkan
	}
}

func getEnv(key string, required bool) string {
	val := os.Getenv(key)
	if required && val == "" {
		log.Fatalf("%s is not set", key)
	}
	return val
}