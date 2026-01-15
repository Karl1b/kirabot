# kirabot
Telegram chatbot that is a virtual "girlfriend" (not spicy!) sorry - not sorry!

Note that this is in GERMAN :-) - you need to translate the prompts for this to be english.

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

