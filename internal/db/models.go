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