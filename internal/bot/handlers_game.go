package bot

import (
	"fmt"
	"html"
	"log"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"detektif-kata-bot/internal/db"
	"detektif-kata-bot/internal/game"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleClueSubmission(message *tgbotapi.Message, player *db.Player, state *game.GameState, chatID int64, lang string) {
	state.ClueGiverReminderTimer.Stop()
	clueText := message.Text
	if len(strings.Fields(clueText)) != 1 {
		b.sendMessage(player.TelegramUserID, b.localizer.Get(lang, "clue_invalid_not_one_word"), true)
		return
	}
	if strings.EqualFold(clueText, state.SecretWord) {
		b.sendMessage(player.TelegramUserID, b.localizer.Get(lang, "clue_invalid_is_secret_word"), true)
		return
	}

	state.Clue = clueText
	state.Status = game.StatusWaitingForGuesses
	log.Printf("Clue received for chat %d: '%s'", chatID, clueText)
	b.sendMessage(player.TelegramUserID, b.localizer.Get(lang, "clue_received"), false)

	announcement := b.localizer.Get(lang, "clue_announcement_in_group")
	announcement = strings.Replace(announcement, "{round}", strconv.Itoa(state.Round), 1)
	announcement = strings.Replace(announcement, "{giver_name}", html.EscapeString(state.ClueGiver.FirstName), 1)
	announcement = strings.Replace(announcement, "{clue}", strings.ToUpper(html.EscapeString(clueText)), 1)

	sentMsg, err := b.sendMessageAndGet(chatID, announcement, true)
	if err != nil {
		log.Printf("Failed to send clue announcement to chat %d: %v", chatID, err)
		delete(b.gameStates, chatID)
		return
	}

	state.ClueMessageID = sentMsg.MessageID
	state.GuessingStartTime = time.Now()
	state.Timer = time.AfterFunc(60*time.Second, func() { b.handleTimesUp(chatID) })
	state.GuessingTimeWarningTimer = time.AfterFunc(45*time.Second, func() { b.handleGuessingTimeWarning(chatID) })
}


// TANDA: Fungsi ini diperbarui untuk menerapkan skor berbasis waktu
func (b *Bot) handleGroupMessage(message *tgbotapi.Message, player *db.Player) {
	chatID := message.Chat.ID
	state, ok := b.gameStates[chatID]
	if !ok || !state.IsActive || state.Status != game.StatusWaitingForGuesses {
		return
	}

	if _, isParticipant := state.Players[player.TelegramUserID]; !isParticipant {
		log.Printf("Non-participant %s tried to guess.", player.FirstName)
		lang := b.getUserLang(message.From)
		warningText := b.localizer.Get(lang, "warning_not_participant")
		b.sendMessage(player.TelegramUserID, warningText, true)
		return
	}

	if message.ReplyToMessage == nil || message.ReplyToMessage.MessageID != state.ClueMessageID {
		return
	}

	if player.TelegramUserID == state.ClueGiver.TelegramUserID {
		log.Printf("Clue giver %s tried to guess.", player.FirstName)
		return
	}

	guess := message.Text
	lang := b.getUserLang(message.From)
	log.Printf("Guess received in chat %d from %s: '%s'", chatID, player.FirstName, guess)

	b.api.Request(tgbotapi.NewDeleteMessage(chatID, message.MessageID))

	if strings.EqualFold(guess, state.SecretWord) {
		log.Printf("Correct guess by %s in chat %d.", player.FirstName, chatID)
		state.Timer.Stop()
		if state.GuessingTimeWarningTimer != nil {
			state.GuessingTimeWarningTimer.Stop()
		}

		// TANDA: Logika skor berbasis waktu hanya untuk Penebak
		timeTaken := time.Since(state.GuessingStartTime).Seconds()
		var points int
		if timeTaken <= 15 {
			points = 20
		} else if timeTaken <= 30 {
			points = 15
		} else if timeTaken <= 45 {
			points = 10
		} else {
			points = 5
		}

		// Tambahkan poin HANYA untuk Penebak
		state.SessionScores[player.TelegramUserID] += points
		// TANDA: Pemberi petunjuk tidak lagi mendapatkan poin

		responseText := b.localizer.Get(lang, "round_won_announcement")
		responseText = strings.Replace(responseText, "{winner_name}", html.EscapeString(player.FirstName), 1)
		responseText = strings.Replace(responseText, "{word}", strings.ToUpper(state.SecretWord), 1)
		responseText = strings.Replace(responseText, "{points}", strconv.Itoa(points), 1)
		b.sendMessage(chatID, responseText, true)
		b.handleEndOfRound(chatID)
		return
	}

	log.Printf("Incorrect guess by %s in chat %d.", player.FirstName, chatID)
	state.WrongGuesses = append(state.WrongGuesses, guess)

	var wrongGuessesText strings.Builder
	for _, wg := range state.WrongGuesses {
		wrongGuessesText.WriteString(fmt.Sprintf("❌ %s\n", html.EscapeString(wg)))
	}

	originalAnnouncement := b.localizer.Get(lang, "clue_announcement_in_group")
	originalAnnouncement = strings.Replace(originalAnnouncement, "{round}", strconv.Itoa(state.Round), 1)
	originalAnnouncement = strings.Replace(originalAnnouncement, "{giver_name}", html.EscapeString(state.ClueGiver.FirstName), 1)
	originalAnnouncement = strings.Replace(originalAnnouncement, "{clue}", strings.ToUpper(html.EscapeString(state.Clue)), 1)

	fullText := fmt.Sprintf("%s\n\n<b>Tebakan salah:</b>\n%s", originalAnnouncement, wrongGuessesText.String())

	editMsg := tgbotapi.NewEditMessageText(chatID, state.ClueMessageID, fullText)
	editMsg.ParseMode = tgbotapi.ModeHTML
	b.api.Send(editMsg)
}


// TANDA: Fungsi ini dirombak untuk menambahkan semua skor sesi ke skor global
func (b *Bot) endGame(chatID int64, reason string) {
	state, ok := b.gameStates[chatID]
	if !ok {
		return
	}

	if state.ClueGiverReminderTimer != nil {
		state.ClueGiverReminderTimer.Stop()
	}
	if state.GuessingTimeWarningTimer != nil {
		state.GuessingTimeWarningTimer.Stop()
	}
	if state.Timer != nil {
		state.Timer.Stop()
	}

	lang := "id"
	finalMsg := reason

	// TANDA: Logika penambahan poin ke DB dimulai di sini
	log.Printf("Game ended in chat %d. Adding session points to global score.", chatID)
	for playerID, points := range state.SessionScores {
		if points > 0 {
			err := b.db.AddPoints(playerID, points)
			if err != nil {
				log.Printf("Failed to add %d points to player %d: %v", points, playerID, err)
			}
		}
	}
	// TANDA: Logika penambahan poin ke DB berakhir di sini

	// Tampilkan papan skor akhir
	var scoreboard strings.Builder
	players := make([]*db.Player, 0, len(state.Players))
	for _, p := range state.Players {
		players = append(players, p)
	}
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

	// TANDA: Logika bonus pemenang dihapus. Hanya menampilkan pemenang.
	var winner *db.Player
	maxPoints := -1
	for playerID, points := range state.SessionScores {
		if points > maxPoints {
			maxPoints = points
			winner = state.Players[playerID]
		}
	}
	if winner != nil {
		winnerAnnounce := b.localizer.Get(lang, "final_winner_announcement")
		winnerAnnounce = strings.Replace(winnerAnnounce, "{winner_name}", html.EscapeString(winner.FirstName), 1)
		finalMsg += winnerAnnounce
	}

	b.sendMessage(chatID, finalMsg, true)
	delete(b.gameStates, chatID)
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
	state.WrongGuesses = make([]string, 0)

	lang := "id"
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
	if !ok {
		return
	}

	var scoreboard strings.Builder
	scoreboard.WriteString(b.localizer.Get("id", "end_of_round_scoreboard_title"))
	players := make([]*db.Player, 0, len(state.Players))
	for _, p := range state.Players {
		players = append(players, p)
	}
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

func (b *Bot) handleTimesUp(chatID int64) {
	state, ok := b.gameStates[chatID]
	if !ok || !state.IsActive || state.Status != game.StatusWaitingForGuesses {
		return
	}
	if state.GuessingTimeWarningTimer != nil {
		state.GuessingTimeWarningTimer.Stop()
	}
	log.Printf("Time's up for game in chat %d. Word was %s", chatID, state.SecretWord)
	lang := "id"
	responseText := b.localizer.Get(lang, "times_up")
	responseText = strings.Replace(responseText, "{word}", strings.ToUpper(state.SecretWord), 1)
	b.sendMessage(chatID, responseText, true)
	b.handleEndOfRound(chatID)
}

func (b *Bot) handleGuessingTimeWarning(chatID int64) {
	state, ok := b.gameStates[chatID]
	if !ok || !state.IsActive || state.Status != game.StatusWaitingForGuesses {
		return
	}
	log.Printf("Sending time warning for game in chat %d", chatID)
	lang := "id"
	b.sendMessage(chatID, b.localizer.Get(lang, "guess_time_warning"), true)
}

func (b *Bot) handleClueGiverReminder(chatID int64, playerID int64) {
	state, ok := b.gameStates[chatID]
	if !ok || !state.IsActive || state.Status != game.StatusWaitingForClue {
		return
	}
	log.Printf("Sending clue giver reminder to player %d for game in chat %d", playerID, chatID)
	lang := "id"
	text := b.localizer.Get(lang, "clue_giver_reminder")
	text = strings.Replace(text, "{name}", state.ClueGiver.FirstName, 1)
	b.sendMessage(playerID, text, true)
}

func (b *Bot) startSoloGame(chatID int64, player *db.Player, lang string) {
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

func (b *Bot) startGame(chatID int64) {
	state := b.gameStates[chatID]
	lang := "id"

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