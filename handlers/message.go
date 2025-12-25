package handlers

import (
	"context"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"
	"time"

	"telegram-gatekeeper/config"
	"telegram-gatekeeper/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mongodb.org/mongo-driver/bson"
)

type BotHandler struct {
    bot     *tgbotapi.BotAPI
    db      *database.MongoDB
    adminID int64
    config *config.Config
}

func NewBotHandler(bot *tgbotapi.BotAPI, db *database.MongoDB, adminID int64) *BotHandler {
    return &BotHandler{
        bot:     bot,
        db:      db,
        adminID: adminID,
    }
}

func (h *BotHandler) HandleUpdate(update tgbotapi.Update) {
    if update.Message != nil {
        h.handleMessage(update.Message)
    } else if update.CallbackQuery != nil {
        h.handleCallback(update.CallbackQuery)
    }
}

func (h *BotHandler) handleMessage(message *tgbotapi.Message) {
    user := message.From
    chatID := message.Chat.ID

    log.Printf("Message from %s (%d): %s", user.FirstName, user.ID, message.Text)
    log.Printf("Chat ID: %d, Message Type: %T", chatID, message)

    // Getting or creating a user in the database
    dbUser, err := h.db.GetOrCreateUser(
        user.ID,
        user.UserName,
        user.FirstName,
        user.LastName,
        user.IsBot,
    )

    if err != nil {
        log.Printf("Error getting user from DB: %v", err)
        return
    }

    // Checking the bot
    if user.IsBot {
        h.handleBotUser(chatID, dbUser)
        return
    }

    // Checking the lock
    if dbUser.IsBlocked {
        h.sendBlockedMessage(chatID)
        return
    }

    // Command Processing
    if message.IsCommand() {
        h.handleCommand(message, dbUser)
        return
    }

    // Verification check
    if !dbUser.IsVerified {
        h.handleUnverifiedUser(chatID, message.Text, dbUser)
        return
    }

    // Messaging message admin
    h.forwardToAdminHTML(message, dbUser)
    h.sendConfirmationToUser(message.Chat.ID)
}

func (h *BotHandler) sendConfirmationToUser(chatID int64) {
    h.sendMessage(chatID, "✅ Your message has been sent to the administrator. Wait for a response.")
}

func (h *BotHandler) handleCallback(callback *tgbotapi.CallbackQuery) {
    data := callback.Data
    userID := callback.From.ID

    log.Printf("Callback from user %d: %s", userID, data)

    // Captcha callback handling
    if strings.HasPrefix(data, "captcha_") {
        h.handleCaptchaCallback(callback)
        return
    }

    // Processing callbacks from admin buttons
    if strings.HasPrefix(data, "accept_") {
        h.handleAcceptUser(callback)
        return
    }

    if strings.HasPrefix(data, "reject_") {
        h.handleRejectUser(callback)
        return
    }

    if strings.HasPrefix(data, "block_") {
        h.handleBlockUser(callback)
        return
    }

    // Response to unknown callback
    h.answerCallback(callback.ID, "Unknown command")
}

