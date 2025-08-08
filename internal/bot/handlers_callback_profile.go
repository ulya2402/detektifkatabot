// TANDA: File baru internal/bot/handlers_callback_profile.go

package bot

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleProfileActionCallback adalah router untuk semua aksi tombol di profil.
func (b *Bot) handleProfileActionCallback(query *tgbotapi.CallbackQuery) {
	action := strings.TrimPrefix(query.Data, "profile_action_")
	// TANDA: Kita ambil messageID dari query di sini
	messageID := query.Message.MessageID

	switch action {
	case "refresh":
		// TANDA: Sekarang kita kirim messageID ke fungsi refresh
		b.refreshProfileView(query, messageID)
	case "leaderboard":
		// TANDA: Sekarang kita kirim messageID ke fungsi leaderboard
		b.displayLeaderboardView(query, messageID)
	case "equip":
		// TANDA: Sekarang kita kirim messageID ke fungsi equip
		b.displayEquipBadgeView(query, messageID)
	}
}

// refreshProfileView memuat ulang dan menampilkan tampilan profil utama.
func (b *Bot) refreshProfileView(query *tgbotapi.CallbackQuery, messageID int) {
	chatID := query.Message.Chat.ID
	userID := query.From.ID
	lang := b.getUserLang(query.From)

	player, err := b.db.GetPlayerByID(userID)
	if err != nil {
		b.answerCallback(query.ID, b.localizer.Get(lang, "profile_load_error"), true)
		return
	}

	// Logika pembuatan teks profil disalin dari handleProfileCommand
	var mainBadgeDisplay string
	if player.EquippedBadgeID != nil {
		badge, err := b.db.GetBadgeByID(*player.EquippedBadgeID)
		if err == nil {
			mainBadgeDisplay = badge.Emoji + " "
		}
	}

	winRate := 0.0
	if player.GamesPlayed > 0 {
		winRate = (float64(player.GamesWon) / float64(player.GamesPlayed)) * 100
	}

	clueSuccessRate := 0.0
	if player.ClueGivenCount > 0 {
		clueSuccessRate = (float64(player.ClueSuccessCount) / float64(player.ClueGivenCount)) * 100
	}
	
	fastestGuessDisplay := "N/A"
	if player.FastestGuess != -1 {
		fastestGuessDisplay = fmt.Sprintf("%.2f detik", player.FastestGuess)
	}

	allBadges, _ := b.db.GetPlayerBadges(player.TelegramUserID)
	var allBadgesDisplay string
	if len(allBadges) > 0 {
		var badgeEmojis []string
		for _, badge := range allBadges {
			badgeEmojis = append(badgeEmojis, badge.Emoji)
		}
		allBadgesDisplay = strings.Join(badgeEmojis, " ")
	} else {
		allBadgesDisplay = b.localizer.Get(lang, "profile_no_badges")
	}

	profileText := fmt.Sprintf(
		"--- 👤 PROFIL PEMAIN ---\n"+
		"<b>Nama:</b> %s%s\n"+
		"<b>Poin:</b> %d\n\n"+
		"--- 📊 STATISTIK ---\n"+
		"• Main: %d | Menang: %d (%.0f%% Win Rate)\n"+
		"• Total Tebakan: %d kata\n"+
		"• Tebakan Tercepat: %s\n"+
		"• Sukses Beri Petunjuk: %.0f%%\n\n"+
		"--- 🎖️ KOLEKSI LENCANA ---\n"+
		"%s",
		mainBadgeDisplay,
		html.EscapeString(player.FirstName),
		player.Points,
		player.GamesPlayed,
		player.GamesWon,
		winRate,
		player.WordsGuessedCount,
		fastestGuessDisplay,
		clueSuccessRate,
		allBadgesDisplay,
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🎽 Pakai Lencana", "profile_action_equip"),
			tgbotapi.NewInlineKeyboardButtonData("🏆 Papan Peringkat", "profile_action_leaderboard"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Segarkan", "profile_action_refresh"),
		),
	)

	// Gunakan EditMessageText untuk memperbarui pesan yang ada
	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, profileText)
	editMsg.ParseMode = tgbotapi.ModeHTML
	editMsg.ReplyMarkup = &keyboard
	b.api.Request(editMsg)
	b.answerCallback(query.ID, "Profil disegarkan!", false)
}


// displayLeaderboardView menampilkan papan peringkat di dalam tampilan profil.
// TANDA: Perbaikan pada fungsi displayLeaderboardView

func (b *Bot) displayLeaderboardView(query *tgbotapi.CallbackQuery, messageID int) {
	chatID := query.Message.Chat.ID
	lang := b.getUserLang(query.From)

	players, _ := b.db.GetTopPlayers(10)
	var leaderboardText strings.Builder
	leaderboardText.WriteString(b.localizer.Get(lang, "leaderboard_title"))
	rankEmojis := []string{"🥇", "🥈", "🥉"}

	for i, p := range players {
		rank := fmt.Sprintf("%d.", i+1)
		if i < len(rankEmojis) {
			rank = rankEmojis[i]
		}

		badgeDisplay := ""
		// Prioritaskan lencana yang dipakai
		if p.EquippedBadgeID != nil {
			badge, err := b.db.GetBadgeByID(*p.EquippedBadgeID)
			if err == nil {
				badgeDisplay = badge.Emoji + " "
			}
		} else {
			// Fallback ke lencana pertama jika tidak ada yang dipakai
			playerBadges, _ := b.db.GetPlayerBadges(p.TelegramUserID)
			if len(playerBadges) > 0 {
				badgeDisplay = playerBadges[0].Emoji + " "
			}
		}

		playerNameDisplay := badgeDisplay + html.EscapeString(p.FirstName)
		entry := b.localizer.Get(lang, "leaderboard_entry")
		entry = strings.Replace(entry, "{rank_emoji}", rank, 1)
		entry = strings.Replace(entry, "{name}", playerNameDisplay, 1)
		entry = strings.Replace(entry, "{points}", strconv.Itoa(p.Points), 1)
		leaderboardText.WriteString(entry)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Kembali ke Profil", "profile_action_refresh"),
		),
	)

	// TANDA: Menggunakan variabel 'leaderboardText.String()' yang benar
	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, leaderboardText.String())
	editMsg.ParseMode = tgbotapi.ModeHTML
	editMsg.ReplyMarkup = &keyboard
	b.api.Request(editMsg)
	b.answerCallback(query.ID, "", false)
}


// displayEquipBadgeView menampilkan menu untuk memilih lencana.
func (b *Bot) displayEquipBadgeView(query *tgbotapi.CallbackQuery, messageID int) {
	chatID := query.Message.Chat.ID
	userID := query.From.ID
	
	playerBadges, _ := b.db.GetPlayerBadges(userID)
	if len(playerBadges) == 0 {
		b.answerCallback(query.ID, "Anda belum memiliki lencana untuk dipakai.", true)
		return
	}

	text := "Silakan pilih lencana yang ingin Anda pakai:"
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, badge := range playerBadges {
		buttonText := fmt.Sprintf("%s %s", badge.Emoji, badge.Name)
		callbackData := fmt.Sprintf("lencana_equip_%d", badge.ID)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(buttonText, callbackData),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Kembali ke Profil", "profile_action_refresh"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, text)
	editMsg.ParseMode = tgbotapi.ModeHTML
	editMsg.ReplyMarkup = &keyboard
	b.api.Request(editMsg)
	b.answerCallback(query.ID, "", false)
}