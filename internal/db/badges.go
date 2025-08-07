package db

import (
	"fmt"
	"log"
	"strconv"
)

// GetPlayerBadges mengambil semua lencana yang dimiliki oleh seorang pemain.
func (c *Client) GetPlayerBadges(playerID int64) ([]Badge, error) {
	var playerBadges []PlayerBadge
	err := c.DB.From("player_badges").Select("badge_id").Eq("player_id", strconv.FormatInt(playerID, 10)).Execute(&playerBadges)
	if err != nil {
		log.Printf("Error fetching player badges for player %d: %v", playerID, err)
		return nil, err
	}

	if len(playerBadges) == 0 {
		return []Badge{}, nil
	}

	var badgeIDs []string
	for _, pb := range playerBadges {
		badgeIDs = append(badgeIDs, strconv.Itoa(pb.BadgeID))
	}

	var badges []Badge
	// Menggunakan format "in" untuk mengambil semua lencana berdasarkan daftar ID
	filter := fmt.Sprintf("(%s)", stringSliceToCommaSeparated(badgeIDs))
	err = c.DB.From("badges").Select("*").Filter("id", "in", filter).Execute(&badges)
	if err != nil {
		log.Printf("Error fetching badge details for player %d: %v", playerID, err)
		return nil, err
	}

	return badges, nil
}

// AwardBadgeToPlayer memberikan sebuah lencana kepada pemain.
func (c *Client) AwardBadgeToPlayer(playerID int64, badgeID int) error {
	newPlayerBadge := PlayerBadge{
		PlayerID: playerID,
		BadgeID:  badgeID,
	}

	var results []PlayerBadge
	err := c.DB.From("player_badges").Insert(newPlayerBadge).Execute(&results)
	if err != nil {
		// Mungkin lencana sudah dimiliki, ini bukan error fatal.
		log.Printf("Could not award badge %d to player %d (maybe already owned): %v", badgeID, playerID, err)
		return err
	}
	log.Printf("Awarded badge %d to player %d successfully.", badgeID, playerID)
	return nil
}

// GetAllBadges retrieves all available badges from the catalog.
func (c *Client) GetAllBadges() ([]Badge, error) {
	var badges []Badge
	err := c.DB.From("badges").Select("*").Execute(&badges)
	if err != nil {
		log.Printf("Error fetching all badges: %v", err)
		return nil, err
	}
	return badges, nil
}

// helper function to convert slice of strings to a single comma-separated string
func stringSliceToCommaSeparated(slice []string) string {
	var result string
	for i, s := range slice {
		if i > 0 {
			result += ","
		}
		result += s
	}
	return result
}

func (c *Client) GetAchievementBadges() ([]Badge, error) {
	var badges []Badge
	err := c.DB.From("badges").Select("*").Eq("type", "achievement").Execute(&badges)
	if err != nil {
		log.Printf("Error fetching achievement badges: %v", err)
		return nil, err
	}
	return badges, nil
}

func (c *Client) GetPurchasableBadges() ([]Badge, error) {
	var badges []Badge
	err := c.DB.From("badges").Select("*").Eq("type", "purchasable").Execute(&badges)
	if err != nil {
		log.Printf("Error fetching purchasable badges: %v", err)
		return nil, err
	}
	return badges, nil
}

// GetBadgeByID mengambil detail satu lencana berdasarkan ID-nya.
func (c *Client) GetBadgeByID(badgeID int) (*Badge, error) {
	var results []Badge
	// TANDA: .Single() dihapus dari sini
	err := c.DB.From("badges").Select("*").Eq("id", strconv.Itoa(badgeID)).Execute(&results)
	if err != nil {
		log.Printf("Error fetching badge by ID %d: %v", badgeID, err)
		return nil, err
	}

	// TANDA: Logika pengecekan ditambahkan setelah query
	if len(results) == 0 {
		return nil, fmt.Errorf("badge with ID %d not found", badgeID)
	}

	// Karena ID unik, kita ambil elemen pertama
	return &results[0], nil
}