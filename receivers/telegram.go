package receivers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/anthdm/hollywood/actor"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/wh1plash/notifier/types"
)

type Telegram struct {
	ActorEngine    *actor.Engine
	PID            *actor.PID
	logger         *slog.Logger
	bot            *tgbotapi.BotAPI
	handlers       map[string]CommandHandler
	cbHandlers     map[string]CallbackHandler
	textHandlers   []TextHandler
	defaultHandler TextMessageHandler

	StoragePID   *actor.PID
	mainImage    string
	instagramUrl string
}

// Handler types
type CommandHandler func(update tgbotapi.Update) tgbotapi.Chattable

type CallbackHandler func(update tgbotapi.Update) tgbotapi.Chattable

type TextMessageHandler func(update tgbotapi.Update) tgbotapi.Chattable

// TextHandler holds a pattern and its handler
type TextHandler struct {
	pattern *regexp.Regexp
	handler TextMessageHandler
}

func NewTelegramApi(token string) actor.Producer {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		panic(err)
	}
	return func() actor.Receiver {
		return &Telegram{
			bot:          bot,
			logger:       slog.Default(),
			handlers:     make(map[string]CommandHandler),
			cbHandlers:   make(map[string]CallbackHandler),
			textHandlers: []TextHandler{},
			defaultHandler: func(update tgbotapi.Update) tgbotapi.Chattable {
				return tgbotapi.NewMessage(update.Message.Chat.ID, "I'm not sure how to respond to that.")
			},
		}
	}
}

func (t *Telegram) Receive(c *actor.Context) {
	switch msg := c.Message().(type) {
	case actor.Initialized:
	case actor.Started:
		t.StoragePID = actor.NewPID("local", "server/system/storage/PocketBase")
		c.Send(t.StoragePID, types.InitConfig{})
		t.logger.Info("[TelegramAPI] started", "PID", c.PID())
		t.ActorEngine = c.Engine()
		t.PID = c.PID()
		go t.start()
	case types.LoadConfig:
		t.mainImage = msg.MainImage
		t.instagramUrl = msg.InstagramUrl
	case types.AllUsers:
		t.allUsers(msg)
	case types.SendMenu:
		t.allMenu(msg)
	case types.SendKind:
		t.kindMenu(msg)
	case types.NotifinationJob:
		job := t.sendNotification(msg)
		c.Send(t.StoragePID, job)
	case actor.Stopped:
		t.logger.Info("[TelegramAPI] stopped", "PID", c.PID())
		t.stop()
		c.Send(c.Parent(), types.ChangeState{ServiceName: "telegram", State: false})
	}
}

// type notifiedUsers struct {
// 	userID string
// 	result string
// }

func (t *Telegram) sendNotification(job types.NotifinationJob) types.NotifinationJob {
	img, err := os.ReadFile(job.Notifications[0].FullImagePath)
	if err != nil {
		fmt.Println("error to open file", err)
	}

	msg := job.Notifications[0].Message

	res := map[string]any{}
	for _, user := range job.Users {
		photo := tgbotapi.NewPhoto(user.ChatID, tgbotapi.FileBytes{
			Name:  "img",
			Bytes: img,
		})
		photo.Caption = msg

		_, err := t.bot.Send(photo)
		if err == nil { // If message was sent successfully, add to notifiedUsers
			res[user.ID] = "Success" //append(res, user.ID)
		} else {
			fmt.Println("Failed to send notification to user:", err)
			res[user.ID] = err
		}
	}

	notifiedUsersJSON, err := json.Marshal(res)
	if err != nil {
		fmt.Println("Error marshalling notifiedUsers:", err)
	} else {
		job.NotifiedUsers = string(notifiedUsersJSON) // Store JSON string in job.NotifiedUsers
	}

	job.Status = "Done"
	return job
}

func (t *Telegram) registerCommands() {
	commands := map[string]CommandHandler{
		"start": func(update tgbotapi.Update) tgbotapi.Chattable {
			// Read the image as bytes
			imageBytes, err := os.ReadFile(t.mainImage)
			if err != nil {
				fmt.Println("Error reading image file:", err)
			}

			photo := tgbotapi.NewPhoto(update.Message.Chat.ID, tgbotapi.FileBytes{
				Name:  "ImageFromPocketBase",
				Bytes: imageBytes,
			})
			photo.Caption = "Welcome! Please select an option:"
			photo.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonURL("Instagram", t.instagramUrl),
					tgbotapi.NewInlineKeyboardButtonData("Button2", "Button2"),
				),
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("GetUsers", "GetUsers"),
				),
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("Categories", "Kind"),
				),
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("Menu", "menu"),
				),
			)

			newUser := types.User{
				UserName:  update.Message.From.UserName,
				FirstName: update.Message.From.FirstName,
				LastName:  update.Message.From.LastName,
				ChatID:    update.Message.From.ID,
			}

			err = t.storeUser(newUser)
			if err != nil {
				fmt.Println("error to save user from telegram", err)
			}

			return photo
		},

		"help": func(update tgbotapi.Update) tgbotapi.Chattable {
			return tgbotapi.NewMessage(update.Message.Chat.ID, "This is the help message")
		},

		// Add more commands here

	}

	for cmd, handler := range commands {
		t.registerCommand(cmd, handler)
	}
}

