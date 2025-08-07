package bot

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"detektif-kata-bot/internal/config"
	"detektif-kata-bot/internal/db"
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

	if strings.HasPrefix(query.Data, "shop_") {
		b.handleShopCallback(query, player)
		return
	}


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

// TANDA: Kode fungsi penuh untuk handleShopCallback yang sudah menggunakan localizer

func (b *Bot) handleShopCallback(query *tgbotapi.CallbackQuery, player *db.Player) {
	data := query.Data
	chatID := query.Message.Chat.ID
	messageID := query.Message.MessageID
	lang := b.getUserLang(query.From)

	// Kasus: Melihat detail item (prefix: "shop_view_")
	if strings.HasPrefix(data, "shop_view_") {
		badgeID, _ := strconv.Atoi(strings.TrimPrefix(data, "shop_view_"))
		badge, err := b.db.GetBadgeByID(badgeID)
		if err != nil {
			b.answerCallback(query.ID, b.localizer.Get(lang, "shop_item_unavailable"), true)
			return
		}

		// Periksa apakah pemain sudah punya lencana ini
		playerBadges, _ := b.db.GetPlayerBadges(player.TelegramUserID)
		alreadyOwned := false
		for _, b := range playerBadges {
			if b.ID == badgeID {
				alreadyOwned = true
				break
			}
		}

		var confirmationText string
		var rows [][]tgbotapi.InlineKeyboardButton
		if alreadyOwned {
			// Jika sudah dimiliki
			t := b.localizer.Get(lang, "shop_already_owned_title")
			t = strings.Replace(t, "{emoji}", badge.Emoji, 1)
			t = strings.Replace(t, "{name}", badge.Name, 1)
			confirmationText = t
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(b.localizer.Get(lang, "button_back_to_shop"), "shop_main"),
			))
		} else {
			// Jika belum dimiliki, tampilkan konfirmasi pembelian
			t := b.localizer.Get(lang, "shop_confirm_purchase")
			t = strings.Replace(t, "{emoji}", badge.Emoji, 1)
			t = strings.Replace(t, "{name}", badge.Name, 1)
			t = strings.Replace(t, "{description}", badge.Description, 1)
			t = strings.Replace(t, "{price}", strconv.Itoa(badge.CriteriaValue), 1)
			// Ambil data poin pemain terbaru untuk ditampilkan
			updatedPlayer, _ := b.db.GetOrCreatePlayer(&config.User{ID: player.TelegramUserID})
			t = strings.Replace(t, "{points}", strconv.Itoa(updatedPlayer.Points), 1)
			confirmationText = t
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(b.localizer.Get(lang, "button_buy"), fmt.Sprintf("shop_buy_%d", badge.ID)),
				tgbotapi.NewInlineKeyboardButtonData(b.localizer.Get(lang, "button_cancel"), "shop_main"),
			))
		}

		keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, confirmationText)
		editMsg.ParseMode = tgbotapi.ModeHTML
		editMsg.ReplyMarkup = &keyboard
		b.api.Request(editMsg)
		b.answerCallback(query.ID, "", false)
	}

	// Kasus: Konfirmasi pembelian (prefix: "shop_buy_")
	if strings.HasPrefix(data, "shop_buy_") {
		badgeID, _ := strconv.Atoi(strings.TrimPrefix(data, "shop_buy_"))
		badgeToBuy, err := b.db.GetBadgeByID(badgeID)
		if err != nil {
			b.answerCallback(query.ID, b.localizer.Get(lang, "shop_item_unavailable"), true)
			return
		}

		updatedPlayer, _ := b.db.GetOrCreatePlayer(&config.User{ID: player.TelegramUserID})
		if updatedPlayer.Points < badgeToBuy.CriteriaValue {
			b.answerCallback(query.ID, b.localizer.Get(lang, "shop_not_enough_points"), true)
			return
		}

		err = b.db.AddPoints(player.TelegramUserID, -badgeToBuy.CriteriaValue)
		if err != nil {
			b.answerCallback(query.ID, b.localizer.Get(lang, "shop_purchase_fail_process"), true)
			return
		}
		err = b.db.AwardBadgeToPlayer(player.TelegramUserID, badgeID)
		if err != nil {
			b.db.AddPoints(player.TelegramUserID, badgeToBuy.CriteriaValue)
			b.answerCallback(query.ID, b.localizer.Get(lang, "shop_purchase_fail_award"), true)
			return
		}

		successText := fmt.Sprintf("✅ Pembelian Berhasil!\n\nAnda telah mendapatkan lencana %s %s.", badgeToBuy.Emoji, badgeToBuy.Name)
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, successText)
		editMsg.ParseMode = tgbotapi.ModeHTML
		b.api.Request(editMsg)
		b.answerCallback(query.ID, b.localizer.Get(lang, "shop_purchase_success"), false)
	}

	// Kasus: Kembali ke menu utama toko
	if data == "shop_main" {
		purchasableBadges, _ := b.db.GetPurchasableBadges()
		updatedPlayer, _ := b.db.GetOrCreatePlayer(&config.User{ID: player.TelegramUserID})

		t := b.localizer.Get(lang, "shop_title")
		t += "\n\n" + strings.Replace(b.localizer.Get(lang, "shop_your_points"), "{points}", strconv.Itoa(updatedPlayer.Points), 1)
		t += "\n\n" + b.localizer.Get(lang, "shop_instruction")
		shopText := t
		
		var rows [][]tgbotapi.InlineKeyboardButton
		for _, badge := range purchasableBadges {
			buttonText := fmt.Sprintf("%s %s (%d Poin)", badge.Emoji, badge.Name, badge.CriteriaValue)
			callbackData := fmt.Sprintf("shop_view_%d", badge.ID)
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(buttonText, callbackData),
			))
		}

		keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, shopText)
		editMsg.ParseMode = tgbotapi.ModeHTML
		editMsg.ReplyMarkup = &keyboard
		b.api.Request(editMsg)
		b.answerCallback(query.ID, "", false)
	}
}