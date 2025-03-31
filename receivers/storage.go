package receivers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/anthdm/hollywood/actor"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/wh1plash/notifier/types"
)

type Storage struct {
	logger *slog.Logger
	store  *pocketbase.PocketBase
}

func NewStorage() actor.Receiver {
	return &Storage{
		logger: slog.Default(),
		store:  pocketbase.New(),
	}
}

func (s *Storage) Receive(c *actor.Context) {
	switch msg := c.Message().(type) {
	case actor.Started:
		started := make(chan struct{})
		go s.start(started)
		<-started
		s.logger.Info("[Storage] started", "PID", c.PID())
		c.Send(c.Parent(), types.Ready{})
	case actor.Stopped:
		s.stop()
		s.logger.Info("[Storage] stopped", "PID", c.PID())
		c.Send(c.Parent(), types.ChangeState{ServiceName: "storage", State: false})
	case types.InitConfig:
		s.logger.Info("[Storage] geting configuration from database")
		loadConfig := s.getConfig()
		c.Send(c.Sender(), loadConfig)
	case types.GetInfo:
		users := s.getusers()
		c.Send(c.Sender(), types.AllUsers{
			ResponseTo: msg.ResponseTo,
			ChatID:     users,
		})
	case types.GetKind:
		kinds, err := s.kind()
		if err != nil {
			s.logger.Error("[Storage] error to get menu categories from Database", "err", err)
		}
		c.Send(c.Sender(), types.SendKind{
			RespondTo: msg.ResponseTo,
			Kind:      kinds,
		})
	case types.GetMenu:
		menu, err := s.menu()
		if err != nil {
			s.logger.Error("[Storage] error to get menu from Database", "err", err)
		}

		c.Send(c.Sender(), types.SendMenu{
			ResponseTo: msg.RespondTo,
			Menu:       menu,
		})
	case types.GetMenuByKind:
		kind := msg.Kind
		menu, err := s.menuByKind(kind)
		if err != nil {
			s.logger.Error("[Storage] error to get menu by kind from Database", "err", err)
		}
		c.Send(c.Sender(), types.SendMenu{
			ResponseTo: msg.RespondTo,
			Menu:       menu,
		})
	case types.User:
		fmt.Println("Get message to store user to DB", msg)
		s.storeUserToDB(msg)
	case types.GetJob:
		jobs, err := s.schedule()
		if err != nil {
			s.logger.Error("[Storage] error to get scheduler from DB", "err", err)
		}
		c.Send(c.Sender(), types.SendJobs{Jobs: jobs})
		//s.jobResultByID("qr7z5l653nw18c0")
	case types.JobResult:
		if err := s.setState(msg.ID, msg.Res); err != nil {
			s.logger.Error("[Storage] error to set state of job", "err", err)
		}
	case types.NotifinationJob:
		if err := s.resJob(msg.ID, msg.Status, msg.NotifiedUsers); err != nil {
			s.logger.Error("[Storage] error to set state of job", "err", err)
		}
	}
}

func (s *Storage) resJob(id string, res string, users string) error {
	record, err := s.store.App.FindRecordById("scheduler", id)
	if err != nil {
		return err
	}
	record.Set("status", res)
	record.Set("notifiedUsers", users)
	if err = s.store.App.Save(record); err != nil {
		return err
	}
	return nil
}

func (s *Storage) jobResultByID(id string) {
	record, err := s.store.App.FindRecordById("scheduler", id)
	if err != nil {
		fmt.Printf("error to get result of job: %s, err:%v", id, err)
	}

	fmt.Printf("Fetch result of job: %+v\n", record.GetString("notifiedUsers"))

	result := map[string]any{}

	if err := json.Unmarshal([]byte(record.GetString("notifiedUsers")), &result); err != nil {
		fmt.Println("error to unmarshal data", err)
	}
	for _, i := range result {
		fmt.Printf("Users: %+v\n", i)
	}

}

func (s *Storage) setState(id string, res string) error {
	record, err := s.store.App.FindRecordById("scheduler", id)
	if err != nil {
		return err
	}
	record.Set("status", res)
	if err = s.store.App.Save(record); err != nil {
		return err
	}
	return nil
}

