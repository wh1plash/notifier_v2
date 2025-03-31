package receivers

import (
	"fmt"
	"log"
	"log/slog"

	//"time"
	"github.com/wh1plash/notifier/types"

	"github.com/anthdm/hollywood/actor"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gopkg.in/gomail.v2"
)

type notifier interface {
	sendNotify() bool
}

func sendNotification(n notifier) bool {
	return n.sendNotify()
}

type sender struct {
	notifier notifier
	logger   *slog.Logger
}

func NewSender() actor.Producer {
	return func() actor.Receiver {
		return &sender{
			notifier: logNotify{},
			logger:   slog.Default(),
		}
	}
}

func (p *sender) Receive(c *actor.Context) {
	switch msg := c.Message().(type) {
	case actor.Started:
		p.logger.Info("[Notifyer] started", "PID", c.PID())
		//res := sendNotification(p.notifier)
		//fmt.Println("Result of notification", res)

		// time.Sleep(time.Second * 5)

		// p.notifier = logNotify{}
		// fmt.Println(p.notifier)
		// sendNotification(p.notifier)
		// time.Sleep(time.Second * 5)

		// p.notifier = newEmailNotify()
		// fmt.Println(p.notifier)
		// sendNotification(p.notifier)
		// time.Sleep(time.Second * 5)
	case types.SendMSG:
		p.logger.Info("[Notifyer] get MSG")
	case types.SendEmails:
		p.logger.Info("[Notifyer] Emails", "emails", msg.Emails)
	case actor.Stopped:
		p.logger.Info("[Notifyer] stopped", "PID", c.PID())
		// case types.NotifinationMessage:
		// 	p.logger.Info("[Notifyer] Message from collection", "text", msg.Notifications)
		// 	//	p.notifier = newTelegramNotify(msg.Message)
		// 	sendNotification(p.notifier)

	}
}

type telegramNotify struct {
	botToken string
	chatID   int
	message  string
}

func newTelegramNotify(m string) *telegramNotify {
	return &telegramNotify{
		botToken: "7736243414:AAEotd7aAzfGblsw8IJmxPoYb-h8evpS5ic",
		chatID:   247048848,
		message:  m,
	}
}

func (t telegramNotify) sendNotify() bool {
	fmt.Println("From telegram notification")
	bot, err := tgbotapi.NewBotAPI(t.botToken)
	if err != nil {
		fmt.Printf("error to create telegram bot: %v", err)
	}
	bot.Debug = true

	msg := tgbotapi.NewMessage(int64(t.chatID), t.message)
	_, err = bot.Send(msg)
	if err != nil {
		fmt.Printf("error to send message: %v", err)
		return false
	}
	return true
}

type emailNotify struct {
	SMTPServer  string
	SMTPPort    int
	EmailSender string
	EmailPass   string
}

func newEmailNotify() *emailNotify {
	return &emailNotify{
		SMTPServer:  "smtp.gmail.com",
		SMTPPort:    465,
		EmailSender: "horbunovabakery@gmail.com",
		EmailPass:   "wqop diyf eydk vqsb",
	}
}

func (e emailNotify) sendNotify() {
	fmt.Println("From email notification")

	m := gomail.NewMessage()
	m.SetHeader("From", e.EmailSender)
	m.SetHeader("To", "whiplash2486@gmail.com")
	m.SetHeader("Subject", "TEST")
	m.SetBody("text/plain", "Test email form notifier")

	d := gomail.NewDialer(e.SMTPServer, e.SMTPPort, e.EmailSender, e.EmailPass)

	if err := d.DialAndSend(m); err != nil {
		log.Println("Failed to send email:", err)
	} else {
		log.Println("Email sent successfully")
	}
}

type logNotify struct{}

func (l logNotify) sendNotify() bool {
	fmt.Println("form log notifier")
	return true
}
