package bot

import (
	"log"

	"detektif-kata-bot/internal/config"
	"detektif-kata-bot/internal/db"
	"detektif-kata-bot/internal/game"
	"detektif-kata-bot/internal/i18n"
	

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api         *tgbotapi.BotAPI
	cfg         *config.Config
	localizer   *i18n.Localizer
	db          *db.Client
	gameStates  map[int64]*game.GameState
	soloGameStates map[int64]*game.SoloGameState
	botUsername string // TANDA: Baris ini ditambahkan
}

func New(cfg *config.Config, localizer *i18n.Localizer, dbClient *db.Client) *Bot {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	log.Printf("Authorized on account %s", api.Self.UserName)

	return &Bot{
		api:         api,
		cfg:         cfg,
		localizer:   localizer,
		db:          dbClient,
		gameStates:  make(map[int64]*game.GameState),
		soloGameStates: make(map[int64]*game.SoloGameState),
		botUsername: api.Self.UserName, // TANDA: Baris ini ditambahkan
	}
}

func (b *Bot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		b.handleUpdate(update)
	}
}