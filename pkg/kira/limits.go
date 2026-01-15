package kira

import (
	"log"
	"time"
)

const (
	dailyLimit = 30 // Here is the daily chat limit per chat! :-)
)

// checkDailyLimit checks if the chat is within daily message limits
func (k *KiraBot) checkDailyLimit(chat CompleteChat) bool {
	today := time.Now().Format("2006-01-02")

	// Reset counter if it's a new day
	if chat.LastMessageDate != today {
		return true // Allow message (counter will be reset)
	}

	// Check if under limit
	limit := chat.DailyLimit
	if limit == 0 {
		limit = dailyLimit
	}

	return chat.DailyMessageCount < limit
}

// incrementDailyCounter increments the daily message counter
func (k *KiraBot) incrementDailyCounter(chatID int64) error {
	k.mu.Lock()

	chat := k.chats[chatID]
	today := time.Now().Format("2006-01-02")

	if chat.LastMessageDate != today {
		// New day, reset counter
		chat.DailyMessageCount = 1
		chat.LastMessageDate = today
		log.Printf("Reset daily counter for chat %d (new day: %s)", chatID, today)
	} else {
		// Same day, increment counter
		chat.DailyMessageCount++
	}

	// Set default limit if not set
	if chat.DailyLimit == 0 {
		chat.DailyLimit = dailyLimit
	}

	k.chats[chatID] = chat

	log.Printf("Daily message count for chat %d: %d/%d", chatID, chat.DailyMessageCount, chat.DailyLimit)
	k.mu.Unlock()
	// Save to file
	return k.saveChatInfo(chatID, chat.Infos)
}
