package bot

import (
	"fmt"
	"html"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"
	"sort"

	"detektif-kata-bot/internal/config"
	"detektif-kata-bot/internal/db"
	"detektif-kata-bot/internal/game"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// --- fungsi checkUserIsMember dan handleUpdate tetap sama ---
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


// --- TANDA: handleCommand diperbarui untuk mengenali /leaderboard & /topglobal ---
func (b *Bot) handleCommand(message *tgbotapi.Message, player *db.Player) {
	switch message.Command() {
	case "start":
		b.handleStartCommand(message)
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
	default:
	}
}

func (b *Bot) handleEndCommand(message *tgbotapi.Message, player *db.Player) {
	chatID := message.Chat.ID
	lang := b.getUserLang(message.From)
	state, ok := b.gameStates[chatID]

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

	// Hentikan permainan dan tampilkan skor akhir
	reason := b.localizer.Get(lang, "game_ended_by_host")
	b.endGame(chatID, reason)
}


// TANDA: Fungsi baru untuk menampilkan leaderboard
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

		entry := b.localizer.Get(lang, "leaderboard_entry")
		entry = strings.Replace(entry, "{rank_emoji}", rank, 1)
		entry = strings.Replace(entry, "{name}", html.EscapeString(p.FirstName), 1)
		entry = strings.Replace(entry, "{points}", strconv.Itoa(p.Points), 1)
		leaderboardText.WriteString(entry)
	}

	b.sendMessage(chatID, leaderboardText.String(), true)
}

// --- TANDA: SEMUA FUNGSI LAINNYA DIPERBARUI UNTUK MENGGUNAKAN HTML ---

func (b *Bot) handleStartGameCommand(message *tgbotapi.Message, player *db.Player) {
	chatID := message.Chat.ID
	lang := b.getUserLang(message.From)
	if !message.Chat.IsGroup() && !message.Chat.IsSuperGroup() {
		b.sendMessage(chatID, b.localizer.Get(lang, "group_command_only"), false)
		return
	}
	if state, ok := b.gameStates[chatID]; ok && state.IsActive {
		b.sendMessage(chatID, b.localizer.Get(lang, "game_already_running"), false)
		return
	}

	b.gameStates[chatID] = game.NewGame(chatID, player)
	b.gameStates[chatID].Players[player.TelegramUserID] = player // Host otomatis join

	lobbyMsg, err := b.updateLobbyMessage(chatID)
	if err != nil {
		delete(b.gameStates, chatID)
		return
	}
	b.gameStates[chatID].LobbyMessageID = lobbyMsg.MessageID
}

func (b *Bot) handleClueSubmission(message *tgbotapi.Message, player *db.Player, state *game.GameState, chatID int64, lang string) {
	state.ClueGiverReminderTimer.Stop()
	clueText := message.Text
	if len(strings.Fields(clueText)) != 1 { b.sendMessage(player.TelegramUserID, b.localizer.Get(lang, "clue_invalid_not_one_word"), true); return }
	if strings.EqualFold(clueText, state.SecretWord) { b.sendMessage(player.TelegramUserID, b.localizer.Get(lang, "clue_invalid_is_secret_word"), true); return }
	
	state.Clue = clueText // Simpan petunjuk ke state
	state.Status = game.StatusWaitingForGuesses
	log.Printf("Clue received for chat %d: '%s'", chatID, clueText)
	b.sendMessage(player.TelegramUserID, b.localizer.Get(lang, "clue_received"), false)
	
	announcement := b.localizer.Get(lang, "clue_announcement_in_group")
	announcement = strings.Replace(announcement, "{round}", strconv.Itoa(state.Round), 1)
	announcement = strings.Replace(announcement, "{giver_name}", html.EscapeString(state.ClueGiver.FirstName), 1)
	announcement = strings.Replace(announcement, "{clue}", strings.ToUpper(html.EscapeString(clueText)), 1)
	
	sentMsg, err := b.sendMessageAndGet(chatID, announcement, true)
	if err != nil { log.Printf("Failed to send clue announcement to chat %d: %v", chatID, err); delete(b.gameStates, chatID); return }
	
	state.ClueMessageID = sentMsg.MessageID
	state.Timer = time.AfterFunc(60*time.Second, func() { b.handleTimesUp(chatID) })
	state.GuessingTimeWarningTimer = time.AfterFunc(45*time.Second, func() { b.handleGuessingTimeWarning(chatID) })
}


