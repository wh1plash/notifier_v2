package types

import "time"

type GetJob struct {
}

type SendJobs struct {
	Jobs []Job
}

type JobResult struct {
	ID            string
	Res           string
	NotifiedUsers []string
}

type Job struct {
	ID            string         `db:"id"`
	Name          string         `db:"name"`
	Time          time.Time      `db:"time"`
	Status        string         `db:"status"`
	NotifiedUsers string         `db:"notifiedUsers"`
	Users         []User         //`db:"users"`
	Notifications []Notification //`db:"notification"`
}

// type Getnotifination struct {
// }

type NotifinationJob struct {
	Job
}

type Notification struct {
	ItemID        string `db:"id"`
	Message       string `db:"message"`
	Image         string `db:"image"`
	FullImagePath string
}

type User struct {
	ID        string `json:"id"`
	UserName  string `json:"userName"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	ChatID    int64  `json:"chatID"`
	IsAdmin   bool   `json:"isAdmin"`
}

type GetInfo struct {
	ResponseTo int64
}

type SendEmails struct {
	Emails []string
}

type SendInfo struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	UserName  string `json:"username"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	ChatID    int    `json:"chatID"`
	Created   string `json:"created"`
	Updated   string `json:"updated"`
}

type SendMSG struct {
}

type InitConfig struct{}

type LoadConfig struct {
	Token        string
	MainImage    string
	InstagramUrl string
}

type AllUsers struct {
	ResponseTo int64
	ChatID     []int64
}

type Ready struct{}

type ChangeState struct {
	ServiceName string
	State       bool
}

type GetMenu struct {
	RespondTo int64
}

type Menu struct {
	//Image       string
	Description   string `db:"description"`
	Kind          string `db:"kindName"`
	ItemID        string `db:"id"`
	Image         string `db:"itemImage"`
	FullImagePath string
	ItemURL       string `db:"itemURL"`
}

type SendMenu struct {
	ResponseTo int64
	Menu       []Menu
}

type GetKind struct {
	ResponseTo int64
}

type Kind struct {
	ItemID        string `db:"id"`
	Name          string `db:"name"`
	Image         string `db:"image"`
	FullImagePath string
}

type SendKind struct {
	RespondTo int64
	Kind      []Kind
}

type GetMenuByKind struct {
	RespondTo int64
	Kind      string
}
