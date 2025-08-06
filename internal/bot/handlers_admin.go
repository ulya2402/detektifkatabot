package bot

import (
	"fmt"
	"log"
	"time"

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

func (b *Bot) handleBroadcast(message *tgbotapi.Message, chatType string) {
	log.Printf("Broadcast initiated by admin for type: %s", chatType)

	caption := message.CommandArguments()
	photoFileID := ""

	if message.ReplyToMessage != nil && message.ReplyToMessage.Photo != nil && len(message.ReplyToMessage.Photo) > 0 {
		photoFileID = message.ReplyToMessage.Photo[len(message.ReplyToMessage.Photo)-1].FileID
	}

	if caption == "" && photoFileID == "" {
		b.sendMessage(message.Chat.ID, "Broadcast message cannot be empty.", false)
		return
	}

	chatIDs, err := b.db.GetAllChatsByType(chatType)
	if err != nil {
		log.Printf("Failed to get chat IDs for broadcast: %v", err)
		b.sendMessage(message.Chat.ID, "Failed to fetch chat list.", false)
		return
	}

	b.sendMessage(message.Chat.ID, fmt.Sprintf("Starting broadcast to %d %s chats...", len(chatIDs), chatType), false)

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

	summary := fmt.Sprintf(
		"Broadcast finished.\nSuccess: %d\nFailed: %d",
		successCount,
		failCount,
	)
	b.sendMessage(message.Chat.ID, summary, false)
	log.Println("Broadcast finished.")
}