func (h *BotHandler) handleCaptchaCallback(callback *tgbotapi.CallbackQuery) {
    parts := strings.Split(callback.Data, "_")
    if len(parts) != 3 {
        h.answerCallback(callback.ID, "Data error")
        return
    }

    telegramID, err := strconv.ParseInt(parts[1], 10, 64)
    if err != nil {
        h.answerCallback(callback.ID, "Error ID")
        return
    }

    optionIndex, err := strconv.Atoi(parts[2])
    if err != nil {
        h.answerCallback(callback.ID, "Index error")
        return
    }

    // Getting the user from the database
    user, err := h.db.GetUserByTelegramID(telegramID)
    if err != nil {
        log.Printf("Error getting user: %v", err)
        h.answerCallback(callback.ID, "Error receiving data")
        return
    }

    if user.CaptchaData == nil {
        h.answerCallback(callback.ID, "Captcha is outdated")
        return
    }

    // Checking the captcha expiration date
    if time.Now().After(user.CaptchaData.ExpiresAt) {
        h.answerCallback(callback.ID, "Captcha time has expired")

        // Sending a new captcha
        h.sendNewCaptcha(callback.Message.Chat.ID, user)
        return
    }

    // Checking the answer
    if optionIndex < len(user.CaptchaData.Options) {
        selectedAnswer := user.CaptchaData.Options[optionIndex]

        if selectedAnswer == user.CaptchaData.Answer {
            // Successful check
            err := h.db.UpdateUserVerification(user.TelegramID, true)
            if err != nil {
                log.Printf("Error updating verification: %v", err)
                h.answerCallback(callback.ID, "Server error")
                return
            }

            // Editing a message with captcha
            editMsg := tgbotapi.NewEditMessageText(
                callback.Message.Chat.ID,
                callback.Message.MessageID,
                "✅ Verification passed!\n\nNow your messages will be forwarded to the administrator.",
            )
            editMsg.ParseMode = ""
            _, err = h.bot.Send(editMsg)
            if err != nil {
                log.Printf("Error editing message: %v", err)
            }

            // Removing buttons
            h.removeButtons(callback.Message.Chat.ID, callback.Message.MessageID)

            // Send confirmation callback
            h.answerCallback(callback.ID, "✅ Right! Verification passed.")

            // We notify the admin
            h.notifyAdmin(user, true, "")
        } else {
            // Wrong answer
            h.db.IncrementAttempts(user.TelegramID)

            // Receiving updated user data
            user, _ = h.db.GetUserByTelegramID(telegramID)

            // Checking the number of attempts
            if user.VerificationAttempts >= 3 {
                // Blocking a user
                h.blockUser(user.TelegramID)

                editMsg := tgbotapi.NewEditMessageText(
                    callback.Message.Chat.ID,
                    callback.Message.MessageID,
                    "❌ Access blocked\n\nYou have exceeded the maximum number of attempts.",
                )
                editMsg.ParseMode = ""
                _, err = h.bot.Send(editMsg)
                if err != nil {
                    log.Printf("Error editing message: %v", err)
                }

                h.answerCallback(callback.ID, "❌ Number of attempts exceeded")

                h.notifyAdmin(user, false, "Number of attempts exceeded")
            } else {
                h.answerCallback(callback.ID,
                    fmt.Sprintf("❌ Wrong. Attempts left: %d/3", 3-user.VerificationAttempts))

                // Sending a new captcha
                time.Sleep(500 * time.Millisecond) 
                h.sendNewCaptcha(callback.Message.Chat.ID, user)
            }
        }
    }
}

func (h *BotHandler) handleAcceptUser(callback *tgbotapi.CallbackQuery) {
    parts := strings.Split(callback.Data, "_")
    if len(parts) != 2 {
        h.answerCallback(callback.ID, "Data error")
        return
    }

    telegramID, err := strconv.ParseInt(parts[1], 10, 64)
    if err != nil {
        h.answerCallback(callback.ID, "Error ID")
        return
    }

    // Update the user as verified
    err = h.db.UpdateUserVerification(telegramID, true)
    if err != nil {
        log.Printf("Error accepting user: %v", err)
        h.answerCallback(callback.ID, "Server error")
        return
    }

    // Getting information about the user
    user, err := h.db.GetUserByTelegramID(telegramID)
    if err != nil {
        log.Printf("Error getting user: %v", err)
    }

    // Editing the message text
    cleanText := h.removeMarkdown(callback.Message.Text)
    newText := cleanText + "\n\n✅ Accepted by administrator"

    editMsg := tgbotapi.NewEditMessageText(
        callback.Message.Chat.ID,
        callback.Message.MessageID,
        newText,
    )
    editMsg.ParseMode = ""
    _, err = h.bot.Send(editMsg)
    if err != nil {
        log.Printf("Error editing message: %v", err)
    }

    // Removing buttons
    h.removeButtons(callback.Message.Chat.ID, callback.Message.MessageID)

    h.answerCallback(callback.ID, "✅ User accepted")

    // We notify the user
    if user != nil {
        h.sendMessage(user.TelegramID,
            "✅ The administrator has accepted your request. You can now send messages.",
        )
    }
}

func (h *BotHandler) handleRejectUser(callback *tgbotapi.CallbackQuery) {
    parts := strings.Split(callback.Data, "_")
    if len(parts) != 2 {
        h.answerCallback(callback.ID, "Data error")
        return
    }

    telegramID, err := strconv.ParseInt(parts[1], 10, 64)
    if err != nil {
        h.answerCallback(callback.ID, "Error ID")
        return
    }

    // Getting information about the user
    user, err := h.db.GetUserByTelegramID(telegramID)
    if err != nil {
        log.Printf("Error getting user: %v", err)
    }

    // Editing the text
    cleanText := h.removeMarkdown(callback.Message.Text)
    newText := cleanText + "\n\n❌ Rejected by administrator"

    editMsg := tgbotapi.NewEditMessageText(
        callback.Message.Chat.ID,
        callback.Message.MessageID,
        newText,
    )
    editMsg.ParseMode = ""
    _, err = h.bot.Send(editMsg)
    if err != nil {
        log.Printf("Error editing message: %v", err)
    }

    // Removing buttons
    h.removeButtons(callback.Message.Chat.ID, callback.Message.MessageID)

    h.answerCallback(callback.ID, "❌ User rejected")

    // We notify the user
    if user != nil {
        h.sendMessage(user.TelegramID,
            "❌ The administrator has rejected your communication request.",
        )
    }
}