func (t *Telegram) registerCallbacks() {
	callbacks := map[string]CallbackHandler{
		"Instagram": func(update tgbotapi.Update) tgbotapi.Chattable {
			return tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "go to the instagram")
		},
		"Button2": func(update tgbotapi.Update) tgbotapi.Chattable {
			return tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Button2 pressed")
		},

		"GetUsers": func(update tgbotapi.Update) tgbotapi.Chattable {
			ID := update.CallbackQuery.Message.Chat.ID
			t.ActorEngine.SendWithSender(t.StoragePID, types.GetInfo{ResponseTo: ID}, t.PID)

			return tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Fetching users, please wait...")
		},

		"Kind": func(update tgbotapi.Update) tgbotapi.Chattable {
			ID := update.CallbackQuery.Message.Chat.ID
			t.ActorEngine.SendWithSender(t.StoragePID, types.GetKind{ResponseTo: ID}, t.PID)

			return tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Fetching menu categories, please wait...")
		},

		"by_kind_menu": func(update tgbotapi.Update) tgbotapi.Chattable {
			callbackData := update.CallbackQuery.Data

			parts := strings.Split(callbackData, ":")
			if len(parts) < 2 {
				t.logger.Warn("Invalid callback data format", "warn", callbackData)
				return tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Something went wrong...")
			}
			itemID := parts[1]

			ID := update.CallbackQuery.Message.Chat.ID
			t.ActorEngine.SendWithSender(t.StoragePID, types.GetMenuByKind{RespondTo: ID, Kind: itemID}, t.PID)

			return tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Fetching menu by categories, please wait...")
		},

		"menu": func(update tgbotapi.Update) tgbotapi.Chattable {
			ID := update.CallbackQuery.Message.Chat.ID
			t.ActorEngine.SendWithSender(t.StoragePID, types.GetMenu{RespondTo: ID}, t.PID)

			return tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Fetching menu, please wait...")
		},

		"Main": t.backToMain,
	}

	for data, handler := range callbacks {
		t.registerCallback(data, handler)
	}
}

func (t *Telegram) allUsers(msg types.AllUsers) {
	// Format user list
	userMsg := "User List:\n"
	for _, user := range msg.ChatID {
		userMsg += fmt.Sprintf("- %d\n", user)
	}

	m := tgbotapi.NewMessage(msg.ResponseTo, userMsg)
	t.bot.Send(m)
}

func (t *Telegram) kindMenu(msg types.SendKind) {
	for _, item := range msg.Kind {
		imageBytes, _ := os.ReadFile(item.FullImagePath)
		photo := tgbotapi.NewPhoto(msg.RespondTo, tgbotapi.FileBytes{
			Name:  "Kind",
			Bytes: imageBytes,
		})

		photo.Caption = item.Name
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("View more", fmt.Sprintf("by_kind_menu:%s", item.ItemID)),
			),
		)
		photo.ReplyMarkup = keyboard
		t.bot.Send(photo)
	}

	back := tgbotapi.NewMessage(msg.RespondTo, "Back to Main Menu")
	back.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Main Menu", "Main"),
		),
	)

	t.bot.Send(back)
}

func (t *Telegram) allMenu(msg types.SendMenu) {

	if len(msg.Menu) > 0 && len(msg.Menu) <= 10 {

		//var mediaGroup []interface{}
		for _, item := range msg.Menu {
			imageBytes, _ := os.ReadFile(item.FullImagePath)
			photo := tgbotapi.NewPhoto(msg.ResponseTo, tgbotapi.FileBytes{
				Name:  "1",
				Bytes: imageBytes,
			})
			photo.Caption = item.Description
			//mediaGroup = append(mediaGroup, firstPhoto)

			keyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonURL("Visit site", item.ItemURL),
					tgbotapi.NewInlineKeyboardButtonData("Click", "button"),
				),
			)

			photo.ReplyMarkup = keyboard

			t.bot.Send(photo)

		}

		//mediaMsg := tgbotapi.NewMediaGroup(msg.ResponseTo, mediaGroup)
		//t.bot.Send(firstPhoto)
	}

	back := tgbotapi.NewMessage(msg.ResponseTo, "Back to...")
	back.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Categories", "Kind"),
			tgbotapi.NewInlineKeyboardButtonData("Main Menu", "Main"),
		),
	)

	t.bot.Send(back)

}

func (t *Telegram) start() {

	t.registerCommands()

	t.registerCallbacks()

	t.registerTextHandlers()

	// Start the bot
	fmt.Printf("Starting bot @%s\n", t.bot.Self.UserName)
	//block
	t.handleUpdate()

}

