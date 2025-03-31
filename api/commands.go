package api

import "strings"

const (
	StartCmd = "/start"
)

func (p *Processor) doCmd(text string, chatID int64, userName string) error {
	text := strings.TrimSpace(text)
}
