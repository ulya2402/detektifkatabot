package db

import (
	"time"
)

// TANDA: Nama struct diubah dari 'Player' menjadi 'Player' (kapital)
type Player struct {
	ID             string    `json:"id,omitempty"`
	TelegramUserID int64     `json:"telegram_user_id"`
	FirstName      string    `json:"first_name"`
	Username       string    `json:"username,omitempty"`
	Points         int       `json:"points,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
}

type Badge struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Emoji         string `json:"emoji"`
	Type          string `json:"type"`
	CriteriaValue int    `json:"criteria_value"`
	CriteriaType  string `json:"criteria_type"`
}

// TANDA: Struct PlayerBadge ditambahkan
type PlayerBadge struct {
	PlayerID int64 `json:"player_id"`
	BadgeID  int   `json:"badge_id"`
}