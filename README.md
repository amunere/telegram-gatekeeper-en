# GateKeeper Bot 🔒

A Telegram bot for spam protection using a captcha system. New users must pass verification before gaining access to the chat.
## 🚀 Features

[x] Multi-type captcha: math problems, text questions, color selection

[x] Flexible configuration: all questions and settings via a .env file

[ ] Moderation: administrators can manage settings via commands

[ ] Statistics: tracking of activity and verification success rates

[ ] Automatic cleanup: removal of inactive captchas and user data

## 📦 Technologies

* Go (Golang) - primary language

* MongoDB - database

* go-telegram-bot-api - for working with the Telegram API

## 🛠 Installation and Setup

Clone the repository
    ```git clone https://github.com/yourusername/gatekeeper-bot.git
    cd gatekeeper-bot```

Configure the environment

Edit the .env file, filling in your data

Build and run
bash

    go mod download  
    go build -o gatekeeper-bot .  
    ./gatekeeper-bot  

## 🏗 Project Architecture

gatekeeper-bot/  
├── main.go  
├── config/  
│   └── config.go  
├── database/  
│   └── mongodb.go  
├── handler/  
│   └── bot_handler.go  
├── models/  
│   └── models.go  
├── .env                     
├── .gitignore  
├── Dockerfile  
├── go.mod  
├── go.sum  
└── README.md  

## MongoDB
### Build
    docker build -t gk-mongo:5.0 .

### Run
    docker run -d \
    --name gk-mongo \
    -p 127.0.0.1:27017:27017 \
    -v mongodb_data:/data/db \
    --memory="250m" \
    --memory-swap="350m" \
    gk-mongo:5.0

## 🔒 Setting up the bot in Telegram

Create a bot via @BotFather

Get the bot token and add it to your .env file (BOT_TOKEN=your_token_here)

Add the bot to your group and grant it Administrator privileges, specifically:

- Delete Messages

- Ban Users

- Invite Users via Link

- Pin Messages

Enable the "Allow Groups" mode in the bot's settings (BotFather -> Bot Settings -> Group Privacy -> Turn off).
