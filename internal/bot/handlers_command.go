package bot

import (
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"

	"detektif-kata-bot/internal/db"
	"detektif-kata-bot/internal/game"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleCommand(message *tgbotapi.Message, player *db.Player) {
	switch message.Command() {
	case "start":
		b.handleStartCommand(message)
	case "help":
		b.handleHelpCommand(message)
	case "startgame":
		b.handleStartGameCommand(message, player)
	case "play":
		b.handlePlayCommand(message, player)
	case "end":
		b.handleEndCommand(message, player)
	case "startalone":
		b.handleStartAloneCommand(message, player)
	case "leaderboard", "topglobal":
		b.handleLeaderboardCommand(message)
	case "profile":
		b.handleProfileCommand(message)
	case "toko", "market":
		b.handleTokoCommand(message, player)
	case "broadcast", "broadcastgroup": 
		b.handleAdminCommand(message)
	default:
	}
}


func (b *Bot) handleStartCommand(message *tgbotapi.Message) {
	lang := b.getUserLang(message.From)
	chatID := message.Chat.ID

	// 1. Siapkan pesan teks dan tombol terlebih dahulu
	welcomeText := b.localizer.Get(lang, "start_welcome")
	personalizedText := strings.Replace(welcomeText, "{name}", html.EscapeString(message.From.FirstName), 1)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(b.localizer.Get(lang, "help_button_how_to_play"), "help_how_to_play"),
			tgbotapi.NewInlineKeyboardButtonData(b.localizer.Get(lang, "help_button_commands"), "help_commands"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏆 Papan Peringkat", "profile_action_leaderboard"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👤 Profil Saya", "profile_action_refresh"),
		),
	)

	// 2. Cek apakah ada URL gambar di konfigurasi
	if b.cfg.StartImageURL != "" {
		// Jika ADA gambar, kirim sebagai foto dengan caption dan tombol
		photoMsg := tgbotapi.NewPhoto(chatID, tgbotapi.FileURL(b.cfg.StartImageURL))
		photoMsg.Caption = personalizedText
		photoMsg.ParseMode = tgbotapi.ModeHTML
		photoMsg.ReplyMarkup = &keyboard // Tambahkan tombol ke pesan foto
		b.api.Send(photoMsg)
	} else {
		// Jika TIDAK ADA gambar, kirim sebagai pesan teks biasa (fallback)
		msg := tgbotapi.NewMessage(chatID, personalizedText)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		b.api.Send(msg)
	}
}

func (b *Bot) handleStartGameCommand(message *tgbotapi.Message, player *db.Player) {
	chatID := message.Chat.ID
	lang := b.getUserLang(message.From)
	if !message.Chat.IsGroup() && !message.Chat.IsSuperGroup() {
		b.sendMessage(chatID, b.localizer.Get(lang, "group_command_only"), false)
		return
	}
	
	b.mu.RLock()
	state, ok := b.gameStates[chatID]
	b.mu.RUnlock()
	if ok && state.IsActive {
		b.sendMessage(chatID, b.localizer.Get(lang, "game_already_running"), false)
		return
	}

	args := message.CommandArguments()
	totalRounds := 10 // Default
	minRounds := 3
	maxRounds := 25

	if args != "" {
		parsedRounds, err := strconv.Atoi(args)
		if err == nil && parsedRounds >= minRounds && parsedRounds <= maxRounds {
			totalRounds = parsedRounds
		} else {
			// Kirim pesan jika input tidak valid, tapi tetap mulai game dengan default
			invalidRoundsMsg := fmt.Sprintf("Jumlah ronde tidak valid. Harus antara %d dan %d. Memulai dengan %d ronde.", minRounds, maxRounds, totalRounds)
			b.sendMessage(chatID, invalidRoundsMsg, false)
		}
	}
	// TANDA: Logika untuk menentukan jumlah ronde berakhir di sini

	b.mu.Lock()
	b.gameStates[chatID] = game.NewGame(chatID, player, totalRounds) // TANDA: totalRounds dimasukkan saat membuat game baru
	b.gameStates[chatID].Players[player.TelegramUserID] = player
	b.mu.Unlock()

	lobbyMsg, err := b.updateLobbyMessage(chatID)
	if err != nil {
		delete(b.gameStates, chatID)
		return
	}
	b.gameStates[chatID].LobbyMessageID = lobbyMsg.MessageID
}

