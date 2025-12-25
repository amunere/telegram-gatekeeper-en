package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type TextQuestion struct {
	Question string
	Answer   string
}

type CaptchaConfig struct {
	TextQuestions []TextQuestion
	Colors        []string
	ButtonText    string
	MathOperators []string
}

type Config struct {
    BotToken    string
    MongoURI    string
    MongoDBName string
    AdminID     int64
    Debug       bool
    Captcha     CaptchaConfig
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
        log.Printf("Warning: .env file not found: %v", err)
    }

    adminID, _ := strconv.ParseInt(os.Getenv("ADMIN_ID"), 10, 64)
    debug, _ := strconv.ParseBool(os.Getenv("DEBUG"))
    
    return &Config{
        BotToken:    os.Getenv("BOT_TOKEN"),
        MongoURI:    os.Getenv("MONGO_URI"),
        MongoDBName: getEnv("MONGO_DB_NAME", "telegram_bot"),
        AdminID:     adminID,
        Debug:       debug,
        Captcha:     loadCaptchaConfig(),
    }
}

func loadCaptchaConfig() CaptchaConfig {
	config := CaptchaConfig{}
	
	// Loading questions
	questionCount := 0
	for i := 1; ; i++ {
		qKey := fmt.Sprintf("CAPTCHA_Q%d_QUESTION", i)
		aKey := fmt.Sprintf("CAPTCHA_Q%d_ANSWER", i)
		
		question := os.Getenv(qKey)
		answer := os.Getenv(aKey)
		
		if question == "" || answer == "" {
			break
		}
		
		config.TextQuestions = append(config.TextQuestions, TextQuestion{
			Question: question,
			Answer:   answer,
		})
		questionCount++
	}
	
	// Loading colors
	if colors := os.Getenv("CAPTCHA_COLORS"); colors != "" {
		config.Colors = splitCommaSeparated(colors)
	}
	
	// Loading button text
	config.ButtonText = os.Getenv("CAPTCHA_BUTTON_TEXT")
	
	// Loading mathematical operators
	if operators := os.Getenv("CAPTCHA_MATH_OPS"); operators != "" {
		config.MathOperators = splitCommaSeparated(operators)
	}
	
	return config
}

func splitCommaSeparated(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	
	return result
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}