package kira

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ChatMessage represents a stored chat message
type ChatMessage struct {
	MessageID        int    `json:"message_id"`
	Text             string `json:"text"`
	SenderID         int64  `json:"sender_id"`
	SenderName       string `json:"sender_name"`
	Username         string `json:"username,omitempty"`
	Timestamp        int64  `json:"timestamp"`
	Date             string `json:"date"`
	ChatID           int64  `json:"chat_id"`
	ChatTitle        string `json:"chat_title,omitempty"`
	MessageType      string `json:"message_type"`
	IsBot            bool   `json:"is_bot"`
	ShouldNotRespond bool   `json:"should_not_respond"`
}

type CompleteChat struct {
	LastHelperScannedTime int64
	LastHelperScannedMsg  int64
	ChatId                int64
	Infos                 KiraHelperForm
	Chats                 map[int]ChatMessage // key is MessageID
	DailyMessageCount     int                 `json:"daily_message_count"`
	LastMessageDate       string              `json:"last_message_date"` // "2025-01-15"
	DailyLimit            int                 `json:"daily_limit"`       // Default 30
}

type KiraBot struct {
	api          *tgbotapi.BotAPI
	updateCfg    tgbotapi.UpdateConfig
	stopChan     chan struct{}
	wg           sync.WaitGroup
	mu           sync.Mutex
	running      bool
	chats        map[int64]CompleteChat // key is ChatID (changed from int to int64)
	llmKey       string                 // key for LLM needed later
	AllowedUsers []string
}

// NewKiraBot creates a new instance of KiraBot
func NewKiraBot(token string, llmkey string) (*KiraBot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	// Optional: Enable debug mode
	// bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Load the last processed update ID from storage
	lastUpdateID := loadLastUpdateID()

	allowedUsers, err := loadAllowedUsers()
	if err != nil {
		return nil, fmt.Errorf("failed to load allowed users: %w", err)
	}

	kiraBot := &KiraBot{
		llmKey:       llmkey,
		api:          bot,
		updateCfg:    tgbotapi.NewUpdate(lastUpdateID),
		stopChan:     make(chan struct{}),
		chats:        make(map[int64]CompleteChat), // Initialize the chats map
		AllowedUsers: allowedUsers,
	}

	// Sync chats at startup
	if err := kiraBot.syncChatsFromStorage(); err != nil {
		log.Printf("Warning: Failed to sync chats from storage: %v", err)
	}

	return kiraBot, nil
}

// loadAllowedUsers reads the allowed_users.txt file and returns a slice of allowed usernames
func loadAllowedUsers() ([]string, error) {
	file, err := os.Open("allowed_users.txt")
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Warning: allowed_users.txt not found, using default allowed users")
			return []string{"karl1b"}, nil // Return default users if file doesn't exist
		}
		return nil, fmt.Errorf("error opening allowed_users.txt: %w", err)
	}
	defer file.Close()

	var users []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") { // Skip empty lines and comments
			users = append(users, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading allowed_users.txt: %w", err)
	}

	if len(users) == 0 {
		log.Fatalf("Warning: No users found in allowed_users.txt")
		return []string{"karl1b"}, nil
	}

	log.Printf("Loaded %d allowed users from allowed_users.txt", len(users))
	return users, nil
}

// syncChatsFromStorage loads all existing chat data from storage into memory
func (k *KiraBot) syncChatsFromStorage() error {
	chatsDir := "chats"

	// Check if chats directory exists
	if _, err := os.Stat(chatsDir); os.IsNotExist(err) {
		log.Println("No chats directory found, starting fresh")
		return nil
	}

	// Walk through all chat directories
	return filepath.WalkDir(chatsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip if not a directory or if it's the root chats directory
		if !d.IsDir() || path == chatsDir {
			return nil
		}

		// Extract chat ID from directory name
		chatIDStr := d.Name()
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil {
			log.Printf("Skipping invalid chat directory: %s", chatIDStr)
			return nil
		}

		// Load chat messages from file
		chatFile := filepath.Join(path, "chat.jsonl")
		if err := k.loadChatFromFile(chatID, chatFile); err != nil {
			log.Printf("Warning: Failed to load chat %d: %v", chatID, err)
		}

		if chat, exists := k.chats[chatID]; exists {
			chat.ChatId = chatID // Set the ChatId
			k.chats[chatID] = chat
		}

		// Load Infos from file info.jsonl
		infoFile := filepath.Join(path, "info.jsonl")
		if err := k.loadChatInfoFromFile(chatID, infoFile); err != nil {
			log.Printf("Warning: Failed to load chat info %d: %v", chatID, err)
		}

		lastScannedFile := filepath.Join(path, "lastscannedmsg.txt")
		if err := k.loadLastHelperScannedMsg(chatID, lastScannedFile); err != nil {
			log.Printf("Warning: Failed to load last scanned msg for chat %d: %v", chatID, err)
		}

		return nil
	})
}

