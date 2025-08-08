package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken string
	SupabaseURL      string
	SupabaseKey      string
	MustJoinChannel  string 
	SuperAdminID     int64
	StartImageURL    string
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

	adminIDStr := getEnv("SUPER_ADMIN_ID", true)
	adminID, err := strconv.ParseInt(adminIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid SUPER_ADMIN_ID: %s. Must be a number.", adminIDStr)
	}

	return &Config{
		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", true),
		SupabaseURL:      getEnv("SUPABASE_URL", true),
		SupabaseKey:      getEnv("SUPABASE_KEY", true),
		MustJoinChannel:  getEnv("MUST_JOIN_CHANNEL", false), 
		SuperAdminID:     adminID,
		StartImageURL:    getEnv("START_IMAGE_URL", false),
	}
}

func getEnv(key string, required bool) string {
	val := os.Getenv(key)
	if required && val == "" {
		log.Fatalf("%s is not set", key)
	}
	return val
}