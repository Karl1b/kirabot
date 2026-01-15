# kirabot
Telegram chatbot that is a virtual "girlfriend" (not spicy!) sorry - not sorry!

Note that this is in GERMAN :-) - you need to translate the prompts for this to be english.

![Karen](./karen.png)

## Features

### Core Functionality
- **Virtual companion chatbot** - Acts as a "virtual girlfriend" designed to combat loneliness (FSK 12 / age-appropriate, explicitly not a "Sexbot")
- **Telegram integration** - Runs as a Telegram bot that users can chat with directly

### Memory System
- **Short-term memory** - Retains the last 20 messages of the conversation
- **Long-term memory** - Stores important information in a JSON structure including:
  - Life events
  - Dreams and wishes
  - Current topics
  - Hard facts (e.g., people in the user's life like family members)
- **Automatic memory updates** - Every 15 messages, a separate LLM analyzes the chat and updates the memory JSON

### Conversation Behavior
- **Proactive messaging** - Can initiate conversations on its own when the user hasn't written in a while
- **Optional responses** - Doesn't have to reply to every message (can return an empty string to stay silent)
- **Multi-message responses** - Can split longer responses into multiple messages sent with time delays
- **Time awareness** - Incorporates timestamps for each message, considering both response time and time of day
- **Consistent personality** - Maintains a coherent persona across unlimited conversation length

### Safety & Access Control
- **User whitelist** - Only responds to users listed in `allowed_users.txt`
- **Daily message limits** - Configurable limits on usage (adjustable in `limits.go`)
- **Content moderation handling** - Includes workarounds for when user input triggers LLM safety filters (Content RAG Poisoning mitigation)

## Setup

$cp examaple.env .env

1. Get your Google API Key and add it to .env

2. Get your Bot token from Telegram (Chat with @BotFather directly in the app) and add it to .env

Also add a nice pic for the bot.
```txt
TELEGRAMTOKEN=
LLMKEY=
```

3. Add your or any TelegramID to allowed_users.txt (without @, only the name)

Bot will only answer to users!
```txt
user1
user2
```

Build and run

$go build

$./kira

4. if you want to increase daily limit go to limits.go 

5. [https://go4lage.com/geminicv](https://go4lage.com/geminicv) Read this about vendor lock-in if you want to use any other LLM API. Gemini is nice, but right now it is only a friendship model.

Have fun ;-)