// TANDA: Seluruh fungsi ini ditulis ulang dengan logika yang lebih jelas
// TANDA: Ganti seluruh fungsi ini dengan versi yang benar
// TANDA: Ganti seluruh fungsi ini dengan versi yang benar
func (b *Bot) handleGroupMessage(message *tgbotapi.Message, player *db.Player) {
	chatID := message.Chat.ID
	state, ok := b.gameStates[chatID]
	if !ok || !state.IsActive || state.Status != game.StatusWaitingForGuesses {
		return
	}

	// Cek apakah penebak terdaftar di lobi
	if _, isParticipant := state.Players[player.TelegramUserID]; !isParticipant {
		log.Printf("Non-participant %s tried to guess.", player.FirstName)
		// Kirim peringatan via PM ke pengguna
		lang := b.getUserLang(message.From)
		warningText := b.localizer.Get(lang, "warning_not_participant")
		b.sendMessage(player.TelegramUserID, warningText, true)
		return // Abaikan tebakan
	}

	if message.ReplyToMessage == nil || message.ReplyToMessage.MessageID != state.ClueMessageID {
		return
	}

	// Cek apakah penebak adalah si Pemberi Petunjuk
	if player.TelegramUserID == state.ClueGiver.TelegramUserID {
		log.Printf("Clue giver %s tried to guess.", player.FirstName)
		return
	}

	guess := message.Text
	lang := b.getUserLang(message.From)
	log.Printf("Guess received in chat %d from %s: '%s'", chatID, player.FirstName, guess)

	// Hapus pesan tebakan dari pengguna agar chat tetap bersih
	b.api.Request(tgbotapi.NewDeleteMessage(chatID, message.MessageID))

	// Kondisi 1: Jawaban BENAR
	if strings.EqualFold(guess, state.SecretWord) {
		log.Printf("Correct guess by %s in chat %d.", player.FirstName, chatID)
		state.Timer.Stop()
		state.GuessingTimeWarningTimer.Stop()
		state.SessionScores[player.TelegramUserID] += 10
		responseText := b.localizer.Get(lang, "round_won_announcement")
		responseText = strings.Replace(responseText, "{winner_name}", html.EscapeString(player.FirstName), 1)
		responseText = strings.Replace(responseText, "{word}", strings.ToUpper(state.SecretWord), 1)
		responseText = strings.Replace(responseText, "{points}", "10", 1)
		b.sendMessage(chatID, responseText, true)
		b.handleEndOfRound(chatID)
		return
	}

	// Kondisi 2: Jawaban SALAH (Logika Message Editing yang Benar)
	log.Printf("Incorrect guess by %s in chat %d.", player.FirstName, chatID)

	// Tambahkan tebakan salah ke daftar
	state.WrongGuesses = append(state.WrongGuesses, guess)

	// Buat ulang teks pesan dengan daftar tebakan salah yang baru
	var wrongGuessesText strings.Builder
	for _, wg := range state.WrongGuesses {
		wrongGuessesText.WriteString(fmt.Sprintf("❌ %s\n", html.EscapeString(wg)))
	}

	// Ambil kembali teks pengumuman ronde yang asli
	roundAnnounce := b.localizer.Get(lang, "round_start_announcement")
	roundAnnounce = strings.Replace(roundAnnounce, "{current_round}", strconv.Itoa(state.Round), 1)
	roundAnnounce = strings.Replace(roundAnnounce, "{total_rounds}", strconv.Itoa(state.TotalRounds), 1)
	roundAnnounce = strings.Replace(roundAnnounce, "{clue_giver_name}", html.EscapeString(state.ClueGiver.FirstName), 1)

	cluePart := b.localizer.Get(lang, "clue_announcement_in_group")
	cluePart = strings.Replace(cluePart, "{clue}", strings.ToUpper(html.EscapeString(state.Clue)), 1)
	
	// Gabungkan semua bagian menjadi satu teks utuh
	fullText := fmt.Sprintf("%s\n\n%s\n\n<b>Tebakan salah:</b>\n%s", roundAnnounce, cluePart, wrongGuessesText.String())
	
	// Edit pesan petunjuk yang ada dengan menambahkan daftar tebakan salah
	editMsg := tgbotapi.NewEditMessageText(chatID, state.ClueMessageID, fullText)
	editMsg.ParseMode = tgbotapi.ModeHTML
	b.api.Send(editMsg)
}