func (t *Telegram) handleUpdate() {
	t.bot.Debug = false
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := t.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			t.handleCallback(update)
		} else if update.Message != nil {
			if update.Message.IsCommand() {
				t.handleCommand(update)
			} else {
				t.handleTextMessage(update)
			}
		}
	}
}

func (t *Telegram) backToMain(update tgbotapi.Update) tgbotapi.Chattable {
	imageBytes, err := os.ReadFile(t.mainImage)
	if err != nil {
		fmt.Println("Error reading image file:", err)
		return tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Error loading the main page")
	}

	photo := tgbotapi.NewPhoto(update.CallbackQuery.Message.Chat.ID, tgbotapi.FileBytes{
		Name:  "MainImage",
		Bytes: imageBytes,
	})
	photo.Caption = "Welcome back! Please select an option:"
	photo.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Instagram", t.instagramUrl),
			tgbotapi.NewInlineKeyboardButtonData("Button2", "Button2"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("GetUsers", "GetUsers"),
		),

		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Categories", "Kind"),
		),

		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Menu", "menu"),
		),
	)

	return photo
}

func (t *Telegram) handleCommand(update tgbotapi.Update) {
	command := update.Message.Command()
	if handler, ok := t.handlers[command]; ok {
		response := handler(update)
		t.bot.Send(response)
	} else {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Unknown command")
		t.bot.Send(msg)
	}
}

func (t *Telegram) handleCallback(update tgbotapi.Update) {
	callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
	t.bot.Request(callback)

	data := update.CallbackQuery.Data

	for prefix, handler := range t.cbHandlers {
		if strings.HasPrefix(data, prefix) {
			response := handler(update)
			t.bot.Send(response)
			return
		}
	}
	msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Unhandled callback")
	t.bot.Send(msg)
}
func (t *Telegram) registerCommand(command string, handler CommandHandler) {
	t.handlers[command] = handler
}

func (t *Telegram) registerCallback(callbackData string, handler CallbackHandler) {
	t.cbHandlers[callbackData] = handler
}

// registerTextHandler adds a new text message handler with regex pattern
func (t *Telegram) registerTextHandler(pattern string, handler TextMessageHandler) error {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

	t.textHandlers = append(t.textHandlers, TextHandler{
		pattern: regex,
		handler: handler,
	})

	return nil
}

func (t *Telegram) storeUser(u types.User) error {
	t.ActorEngine.SendWithSender(t.StoragePID, u, t.PID)
	return nil
}

// SetDefaultHandler sets the handler for messages that don't match any pattern
func (t *Telegram) setDefaultHandler(handler TextMessageHandler) {
	t.defaultHandler = handler
}

func (t *Telegram) stop() {
	fmt.Println("Stopping Telegram bot...")
	t.bot.StopReceivingUpdates()
}

// --------actually trash
// Register text message handlers
func (t *Telegram) registerTextHandlers() {
	// Register patterns with handlers
	patternHandlers := []struct {
		pattern string
		handler TextMessageHandler
	}{
		{
			// Handle greetings
			pattern: "(?i)(hello|hi|hey)",
			handler: func(update tgbotapi.Update) tgbotapi.Chattable {
				return tgbotapi.NewMessage(update.Message.Chat.ID, "Hello! How can I help you today?")
			},
		},
		{
			// Handle questions
			pattern: "(?i).*\\?$",
			handler: func(update tgbotapi.Update) tgbotapi.Chattable {
				return tgbotapi.NewMessage(update.Message.Chat.ID, "That's a good question. Let me think about it.")
			},
		},
		{
			// Handle thanks
			pattern: "(?i)(thanks|thank you)",
			handler: func(update tgbotapi.Update) tgbotapi.Chattable {
				return tgbotapi.NewMessage(update.Message.Chat.ID, "You're welcome!")
			},
		},
		// Add more patterns and handlers as needed
	}

	for _, ph := range patternHandlers {
		if err := t.registerTextHandler(ph.pattern, ph.handler); err != nil {
			fmt.Printf("Error registering text handler pattern '%s': %v", ph.pattern, err)
		}
	}

	// Set default handler for unmatched messages
	t.setDefaultHandler(func(update tgbotapi.Update) tgbotapi.Chattable {
		return tgbotapi.NewMessage(update.Message.Chat.ID, "I'm not sure how to respond to that. Type /help for assistance.")
	})
}

// Handle text messages
func (t *Telegram) handleTextMessage(update tgbotapi.Update) {
	text := update.Message.Text

	// Try to match against registered patterns
	for _, handler := range t.textHandlers {
		if handler.pattern.MatchString(text) {
			response := handler.handler(update)
			t.bot.Send(response)
			return
		}
	}

	// If no pattern matched, use the default handler
	response := t.defaultHandler(update)
	t.bot.Send(response)
}
