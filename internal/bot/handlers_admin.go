package bot

import (
	"log"
	"time"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleAdminCommand(message *tgbotapi.Message) {
	if message.From.ID != b.cfg.SuperAdminID {
		return
	}

	switch message.Command() {
	case "broadcast":
		go b.handleBroadcast(message, "private")
	case "broadcastgroup":
		go b.handleBroadcast(message, "group")
	}
}

// TANDA: Kode fungsi penuh untuk handleBroadcast yang sudah menggunakan localizer

func (b *Bot) handleBroadcast(message *tgbotapi.Message, chatType string) {
	// Untuk perintah admin, kita bisa tentukan bahasanya secara manual, misal "id"
	lang := "id"
	log.Printf("Broadcast initiated by admin for type: %s", chatType)

	caption := message.CommandArguments()
	photoFileID := ""

	if message.ReplyToMessage != nil && message.ReplyToMessage.Photo != nil && len(message.ReplyToMessage.Photo) > 0 {
		photoFileID = message.ReplyToMessage.Photo[len(message.ReplyToMessage.Photo)-1].FileID
	}

	if caption == "" && photoFileID == "" {
		b.sendMessage(message.Chat.ID, b.localizer.Get(lang, "broadcast_cannot_be_empty"), false)
		return
	}

	chatIDs, err := b.db.GetAllChatsByType(chatType)
	if err != nil {
		log.Printf("Failed to get chat IDs for broadcast: %v", err)
		b.sendMessage(message.Chat.ID, b.localizer.Get(lang, "broadcast_fetch_fail"), false)
		return
	}

	startMsg := b.localizer.Get(lang, "broadcast_starting")
	startMsg = strings.Replace(startMsg, "{count}", strconv.Itoa(len(chatIDs)), 1)
	startMsg = strings.Replace(startMsg, "{type}", chatType, 1)
	b.sendMessage(message.Chat.ID, startMsg, false)

	successCount := 0
	failCount := 0

	for _, chatID := range chatIDs {
		var err error
		if photoFileID != "" {
			photoMsg := tgbotapi.NewPhoto(chatID, tgbotapi.FileID(photoFileID))
			photoMsg.Caption = caption
			photoMsg.ParseMode = tgbotapi.ModeHTML
			_, err = b.api.Send(photoMsg)
		} else {
			err = b.sendMessage(chatID, caption, true)
		}

		if err != nil {
			log.Printf("Broadcast failed for chat ID %d: %v", chatID, err)
			failCount++
		} else {
			successCount++
		}
		time.Sleep(1 * time.Second)
	}

	summaryMsg := b.localizer.Get(lang, "broadcast_finished_summary")
	summaryMsg = strings.Replace(summaryMsg, "{success}", strconv.Itoa(successCount), 1)
	summaryMsg = strings.Replace(summaryMsg, "{fail}", strconv.Itoa(failCount), 1)
	b.sendMessage(message.Chat.ID, summaryMsg, false)
	log.Println("Broadcast finished.")
}