func (b *Bot) handleClueGiverReminder(chatID int64, playerID int64) {
	state, ok := b.gameStates[chatID]
	if !ok || !state.IsActive || state.Status != game.StatusWaitingForClue {
		return
	}
	log.Printf("Sending clue giver reminder to player %d for game in chat %d", playerID, chatID)
	lang := "id" // Default
	text := b.localizer.Get(lang, "clue_giver_reminder")
	text = strings.Replace(text, "{name}", state.ClueGiver.FirstName, 1)
	b.sendMessage(playerID, text, true)
}

// TANDA: Fungsi baru untuk peringatan waktu habis
func (b *Bot) handleGuessingTimeWarning(chatID int64) {
	state, ok := b.gameStates[chatID]
	if !ok || !state.IsActive || state.Status != game.StatusWaitingForGuesses {
		return
	}
	log.Printf("Sending time warning for game in chat %d", chatID)
	lang := "id" // Default
	b.sendMessage(chatID, b.localizer.Get(lang, "guess_time_warning"), true)
}

func (b *Bot) handleTimesUp(chatID int64) {
	state, ok := b.gameStates[chatID]
	if !ok || !state.IsActive || state.Status != game.StatusWaitingForGuesses { return }
	state.GuessingTimeWarningTimer.Stop()
	log.Printf("Time's up for game in chat %d. Word was %s", chatID, state.SecretWord)
	lang := "id"
	responseText := b.localizer.Get(lang, "times_up")
	responseText = strings.Replace(responseText, "{word}", strings.ToUpper(state.SecretWord), 1)
	b.sendMessage(chatID, responseText, true)
	b.handleEndOfRound(chatID)
}

func (b *Bot) sendTemporaryReply(chatID int64, replyToMessageID int, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyToMessageID = replyToMessageID

	sentMsg, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Failed to send temporary reply: %v", err)
		return
	}

	go func() {
		time.Sleep(2 * time.Second)
		b.api.Request(tgbotapi.NewDeleteMessage(chatID, sentMsg.MessageID))
	}()
}

func (b *Bot) handleStartCommand(message *tgbotapi.Message) {
	lang := b.getUserLang(message.From)
	welcomeText := b.localizer.Get(lang, "start_welcome")
	personalizedText := strings.Replace(welcomeText, "{name}", html.EscapeString(message.From.FirstName), 1)
	b.sendMessage(message.Chat.ID, personalizedText, true)
}

func (b *Bot) handleStartAloneCommand(message *tgbotapi.Message, player *db.Player) {
	chatID := message.Chat.ID
	lang := b.getUserLang(message.From)
	if !message.Chat.IsPrivate() {
		b.sendMessage(chatID, b.localizer.Get(lang, "private_chat_only"), false)
		return
	}
	if state, ok := b.soloGameStates[player.TelegramUserID]; ok && state.IsActive {
		b.sendMessage(chatID, b.localizer.Get(lang, "solo_game_already_running"), false)
		return
	}
	rand.Seed(time.Now().UnixNano())
	wordData := game.SoloWordList[rand.Intn(len(game.SoloWordList))]
	b.soloGameStates[player.TelegramUserID] = &game.SoloGameState{
		UserID:      player.TelegramUserID,
		IsActive:    true,
		CurrentWord: wordData,
		HintsGiven:  1,
	}
	b.sendMessage(chatID, b.localizer.Get(lang, "solo_game_started"), false)
	time.Sleep(1 * time.Second)
	firstHintText := b.localizer.Get(lang, "solo_first_hint")
	firstHintText = strings.Replace(firstHintText, "{hint}", wordData.Hints[0], 1)
	b.sendMessage(chatID, firstHintText, true)
}

