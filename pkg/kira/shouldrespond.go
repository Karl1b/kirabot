package kira

import (
	"log"
	"math"
	"time"
)

// shouldRespondToMessage determines if we should respond to a message
func (k *KiraBot) shouldRespondToMessage(lastMsg ChatMessage, lastMessages []ChatMessage) (shouldRespond bool, shouldEngageWithExtraStory bool) {
	log.Println("Should Respond?")

	now := time.Now()
	lastMsgTime, err := time.ParseInLocation("2006-01-02 15:04:05", lastMsg.Date, time.Local)

	if err != nil {
		// If we can't parse the date, fall back to responding
		log.Printf("Could not parse: %v", lastMsg.Date)

		return true, false
	}

	// if the last message is fresh still respond
	timeSinceLastMsg := now.Sub(lastMsgTime)
	if timeSinceLastMsg < 5*time.Minute && !lastMsg.IsBot && !lastMsg.ShouldNotRespond {
		log.Printf("Fresh Message")
		return true, false
	}
	hour := now.Hour()
	// Check if there was fast chatting.
	if lastMsg.IsBot && !lastMsg.ShouldNotRespond {
		log.Println("last msgbot")
		//TODO: Check if msgs are extremly fresh shorter than typing time.
		//Happens if user and bot write in between.
		if timeSinceLastMsg > 24*time.Hour && hour > 10 && hour <= 22 {
			log.Printf("Old message - Provide extra story")
			return true, true
		}
		for i := 2; i < 6; i++ {
			if i >= len(lastMessages) {
				break
			}
			if !lastMessages[len(lastMessages)-i].IsBot {
				timeDiff := math.Abs(float64(lastMsg.Timestamp) - float64(lastMessages[len(lastMessages)-i].Timestamp))
				if timeDiff > 30 {
					log.Println("was read")
					// We found a user message that is older than 15s. it was read
					return false, false
				} else {
					log.Println("newer msg.")
					// We found a user message that is newer than 15s. It was not read.
					return true, false
				}
			}
		}

		return false, false
	}

	// If it is before 10:00 and after 22:00 return false as Kira sleeps

	if hour < 10 || hour >= 22 {
		return false, false
	}

	// If the last message is longer than 24h return true with extra story to engage
	if timeSinceLastMsg > 24*time.Hour {
		log.Printf("Old message - Provide extra story")
		return true, true
	}

	if lastMsg.ShouldNotRespond {
		log.Println("should not reply to last msg")
		return false, false
	}

	log.Println("All checks have passed Responding with stdt.")

	return true, false
}
