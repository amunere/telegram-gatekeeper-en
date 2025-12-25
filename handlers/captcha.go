package handlers

import (
	"fmt"
	"log"
	"math/rand"
	"slices"
	"time"

	"telegram-gatekeeper/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *BotHandler) sendNewCaptcha(chatID int64, user *database.User) {
    captcha := h.generateCaptcha()
    
    // Saving the captcha in the database
    h.db.SaveCaptcha(user.TelegramID, captcha)
    
    var msg tgbotapi.MessageConfig
    
    switch captcha.Type {
    case "math":
        msg = tgbotapi.NewMessage(chatID, 
            fmt.Sprintf("🔐 *Security check*\n\nSolve the example:\n`%s`", captcha.Question),
        )
        msg.ParseMode = "Markdown"
        
    case "text":
        msg = tgbotapi.NewMessage(chatID, 
            fmt.Sprintf("🔐 *Security check*\n\nAnswer the question:\n%s", captcha.Question),
        )
        msg.ParseMode = "Markdown"
        
    case "button":
        msg = tgbotapi.NewMessage(chatID, 
            "🔐 *Security check*\n\nChoose the correct answer:",
        )
        msg.ParseMode = "Markdown"
        
        // Creating buttons
        var rows [][]tgbotapi.InlineKeyboardButton
        for i, option := range captcha.Options {
            callbackData := fmt.Sprintf("captcha_%d_%d", user.TelegramID, i)
            button := tgbotapi.NewInlineKeyboardButtonData(option, callbackData)
            rows = append(rows, tgbotapi.NewInlineKeyboardRow(button))
        }
        
        keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
        msg.ReplyMarkup = keyboard
    }
    
    _, err := h.bot.Send(msg)
    if err != nil {
        log.Printf("Error sending captcha: %v", err)
    }
}

func (h *BotHandler) generateCaptcha() *database.Captcha {
	captchaTypes := []string{"math", "text", "button"}
	captchaType := captchaTypes[rand.Intn(len(captchaTypes))]
	
	switch captchaType {
	case "math":
		if len(h.config.Captcha.MathOperators) == 0 {
            a, b := rand.Intn(10)+1, rand.Intn(10)+1
            return &database.Captcha{
                Type:      "math",
                Question:  fmt.Sprintf("%d + %d", a, b),
                Answer:    fmt.Sprintf("%d", a+b),
                CreatedAt: time.Now(),
                ExpiresAt: time.Now().Add(2 * time.Minute),
            }
        }
		
		a, b := rand.Intn(10)+1, rand.Intn(10)+1
		ops := h.config.Captcha.MathOperators
		op := ops[rand.Intn(len(ops))]
		
		var question string
		var answer string
		
		switch op {
		case "+":
			question = fmt.Sprintf("%d + %d", a, b)
			answer = fmt.Sprintf("%d", a+b)
		case "-":
			question = fmt.Sprintf("%d - %d", a+b, b)
			answer = fmt.Sprintf("%d", a)
		case "×":
			question = fmt.Sprintf("%d × %d", a, b)
			answer = fmt.Sprintf("%d", a*b)
		case "÷":
			question = fmt.Sprintf("%d ÷ %d", a*b, b)
			answer = fmt.Sprintf("%d", a)
		default:
			question = fmt.Sprintf("%d + %d", a, b)
			answer = fmt.Sprintf("%d", a+b)
		}
		
		return &database.Captcha{
			Type:      "math",
			Question:  question,
			Answer:    answer,
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(2 * time.Minute),
		}
		
	case "text":
		if len(h.config.Captcha.TextQuestions) == 0 {
            a, b := rand.Intn(10)+1, rand.Intn(10)+1
            return &database.Captcha{
                Type:      "math",
                Question:  fmt.Sprintf("%d + %d", a, b),
                Answer:    fmt.Sprintf("%d", a+b),
                CreatedAt: time.Now(),
                ExpiresAt: time.Now().Add(2 * time.Minute),
            }
        }
		
		q := h.config.Captcha.TextQuestions[rand.Intn(len(h.config.Captcha.TextQuestions))]
		
		return &database.Captcha{
			Type:      "text",
			Question:  q.Question,
			Answer:    q.Answer,
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(2 * time.Minute),
		}
		
	case "button":
		if len(h.config.Captcha.Colors) < 4 {
            a, b := rand.Intn(10)+1, rand.Intn(10)+1
            return &database.Captcha{
                Type:      "math",
                Question:  fmt.Sprintf("%d + %d", a, b),
                Answer:    fmt.Sprintf("%d", a+b),
                CreatedAt: time.Now(),
                ExpiresAt: time.Now().Add(2 * time.Minute),
            }
        }
		
		colors := h.config.Captcha.Colors
		correct := colors[rand.Intn(len(colors))]
		
		// Creating answer options
		options := []string{correct}
		for len(options) < 4 {
			color := colors[rand.Intn(len(colors))]
			if !slices.Contains(options, color) {
				options = append(options, color)
			}
		}
		
		// Mix
		rand.Shuffle(len(options), func(i, j int) {
			options[i], options[j] = options[j], options[i]
		})
		
		buttonText := h.config.Captcha.ButtonText
		if buttonText == "" {
			buttonText = "Select color:"
		}
		
		return &database.Captcha{
			Type:      "button",
			Question:  buttonText + " " + correct,
			Answer:    correct,
			Options:   options,
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(2 * time.Minute),
		}
	}
	
	return &database.Captcha{
        Type:      "math",
        Question:  "2 + 2",
        Answer:    "4",
        CreatedAt: time.Now(),
        ExpiresAt: time.Now().Add(2 * time.Minute),
    }
}
