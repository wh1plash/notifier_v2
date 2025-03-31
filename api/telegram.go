package api

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var commands = map[string]CommandHandler{
	"/start": startHandler,
	"/help":  helpHandler,
}

type CommandHandler func(*tgbotapi.BotAPI, *tgbotapi.Message)

type App struct {
	bot      *tgbotapi.BotAPI
	handlers map[string]CommandHandler
}

func newApp() *App {
	bot, err := tgbotapi.NewBotAPI("7017955166:AAFkXZYyed3Py9HyrALKgdmmjgh6z15endk")
	if err != nil {
		log.Fatal("error to create bot")
	}
	return &App{
		bot:      bot,
		handlers: make(map[string]CommandHandler),
	}
}

func startHandler(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	reply := tgbotapi.NewMessage(msg.Chat.ID, "Main message")
	bot.Send(reply)
}

func helpHandler(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	reply := tgbotapi.NewMessage(msg.Chat.ID, "Available commands:\n/start - Start the bot\n/help - Show this message")
	bot.Send(reply)
}

func unknownCommand(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	reply := tgbotapi.NewMessage(msg.Chat.ID, "Sorry, I don't understand that command.")
	bot.Send(reply)
}

func handleUpdate(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {

	if handler, exists := commands[msg.Text]; exists {
		handler(bot, msg) // Call the function dynamically
	} else {
		unknownCommand(bot, msg) // Default handler
	}
}

func main() {
	newApp()
}

func (a *App) start() {
	fmt.Printf("Starting bot @%s\n", a.bot.Self.UserName)
	a.handleUpdate()
}

func (a *App) handleUpdate() {

	a.bot.Debug = true
	fmt.Println("Bot is running...")

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := a.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			go handleUpdate(a.bot, update.Message) // Process messages asynchronously
		}
	}

}
