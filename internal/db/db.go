package db

import (
	"log"
	"strconv" 
	"sort"

	"detektif-kata-bot/internal/config"

	"github.com/nedpals/supabase-go"
)

type Client struct {
	*supabase.Client
}

func NewClient(cfg *config.Config) *Client {
	sbClient := supabase.CreateClient(cfg.SupabaseURL, cfg.SupabaseKey)
	log.Println("Successfully connected to Supabase.")
	return &Client{sbClient}
}

func (c *Client) GetOrCreatePlayer(tgUser *config.User) (*Player, error) {
	var results []Player
	
	// TANDA: Baris ini telah diperbaiki
	err := c.DB.From("players").Select("*").Eq("telegram_user_id", strconv.FormatInt(tgUser.ID, 10)).Execute(&results)
	
	if err != nil {
		log.Printf("Error checking for player: %v", err)
		return nil, err
	}

	if len(results) > 0 {
		log.Printf("Found existing player: %s (ID: %d)", results[0].FirstName, results[0].TelegramUserID)
		return &results[0], nil
	}

	log.Printf("Player not found. Creating new player: %s (ID: %d)", tgUser.FirstName, tgUser.ID)
	newPlayer := Player{
		TelegramUserID: tgUser.ID,
		FirstName:      tgUser.FirstName,
		Username:       tgUser.Username,
	}

	var newResults []Player
	err = c.DB.From("players").Insert(newPlayer).Execute(&newResults)
	if err != nil {
		log.Printf("Error creating new player: %v", err)
		return nil, err
	}

	if len(newResults) == 0 {
		log.Println("Failed to create player, no data returned.")
		return nil, err
	}

	log.Printf("Successfully created new player: %s (ID: %d)", newResults[0].FirstName, newResults[0].TelegramUserID)
	return &newResults[0], nil
}

func (c *Client) AddPoints(playerID int64, pointsToAdd int) error {
	var results []Player
	err := c.DB.From("players").Select("points").Eq("telegram_user_id", strconv.FormatInt(playerID, 10)).Execute(&results)
	if err != nil || len(results) == 0 {
		log.Printf("Error fetching player for points update: %v", err)
		return err
	}

	currentPoints := results[0].Points
	newPoints := currentPoints + pointsToAdd

	var updateResults []interface{}
	err = c.DB.From("players").Update(map[string]interface{}{"points": newPoints}).Eq("telegram_user_id", strconv.FormatInt(playerID, 10)).Execute(&updateResults)
	if err != nil {
		log.Printf("Error updating points for player %d: %v", playerID, err)
		return err
	}

	log.Printf("Player %d awarded %d points. New total: %d", playerID, pointsToAdd, newPoints)
	return nil
}

func (c *Client) GetTopPlayers(limit int) ([]Player, error) {
	var results []Player
	// 1. Ambil semua pemain yang memiliki poin lebih dari 0
	err := c.DB.From("players").Select("*").Gt("points", "0").Execute(&results)
	if err != nil {
		log.Printf("Error fetching players for leaderboard: %v", err)
		return nil, err
	}

	// 2. Urutkan pemain berdasarkan poin (tertinggi ke terendah) di dalam Go
	sort.Slice(results, func(i, j int) bool {
		return results[i].Points > results[j].Points
	})

	// 3. Batasi jumlah pemain sesuai limit setelah diurutkan
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}