func (s *Storage) schedule() ([]types.Job, error) {
	var rawJobs []struct {
		JobID  string `db:"id"`
		Name   string `db:"name"`
		Time   string `db:"time"`
		Status string `db:"status"`
	}

	err := s.store.App.DB().NewQuery("SELECT id, name, time, status from scheduler where status not in ('Done')").All(&rawJobs)
	if err != nil {
		return nil, err
	}

	layout := "2006-01-02 15:04:05Z"

	jobMap := make(map[string]*types.Job)

	for _, job := range rawJobs {
		parsedTime, err := time.Parse(layout, job.Time)
		if err != nil {
			return nil, fmt.Errorf("error parsing time: %v", err)
		}
		j := &types.Job{
			ID:            job.JobID,
			Name:          job.Name,
			Time:          parsedTime,
			Status:        job.Status,
			Users:         []types.User{},
			Notifications: []types.Notification{},
		}
		jobMap[job.JobID] = j
	}

	var rawUsers []struct {
		JobID     string `db:"job_id"`
		UserID    string `db:"id"`
		UserName  string `db:"userName"`
		FirstName string `db:"firstName"`
		LastName  string `db:"lastName"`
		ChatID    int64  `db:"chatID"`
		IsAdmin   bool   `db:"isAdmin"`
	}

	err = s.store.App.DB().NewQuery("SELECT s.id as job_id, c.id, c.userName, c.firstName, c.lastName, c.chatID, c.isAdmin from scheduler s JOIN clients c ON INSTR(s.users, c.id) > 0").All(&rawUsers)
	if err != nil {
		return nil, err
	}

	for _, user := range rawUsers {
		if job, exists := jobMap[user.JobID]; exists {

			job.Users = append(job.Users, types.User{
				ID:        user.UserID,
				UserName:  user.UserName,
				FirstName: user.FirstName,
				LastName:  user.LastName,
				ChatID:    user.ChatID,
				IsAdmin:   user.IsAdmin,
			})
		}
	}

	var rawNotifications []struct {
		JobID   string `db:"job_id"`
		ID      string `db:"id"`
		Message string `db:"message"`
		Image   string `db:"image"`
	}

	err = s.store.App.DB().NewQuery("SELECT n.id, n.message, n.image, s.id as job_id FROM notifications n JOIN scheduler s ON n.id = s.notification").All(&rawNotifications)
	if err != nil {
		return nil, err
	}

	for _, n := range rawNotifications {
		if job, exists := jobMap[n.JobID]; exists {
			imagePath := s.fullImagePath("notifications", n.ID, n.Image)
			job.Notifications = append(job.Notifications, types.Notification{
				ItemID:        n.ID,
				Message:       n.Message,
				Image:         n.Image,
				FullImagePath: imagePath,
			})

		}
	}

	var jobs []types.Job
	for job, i := range jobMap {
		jobs = append(jobs, types.Job{
			ID:            job,
			Name:          i.Name,
			Time:          i.Time,
			Status:        i.Status,
			Users:         i.Users,
			Notifications: i.Notifications,
		})
	}

	return jobs, nil
}

func (s *Storage) menu() ([]types.Menu, error) {
	menu := []types.Menu{}
	err := s.store.App.DB().NewQuery("select menu.description, menu.itemImage, kind.name as kindName, menu.id, menu.itemURL from menu join kind on menu.kind = kind.id").All(&menu)
	if err != nil {
		return nil, err
	}

	for i := range menu {
		str := s.fullImagePath("menu", menu[i].ItemID, menu[i].Image)
		menu[i].FullImagePath = str
	}

	return menu, nil
}

func (s *Storage) menuByKind(k string) ([]types.Menu, error) {
	menu := []types.Menu{}
	err := s.store.App.DB().NewQuery("select menu.id, menu.description, menu.itemImage, menu.itemURL, kind.name as kindName from menu join kind on menu.kind = kind.id where kind.id = {:kind}").
		Bind(dbx.Params{
			"kind": k,
		}).All(&menu)
	if err != nil {
		return nil, err
	}

	for i := range menu {
		str := s.fullImagePath("menu", menu[i].ItemID, menu[i].Image)
		menu[i].FullImagePath = str
	}
	return menu, nil
}

