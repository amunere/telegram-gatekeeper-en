package database

import (
    "time"
    "go.mongodb.org/mongo-driver/bson/primitive"
)

// User model
type User struct {
    ID           primitive.ObjectID `bson:"_id,omitempty"`
    TelegramID   int64              `bson:"telegram_id"`
    Username     string             `bson:"username,omitempty"`
    FirstName    string             `bson:"first_name"`
    LastName     string             `bson:"last_name,omitempty"`
    LanguageCode string             `bson:"language_code,omitempty"`
    IsBot        bool               `bson:"is_bot"`
    IsVerified   bool               `bson:"is_verified"`
    IsBlocked    bool               `bson:"is_blocked"`
    VerifiedAt   *time.Time         `bson:"verified_at,omitempty"`
    CreatedAt    time.Time          `bson:"created_at"`
    UpdatedAt    time.Time          `bson:"updated_at"`
    
    // To check
    VerificationAttempts int       `bson:"verification_attempts"`
    LastAttemptAt        time.Time `bson:"last_attempt_at"`
    CaptchaData          *Captcha  `bson:"captcha_data,omitempty"`
}

// Captcha model
type Captcha struct {
    Type        string    `bson:"type"` // "math", "text", "button"
    Question    string    `bson:"question"`
    Answer      string    `bson:"answer"`
    Options     []string  `bson:"options,omitempty"` 
    CreatedAt   time.Time `bson:"created_at"`
    ExpiresAt   time.Time `bson:"expires_at"`
}

// Message model
type Message struct {
    ID          primitive.ObjectID `bson:"_id,omitempty"`
    TelegramID  int                `bson:"telegram_id"`
    UserID      primitive.ObjectID `bson:"user_id"`
    Text        string             `bson:"text"`
    IsForwarded bool               `bson:"is_forwarded"`
    ForwardedTo []int64            `bson:"forwarded_to,omitempty"` // ID admin
    CreatedAt   time.Time          `bson:"created_at"`
}

// AdminSettings
type AdminSettings struct {
    ID                    primitive.ObjectID `bson:"_id,omitempty"`
    AdminID               int64              `bson:"admin_id"`
    AutoForwardEnabled    bool               `bson:"auto_forward_enabled"`
    CaptchaType           string             `bson:"captcha_type"`
    MaxAttempts           int                `bson:"max_attempts"`
    BlockDuration         time.Duration      `bson:"block_duration"`
    WelcomeMessage        string             `bson:"welcome_message"`
    VerifiedMessage       string             `bson:"verified_message"`
    CreatedAt             time.Time          `bson:"created_at"`
    UpdatedAt             time.Time          `bson:"updated_at"`
}