package bot

import (
	"fmt"
	"html"
	"strings"
	"log"
	"strconv"

	"detektif-kata-bot/internal/db"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleProfileCommand dipanggil ketika pemain menggunakan perintah /profile
func (b *Bot) handleProfileCommand(message *tgbotapi.Message) {
	lang := b.getUserLang(message.From)

	// Ambil data pemain yang sudah lengkap dari database
	player, err := b.db.GetPlayerByID(message.From.ID)
	if err != nil {
		log.Printf("Failed to get full player data for %d: %v", message.From.ID, err)
		b.sendMessage(message.Chat.ID, b.localizer.Get(lang, "profile_load_error"), false)
		return
	}

	// 1. Siapkan Tampilan Lencana Utama
	var mainBadgeDisplay string
	if player.EquippedBadgeID != nil {
		badge, err := b.db.GetBadgeByID(*player.EquippedBadgeID)
		if err == nil {
			mainBadgeDisplay = badge.Emoji + " "
		}
	}

	// 2. Hitung Win Rate
	winRate := 0.0
	if player.GamesPlayed > 0 {
		winRate = (float64(player.GamesWon) / float64(player.GamesPlayed)) * 100
	}

	// 3. Siapkan Tampilan Statistik Peran (Clue Giver Success Rate)
	clueSuccessRate := 0.0
	if player.ClueGivenCount > 0 {
		clueSuccessRate = (float64(player.ClueSuccessCount) / float64(player.ClueGivenCount)) * 100
	}
	
	// 4. Siapkan Tampilan Tebakan Tercepat
	fastestGuessDisplay := "N/A"
	if player.FastestGuess != -1 {
		fastestGuessDisplay = fmt.Sprintf("%.2f detik", player.FastestGuess)
	}

	// 5. Ambil semua lencana untuk ditampilkan di koleksi
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

	// 6. Gabungkan semua menjadi satu pesan profil yang lengkap
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

	// 7. Buat Tombol Interaktif
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			// Callback data akan berisi prefix "profile_action_"
			tgbotapi.NewInlineKeyboardButtonData("🎽 Pakai Lencana", "profile_action_equip"),
			tgbotapi.NewInlineKeyboardButtonData("🏆 Papan Peringkat", "profile_action_leaderboard"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Segarkan", "profile_action_refresh"),
		),
	)

	msg := tgbotapi.NewMessage(message.Chat.ID, profileText)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)
}

func (b *Bot) handleTokoCommand(message *tgbotapi.Message, player *db.Player) {
	lang := b.getUserLang(message.From)
	purchasableBadges, err := b.db.GetPurchasableBadges()
	if err != nil {
		log.Printf("Failed to display toko: %v", err)
		b.sendMessage(message.Chat.ID, b.localizer.Get(lang, "shop_load_error"), false)
		return
	}

	shopText := fmt.Sprintf("%s\n\n%s\n\n%s",
		b.localizer.Get(lang, "shop_title"),
		strings.Replace(b.localizer.Get(lang, "shop_your_points"), "{points}", strconv.Itoa(player.Points), 1),
		b.localizer.Get(lang, "shop_instruction"),
	)

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, badge := range purchasableBadges {
		buttonText := fmt.Sprintf("%s %s (%d Poin)", badge.Emoji, badge.Name, badge.CriteriaValue)
		callbackData := fmt.Sprintf("shop_view_%d", badge.ID)
		
		row := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(buttonText, callbackData),
		)
		rows = append(rows, row)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	msg := tgbotapi.NewMessage(message.Chat.ID, shopText)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)
}

// displayToko menampilkan daftar barang yang bisa dibeli.
func (b *Bot) displayToko(message *tgbotapi.Message, player *db.Player) {
	purchasableBadges, err := b.db.GetPurchasableBadges()
	if err != nil {
		log.Printf("Failed to display toko: %v", err)
		b.sendMessage(message.Chat.ID, "Gagal memuat toko, coba lagi nanti.", false)
		return
	}

	var shopText strings.Builder
	shopText.WriteString(fmt.Sprintf("--- 🏪 TOKO LENCANA ---\n\n<b>Poin Anda: %d Poin</b>\n\n", player.Points))

	if len(purchasableBadges) == 0 {
		shopText.WriteString("<i>Saat ini tidak ada barang yang dijual.</i>")
	} else {
		for _, badge := range purchasableBadges {
			shopText.WriteString(fmt.Sprintf("<b>ID: %d</b>\n%s <b>%s</b> - %d Poin\n<i>%s</i>\n\n",
				badge.ID,
				badge.Emoji,
				badge.Name,
				badge.CriteriaValue,
				badge.Description,
			))
		}
	}
	// TANDA: Baris ini diubah untuk menggunakan tag <code> dan escaping HTML
	shopText.WriteString("\nUntuk membeli, gunakan format:\n<code>/toko buy &lt;ID&gt;</code>")

	b.sendMessage(message.Chat.ID, shopText.String(), true)
}

// purchaseBadge menangani logika pembelian lencana.
func (b *Bot) purchaseBadge(message *tgbotapi.Message, player *db.Player, badgeID int) {
	// 1. Dapatkan detail lencana yang ingin dibeli
	badgeToBuy, err := b.db.GetBadgeByID(badgeID)
	if err != nil || badgeToBuy.Type != "purchasable" {
		b.sendMessage(message.Chat.ID, "Lencana dengan ID tersebut tidak ditemukan atau tidak dapat dibeli.", false)
		return
	}

	// 2. Periksa apakah pemain sudah memiliki lencana tersebut
	playerBadges, _ := b.db.GetPlayerBadges(player.TelegramUserID)
	for _, ownedBadge := range playerBadges {
		if ownedBadge.ID == badgeID {
			b.sendMessage(message.Chat.ID, "Anda sudah memiliki lencana ini!", false)
			return
		}
	}

	// 3. Periksa apakah poin pemain mencukupi
	if player.Points < badgeToBuy.CriteriaValue {
		b.sendMessage(message.Chat.ID, fmt.Sprintf("Poin Anda tidak cukup! Butuh %d Poin, Anda hanya punya %d Poin.", badgeToBuy.CriteriaValue, player.Points), false)
		return
	}

	// 4. Proses transaksi: kurangi poin dan berikan lencana
	err = b.db.AddPoints(player.TelegramUserID, -badgeToBuy.CriteriaValue)
	if err != nil {
		log.Printf("Failed to subtract points for badge purchase: %v", err)
		b.sendMessage(message.Chat.ID, "Terjadi kesalahan saat transaksi, coba lagi nanti.", false)
		return
	}

	err = b.db.AwardBadgeToPlayer(player.TelegramUserID, badgeID)
	if err != nil {
		log.Printf("Failed to award badge after purchase: %v", err)
		// Kembalikan poin jika gagal memberikan lencana
		b.db.AddPoints(player.TelegramUserID, badgeToBuy.CriteriaValue)
		b.sendMessage(message.Chat.ID, "Terjadi kesalahan saat memberikan lencana, poin Anda telah dikembalikan.", false)
		return
	}

	// 5. Kirim pesan konfirmasi sukses
	successMsg := fmt.Sprintf("✅ Pembelian Berhasil!\n\nAnda telah membeli lencana %s %s. Poin Anda sekarang %d.",
		badgeToBuy.Emoji,
		badgeToBuy.Name,
		player.Points-badgeToBuy.CriteriaValue,
	)
	b.sendMessage(message.Chat.ID, successMsg, true)
}
