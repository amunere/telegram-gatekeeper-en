package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"telegram-gatekeeper/config"
	"telegram-gatekeeper/database"
	"telegram-gatekeeper/handlers"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
    bot        *tgbotapi.BotAPI
    botHandler *handlers.BotHandler
)

func main() {
    // Loading configuration
    cfg := config.Load()
    
    // Connecting to MongoDB
    mongoDB, err := database.Connect(cfg.MongoURI, cfg.MongoDBName)
    if err != nil {
        log.Fatalf("Failed to connect to MongoDB: %v", err)
    }
    defer mongoDB.Disconnect()
    
    // Bot initialization
    bot, err = tgbotapi.NewBotAPI(cfg.BotToken)
    if err != nil {
        log.Fatalf("Failed to create bot: %v", err)
    }
    
    bot.Debug = cfg.Debug
    log.Printf("Authorized on account %s", bot.Self.UserName)
    
    // Initialize the handler
    botHandler = handlers.NewBotHandler(bot, mongoDB, cfg.AdminID)
    
    // Installing commands
    setupCommands()
    
    // Setting up polling (long polling)
    setupPolling()
    
    // Waiting for completion signal
    waitForShutdown()
}

func setupCommands() {
    commands := tgbotapi.NewSetMyCommands(
        tgbotapi.BotCommand{
            Command:     "start",
            Description: "Start chatting with a bot",
        },
        tgbotapi.BotCommand{
            Command:     "verify",
            Description: "Pass verification",
        },
        tgbotapi.BotCommand{
            Command:     "status",
            Description: "Verification status",
        },
        tgbotapi.BotCommand{
            Command:     "help",
            Description: "Help by command",
        },
    )
    
    _, err := bot.Request(commands)
    if err != nil {
        log.Printf("Failed to set commands: %v", err)
    }
}

func setupPolling() {
    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60
    
    updates := bot.GetUpdatesChan(u)
    
    log.Println("Bot started polling for updates...")
    
    // Processing updates
    for update := range updates {
        go botHandler.HandleUpdate(update)
    }
}

func waitForShutdown() {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
    
    log.Println("Bot is running. Press Ctrl+C to stop.")
    
    // Waiting for the completion signal
    <-sigChan
    
    log.Println("\nShutting down bot...")
    
    // We give time to complete operations
    time.Sleep(2 * time.Second)
    
    log.Println("Bot shutdown complete")
}