func (h *BotHandler) handleBlockUser(callback *tgbotapi.CallbackQuery) {
    parts := strings.Split(callback.Data, "_")
    if len(parts) != 2 {
        h.answerCallback(callback.ID, "Data error")
        return
    }

    telegramID, err := strconv.ParseInt(parts[1], 10, 64)
    if err != nil {
        h.answerCallback(callback.ID, "Error ID")
        return
    }

    // Blocking a user
    h.blockUser(telegramID)

    // Getting information about the user
    user, err := h.db.GetUserByTelegramID(telegramID)
    if err != nil {
        log.Printf("Error getting user: %v", err)
    }

    // Editing the text
    cleanText := h.removeMarkdown(callback.Message.Text)
    newText := cleanText + "\n\n⛔ Blocked by administrator"

    editMsg := tgbotapi.NewEditMessageText(
        callback.Message.Chat.ID,
        callback.Message.MessageID,
        newText,
    )
    editMsg.ParseMode = ""
    _, err = h.bot.Send(editMsg)
    if err != nil {
        log.Printf("Error editing message: %v", err)
    }

    // Removing buttons
    h.removeButtons(callback.Message.Chat.ID, callback.Message.MessageID)

    h.answerCallback(callback.ID, "⛔ User is blocked")

    // Notify the user
    if user != nil {
        h.sendMessage(user.TelegramID,
            "⛔ The administrator has blocked your access.",
        )
    }
}

// Universal method for removing buttons
func (h *BotHandler) removeButtons(chatID int64, messageID int) {
    // Method 1: Empty keyboard with empty array
    emptyKeyboard := tgbotapi.InlineKeyboardMarkup{
        InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{},
    }
    
    editMarkup := tgbotapi.NewEditMessageReplyMarkup(
        chatID,
        messageID,
        emptyKeyboard,
    )
    
    _, err := h.bot.Send(editMarkup)
    if err != nil {
        log.Printf("Error removing buttons (method 1): %v", err)
        
        // Method 2: Let's try it via NewInlineKeyboardMarkup()
        editMarkup2 := tgbotapi.NewEditMessageReplyMarkup(
            chatID,
            messageID,
            tgbotapi.NewInlineKeyboardMarkup(),
        )
        
        _, err = h.bot.Send(editMarkup2)
        if err != nil {
            log.Printf("Error removing buttons (method 2): %v", err)
            
            // Method 3: Delete the entire message
            deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
            _, err = h.bot.Send(deleteMsg)
            if err != nil {
                log.Printf("Error deleting message: %v", err)
            }
        }
    }
}

func (h *BotHandler) removeMarkdown(text string) string {
    // Removing all Markdown characters
    replacer := strings.NewReplacer(
        "*", "",
        "_", "",
        "`", "",
        "[", "",
        "]", "",
        "(", "",
        ")", "",
        "~", "",
        ">", "",
        "#", "",
        "+", "",
        "-", "",
        "=", "",
        "|", "",
        "{", "",
        "}", "",
        ".", "",
        "!", "",
        "\\", "",
    )
    return replacer.Replace(text)
}

func (h *BotHandler) answerCallback(callbackID string, text string) {
    callbackConfig := tgbotapi.CallbackConfig{
        CallbackQueryID: callbackID,
        Text:            text,
        ShowAlert:       false,
    }

    _, err := h.bot.Request(callbackConfig)
    if err != nil {
        log.Printf("Error answering callback: %v", err)
    }
}

func (h *BotHandler) blockUser(telegramID int64) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    _, err := h.db.Users.UpdateOne(
        ctx,
        bson.M{"telegram_id": telegramID},
        bson.M{
            "$set": bson.M{
                "is_blocked":  true,
                "is_verified": false,
                "updated_at":  time.Now(),
            },
        },
    )

    if err != nil {
        log.Printf("Error blocking user: %v", err)
    }
}

func (h *BotHandler) handleBotUser(chatID int64, user *database.User) {
    h.sendMessage(chatID,
        "❌ Bots cannot be verified.")

    // Notify the admin about the bot attempt
    h.notifyAdmin(user, false, "Bot attempt")
}

