package kira

import (
	"log"
	"regexp"
	"strings"
	"unicode"
)

// FilterItem represents a single filter entry (word or phrase)
type FilterItem struct {
	Original   string // Original text as provided
	Normalized string // Normalized version for matching
	IsPhrase   bool   // True if this contains multiple words
}

// SimpleSanitizer handles content sanitization without LLM calls
type SimpleSanitizer struct {
	filterItems []FilterItem
	// Precompiled regex for better performance with phrases
	phraseRegexes []*regexp.Regexp
}

// NewSimpleSanitizer creates a new sanitizer with predefined bad words and phrases
func NewSimpleSanitizer() *SimpleSanitizer {
	sanitizer := &SimpleSanitizer{
		filterItems:   make([]FilterItem, 0),
		phraseRegexes: make([]*regexp.Regexp, 0),
	}

	// Add default bad words and phrases
	defaultItems := []string{
		// Single words
		"fotze",
		"hure",
		"fick",
		"arschfotze",
		"pussy",

		// Multi-word phrases
		"tank man",

		// Add more as needed
	}

	sanitizer.AddFilterItems(defaultItems)
	return sanitizer
}

// normalize converts text to lowercase and normalizes whitespace
func (s *SimpleSanitizer) normalize(text string) string {
	if text == "" {
		return ""
	}

	var builder strings.Builder
	var lastWasSpace bool

	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastWasSpace = false
		} else if unicode.IsSpace(r) || unicode.IsPunct(r) {
			// Replace punctuation and multiple spaces with single space
			if !lastWasSpace {
				builder.WriteRune(' ')
				lastWasSpace = true
			}
		}
	}

	// Trim leading/trailing spaces
	return strings.TrimSpace(builder.String())
}

// createPhraseRegex creates a regex pattern for phrase matching
func (s *SimpleSanitizer) createPhraseRegex(phrase string) *regexp.Regexp {
	// Escape special regex characters and replace spaces with flexible whitespace/punctuation pattern
	escaped := regexp.QuoteMeta(phrase)
	// Allow any amount of whitespace or punctuation between words
	pattern := strings.ReplaceAll(escaped, `\ `, `[\s\p{P}]+`)
	// Add word boundaries
	pattern = `\b` + pattern + `\b`

	regex, err := regexp.Compile(`(?i)` + pattern) // (?i) makes it case-insensitive
	if err != nil {
		log.Printf("Error compiling regex for phrase '%s': %v", phrase, err)
		return nil
	}
	return regex
}

// containsBadContent checks if text contains any filtered words or phrases
func (s *SimpleSanitizer) containsBadContent(text string) bool {
	if text == "" {
		return false
	}

	normalizedText := s.normalize(text)

	// Check against all filter items
	for i, item := range s.filterItems {
		if item.IsPhrase {
			// Use regex for phrase matching
			if i < len(s.phraseRegexes) && s.phraseRegexes[i] != nil {
				if s.phraseRegexes[i].MatchString(text) {
					return true
				}
			}
		} else {
			// Simple word boundary check for single words
			if s.matchesWord(normalizedText, item.Normalized) {
				return true
			}
		}
	}

	return false
}

// matchesWord checks if a single word matches with word boundaries
func (s *SimpleSanitizer) matchesWord(normalizedText, normalizedWord string) bool {
	// Check for word boundaries to avoid false positives
	textWithSpaces := " " + normalizedText + " "
	wordWithSpaces := " " + normalizedWord + " "

	return strings.Contains(textWithSpaces, wordWithSpaces)
}

// AddFilterItem adds a single word or phrase to the filter
func (s *SimpleSanitizer) AddFilterItem(item string) {
	if item == "" {
		return
	}

	trimmed := strings.TrimSpace(item)
	normalized := s.normalize(trimmed)

	if normalized == "" {
		return
	}

	isPhrase := strings.Contains(normalized, " ")

	filterItem := FilterItem{
		Original:   trimmed,
		Normalized: normalized,
		IsPhrase:   isPhrase,
	}

	s.filterItems = append(s.filterItems, filterItem)

	// Create regex for phrases
	if isPhrase {
		regex := s.createPhraseRegex(normalized)
		s.phraseRegexes = append(s.phraseRegexes, regex)
	}

	log.Printf("Added filter item: '%s' (phrase: %t)", trimmed, isPhrase)
}

// AddFilterItems adds multiple words or phrases to the filter
func (s *SimpleSanitizer) AddFilterItems(items []string) {
	for _, item := range items {
		s.AddFilterItem(item)
	}
}

// Deprecated: Use AddFilterItem instead
func (s *SimpleSanitizer) AddBadWord(word string) {
	s.AddFilterItem(word)
}

// Deprecated: Use AddFilterItems instead
func (s *SimpleSanitizer) AddBadWords(words []string) {
	s.AddFilterItems(words)
}

// GetFilterItemCount returns the number of filter items
func (s *SimpleSanitizer) GetFilterItemCount() int {
	return len(s.filterItems)
}

// GetBadWordCount returns the number of filter items (for backward compatibility)
func (s *SimpleSanitizer) GetBadWordCount() int {
	return s.GetFilterItemCount()
}

