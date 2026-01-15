// kira_ai.go
package kira

import (
	"log"
	"math/rand/v2"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// AIRun is the main loop for kira AI responses
func (k *KiraBot) AIRun() error {
	log.Println("Starting AI response loop...")

	// Get current time
	currentTime := time.Now()
	log.Printf("AI Run started at: %s", currentTime.Format("2006-01-02 15:04:05"))

	// Get all chat IDs

	// Loop through all chats
	for _, chat := range k.chats {
		// Get the last 5 chat messages for this chat
		lastMessages := k.GetLastMessages(chat.Chats, 20)

		if len(lastMessages) == 0 {
			log.Printf("No messages found for chat")
			continue
		}
		// Check if the last message is from a user (not the bot)
		// and if it needs a response
		if len(lastMessages) > 0 {
			lastMsg := lastMessages[len(lastMessages)-1]

			if lastMsg.MessageID > int(chat.LastHelperScannedMsg)+15 {

				log.Printf("Scanning new, lastMsg: %v , lastScanned %v\n", lastMsg.MessageID, chat.LastHelperScannedMsg)

				k.generateInfoHelper(lastMessages, chat, lastMsg.MessageID)
			}

			log.Printf("After scanning new")

			shouldRespond, shouldProvideExtraStory := k.shouldRespondToMessage(lastMsg, lastMessages)

			if shouldRespond {

				err := k.sendTypingAction(chat.ChatId)
				if err != nil {
					log.Println("Send Typing Action failed, skipping response generation")
					continue
				}
				response := k.generateAIResponse(lastMessages, chat, shouldProvideExtraStory)
				if response == "" || response == "\"\"\n" || response == "\"\"" {
					log.Println("Marking message as not respond to.")
					k.markLastMessageAsShouldNotRespondTo(chat, lastMsg)
					continue
				}

				if response == "TIMEOUT" {
					continue
				}

				// Send the response
				//k.SendResponse(chat.ChatId, response)
				k.sendResponseWithSplitting(chat.ChatId, response)
			} else {
				log.Printf("Skipping response for chat")
			}
		}
	}

	log.Println("AI Run completed successfully")
	return nil
}

func (k *KiraBot) sendResponseWithSplitting(chatId int64, response string) {
	messages := k.splitMessage(response)

	for _, msg := range messages {
		// Send typing action to indicate bot is "typing"
		k.sendTypingAction(chatId)

		// Calculate per-character delay (30-50ms per char)
		charDelay := time.Duration(120+rand.IntN(50)) * time.Millisecond // Use IntN from math/rand/v2

		// Simulate typing by waiting per character
		for range []rune(msg) {
			time.Sleep(charDelay)
		}

		// Send the message
		k.SendResponse(chatId, msg)
	}
}

func (k *KiraBot) splitMessage(message string) []string {
	// Define what we consider "long" - adjust this threshold as needed
	const longMessageThreshold = 150

	// Check if message is long and should be split (50% chance)
	isLong := utf8.RuneCountInString(message) > longMessageThreshold
	shouldSplitLong := isLong && rand.Float64() < 0.5

	// Check if message ends with smiley and should be split
	shouldSplitSmiley := k.shouldSplitSmiley(message)

	// If no splitting needed, return original message
	if !shouldSplitLong && !shouldSplitSmiley {
		return []string{message}
	}

	var messages []string
	currentMessage := message

	// First, handle smiley splitting if needed
	if shouldSplitSmiley {
		mainPart, smiley := k.extractEndingSmiley(currentMessage)
		currentMessage = mainPart
		// We'll add the smiley as a separate message later
		defer func() {
			messages = append(messages, smiley)
		}()
	}

	// Then handle long message splitting if needed
	if shouldSplitLong && utf8.RuneCountInString(currentMessage) > longMessageThreshold {
		parts := k.splitLongMessage(currentMessage)
		messages = append(messages, parts...)
	} else {
		messages = append(messages, currentMessage)
	}

	return messages
}

func (k *KiraBot) shouldSplitSmiley(message string) bool {
	if !k.endsWithSmiley(message) {
		return false
	}

	// Random choice to split smiley
	return rand.Float64() < 0.5
}

func (k *KiraBot) endsWithSmiley(message string) bool {
	// Common emoji patterns - you can expand this list
	emojiPattern := `[\x{1F600}-\x{1F64F}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{1F1E0}-\x{1F1FF}]|[\x{2600}-\x{26FF}]|[\x{2700}-\x{27BF}]|😊|😉|😄|😃|😀|😁|🙂|🙃|😍|😘|😗|😚|😙|🤗|🤔|😋|😛|😜|😝|🤤|😴|😪|😇|🤠|🤡|🤢|🤧|🤥|🤫|🤭|🧐|🤓|😈|👿|😡|😠|🤬|😤|😤|😩|😫|😵|🤯|🤪|😦|😧|😮|😯|😲|😱|🤨|🧐`

	// Also check for text smileys
	textSmileyPattern := `:\)|:\(|:D|:P|:p|;\)|;D|<3|:\*|:-\)|:-\(|:-D|:-P|:-p|;-\)|;-D`

	// Combine patterns
	combinedPattern := "(" + emojiPattern + "|" + textSmileyPattern + ")\\s*$"

	matched, err := regexp.MatchString(combinedPattern, message)
	if err != nil {
		return false
	}

	return matched
}

func (k *KiraBot) extractEndingSmiley(message string) (string, string) {
	// Regex to capture the ending smiley/emoji
	emojiPattern := `[\x{1F600}-\x{1F64F}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{1F1E0}-\x{1F1FF}]|[\x{2600}-\x{26FF}]|[\x{2700}-\x{27BF}]|😊|😉|😄|😃|😀|😁|🙂|🙃|😍|😘|😗|😚|😙|🤗|🤔|😋|😛|😜|😝|🤤|😴|😪|😇|🤠|🤡|🤢|🤧|🤥|🤫|🤭|🧐|🤓|😈|👿|😡|😠|🤬|😤|😤|😩|😫|😵|🤯|🤪|😦|😧|😮|😯|😲|😱|🤨|🧐`
	textSmileyPattern := `:\)|:\(|:D|:P|:p|;\)|;D|<3|:\*|:-\)|:-\(|:-D|:-P|:-p|;-\)|;-D`

	combinedPattern := "(.*)(" + emojiPattern + "|" + textSmileyPattern + ")\\s*$"

	re := regexp.MustCompile(combinedPattern)
	matches := re.FindStringSubmatch(message)

	if len(matches) >= 3 {
		mainPart := strings.TrimSpace(matches[1])
		smiley := matches[2]
		return mainPart, smiley
	}

	return message, ""
}

func (k *KiraBot) markLastMessageAsShouldNotRespondTo(chat CompleteChat, lastMsg ChatMessage) {
	k.mu.Lock()
	defer k.mu.Unlock()

	updatedMsg := lastMsg
	updatedMsg.ShouldNotRespond = true
	k.chats[chat.ChatId].Chats[updatedMsg.MessageID] = updatedMsg
}
func (k *KiraBot) generateInfoHelper(messages []ChatMessage, completeChat CompleteChat, lastMSGID int) {

	updatedChat := k.chats[completeChat.ChatId]
	oldInfo := updatedChat.Infos

	newInfo, err := k.callGeminiHelper(completeChat.Infos, messages, completeChat)
	k.mu.Lock()
	defer k.mu.Unlock()
	if err != nil {
		if strings.Contains(err.Error(), "blocked:") {
			log.Printf("blocked:")

			sanitizer := NewSimpleSanitizer()

			// Clean messages before processing
			cleanedMessages := sanitizer.CleanChatMessages(messages)
			// Clean the form
			cleanedForm := sanitizer.CleanKiraHelperForm(completeChat.Infos)
			// Now use cleaned data with Gemini
			newInfo, err = k.callGeminiHelper(cleanedForm, cleanedMessages, completeChat)
			if err != nil {
				log.Printf("Error after cleaning: %v", err)

			}

		} else {

			if strings.Contains(err.Error(), "limit") {

				log.Printf("Error: %v", err)
				updatedChat.Infos = oldInfo
				// Dont update last msg scanned
				k.chats[completeChat.ChatId] = updatedChat
				k.saveChatInfo(completeChat.ChatId, oldInfo)
				if saveErr := k.saveLastHelperScannedMsg(completeChat.ChatId, int64(lastMSGID)); saveErr != nil {
					log.Printf("Error saving last scanned msg after error: %v", saveErr)
				}
				return

			}

			log.Printf("Error: %v", err)
			updatedChat.Infos = oldInfo
			updatedChat.LastHelperScannedMsg = int64(lastMSGID)
			k.chats[completeChat.ChatId] = updatedChat
			k.saveChatInfo(completeChat.ChatId, oldInfo)
			if saveErr := k.saveLastHelperScannedMsg(completeChat.ChatId, int64(lastMSGID)); saveErr != nil {
				log.Printf("Error saving last scanned msg after error: %v", saveErr)
			}
			return
		}
	}

	updatedChat.Infos = newInfo
	updatedChat.LastHelperScannedMsg = int64(lastMSGID)
	k.chats[completeChat.ChatId] = updatedChat
	k.saveChatInfo(completeChat.ChatId, newInfo)
	if err := k.saveLastHelperScannedMsg(completeChat.ChatId, int64(lastMSGID)); err != nil {
		log.Printf("Error saving last scanned msg: %v", err)
	}

}

func (k *KiraBot) generateAIResponse(messages []ChatMessage, completeChat CompleteChat, shouldProvideExtraStory bool) string {

	response, err := k.callGeminiTalk(completeChat.Infos, messages, shouldProvideExtraStory, completeChat)
	if err != nil {
		if strings.Contains(err.Error(), "block:") {
			log.Printf("Block encountered, trying fallback with cleaning infos")

			sanitizer := NewSimpleSanitizer()

			// Clean messages before processing
			cleanedMessages := sanitizer.CleanChatMessages(messages)
			cleanedForm := sanitizer.CleanKiraHelperForm(completeChat.Infos)

			response, err = k.callGeminiTalk(cleanedForm, cleanedMessages, shouldProvideExtraStory, completeChat)
			if err != nil {
				if strings.Contains(err.Error(), "block:") {
					log.Printf("block: encountered again, trying complete fallback")
					// Create empty KiraHelperForm instead of using zero value
					emptyForm := createEmptyKiraHelperForm()

					// Complete new with story - use empty messages and force story mode
					response, err = k.callGeminiTalk(emptyForm, []ChatMessage{}, true, completeChat)
					if err != nil {
						log.Printf("Error new with story: %v", err)
						return ""
					}
					return response
				}
				log.Printf("Error in fallback callGeminiTalk: %v", err)
				return ""
			}
			return response
		}

		log.Printf("Error callGeminiTalk: %v", err)
		return ""
	}

	return response
}

// truncateText truncates text to a maximum length for logging
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

func (k *KiraBot) splitLongMessage(message string) []string {
	// Try to split on natural boundaries like sentences, then clauses
	var parts []string

	// First try to split on sentence boundaries
	sentences := k.splitOnSentences(message)

	if len(sentences) > 1 {
		// If we have multiple sentences, group them reasonably
		var currentPart strings.Builder
		const maxPartLength = 120

		for _, sentence := range sentences {
			if currentPart.Len() > 0 && currentPart.Len()+len(sentence) > maxPartLength {
				// Current part would be too long, start a new one
				parts = append(parts, strings.TrimSpace(currentPart.String()))
				currentPart.Reset()
			}

			if currentPart.Len() > 0 {
				currentPart.WriteString(" ")
			}
			currentPart.WriteString(sentence)
		}

		if currentPart.Len() > 0 {
			parts = append(parts, strings.TrimSpace(currentPart.String()))
		}
	} else {
		// Single long sentence, split on commas or other punctuation
		parts = k.splitOnPunctuation(message)
	}

	// If we still have only one part, do a simple word-based split
	if len(parts) <= 1 {
		parts = k.splitOnWords(message, 100)
	}

	return parts
}

// splitOnSentences splits text on sentence boundaries
func (k *KiraBot) splitOnSentences(text string) []string {
	// Simple sentence splitting on . ! ?
	re := regexp.MustCompile(`([.!?]+)\s+`)
	parts := re.Split(text, -1)

	// Remove empty parts
	var result []string
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// splitOnPunctuation splits on commas and other punctuation
func (k *KiraBot) splitOnPunctuation(text string) []string {
	// Split on commas, semicolons, etc.
	re := regexp.MustCompile(`([,;]+)\s+`)
	parts := re.Split(text, -1)

	if len(parts) > 1 {
		var result []string
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	}

	return []string{text}
}

// splitOnWords splits text by words when other methods fail
func (k *KiraBot) splitOnWords(text string, maxLength int) []string {
	words := strings.Fields(text)
	var parts []string
	var currentPart strings.Builder

	for _, word := range words {
		if currentPart.Len() > 0 && currentPart.Len()+len(word)+1 > maxLength {
			parts = append(parts, strings.TrimSpace(currentPart.String()))
			currentPart.Reset()
		}

		if currentPart.Len() > 0 {
			currentPart.WriteString(" ")
		}
		currentPart.WriteString(word)
	}

	if currentPart.Len() > 0 {
		parts = append(parts, strings.TrimSpace(currentPart.String()))
	}

	return parts
}
