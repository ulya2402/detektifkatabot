package bot

import (
	"fmt"
	"html"
	"log"
)

// checkAndAwardAchievements memeriksa apakah pemain berhak mendapatkan lencana baru berdasarkan aksinya.
func (b *Bot) checkAndAwardAchievements(playerID int64, chatID int64, playerName string, timeTaken float64) {
	// 1. Ambil semua lencana tipe 'achievement' dari database
	allAchievements, err := b.db.GetAchievementBadges()
	if err != nil {
		log.Printf("Failed to get all achievement badges: %v", err)
		return
	}

	// 2. Ambil semua lencana yang sudah dimiliki pemain
	playerBadges, err := b.db.GetPlayerBadges(playerID)
	if err != nil {
		log.Printf("Failed to get player badges for achievement check: %v", err)
		return
	}

	// Buat sebuah map untuk mengecek kepemilikan lencana dengan cepat
	playerHasBadge := make(map[int]bool)
	for _, badge := range playerBadges {
		playerHasBadge[badge.ID] = true
	}

	// 3. Loop melalui semua lencana achievement dan periksa satu per satu
	for _, achievement := range allAchievements {
		// Lewati jika pemain sudah punya lencana ini
		if playerHasBadge[achievement.ID] {
			continue
		}

		// Periksa kriteria untuk setiap lencana
		switch achievement.CriteriaType {
		case "guess_time":
			if timeTaken < float64(achievement.CriteriaValue) {
				// Pemain memenuhi syarat! Berikan lencana.
				err := b.db.AwardBadgeToPlayer(playerID, achievement.ID)
				if err == nil {
					// Kirim pesan selamat ke grup
					announcement := fmt.Sprintf(
						"🎉 <b>PENCAPAIAN TERBUKA!</b> 🎉\n\n%s mendapatkan lencana <b>%s %s</b>: <i>%s</i>",
						html.EscapeString(playerName),
						achievement.Emoji,
						achievement.Name,
						achievement.Description,
					)
					b.sendMessage(chatID, announcement, true)
				}
			}
		// case "total_guesses":
		// Nanti kita bisa tambahkan logika untuk lencana lain di sini
		}
	}
}