func (h *BotHandler) sendBlockedMessage(chatID int64) {
    h.sendMessage(chatID,
        "⛔ Your access is blocked.")
}

func (h *BotHandler) handleUnverifiedUser(chatID int64, messageText string, user *database.User) {
    // Checking if there is an active captcha
    if user.CaptchaData != nil && time.Now().Before(user.CaptchaData.ExpiresAt) {
        h.checkCaptchaAnswer(chatID, messageText, user)
        return
    }

    // Sending a new captcha
    h.sendNewCaptcha(chatID, user)
}

func (h *BotHandler) checkCaptchaAnswer(chatID int64, answer string, user *database.User) {
    // Increase the attempt counter
    h.db.IncrementAttempts(user.TelegramID)

    // Checking the answer
    if strings.EqualFold(strings.TrimSpace(answer), user.CaptchaData.Answer) {
        // Successful check
        h.db.UpdateUserVerification(user.TelegramID, true)

        h.sendMessage(chatID,
            "✅ Verification passed!\n\nNow your messages will be forwarded to the administrator.")

        // Notice to admin
        h.notifyAdmin(user, true, "")
    } else {
        // Failed attempt
        attempts := user.VerificationAttempts + 1

        if attempts >= 3 {
            // Blocking when attempts are exceeded
            h.blockUser(user.TelegramID)
            h.sendMessage(chatID,
                "❌ Access blocked\n\nYou have exceeded the maximum number of attempts.")
            h.notifyAdmin(user, false, "Number of attempts exceeded")
        } else {
            h.sendMessage(chatID,
                fmt.Sprintf("❌ Wrong answer. Attempts left: %d/3", 3-attempts))
            h.sendNewCaptcha(chatID, user)
        }
    }
}

func (h *BotHandler) forwardToAdminHTML(message *tgbotapi.Message, user *database.User) {
    // We send the original
    forwardMsg := tgbotapi.NewForward(h.adminID, message.Chat.ID, message.MessageID)
    _, err := h.bot.Send(forwardMsg)
    if err != nil {
        log.Printf("Error forwarding message: %v", err)
    }

    // Escaping HTML
    safeFirstName := html.EscapeString(user.FirstName)
    safeLastName := html.EscapeString(user.LastName)
    safeText := html.EscapeString(message.Text)

    username := "not indicated"
    if user.Username != "" {
        username = "@" + user.Username
    }

    text := fmt.Sprintf(
        "<b>📨 Sender information</b>\n\n"+
            "👤 From: %s %s\n"+
            "🆔 ID: <code>%d</code>\n"+
            "📝 Username: %s\n"+
            "⏰ Time: %s\n\n"+
            "💬 <b>Message:</b>\n%s",
        safeFirstName,
        safeLastName,
        user.TelegramID,
        username,
        time.Now().Format("15:04:05"),
        safeText,
    )

    infoMsg := tgbotapi.NewMessage(h.adminID, text)
    infoMsg.ParseMode = "HTML"

    replyMarkup := tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("✅ Accept", fmt.Sprintf("accept_%d", user.TelegramID)),
            tgbotapi.NewInlineKeyboardButtonData("❌ Reject", fmt.Sprintf("reject_%d", user.TelegramID)),
            tgbotapi.NewInlineKeyboardButtonData("⛔ Block", fmt.Sprintf("block_%d", user.TelegramID)),
        ),
    )
    infoMsg.ReplyMarkup = replyMarkup

    _, err = h.bot.Send(infoMsg)
    if err != nil {
        log.Printf("Error sending HTML message to admin: %v", err)
    }
}

func (h *BotHandler) notifyAdmin(user *database.User, success bool, reason string) {
    status := "✅ passed the test"
    if !success {
        status = "❌ failed verification"
    }

    username := "not indicated"
    if user.Username != "" {
        username = "@" + user.Username
    }

    text := fmt.Sprintf(
        "<b>👤 User %s</b>\n"+
            "🆔 ID: <code>%d</code>\n"+
            "📝 Username: %s\n"+
            "📊 Status: %s\n"+
            "⏰ Time: %s",
        html.EscapeString(user.FirstName),
        user.TelegramID,
        username,
        status,
        time.Now().Format("15:04:05"),
    )

    if reason != "" {
        text += fmt.Sprintf("\n📋 Reason: %s", html.EscapeString(reason))
    }

    msg := tgbotapi.NewMessage(h.adminID, text)
    msg.ParseMode = "HTML"
    _, err := h.bot.Send(msg)
    if err != nil {
        log.Printf("Error notifying admin: %v", err)
    }
}