func (s *Storage) kind() ([]types.Kind, error) {
	kind := []types.Kind{}
	err := s.store.App.DB().NewQuery("select * From kind ").All(&kind)
	if err != nil {
		return nil, err
	}

	for i := range kind {
		str := s.fullImagePath("kind", kind[i].ItemID, kind[i].Image)
		kind[i].FullImagePath = str
	}

	return kind, nil
}

func (s *Storage) fullImagePath(c string, str string, img string) string {
	collection, err := s.store.App.FindCollectionByNameOrId(c)
	if err != nil {
		fmt.Println("error get collection ID")
	}
	imgPath := fmt.Sprintf("./bin/pb_data/storage/%s/%s/%s", collection.Id, str, img)
	return imgPath
}

func (s *Storage) getusers() []int64 {
	chats := []Chat{}
	err := s.store.App.DB().NewQuery("select chatID from clients").All(&chats)
	if err != nil {
		fmt.Println("No users found in database")
	}

	var tele []int64
	for _, chat := range chats {
		tele = append(tele, chat.ChatID)
	}

	return tele

}

func (s *Storage) storeUserToDB(msg types.User) {
	collection, err := s.store.App.FindCollectionByNameOrId("clients")
	if err != nil {
		fmt.Println("Error to open collection")
	}

	user, _ := s.store.App.FindFirstRecordByData("clients", "userName", msg.UserName)
	if user != nil {
		fmt.Println("user alredy exist. Updating user info", user)
		user.Set("userName", msg.UserName)
		user.Set("firstName", msg.FirstName)
		user.Set("lastName", msg.LastName)
		user.Set("chatID", msg.ChatID)
		s.store.App.Save(user)
		return
	}

	record := core.NewRecord(collection)
	record.Set("userName", msg.UserName)
	record.Set("firstName", msg.FirstName)
	record.Set("lastName", msg.LastName)
	record.Set("chatID", msg.ChatID)

	err = s.store.App.Save(record)
	if err != nil {
		fmt.Println("Error to save user to DB")
	}
	fmt.Println("user stored sucessfuly")

}

func (s *Storage) getConfig() types.LoadConfig {
	rec, err := s.store.App.FindFirstRecordByData("configuration", "active", true)
	if err != nil {
		fmt.Println("error to get configuration data", err)
	}

	str, err := s.getFullFilePath("configuration", rec)

	if err != nil {
		fmt.Println("Error to get full file path:", err)
	}

	resp := types.LoadConfig{
		Token:        rec.GetString("token"),
		MainImage:    str,
		InstagramUrl: rec.GetString("instagram"),
	}
	return resp
}

func (s *Storage) getFullFilePath(c string, rec *core.Record) (string, error) {
	collection, err := s.store.App.FindCollectionByNameOrId(c)
	if err != nil {
		return "", err
	}

	imgPath := fmt.Sprintf("./bin/pb_data/storage/%s/%s/%s", collection.Id, rec.Id, rec.GetString("mainImage"))

	return imgPath, nil
}

type Chat struct {
	ChatID int64 `db:"chatID"`
}

type Client struct {
	ID      string  `json:"id"`
	Email   string  `json:"email"`
	Name    string  `json:"name"`
	Phone   string  `json:"phone"`
	ChatID  float64 `json:"chatID"`
	Created string  `json:"created"`
	Updated string  `json:"updated"`
}

func (s *Storage) start(started chan<- struct{}) {

	s.store.OnServe().BindFunc(func(se *core.ServeEvent) error {
		time.Sleep(time.Second)
		close(started)
		return se.Next()
	})

	if err := s.store.Start(); err != nil {
		s.logger.Error("error starting storage", "err", err)
	}

}

func (s *Storage) stop() {
	s.store.Cron().Stop()
}