func (b *Bot) handleSoloGuess(message *tgbotapi.Message, player *db.Player, state *game.SoloGameState, lang string) {
	guess := message.Text
	if strings.EqualFold(guess, state.CurrentWord.Word) {
		score := 100 - (state.HintsGiven-1)*10
		if score < 10 {
			score = 10
		}
		err := b.db.AddPoints(player.TelegramUserID, score)
		if err != nil {
			log.Printf("Failed to add points for solo game winner %d", player.TelegramUserID)
		}
		responseText := b.localizer.Get(lang, "solo_guess_correct")
		responseText = strings.Replace(responseText, "{hints_given}", strconv.Itoa(state.HintsGiven), 1)
		responseText = strings.Replace(responseText, "{word}", strings.ToUpper(state.CurrentWord.Word), 1)
		responseText = strings.Replace(responseText, "{score}", strconv.Itoa(score), 1)
		b.sendMessage(message.Chat.ID, responseText, true)
		delete(b.soloGameStates, player.TelegramUserID)
	} else {
		if state.HintsGiven < len(state.CurrentWord.Hints) {
			state.HintsGiven++
			nextHint := state.CurrentWord.Hints[state.HintsGiven-1]
			responseText := b.localizer.Get(lang, "solo_next_hint")
			responseText = strings.Replace(responseText, "{hint_number}", strconv.Itoa(state.HintsGiven), 1)
			responseText = strings.Replace(responseText, "{hint}", nextHint, 1)
			b.sendMessage(message.Chat.ID, responseText, true)
		} else {
			responseText := b.localizer.Get(lang, "solo_no_more_hints")
			responseText = strings.Replace(responseText, "{word}", strings.ToUpper(state.CurrentWord.Word), 1)
			b.sendMessage(message.Chat.ID, responseText, true)
			delete(b.soloGameStates, player.TelegramUserID)
		}
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
func (b *Bot) handlePrivateMessage(message *tgbotapi.Message, player *db.Player) {
	lang := b.getUserLang(message.From)
	for chatID, state := range b.gameStates {
		if state.IsActive && state.Status == game.StatusWaitingForClue && state.ClueGiver.TelegramUserID == player.TelegramUserID {
			b.handleClueSubmission(message, player, state, chatID, lang)
			return
		}
	}
	if state, ok := b.soloGameStates[player.TelegramUserID]; ok && state.IsActive {
		b.handleSoloGuess(message, player, state, lang)
		return
	}
}

func (b *Bot) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	chatID := query.Message.Chat.ID
	lang := b.getUserLang(query.From)
	user := &config.User{ID: query.From.ID, FirstName: query.From.FirstName, Username: query.From.UserName}
	player, _ := b.db.GetOrCreatePlayer(user)

	if strings.HasPrefix(query.Data, "join_game") {
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
		b.updateLobbyMessage(chatID)
	}
}

func (b *Bot) handlePlayCommand(message *tgbotapi.Message, player *db.Player) {
	chatID := message.Chat.ID
	lang := b.getUserLang(message.From)
	state, ok := b.gameStates[chatID]

	if !ok || !state.IsActive || state.Status != game.StatusLobby {
		return // Abaikan jika tidak ada lobi aktif
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

	// Kunci Lobi dan Acak Giliran
	msg := tgbotapi.NewEditMessageReplyMarkup(chatID, state.LobbyMessageID, tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}})
	b.api.Send(msg)

	for _, p := range state.Players {
		state.TurnOrder = append(state.TurnOrder, p)
	}
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(state.TurnOrder), func(i, j int) {
		state.TurnOrder[i], state.TurnOrder[j] = state.TurnOrder[j], state.TurnOrder[i]
	})

	b.sendMessage(chatID, b.localizer.Get(lang, "game_started_announcement"), true)
	time.Sleep(2 * time.Second)
	b.startRound(chatID)
}

func (b *Bot) startRound(chatID int64) {
	state, ok := b.gameStates[chatID]
	if !ok {
		return
	}

	state.Round++
	state.CurrentTurnIndex = (state.Round - 1) % len(state.TurnOrder)
	clueGiver := state.TurnOrder[state.CurrentTurnIndex]
	state.ClueGiver = clueGiver
	state.Status = game.StatusWaitingForClue

	lang := "id" // Default
	announcement := b.localizer.Get(lang, "round_start_announcement")
	announcement = strings.Replace(announcement, "{current_round}", strconv.Itoa(state.Round), 1)
	announcement = strings.Replace(announcement, "{total_rounds}", strconv.Itoa(state.TotalRounds), 1)
	announcement = strings.Replace(announcement, "{clue_giver_name}", html.EscapeString(clueGiver.FirstName), 1)
	b.sendMessage(chatID, announcement, true)

	secretWord := game.WordList[rand.Intn(len(game.WordList))]
	state.SecretWord = secretWord

	promptText := b.localizer.Get(lang, "secret_word_prompt")
	promptText = strings.Replace(promptText, "{name}", html.EscapeString(clueGiver.FirstName), -1)
	promptText = strings.Replace(promptText, "{word}", secretWord, -1)
	err := b.sendMessage(clueGiver.TelegramUserID, promptText, true)
	if err != nil {
		// Jika gagal kirim PM, lewati giliran pemain ini
		b.sendMessage(chatID, fmt.Sprintf("Gagal mengirim PM ke %s, giliran dilewati.", clueGiver.FirstName), false)
		time.Sleep(2 * time.Second)
		b.handleEndOfRound(chatID)
		return
	}
	
	state.ClueGiverReminderTimer = time.AfterFunc(90*time.Second, func() {
		b.handleClueGiverReminder(chatID, clueGiver.TelegramUserID)
	})
}