func (h *BotHandler) handleCommand(message *tgbotapi.Message, user *database.User) {
    switch message.Command() {
    case "start":
        h.handleStartCommand(message, user)
    case "verify":
        h.handleVerifyCommand(message, user)
    case "status":
        h.handleStatusCommand(message, user)
    case "help":
        h.handleHelpCommand(message)
    default:
        h.handleUnknownCommand(message)
    }
}

func (h *BotHandler) handleStartCommand(message *tgbotapi.Message, user *database.User) {
    chatID := message.Chat.ID

    if user.IsVerified {
        msgText := fmt.Sprintf(
            "✅ <b>Hi, %s!</b>\n\n"+
                "You have already been verified.\n"+
                "You can send messages, they will be forwarded to the administrator.\n\n"+
                "🆔 Your ID: <code>%d</code>\n"+
                "📊 Status: ✅ Checked\n"+
                "📅 Registration: %s",
            html.EscapeString(user.FirstName),
            user.TelegramID,
            user.CreatedAt.Format("02.01.2006"),
        )

        h.sendMessageHTML(chatID, msgText)
        return
    }

    // Sending a greeting
    welcomeMsg := fmt.Sprintf(
        "👋 <b>Hi, %s!</b>\n\n"+
            "I'm a helper bot. To contact the administrator, you need to pass a simple verification.\n\n"+
            "Use the /verify command to start checking.",
        html.EscapeString(user.FirstName),
    )

    h.sendMessageHTML(chatID, welcomeMsg)
}

func (h *BotHandler) handleVerifyCommand(message *tgbotapi.Message, user *database.User) {
    if user.IsVerified {
        h.sendMessage(message.Chat.ID, "✅ You have already been verified.")
        return
    }

    if user.IsBlocked {
        h.sendMessage(message.Chat.ID, "⛔ Your access is blocked.")
        return
    }

    // Sending a new captcha
    h.sendNewCaptcha(message.Chat.ID, user)
}

func (h *BotHandler) handleStatusCommand(message *tgbotapi.Message, user *database.User) {
    chatID := message.Chat.ID
    var status string

    if user.IsBlocked {
        status = "⛔ Blocked"
    } else if user.IsVerified {
        status = "✅ Checked"
    } else {
        status = "⏳ Awaiting review"
    }

    username := "not indicated"
    if user.Username != "" {
        username = "@" + user.Username
    }

    msgText := fmt.Sprintf(
        "📊 <b>Your status</b>\n\n"+
            "👤 Name: %s\n"+
            "🆔 ID: <code>%d</code>\n"+
            "📝 Username: %s\n"+
            "📊 Status: %s\n"+
            "🔄 Attempts: %d/3\n"+
            "📅 Registration: %s",
        html.EscapeString(user.FirstName),
        user.TelegramID,
        username,
        status,
        user.VerificationAttempts,
        user.CreatedAt.Format("02.01.2006"),
    )

    h.sendMessageHTML(chatID, msgText)
}

func (h *BotHandler) handleHelpCommand(message *tgbotapi.Message) {
    helpText := `🆘 <b>Available commands</b>

/start - Start working with the bot
/verify - Pass verification
/status - Find out your status
/help - Show this message

<b>How does it work?</b>
1. You send a message to a bot
2. Pass a simple verification (captcha)
3. After successful verification, your messages are forwarded to the administrator
4. The administrator can answer you

<b>Rules:</b>
- You have 3 attempts to pass the test
- It is prohibited to use bots to bypass verification
- Messages with insults will not be forwarded`

    h.sendMessageHTML(message.Chat.ID, helpText)
}

func (h *BotHandler) handleUnknownCommand(message *tgbotapi.Message) {
    h.sendMessage(message.Chat.ID, "❌ Unknown team. Use /help for a list of commands.")
}

func (h *BotHandler) sendMessage(chatID int64, text string) {
    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"

    _, err := h.bot.Send(msg)
    if err != nil {
        log.Printf("Error sending message to %d: %v", chatID, err)
        if strings.Contains(err.Error(), "Forbidden") {
            log.Printf("Bot was blocked by user %d", chatID)
        }
    }
}

func (h *BotHandler) sendMessageHTML(chatID int64, text string) {
    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "HTML"

    _, err := h.bot.Send(msg)
    if err != nil {
        log.Printf("Error sending HTML message to %d: %v", chatID, err)
        h.sendMessage(chatID, text)
    }
}