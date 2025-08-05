package game

import (
	"time"

	"detektif-kata-bot/internal/db"
)

const (
	StatusLobby             = "lobby"
	StatusWaitingForClue    = "waiting_for_clue"
	StatusWaitingForGuesses = "waiting_for_guesses"
)

type GameState struct {
	ChatID                   int64
	Status                   string
	Host                     *db.Player
	Players                  map[int64]*db.Player
	SessionScores            map[int64]int
	TurnOrder                []*db.Player
	CurrentTurnIndex         int
	Round                    int
	TotalRounds              int
	LobbyMessageID           int
	IsActive                 bool
	SecretWord               string
	Clue                     string
	ClueMessageID            int
	ClueGiver                *db.Player
	Timer                    *time.Timer
	ClueGiverReminderTimer   *time.Timer
	GuessingTimeWarningTimer *time.Timer
	WrongGuesses             []string // TANDA: Field yang hilang ditambahkan di sini
	GuessingStartTime        time.Time
}

type SoloGameState struct {
	UserID      int64
	IsActive    bool
	CurrentWord WordData
	HintsGiven  int
}

func NewGame(chatID int64, host *db.Player, totalRounds int) *GameState {
	return &GameState{
		ChatID:           chatID,
		Status:           StatusLobby,
		Host:             host,
		Players:          make(map[int64]*db.Player),
		SessionScores:    make(map[int64]int),
		TurnOrder:        make([]*db.Player, 0),
		CurrentTurnIndex: 0,
		Round:            0,
		TotalRounds:      totalRounds, // TANDA: Menggunakan nilai dari parameter
		IsActive:         true,
		WrongGuesses:     make([]string, 0),
	}
}