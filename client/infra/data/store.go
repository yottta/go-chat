package data

import (
	"context"
	"errors"
	"github.com/yottta/chat/client/domain"
)

var (
	UserNotInChatErr     = errors.New("user not found in chat")
	ChatNotFoundErr      = errors.New("chat not found")
	WrongNewChatUsersErr = errors.New("a new chat should not include the current user")
)

// MessageHandler is the function that is going to receive any domain.Message object that is added to the store
type MessageHandler func(ctx context.Context, m domain.Message)

// ChatHandler is the function that is going to receive any domain.Chat object that is added to the store
type ChatHandler func(ctx context.Context, chatId string)

// Store describes the functionality needed for the application to work. This is the central point
// of the app as the communication between socket connectivity layer and UI layer is done through this.
type Store interface {
	RefreshUsers(users []domain.User) error
	AddChatLine(m domain.Message) error
	GetChat(chatId string) (*domain.Chat, error)
	GetChats() map[string]domain.Chat
	CurrentUser() domain.User

	RegisterMessageHandler(handler MessageHandler)
	RegisterChatHandler(handler ChatHandler)
}
