package receivers

import (
	"fmt"
	"log/slog"

	"github.com/anthdm/hollywood/actor"
	"github.com/wh1plash/notifier/types"
)

type serviceState struct {
	storageRunning   bool
	schedulerRunning bool
	telegramRunning  bool
}

type server struct {
	logger       *slog.Logger
	storagePID   *actor.PID
	schedulerPID *actor.PID
	telegramPID  *actor.PID
	serviceState
}

func (s *server) isAllRunning() bool {
	if ok := s.storageRunning && s.schedulerRunning && s.telegramRunning; !ok {
		return false
	}
	return true
}

func NewServer() actor.Receiver {
	return &server{
		logger:       slog.Default(),
		serviceState: serviceState{},
	}
}

func (s *server) Receive(c *actor.Context) {
	switch msg := c.Message().(type) {
	case actor.Started:
		s.logger.Info("[Server] started with", "PID", c.PID())
		s.logger.Info("[Server] starting Pocket Base...")
		s.storagePID = c.SpawnChild(NewStorage, "storage", actor.WithID("PocketBase"))
	case types.Ready:
		s.serviceState.storageRunning = true
		s.logger.Info("[Server] DB has ben started. continue...")
		c.Send(s.storagePID, types.InitConfig{})
	case types.LoadConfig:
		s.logger.Info("[Server] starting Telegram Bot...")
		s.telegramPID = c.SpawnChild(NewTelegramApi(msg.Token), "frontend", actor.WithID("telegram"))
		s.serviceState.telegramRunning = true
		s.logger.Info("[Server] starting Scheduler...")
		s.schedulerPID = c.SpawnChild(NewScheduler, "processor", actor.WithID("scheduler"))
		s.serviceState.schedulerRunning = true

		state := s.isAllRunning()
		fmt.Println("State of all services running:", state)
	case types.ChangeState:
		switch msg.ServiceName {
		case "storage":
			s.serviceState.schedulerRunning = msg.State
		case "scheduler":
			s.serviceState.schedulerRunning = msg.State
		case "telegram":
			s.serviceState.schedulerRunning = msg.State
		}

	case actor.Stopped:
		s.logger.Info("[Server] actor stopped", "PID", c.PID())
	}
}
