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
func (b *Bot) handleProfileCommand(message *tgbotapi.Message, player *db.Player) {
	lang := b.getUserLang(message.From)
	badges, err := b.db.GetPlayerBadges(player.TelegramUserID)
	if err != nil {
		log.Printf("Failed to get player badges for %d: %v", player.TelegramUserID, err)
		b.sendMessage(message.Chat.ID, b.localizer.Get(lang, "profile_load_error"), false)
		return
	}

	var badgesDisplay string
	if len(badges) > 0 {
		var badgeEmojis []string
		for _, badge := range badges {
			badgeEmojis = append(badgeEmojis, badge.Emoji)
		}
		badgesDisplay = strings.Join(badgeEmojis, " ")
	} else {
		badgesDisplay = b.localizer.Get(lang, "profile_no_badges")
	}

	profileText := fmt.Sprintf(
		"%s\n%s\n%s\n%s",
		b.localizer.Get(lang, "profile_title"),
		strings.Replace(b.localizer.Get(lang, "profile_name"), "{name}", html.EscapeString(player.FirstName), 1),
		strings.Replace(b.localizer.Get(lang, "profile_points"), "{points}", strconv.Itoa(player.Points), 1),
		strings.Replace(b.localizer.Get(lang, "profile_badges"), "{badges_display}", badgesDisplay, 1),
	)

	b.sendMessage(message.Chat.ID, profileText, true)
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
