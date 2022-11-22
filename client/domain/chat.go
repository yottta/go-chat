package domain

import (
	"fmt"
)

var ChatUserNotFoundErr = fmt.Errorf("chat user not found")

type Chat struct {
	Id        string
	OwnerUser User
	Users     []User
	Content   []Message
	Offline   bool
}

func (c Chat) GetOtherUsers() []User {
	return c.Users
}

func (c Chat) GetUser(id string) (*User, error) {
	for _, u := range c.Users {
		if u.Id == id {
			return &u, nil
		}
	}
	if id == c.OwnerUser.Id {
		return &c.OwnerUser, nil
	}
	return nil, ChatUserNotFoundErr
}

func (c Chat) GetAllUsers() []User {
	return append(c.Users, c.OwnerUser)
}
