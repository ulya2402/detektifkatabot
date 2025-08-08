// TANDA: File baru internal/db/stats.go

package db

import (
	"log"
	"strconv"
)

// IncrementPlayerStats menambah nilai statistik pemain.
// Contoh: field = "games_played", value = 1
func (c *Client) IncrementPlayerStats(playerID int64, field string, value int) error {
	var results []Player
	err := c.DB.From("players").Select(field).Eq("telegram_user_id", strconv.FormatInt(playerID, 10)).Execute(&results)
	if err != nil || len(results) == 0 {
		log.Printf("Error fetching player for stats update: %v", err)
		return err
	}

	var currentVal int
	switch field {
	case "games_played":
		currentVal = results[0].GamesPlayed
	case "games_won":
		currentVal = results[0].GamesWon
	case "clue_given_count":
		currentVal = results[0].ClueGivenCount
	case "clue_success_count":
		currentVal = results[0].ClueSuccessCount
	case "words_guessed_count":
		currentVal = results[0].WordsGuessedCount
	}
	
	newVal := currentVal + value
	err = c.DB.From("players").Update(map[string]interface{}{field: newVal}).Eq("telegram_user_id", strconv.FormatInt(playerID, 10)).Execute(nil)
	if err != nil {
		log.Printf("Error updating stats for player %d: %v", playerID, err)
	}
	return err
}

// UpdatePlayerFastestGuess memperbarui rekor tebakan tercepat pemain.
func (c *Client) UpdatePlayerFastestGuess(playerID int64, newTime float64) error {
	var results []Player
	err := c.DB.From("players").Select("fastest_guess").Eq("telegram_user_id", strconv.FormatInt(playerID, 10)).Execute(&results)
	if err != nil || len(results) == 0 {
		return err
	}
	
	// Perbarui hanya jika rekor baru lebih cepat, atau jika belum ada rekor (-1)
	if results[0].FastestGuess == -1 || newTime < results[0].FastestGuess {
		err = c.DB.From("players").Update(map[string]interface{}{"fastest_guess": newTime}).Eq("telegram_user_id", strconv.FormatInt(playerID, 10)).Execute(nil)
		if err != nil {
			log.Printf("Error updating fastest guess for player %d: %v", playerID, err)
		}
	}
	return err
}

// SetEquippedBadge menetapkan lencana yang dipakai oleh pemain.
func (c *Client) SetEquippedBadge(playerID int64, badgeID int) error {
	err := c.DB.From("players").Update(map[string]interface{}{"equipped_badge_id": badgeID}).Eq("telegram_user_id", strconv.FormatInt(playerID, 10)).Execute(nil)
	if err != nil {
		log.Printf("Error setting equipped badge for player %d: %v", playerID, err)
	}
	return err
}

// GetPlayerByID mengambil data lengkap pemain, termasuk statistik baru.
func (c *Client) GetPlayerByID(playerID int64) (*Player, error) {
	var results []Player
	err := c.DB.From("players").Select("*").Eq("telegram_user_id", strconv.FormatInt(playerID, 10)).Execute(&results)
	if err != nil || len(results) == 0 {
		return nil, err
	}
	return &results[0], nil
}