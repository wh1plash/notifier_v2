package receivers

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/anthdm/hollywood/actor"
	"github.com/wh1plash/notifier/types"
)

type executer struct {
	job    types.Job
	quitch chan struct{}
}

func newExecuter(job types.Job) actor.Producer {
	return func() actor.Receiver {
		return &executer{
			job:    job,
			quitch: make(chan struct{}),
		}
	}
}

func (e *executer) Receive(c *actor.Context) {
	switch c.Message().(type) {
	case actor.Started:
		fmt.Println("Executer actor started with PID", c.PID())

		go e.execut(e.job, c)
	case actor.Stopped:
		e.stop()
		fmt.Println("Executer actor stopped", c.PID())
	}
}

func (e *executer) execut(job types.Job, c *actor.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

loop:
	for {
		now := time.Now().UTC()
		jobTime := job.Time.UTC()
		select {
		case <-e.quitch:
			fmt.Println("Before stoopping, send status")
			c.Send(c.Parent(), types.JobResult{ID: job.ID, Res: "Stopped"})
			break loop
		case <-ticker.C:
			if jobTime.Before(now) {
				//fmt.Printf("Job %s is overdue. Scheduled for %v, current time is %v\n", job.Name, jobTime, now)
				c.Send(c.Parent(), types.JobResult{ID: job.ID, Res: "Executing"})
				tPID := actor.NewPID("local", "server/system/frontend/telegram")
				c.Send(tPID, types.NotifinationJob{Job: job})
				return
			} else {
				timeUntilJob := jobTime.Sub(now)
				fmt.Printf("Job %s will run in %v\n", job.Name, timeUntilJob)
			}
		}
	}
	fmt.Printf("Executer with job %s stopped by quit channel\n", job.ID)
}

func (e *executer) stop() {
	close(e.quitch)
}

type Scheduler struct {
	logger *slog.Logger
	//notifierPID *actor.PID
	executers  map[string]*actor.PID
	StoragePID *actor.PID
}

func NewScheduler() actor.Receiver {
	return &Scheduler{
		logger:    slog.Default(),
		executers: make(map[string]*actor.PID),
	}
}

func (s *Scheduler) Receive(c *actor.Context) {
	switch msg := c.Message().(type) {
	case actor.Started:
		s.logger.Info("[Scheduler] started", "PID", c.PID())
		//s.notifierPID = c.SpawnChild(NewSender(), "notifier", actor.WithID("log"))
		s.getJob(c)
	case types.SendJobs:
		for _, job := range msg.Jobs {
			switch job.Status {
			case "Done":
				//fmt.Println("job is Done, skipping")
			case "Processing":
				//fmt.Println("I dint known yet, skipping")
			case "":
				executerPID := c.SpawnChild(newExecuter(job), "executer", actor.WithID(job.ID))
				s.executers[job.ID] = executerPID
				c.Send(s.StoragePID, types.JobResult{ID: job.ID, Res: "Processing"})
				//fmt.Println("=========list of executers", s.executers)
			case "Stopped":
				if _, ok := s.executers[job.ID]; ok {
					c.Engine().Poison(s.executers[job.ID])
					delete(s.executers, job.ID)
				}
			}
		}
	case types.JobResult:
		fmt.Printf("Received message from executer %s result of job %s\n", c.Sender().ID, msg.Res)
		c.Send(s.StoragePID, types.JobResult{ID: msg.ID, Res: msg.Res})
		c.Engine().Poison(c.Sender())
	case actor.Stopped:
		s.logger.Info("[Scheduler] stopped", "PID", c.PID())
		c.Send(c.Parent(), types.ChangeState{ServiceName: "scheduler", State: false})
	}

}

func (s *Scheduler) getJob(c *actor.Context) {
	s.StoragePID = actor.NewPID("local", "server/system/storage/PocketBase")
	c.SendRepeat(s.StoragePID, types.GetJob{}, time.Second*30)
}