// loadChatInfoFromFile loads chat info from info.jsonl file
func (k *KiraBot) loadChatInfoFromFile(chatID int64, filePath string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	// Get existing chat or create new one
	chat, exists := k.chats[chatID]
	if !exists {
		chat = CompleteChat{
			ChatId:     chatID,
			Chats:      make(map[int]ChatMessage),
			DailyLimit: 30, // Set default limit
		}
	}

	// Check if info file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// File doesn't exist, generate it with Go's default values
		log.Printf("Info file doesn't exist for chat %d, creating with default values", chatID)

		defaultInfo := KiraHelperForm{
			Kira: Character{
				EchterName:       "Kira",
				Beziehungsstatus: "Single",
				FlirtLevel:       "hoch",
			},
			User: Character{
				EchterName:               "",
				Alter:                    0,
				Beruf:                    "",
				Wohnort:                  "",
				Beziehungsstatus:         "",
				Lieblingsfarbe:           "",
				FlirtLevel:               "",
				Interessen:               []string{},
				TraeumeUndWuensche:       []string{},
				GespeicherteErinnerungen: []string{},
				AktuelleThemen:           []string{},
				TabuThemen:               []string{},
				PersonenImLeben:          []PersonImLeben{},
			},
		}
		// Create default KiraHelperForm with zero values

		chat.ChatId = chatID
		chat.Infos = defaultInfo
		k.chats[chatID] = chat

		// Save the default info to file
		return k.saveChatInfo(chatID, defaultInfo)
	}

	// File exists, load it
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open info file: %v", err)
	}
	defer file.Close()

	// Read the JSON from the file (assuming it's a single JSON object, not JSONL)
	var info KiraHelperForm
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&info); err != nil {
		return fmt.Errorf("failed to decode info file: %v", err)
	}

	chat.Infos = info
	k.chats[chatID] = chat

	log.Printf("Loaded chat info for chat %d", chatID)
	return nil
}

// saveChatInfo saves chat info to info.jsonl file
func (k *KiraBot) saveChatInfo(chatID int64, info KiraHelperForm) error {
	// Create chats directory if it doesn't exist
	chatsDir := "chats"
	if err := os.MkdirAll(chatsDir, 0755); err != nil {
		return fmt.Errorf("failed to create chats directory: %v", err)
	}

	// Create chat-specific directory
	chatDir := filepath.Join(chatsDir, fmt.Sprintf("%d", chatID))
	if err := os.MkdirAll(chatDir, 0755); err != nil {
		return fmt.Errorf("failed to create chat directory: %v", err)
	}

	// Info file path
	infoFile := filepath.Join(chatDir, "info.jsonl")

	// Convert info to JSON
	infoJSON, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal chat info: %v", err)
	}

	// Write to file (overwrite existing)
	if err := os.WriteFile(infoFile, infoJSON, 0644); err != nil {
		return fmt.Errorf("failed to write info file: %v", err)
	}

	log.Printf("Saved chat info for chat %d", chatID)
	return nil
}

