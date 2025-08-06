package db

import (
	"log"
	"strconv"
)

type Chat struct {
	ChatID int64  `json:"chat_id"`
	Type   string `json:"type"`
}

func (c *Client) GetOrCreateChat(chatID int64, chatType string) error {
	var results []Chat
	err := c.DB.From("chats").Select("chat_id").Eq("chat_id", strconv.FormatInt(chatID, 10)).Execute(&results)
	if err != nil {
		log.Printf("Error checking for chat: %v", err)
		return err
	}

	if len(results) > 0 {
		return nil
	}

	log.Printf("Chat not found. Creating new chat entry: %d (%s)", chatID, chatType)
	newChat := Chat{
		ChatID: chatID,
		Type:   chatType,
	}

	var newResults []Chat
	err = c.DB.From("chats").Insert(newChat).Execute(&newResults)
	if err != nil {
		log.Printf("Error creating new chat entry: %v", err)
		return err
	}

	log.Printf("Successfully created new chat entry: %d", chatID)
	return nil
}

func (c *Client) GetAllChatsByType(chatType string) ([]int64, error) {
	var results []Chat
	var err error

	// TANDA: Logika perbaikan dimulai di sini dengan sintaks yang benar
	if chatType == "group" {
		// Menggunakan .Filter() dengan operator "in" untuk mencari "group" ATAU "supergroup"
		err = c.DB.From("chats").Select("chat_id").Filter("type", "in", "(\"group\",\"supergroup\")").Execute(&results)
	} else {
		// Untuk chat pribadi, perilakunya tetap sama
		err = c.DB.From("chats").Select("chat_id").Eq("type", chatType).Execute(&results)
	}
	// TANDA: Logika perbaikan berakhir di sini

	if err != nil {
		log.Printf("Error fetching chats by type '%s': %v", chatType, err)
		return nil, err
	}

	var chatIDs []int64
	for _, chat := range results {
		chatIDs = append(chatIDs, chat.ChatID)
	}
	return chatIDs, nil
}