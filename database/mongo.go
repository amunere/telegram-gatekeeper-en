package database

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
    "go.mongodb.org/mongo-driver/mongo/readpref"
)

type MongoDB struct {
    Client     *mongo.Client
    Database   *mongo.Database
    
    // Collections
    Users      *mongo.Collection
    Messages   *mongo.Collection
    Settings   *mongo.Collection
    Blacklist  *mongo.Collection
}

var DB *MongoDB

func Connect(uri, dbName string) (*MongoDB, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    clientOptions := options.Client().ApplyURI(uri)
    client, err := mongo.Connect(ctx, clientOptions)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
    }
    
    // Checking the connection
    ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    
    if err := client.Ping(ctx, readpref.Primary()); err != nil {
        return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
    }
    
    db := client.Database(dbName)
    
    mongoDB := &MongoDB{
        Client:    client,
        Database:  db,
        Users:     db.Collection("users"),
        Messages:  db.Collection("messages"),
        Settings:  db.Collection("settings"),
        Blacklist: db.Collection("blacklist"),
    }
    
    // Creating indexes
    createIndexes(mongoDB)
    
    DB = mongoDB
    log.Println("Connected to MongoDB successfully")
    return mongoDB, nil
}

func createIndexes(db *MongoDB) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    // Indexes for users
    usersIndexes := []mongo.IndexModel{
        {
            Keys: bson.D{{Key: "telegram_id", Value: 1}},
            Options: options.Index().SetUnique(true),
        },
        {
            Keys: bson.D{{Key: "username", Value: 1}},
        },
        {
            Keys: bson.D{{Key: "is_verified", Value: 1}},
        },
        {
            Keys: bson.D{{Key: "created_at", Value: -1}},
        },
    }
    
    _, err := db.Users.Indexes().CreateMany(ctx, usersIndexes)
    if err != nil {
        log.Printf("Error creating users indexes: %v", err)
    }
    
    // Indexes for messages
    messagesIndexes := []mongo.IndexModel{
        {
            Keys: bson.D{{Key: "user_id", Value: 1}},
        },
        {
            Keys: bson.D{{Key: "created_at", Value: -1}},
        },
        {
            Keys: bson.D{{Key: "is_forwarded", Value: 1}},
        },
    }
    
    _, err = db.Messages.Indexes().CreateMany(ctx, messagesIndexes)
    if err != nil {
        log.Printf("Error creating messages indexes: %v", err)
    }
}

func (db *MongoDB) Disconnect() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    if err := db.Client.Disconnect(ctx); err != nil {
        log.Printf("Error disconnecting from MongoDB: %v", err)
    }
    log.Println("Disconnected from MongoDB")
}

// CRUD operations for users
func (db *MongoDB) GetOrCreateUser(telegramID int64, username, firstName, lastName string, isBot bool) (*User, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // First we try to find the user
    var user User
    err := db.Users.FindOne(ctx, bson.M{"telegram_id": telegramID}).Decode(&user)
    
    if err == nil {
        // User found - update data if necessary
        updateFields := bson.M{}
        
        // We update only if the data has changed
        if username != user.Username && username != "" {
            updateFields["username"] = username
        }
        if firstName != user.FirstName {
            updateFields["first_name"] = firstName
        }
        if lastName != user.LastName {
            updateFields["last_name"] = lastName
        }
        
        if len(updateFields) > 0 {
            updateFields["updated_at"] = time.Now()
            _, err = db.Users.UpdateOne(
                ctx,
                bson.M{"telegram_id": telegramID},
                bson.M{"$set": updateFields},
            )
            if err != nil {
                log.Printf("Error updating user: %v", err)
            }
        }
        
        return &user, nil
    }
    
    // If the user is not found (err == mongo.ErrNoDocuments)
    // Create a new user
    now := time.Now()
    newUser := &User{
        TelegramID:   telegramID,
        Username:     username,
        FirstName:    firstName,
        LastName:     lastName,
        IsBot:        isBot,
        IsVerified:   false,
        IsBlocked:    false,
        CreatedAt:    now,
        UpdatedAt:    now,
        VerificationAttempts: 0,
    }
    
    _, err = db.Users.InsertOne(ctx, newUser)
    if err != nil {
        return nil, err
    }
    
    return newUser, nil
}

func (db *MongoDB) UpdateUserVerification(telegramID int64, isVerified bool) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    update := bson.M{
        "$set": bson.M{
            "is_verified": isVerified,
            "updated_at":  time.Now(),
        },
    }
    
    if isVerified {
        now := time.Now()
        update["$set"].(bson.M)["verified_at"] = now
    }
    
    _, err := db.Users.UpdateOne(
        ctx,
        bson.M{"telegram_id": telegramID},
        update,
    )
    
    return err
}

func (db *MongoDB) IncrementAttempts(telegramID int64) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    _, err := db.Users.UpdateOne(
        ctx,
        bson.M{"telegram_id": telegramID},
        bson.M{
            "$inc": bson.M{"verification_attempts": 1},
            "$set": bson.M{
                "last_attempt_at": time.Now(),
                "updated_at":      time.Now(),
            },
        },
    )
    
    return err
}

func (db *MongoDB) SaveCaptcha(telegramID int64, captcha *Captcha) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    _, err := db.Users.UpdateOne(
        ctx,
        bson.M{"telegram_id": telegramID},
        bson.M{
            "$set": bson.M{
                "captcha_data": captcha,
                "updated_at":   time.Now(),
            },
        },
    )
    
    return err
}

func (db *MongoDB) GetUserByTelegramID(telegramID int64) (*User, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    var user User
    err := db.Users.FindOne(ctx, bson.M{"telegram_id": telegramID}).Decode(&user)
    if err != nil {
        return nil, err
    }
    
    return &user, nil
}