// loadChatFromFile loads a single chat's messages from its JSONL file
func (k *KiraBot) loadChatFromFile(chatID int64, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, initialize empty chat
			k.mu.Lock()
			if _, exists := k.chats[chatID]; !exists {
				k.chats[chatID] = CompleteChat{
					Chats: make(map[int]ChatMessage),
				}
			}
			k.mu.Unlock()
			return nil
		}
		return err
	}
	defer file.Close()

	k.mu.Lock()
	defer k.mu.Unlock()

	chat, exists := k.chats[chatID]
	if !exists {
		chat = CompleteChat{
			ChatId: chatID,
			Chats:  make(map[int]ChatMessage),
		}
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var msg ChatMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			log.Printf("Warning: Failed to parse message in chat %d: %v", chatID, err)
			continue
		}

		chat.Chats[msg.MessageID] = msg
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	chat.ChatId = chatID
	k.chats[chatID] = chat

	log.Printf("Loaded %d messages for chat %d", len(chat.Chats), chatID)
	return nil
}

// updateChatInMemory updates the in-memory chat data
func (k *KiraBot) updateChatInMemory(msg ChatMessage) {
	k.mu.Lock()
	defer k.mu.Unlock()

	chat, exists := k.chats[msg.ChatID]
	if !exists {
		chat = CompleteChat{
			ChatId: msg.ChatID,
			Chats:  make(map[int]ChatMessage),
		}
	}

	chat.Chats[msg.MessageID] = msg

	k.chats[msg.ChatID] = chat
}

// GetLastMessages returns the last N messages from a chat
func (k *KiraBot) GetLastMessages(chats map[int]ChatMessage, count int) []ChatMessage {
	k.mu.Lock()
	defer k.mu.Unlock()

	// Convert map to slice and sort by message ID
	messages := make([]ChatMessage, 0, len(chats))
	for _, msg := range chats {
		messages = append(messages, msg)
	}

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].MessageID < messages[j].MessageID
	})

	// Return last N messages
	if len(messages) <= count {
		return messages
	}
	return messages[len(messages)-count:]
}

// GetAllChatIDs returns all chat IDs that have messages

// Run starts the main bot loop
func (k *KiraBot) Run() error {
	k.mu.Lock()
	if k.running {
		k.mu.Unlock()
		return nil
	}
	k.running = true
	k.mu.Unlock()

	// Ensure we clean up on exit
	defer func() {
		k.mu.Lock()
		k.running = false
		k.mu.Unlock()
	}()

	k.updateCfg.Timeout = 60 // Long polling timeout

	// Create a context for better cancellation handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start getting updates
	updates := k.api.GetUpdatesChan(k.updateCfg)

	log.Println("Bot is running and listening for updates...")

	// Handle updates
	for {
		select {
		case update, ok := <-updates:
			if !ok {
				log.Println("Updates channel closed")
				return nil
			}

			// Save the update ID to ensure we don't process it again
			if update.UpdateID >= k.updateCfg.Offset {
				saveLastUpdateID(update.UpdateID + 1)
				k.updateCfg.Offset = update.UpdateID + 1
			}

			if update.Message != nil {
				k.wg.Add(1)
				go func(msg *tgbotapi.Message) {
					defer k.wg.Done()
					k.handleMessage(msg)
				}(update.Message)
			}

		case <-k.stopChan:
			log.Println("Stop signal received")
			k.stopUpdatesGracefully()
			k.wg.Wait() // Wait for all message handlers to complete
			return nil

		case <-ctx.Done():
			log.Println("Context cancelled")
			k.stopUpdatesGracefully()
			k.wg.Wait()
			return ctx.Err()
		}
	}
}

