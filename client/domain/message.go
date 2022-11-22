package domain

import "time"

type Message struct {
	ChatId       string
	UserId       string
	UserName     string
	Text         string
	At           time.Time
	ErrorMessage bool
}