func (b *Bot) handlePlayCommand(message *tgbotapi.Message, player *db.Player) {
	chatID := message.Chat.ID
	lang := b.getUserLang(message.From)
	
	b.mu.RLock()
	state, ok := b.gameStates[chatID]
	b.mu.RUnlock()

	if !ok || !state.IsActive || state.Status != game.StatusLobby {
		return
	}

	if player.TelegramUserID != state.Host.TelegramUserID {
		text := b.localizer.Get(lang, "play_command_not_host")
		text = strings.Replace(text, "{host_name}", html.EscapeString(state.Host.FirstName), 1)
		b.sendMessage(chatID, text, true)
		return
	}

	if len(state.Players) < 2 {
		b.sendMessage(chatID, b.localizer.Get(lang, "play_command_not_enough_players"), false)
		return
	}

	b.startGame(chatID)
}

func (b *Bot) handleEndCommand(message *tgbotapi.Message, player *db.Player) {
	chatID := message.Chat.ID
	lang := b.getUserLang(message.From)
	
	b.mu.RLock()
	state, ok := b.gameStates[chatID]
	b.mu.RUnlock()

	if !ok || !state.IsActive {
		b.sendMessage(chatID, b.localizer.Get(lang, "game_not_found"), false)
		return
	}

	if player.TelegramUserID != state.Host.TelegramUserID {
		text := b.localizer.Get(lang, "end_command_not_host")
		text = strings.Replace(text, "{host_name}", html.EscapeString(state.Host.FirstName), 1)
		b.sendMessage(chatID, text, true)
		return
	}

	reason := b.localizer.Get(lang, "game_ended_by_host")
	b.endGame(chatID, reason)
}

func (b *Bot) handleStartAloneCommand(message *tgbotapi.Message, player *db.Player) {
	chatID := message.Chat.ID
	lang := b.getUserLang(message.From)
	if !message.Chat.IsPrivate() {
		b.sendMessage(chatID, b.localizer.Get(lang, "private_chat_only"), false)
		return
	}

	b.mu.RLock()
	state, ok := b.soloGameStates[player.TelegramUserID]
	b.mu.RUnlock()
	if ok && state.IsActive {
		b.sendMessage(chatID, b.localizer.Get(lang, "solo_game_already_running"), false)
		return
	}
	b.startSoloGame(chatID, player, lang)
}

func (b *Bot) handleLeaderboardCommand(message *tgbotapi.Message) {
	chatID := message.Chat.ID
	lang := b.getUserLang(message.From)

	players, err := b.db.GetTopPlayers(10)
	if err != nil {
		log.Printf("Failed to get top players: %v", err)
		return
	}

	if len(players) == 0 {
		b.sendMessage(chatID, b.localizer.Get(lang, "leaderboard_empty"), true)
		return
	}

	var leaderboardText strings.Builder
	leaderboardText.WriteString(b.localizer.Get(lang, "leaderboard_title"))

	rankEmojis := []string{"🥇", "🥈", "🥉"}

	for i, p := range players {
		var rank string
		if i < len(rankEmojis) {
			rank = rankEmojis[i]
		} else {
			rank = fmt.Sprintf("%d.", i+1)
		}

		// TANDA: Logika untuk mengambil dan menampilkan lencana ditambahkan di sini
		
		badgeDisplay := ""
		if p.EquippedBadgeID != nil {
			badge, err := b.db.GetBadgeByID(*p.EquippedBadgeID)
			if err == nil {
				badgeDisplay = badge.Emoji + " "
			}
		} else {
			playerBadges, _ := b.db.GetPlayerBadges(p.TelegramUserID)
			if len(playerBadges) > 0 {
				badgeDisplay = playerBadges[0].Emoji + " "
			}
		}
		playerNameDisplay := badgeDisplay + html.EscapeString(p.FirstName)
		// TANDA: Akhir dari blok yang ditambahkan

		entry := b.localizer.Get(lang, "leaderboard_entry")
		entry = strings.Replace(entry, "{rank_emoji}", rank, 1)
		entry = strings.Replace(entry, "{name}", playerNameDisplay, 1)
		entry = strings.Replace(entry, "{points}", strconv.Itoa(p.Points), 1)
		leaderboardText.WriteString(entry)
	}

	b.sendMessage(chatID, leaderboardText.String(), true)
}

func (b *Bot) handleHelpCommand(message *tgbotapi.Message) {
	chatID := message.Chat.ID
	lang := b.getUserLang(message.From)

	text := b.localizer.Get(lang, "help_main_title")
	keyboard := b.createHelpKeyboard(lang)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)
}

func (b *Bot) createHelpKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(b.localizer.Get(lang, "help_button_how_to_play"), "help_how_to_play"),
			tgbotapi.NewInlineKeyboardButtonData(b.localizer.Get(lang, "help_button_commands"), "help_commands"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(b.localizer.Get(lang, "help_button_scoring"), "help_scoring"),
		),
	)
}