// ListFilterItems returns a copy of all filter items for debugging
func (s *SimpleSanitizer) ListFilterItems() []FilterItem {
	items := make([]FilterItem, len(s.filterItems))
	copy(items, s.filterItems)
	return items
}

// CheckAndLogViolation checks text and logs if it contains filtered content
func (s *SimpleSanitizer) CheckAndLogViolation(text string, source string) bool {
	if s.containsBadContent(text) {
		log.Printf("[CONTENT_VIOLATION] Filtered content detected in %s", source)
		return true
	}
	return false
}

// CleanChatMessage removes a message if it contains filtered content
func (s *SimpleSanitizer) CleanChatMessage(msg ChatMessage) (ChatMessage, bool) {
	if s.containsBadContent(msg.Text) {
		// Return empty text to indicate this message should be filtered
		msg.Text = ""
		return msg, true // true indicates the message was modified
	}
	return msg, false
}

// CleanChatMessages filters out messages containing bad content
func (s *SimpleSanitizer) CleanChatMessages(messages []ChatMessage) []ChatMessage {
	cleaned := make([]ChatMessage, 0)
	removedCount := 0

	for _, msg := range messages {
		if !s.containsBadContent(msg.Text) {
			cleaned = append(cleaned, msg)
		} else {
			removedCount++
			log.Printf("Removed message %d from chat %d due to inappropriate content",
				msg.MessageID, msg.ChatID)
		}
	}

	if removedCount > 0 {
		log.Printf("Cleaned chat messages: removed %d out of %d messages",
			removedCount, len(messages))
	}

	return cleaned
}

// CleanStringSlice removes strings containing filtered content from a slice
func (s *SimpleSanitizer) CleanStringSlice(items []string) []string {
	cleaned := make([]string, 0)

	for _, item := range items {
		if !s.containsBadContent(item) {
			cleaned = append(cleaned, item)
		}
	}

	return cleaned
}

// CleanPersonenImLeben cleans PersonImLeben entries
func (s *SimpleSanitizer) CleanPersonenImLeben(persons []PersonImLeben) []PersonImLeben {
	cleaned := make([]PersonImLeben, 0)

	for _, person := range persons {
		// Check all text fields in PersonImLeben
		if s.containsBadContent(person.Name) ||
			s.containsBadContent(person.Alter) ||
			s.containsBadContent(person.BeziehungZumUser) ||
			s.containsBadContent(person.GeschichteMitUser) {
			// Skip this entire person entry if any field contains filtered content
			log.Printf("Removed person entry '%s' due to inappropriate content", person.Name)
			continue
		}
		cleaned = append(cleaned, person)
	}

	return cleaned
}

// CleanCharacter cleans a Character struct
func (s *SimpleSanitizer) CleanCharacter(char Character) Character {
	cleaned := char

	// Check and clean simple string fields
	if s.containsBadContent(char.EchterName) {
		cleaned.EchterName = ""
	}
	if s.containsBadContent(char.Beruf) {
		cleaned.Beruf = ""
	}
	if s.containsBadContent(char.Wohnort) {
		cleaned.Wohnort = ""
	}
	if s.containsBadContent(char.Beziehungsstatus) {
		cleaned.Beziehungsstatus = ""
	}
	if s.containsBadContent(char.Lieblingsfarbe) {
		cleaned.Lieblingsfarbe = ""
	}
	if s.containsBadContent(char.FlirtLevel) {
		cleaned.FlirtLevel = ""
	}

	// Clean slice fields
	cleaned.Interessen = s.CleanStringSlice(char.Interessen)
	cleaned.TraeumeUndWuensche = s.CleanStringSlice(char.TraeumeUndWuensche)
	cleaned.GespeicherteErinnerungen = s.CleanStringSlice(char.GespeicherteErinnerungen)
	cleaned.AktuelleThemen = s.CleanStringSlice(char.AktuelleThemen)
	cleaned.TabuThemen = s.CleanStringSlice(char.TabuThemen)

	// Clean PersonenImLeben
	cleaned.PersonenImLeben = s.CleanPersonenImLeben(char.PersonenImLeben)

	return cleaned
}

// CleanKiraHelperForm cleans the entire KiraHelperForm
func (s *SimpleSanitizer) CleanKiraHelperForm(form KiraHelperForm) KiraHelperForm {
	cleaned := KiraHelperForm{
		User: s.CleanCharacter(form.User),
		Kira: s.CleanCharacter(form.Kira),
	}

	log.Printf("Cleaned KiraHelperForm")

	return cleaned
}

// CleanCompleteChat cleans a CompleteChat including all messages
func (s *SimpleSanitizer) CleanCompleteChat(chat CompleteChat) CompleteChat {
	cleaned := CompleteChat{
		LastHelperScannedMsg: chat.LastHelperScannedMsg,
		ChatId:               chat.ChatId,
		Infos:                s.CleanKiraHelperForm(chat.Infos),
		Chats:                make(map[int]ChatMessage),
	}

	// Clean all chat messages
	removedCount := 0
	for msgID, msg := range chat.Chats {
		if !s.containsBadContent(msg.Text) {
			cleaned.Chats[msgID] = msg
		} else {
			removedCount++
		}
	}

	if removedCount > 0 {
		log.Printf("Cleaned CompleteChat %d: removed %d messages", chat.ChatId, removedCount)
	}

	return cleaned
}