func (b *Bot) handleEndOfRound(chatID int64) {
	state, ok := b.gameStates[chatID]
	if !ok { return }
	
	var scoreboard strings.Builder
	scoreboard.WriteString(b.localizer.Get("id", "end_of_round_scoreboard_title"))
	players := make([]*db.Player, 0, len(state.Players))
	for _, p := range state.Players { players = append(players, p) }
	sort.Slice(players, func(i, j int) bool {
		return state.SessionScores[players[i].TelegramUserID] > state.SessionScores[players[j].TelegramUserID]
	})
	for _, p := range players {
		entry := b.localizer.Get("id", "end_of_round_scoreboard_entry")
		entry = strings.Replace(entry, "{player_name}", html.EscapeString(p.FirstName), 1)
		entry = strings.Replace(entry, "{points}", strconv.Itoa(state.SessionScores[p.TelegramUserID]), 1)
		scoreboard.WriteString(entry)
	}
	b.sendMessage(chatID, scoreboard.String(), true)
	time.Sleep(4 * time.Second)

	if state.Round >= state.TotalRounds {
		reason := b.localizer.Get("id", "game_over_announcement")
		reason = strings.Replace(reason, "{total_rounds}", strconv.Itoa(state.Round), 1)
		b.endGame(chatID, reason)
	} else {
		b.startRound(chatID)
	}
}


func (b *Bot) endGame(chatID int64, reason string) {
	state, ok := b.gameStates[chatID]
	if !ok { return }

	// Hentikan semua timer yang mungkin masih berjalan
	if state.ClueGiverReminderTimer != nil { state.ClueGiverReminderTimer.Stop() }
	if state.GuessingTimeWarningTimer != nil { state.GuessingTimeWarningTimer.Stop() }
	if state.Timer != nil { state.Timer.Stop() }

	lang := "id"
	var winner *db.Player
	maxPoints := -1
	
	for playerID, points := range state.SessionScores {
		if points > maxPoints {
			maxPoints = points
			winner = state.Players[playerID]
		}
	}
	
	finalMsg := reason // Gunakan alasan dari parameter
	
	// Tambahkan papan skor akhir
	var scoreboard strings.Builder
	players := make([]*db.Player, 0, len(state.Players))
	for _, p := range state.Players { players = append(players, p) }
	sort.Slice(players, func(i, j int) bool {
		return state.SessionScores[players[i].TelegramUserID] > state.SessionScores[players[j].TelegramUserID]
	})
	for _, p := range players {
		entry := b.localizer.Get(lang, "end_of_round_scoreboard_entry")
		entry = strings.Replace(entry, "{player_name}", html.EscapeString(p.FirstName), 1)
		entry = strings.Replace(entry, "{points}", strconv.Itoa(state.SessionScores[p.TelegramUserID]), 1)
		scoreboard.WriteString(entry)
	}
	finalMsg += scoreboard.String()

	if winner != nil {
		bonusPoints := 50
		b.db.AddPoints(winner.TelegramUserID, bonusPoints)
		winnerAnnounce := b.localizer.Get(lang, "final_winner_announcement")
		winnerAnnounce = strings.Replace(winnerAnnounce, "{winner_name}", html.EscapeString(winner.FirstName), 1)
		finalMsg += winnerAnnounce
		finalMsg += fmt.Sprintf("\n<i>Sebagai hadiah, kamu dapat bonus <b>%d Poin</b> ke skor global!</i>", bonusPoints)
	}

	b.sendMessage(chatID, finalMsg, true)
	delete(b.gameStates, chatID)
}



func (b *Bot) updateLobbyMessage(chatID int64) (tgbotapi.Message, error) {
	state := b.gameStates[chatID]
	lang := "id" // Default to ID for lobby messages

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
