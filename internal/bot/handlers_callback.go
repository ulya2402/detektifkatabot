package bot

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"detektif-kata-bot/internal/config"
	"detektif-kata-bot/internal/game"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	chatID := query.Message.Chat.ID
	lang := b.getUserLang(query.From)
	user := &config.User{ID: query.From.ID, FirstName: query.From.FirstName, Username: query.From.UserName}
	player, _ := b.db.GetOrCreatePlayer(user)

	messageID := query.Message.MessageID
	data := query.Data

	if strings.HasPrefix(query.Data, "join_game") {
		b.mu.Lock()
		defer b.mu.Unlock()

		state, ok := b.gameStates[chatID]
		if !ok || !state.IsActive || state.Status != game.StatusLobby {
			b.answerCallback(query.ID, "Lobi sudah ditutup.", true)
			return
		}

		if _, joined := state.Players[player.TelegramUserID]; joined {
			b.answerCallback(query.ID, b.localizer.Get(lang, "callback_already_joined"), true)
			return
		}

		state.Players[player.TelegramUserID] = player
		b.answerCallback(query.ID, b.localizer.Get(lang, "callback_join_success"), false)

		go b.updateLobbyMessage(chatID)
		return
	}
	
	if strings.HasPrefix(data, "help_") {
		var text string
		var keyboard tgbotapi.InlineKeyboardMarkup

		switch data {
		case "help_how_to_play":
			text = b.localizer.Get(lang, "help_text_how_to_play")
			keyboard = b.createHelpBackButton(lang)
		case "help_commands":
			text = b.localizer.Get(lang, "help_text_commands")
			keyboard = b.createHelpBackButton(lang)
		case "help_scoring":
			text = b.localizer.Get(lang, "help_text_scoring")
			keyboard = b.createHelpBackButton(lang)
		case "help_back":
			text = b.localizer.Get(lang, "help_main_title")
			keyboard = b.createHelpKeyboard(lang)
		}

		// Edit pesan yang ada untuk menampilkan konten baru
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, text)
		editMsg.ParseMode = tgbotapi.ModeHTML
		editMsg.ReplyMarkup = &keyboard
		b.api.Request(editMsg)

		// Jawab callback agar tombol tidak loading terus
		b.answerCallback(query.ID, "", false)
	}

}

func (b *Bot) createHelpBackButton(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(b.localizer.Get(lang, "help_button_back"), "help_back"),
		),
	)
}

func (b *Bot) updateLobbyMessage(chatID int64) (tgbotapi.Message, error) {
	b.mu.RLock()
	state := b.gameStates[chatID]
	b.mu.RUnlock()
	
	lang := "id"

	var playerList strings.Builder
	if len(state.Players) == 0 {
		playerList.WriteString(b.localizer.Get(lang, "lobby_no_players"))
	} else {
		i := 1
		for _, p := range state.Players {
			playerList.WriteString(fmt.Sprintf("%d. %s\n", i, html.EscapeString(p.FirstName)))
			i++
		}
	}

	playersJoinedText := b.localizer.Get(lang, "lobby_players_joined")
	playersJoinedText = strings.Replace(playersJoinedText, "{player_count}", strconv.Itoa(len(state.Players)), 1)
	playersJoinedText = strings.Replace(playersJoinedText, "{max_players}", strconv.Itoa(state.TotalRounds), 1)
	playersJoinedText = strings.Replace(playersJoinedText, "{player_list}", playerList.String(), 1)

	hostText := b.localizer.Get(lang, "lobby_host")
	hostText = strings.Replace(hostText, "{host_name}", html.EscapeString(state.Host.FirstName), 1)

	joinPromptText := b.localizer.Get(lang, "lobby_join_prompt")
	joinPromptText = strings.Replace(joinPromptText, "{total_rounds}", strconv.Itoa(state.TotalRounds), 1)

	playInstructionText := b.localizer.Get(lang, "lobby_play_instruction")
	playInstructionText = strings.Replace(playInstructionText, "{host_name}", html.EscapeString(state.Host.FirstName), 1)

	fullText := fmt.Sprintf("%s\n%s\n\n%s\n\n%s\n%s",
		b.localizer.Get(lang, "lobby_opened"),
		hostText,
		joinPromptText,
		playersJoinedText,
		playInstructionText,
	)

	button := tgbotapi.NewInlineKeyboardButtonData(b.localizer.Get(lang, "button_join_game"), "join_game")
	keyboard := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(button))

	if state.LobbyMessageID == 0 {
		msg := tgbotapi.NewMessage(chatID, fullText)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		return b.api.Send(msg)
	} else {
		msg := tgbotapi.NewEditMessageText(chatID, state.LobbyMessageID, fullText)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = &keyboard
		_, err := b.api.Request(msg)
		return tgbotapi.Message{}, err
	}
}

func (b *Bot) answerCallback(queryID string, text string, showAlert bool) {
	callback := tgbotapi.NewCallback(queryID, text)
	callback.ShowAlert = showAlert
	b.api.Request(callback)
}