func (k *KiraBot) SendResponse(chatId int64, response string) {

	// Send response
	if err := k.sendMessage(chatId, response); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

// handleMessage processes incoming messages
func (k *KiraBot) handleMessage(message *tgbotapi.Message) {

	var ok bool
	if slices.Contains(k.AllowedUsers, message.From.UserName) {
		ok = true
	}
	if !ok {
		log.Printf("Username: %v not in Allowed users", message)
		k.sendMessage(message.Chat.ID, "Sorry leider musst du dich erst von Karl freischalten lassen. :)")
		return
	}

	log.Printf("Received message from %s (%d): %s",
		message.From.UserName,
		message.From.ID,
		message.Text)

	// Store the message
	if err := k.storeMessage(message); err != nil {
		log.Printf("Error storing message: %v", err)
	}

}

// sendTypingAction shows "typing..." indicator
func (k *KiraBot) sendTypingAction(chatID int64) error {
	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	if _, err := k.api.Request(action); err != nil {
		log.Printf("Error sending typing action: %v", err)
		return err
	}

	return nil
}

// sendMessage sends a text message
func (k *KiraBot) sendMessage(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	sentMsg, err := k.api.Send(msg)
	if err != nil {
		return err
	}

	// Store the bot's response as well
	if err := k.storeBotMessage(&sentMsg, text); err != nil {
		log.Printf("Error storing bot message: %v", err)
	}

	return nil
}

// Shutdown gracefully stops the bot
func (k *KiraBot) Shutdown() {
	k.mu.Lock()
	if !k.running {
		k.mu.Unlock()
		return
	}
	k.mu.Unlock()

	close(k.stopChan)
	log.Println("Kira bot shutdown complete")
}

// stopUpdatesGracefully stops receiving updates without panicking
func (k *KiraBot) stopUpdatesGracefully() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic during stop: %v", r)
		}
	}()

	k.api.StopReceivingUpdates()
}

// Simple file-based storage for update ID
const updateIDFile = "last_update_id.txt"

func loadLastUpdateID() int {
	data, err := os.ReadFile(updateIDFile)
	if err != nil {
		log.Printf("Could not read last update ID (starting fresh): %v", err)
		return 0
	}

	updateID := 0
	if _, err := fmt.Sscanf(string(data), "%d", &updateID); err != nil {
		log.Printf("Could not parse last update ID (starting fresh): %v", err)
		return 0
	}

	log.Printf("Resuming from update ID: %d", updateID)
	return updateID
}

func saveLastUpdateID(updateID int) {
	data := fmt.Sprintf("%d", updateID)
	if err := os.WriteFile(updateIDFile, []byte(data), 0644); err != nil {
		log.Printf("Could not save last update ID: %v", err)
	}
}

// storeMessage saves a user message to the chat file
func (k *KiraBot) storeMessage(message *tgbotapi.Message) error {
	chatMsg := ChatMessage{
		MessageID:   message.MessageID,
		Text:        message.Text,
		SenderID:    message.From.ID,
		SenderName:  fmt.Sprintf("%s %s", message.From.FirstName, message.From.LastName),
		Username:    message.From.UserName,
		Timestamp:   int64(message.Date),
		Date:        time.Unix(int64(message.Date), 0).Format("2006-01-02 15:04:05"),
		ChatID:      message.Chat.ID,
		ChatTitle:   message.Chat.Title,
		MessageType: "text",
		IsBot:       message.From.IsBot,
	}

	// Handle different message types
	if message.Photo != nil {
		chatMsg.MessageType = "photo"
		chatMsg.Text = message.Caption
	} else if message.Document != nil {
		chatMsg.MessageType = "document"
		chatMsg.Text = fmt.Sprintf("[Document: %s] %s", message.Document.FileName, message.Caption)
	} else if message.Audio != nil {
		chatMsg.MessageType = "audio"
		chatMsg.Text = fmt.Sprintf("[Audio] %s", message.Caption)
	} else if message.Video != nil {
		chatMsg.MessageType = "video"
		chatMsg.Text = fmt.Sprintf("[Video] %s", message.Caption)
	} else if message.Voice != nil {
		chatMsg.MessageType = "voice"
		chatMsg.Text = "[Voice message]"
	} else if message.Sticker != nil {
		chatMsg.MessageType = "sticker"
		chatMsg.Text = fmt.Sprintf("[Sticker: %s]", message.Sticker.Emoji)
	} else if message.Location != nil {
		chatMsg.MessageType = "location"
		chatMsg.Text = fmt.Sprintf("[Location: %f, %f]", message.Location.Latitude, message.Location.Longitude)
	} else if message.Contact != nil {
		chatMsg.MessageType = "contact"
		chatMsg.Text = fmt.Sprintf("[Contact: %s %s, %s]", message.Contact.FirstName, message.Contact.LastName, message.Contact.PhoneNumber)
	}

	// Update in-memory chat data
	k.updateChatInMemory(chatMsg)

	return k.saveChatMessage(chatMsg)
}

