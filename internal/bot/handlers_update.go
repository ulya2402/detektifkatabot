package bot

import (
	"log"
	"strings"

	"detektif-kata-bot/internal/config"
	"detektif-kata-bot/internal/db"
	"detektif-kata-bot/internal/game"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleUpdate(update tgbotapi.Update) {
	if update.Message == nil && update.CallbackQuery == nil {
		return
	}

	var from *tgbotapi.User
	var chat *tgbotapi.Chat
	var message *tgbotapi.Message

	if update.Message != nil {
		from = update.Message.From
		chat = update.Message.Chat
		message = update.Message
	} else if update.CallbackQuery != nil {
		from = update.CallbackQuery.From
		chat = update.CallbackQuery.Message.Chat
		message = update.CallbackQuery.Message
	}

	go func() {
		// TANDA: Logika perbaikan dimulai di sini
		chatTypeToSave := chat.Type
		if chat.IsSuperGroup() {
			chatTypeToSave = "group"
		}
		// TANDA: Logika perbaikan berakhir di sini

		err := b.db.GetOrCreateChat(chat.ID, chatTypeToSave) // TANDA: Menggunakan variabel yang sudah dinormalisasi
		if err != nil {
			log.Printf("Failed to save chat info for chat ID %d: %v", chat.ID, err)
		}
	}()

	isMember, err := b.checkUserIsMember(from)
	if err != nil {
		log.Printf("Error checking channel membership for %s: %v", from.UserName, err)
		return
	}
	if !isMember {
		lang := b.getUserLang(from)
		text := b.localizer.Get(lang, "must_join_channel")
		buttonText := b.localizer.Get(lang, "button_join_channel")
		keyboard := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonURL(buttonText, "https://t.me/"+strings.TrimPrefix(b.cfg.MustJoinChannel, "@"))))
		msg := tgbotapi.NewMessage(chat.ID, text)
		msg.ReplyMarkup = keyboard
		b.api.Send(msg)
		return
	}

	if update.CallbackQuery != nil {
		b.handleCallbackQuery(update.CallbackQuery)
		return
	}

	log.Printf("Received message: From=[%s] ChatID=[%d] Type=[%s] Text=[%s]", from.UserName, chat.ID, chat.Type, message.Text)
	user := &config.User{ID: from.ID, FirstName: from.FirstName, Username: from.UserName}
	player, err := b.db.GetOrCreatePlayer(user)
	if err != nil {
		log.Printf("Could not process player: %v", err)
		return
	}

	if message.IsCommand() {
		b.handleCommand(message, player)
	} else if chat.IsPrivate() {
		b.handlePrivateMessage(message, player)
	} else if chat.IsGroup() || chat.IsSuperGroup() {
		b.handleGroupMessage(message, player)
	}
}

func (b *Bot) handlePrivateMessage(message *tgbotapi.Message, player *db.Player) {
	lang := b.getUserLang(message.From)

	b.mu.RLock()
	gameStatesCopy := make(map[int64]*game.GameState)
	for k, v := range b.gameStates {
		gameStatesCopy[k] = v
	}
	soloState, soloOk := b.soloGameStates[player.TelegramUserID]
	b.mu.RUnlock()

	for chatID, state := range gameStatesCopy {
		if state.IsActive && state.Status == "waiting_for_clue" && state.ClueGiver != nil && state.ClueGiver.TelegramUserID == player.TelegramUserID {
			b.handleClueSubmission(message, player, state, chatID, lang)
			return
		}
	}

	if soloOk && soloState.IsActive {
		b.handleSoloGuess(message, player, soloState, lang)
		return
	}
}

func (b *Bot) checkUserIsMember(user *tgbotapi.User) (bool, error) {
	if b.cfg.MustJoinChannel == "" {
		return true, nil
	}
	config := tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			SuperGroupUsername: b.cfg.MustJoinChannel,
			UserID:             user.ID,
		},
	}
	member, err := b.api.GetChatMember(config)
	if err != nil {
		if strings.Contains(err.Error(), "chat not found") {
			log.Printf("Warning: Must-join channel '%s' not found or bot is not an admin.", b.cfg.MustJoinChannel)
			return false, nil
		}
		return false, err
	}
	switch member.Status {
	case "creator", "administrator", "member":
		return true, nil
	default:
		return false, nil
	}
}

func (b *Bot) sendMessage(chatID int64, text string, useHTML bool) error {
	msg := tgbotapi.NewMessage(chatID, text)
	if useHTML {
		msg.ParseMode = tgbotapi.ModeHTML
	}
	_, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Failed to send message to chat %d: %v", chatID, err)
	}
	return err
}

func (b *Bot) sendMessageAndGet(chatID int64, text string, useHTML bool) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	if useHTML {
		msg.ParseMode = tgbotapi.ModeHTML
	}
	return b.api.Send(msg)
}

func (b *Bot) getUserLang(user *tgbotapi.User) string {
	if user.LanguageCode == "id" {
		return "id"
	}
	return "en"
}