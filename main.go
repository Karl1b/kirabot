package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	kira "gitea.karlbreuer.com/karl1b/kira/pkg/kira"
	settings "gitea.karlbreuer.com/karl1b/kira/pkg/settings"
)

func main() {
	// Initialize Kira bot
	bot, err := kira.NewKiraBot(settings.Settings.TelegramToken, settings.Settings.LlmKey)
	if err != nil {
		log.Fatal("Failed to initialize Kira bot:", err)
	}

	log.Println("Kira bot started successfully")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Channel to stop the AI loop
	aiStopChan := make(chan struct{})

	// Start the AI response loop in a separate goroutine
	go func() {
		aiTicker := time.NewTicker(15 * time.Second)
		defer aiTicker.Stop()

		log.Println("Starting AI response loop (runs every 15 seconds)")

		for {
			select {
			case <-aiTicker.C:
				if err := bot.AIRun(); err != nil {
					log.Printf("AI Run error: %v", err)
				}
			case <-aiStopChan:
				log.Println("AI loop stopped")
				return
			}
		}
	}()

	for {
		// Run the bot in a goroutine
		botErrChan := make(chan error, 1)
		go func() {
			botErrChan <- bot.Run()
		}()

		select {
		case sig := <-sigChan:
			log.Printf("Received signal: %v. Shutting down gracefully...", sig)

			// Stop the AI loop first
			close(aiStopChan)

			// Then stop the bot
			bot.Shutdown()
			return

		case err := <-botErrChan:
			if err != nil {
				// Check if it's a conflict error (multiple instances)
				if strings.Contains(err.Error(), "Conflict") {
					log.Printf("Bot conflict detected (multiple instances running?): %v", err)
					log.Println("Waiting 30 seconds before retry...")
					time.Sleep(30 * time.Second)
				} else {
					log.Printf("Bot error: %v. Restarting in 10 seconds...", err)
					time.Sleep(10 * time.Second)
				}
			} else {
				log.Println("Bot stopped normally. Restarting in 5 seconds...")
				time.Sleep(5 * time.Second)
			}
		}
	}
}