// storeBotMessage saves the bot's response message
func (k *KiraBot) storeBotMessage(message *tgbotapi.Message, text string) error {
	chatMsg := ChatMessage{
		MessageID:   message.MessageID,
		Text:        text,
		SenderID:    message.From.ID,
		SenderName:  message.From.FirstName,
		Username:    message.From.UserName,
		Timestamp:   int64(message.Date),
		Date:        time.Unix(int64(message.Date), 0).Format("2006-01-02 15:04:05"),
		ChatID:      message.Chat.ID,
		ChatTitle:   message.Chat.Title,
		MessageType: "text",
		IsBot:       true,
	}

	// Update in-memory chat data
	k.updateChatInMemory(chatMsg)

	return k.saveChatMessage(chatMsg)
}

// saveChatMessage saves a message to the appropriate chat file
func (k *KiraBot) saveChatMessage(msg ChatMessage) error {
	// Create chats directory if it doesn't exist
	chatsDir := "chats"
	if err := os.MkdirAll(chatsDir, 0755); err != nil {
		return fmt.Errorf("failed to create chats directory: %v", err)
	}

	// Create chat-specific directory
	chatDir := filepath.Join(chatsDir, fmt.Sprintf("%d", msg.ChatID))
	if err := os.MkdirAll(chatDir, 0755); err != nil {
		return fmt.Errorf("failed to create chat directory: %v", err)
	}

	// Chat file path
	chatFile := filepath.Join(chatDir, "chat.jsonl")

	// Convert message to JSON
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	// Append to file (create if doesn't exist)
	file, err := os.OpenFile(chatFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open chat file: %v", err)
	}
	defer file.Close()

	// Write JSON line
	if _, err := file.WriteString(string(msgJSON) + "\n"); err != nil {
		return fmt.Errorf("failed to write message: %v", err)
	}

	return nil
}

// loadLastHelperScannedMsg loads the last helper scanned message ID from file
func (k *KiraBot) loadLastHelperScannedMsg(chatID int64, filePath string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	// Get existing chat or create new one
	chat, exists := k.chats[chatID]
	if !exists {
		chat = CompleteChat{
			ChatId: chatID,
			Chats:  make(map[int]ChatMessage),
		}
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// File doesn't exist, set default value (0)
		chat.LastHelperScannedMsg = 0
		k.chats[chatID] = chat
		log.Printf("Last scanned msg file doesn't exist for chat %d, setting to 0", chatID)
		return nil
	}

	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read last scanned msg file: %v", err)
	}

	// Parse the message ID
	msgIDStr := strings.TrimSpace(string(data))
	if msgIDStr == "" {
		chat.LastHelperScannedMsg = 0
	} else {
		msgID, err := strconv.ParseInt(msgIDStr, 10, 64)
		if err != nil {
			log.Printf("Warning: Invalid message ID in last scanned file for chat %d: %s", chatID, msgIDStr)
			chat.LastHelperScannedMsg = 0
		} else {
			chat.LastHelperScannedMsg = msgID
		}
	}

	k.chats[chatID] = chat
	log.Printf("Loaded last scanned msg for chat %d: %d", chatID, chat.LastHelperScannedMsg)
	return nil
}

// saveLastHelperScannedMsg saves the last helper scanned message ID to file
func (k *KiraBot) saveLastHelperScannedMsg(chatID int64, msgID int64) error {
	// Create chats directory if it doesn't exist
	chatsDir := "chats"
	if err := os.MkdirAll(chatsDir, 0755); err != nil {
		return fmt.Errorf("failed to create chats directory: %v", err)
	}

	// Create chat-specific directory
	chatDir := filepath.Join(chatsDir, fmt.Sprintf("%d", chatID))
	if err := os.MkdirAll(chatDir, 0755); err != nil {
		return fmt.Errorf("failed to create chat directory: %v", err)
	}

	// File path
	filePath := filepath.Join(chatDir, "lastscannedmsg.txt")

	// Write message ID to file
	data := fmt.Sprintf("%d", msgID)
	if err := os.WriteFile(filePath, []byte(data), 0644); err != nil {
		return fmt.Errorf("failed to write last scanned msg file: %v", err)
	}

	log.Printf("Saved last scanned msg for chat %d: %d", chatID, msgID)
	return nil
}
