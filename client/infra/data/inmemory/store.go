package inmemory

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/yottta/chat/client/domain"
	"github.com/yottta/chat/client/infra/data"
	"log"
	"sort"
	"strings"
	"sync"
)

type store struct {
	currentUser domain.User

	m     *sync.Mutex
	chats map[string]domain.Chat

	hm                        *sync.Mutex
	chatLinesUpdatesListeners []data.MessageHandler
	cm                        *sync.Mutex
	chatUpdatesListeners      []data.ChatHandler

	chatLineUpdates chan domain.Message
	chatsUpdates    chan string
}

// NewStore creates the object that is the heart of the application.
// Careful, because this constructor spawns a goroutine everytime is called, so be sure that the context that you are giving to it is cancelled
// once your work with the store is done.
//
// This also needs the information of the current user. The purpose is to know what actor is the one that is running locally.
func NewStore(ctx context.Context, currentUser domain.User) data.Store {
	s := &store{
		currentUser: currentUser,

		m:     &sync.Mutex{},
		chats: make(map[string]domain.Chat),

		hm:              &sync.Mutex{},
		chatLineUpdates: make(chan domain.Message, 10),
		cm:              &sync.Mutex{},
		chatsUpdates:    make(chan string, 10),
	}

	go func() {
		defer log.Printf("store data updates closed")
		for {
			select {
			case <-ctx.Done():
				return
			case m := <-s.chatLineUpdates:
				for _, l := range s.chatLinesUpdatesListeners {
					go l(ctx, m)
				}
			case cId := <-s.chatsUpdates:
				for _, l := range s.chatUpdatesListeners {
					go l(ctx, cId)
				}
			}
		}
	}()
	return s
}

// AddChatLine stores a new domain.Message into the store.
// In case the chat is not in the store, an error is raised.
// In case that the targeted chat does not contain the targeted user, an error is raised.
// Once the message is added to the store, the message is scheduled to be sent to the handlers registered using #RegisterMessageHandler.
func (s *store) AddChatLine(message domain.Message) error {
	s.m.Lock()
	defer s.m.Unlock()
	c, ok := s.chats[message.ChatId]
	if !ok {
		return fmt.Errorf("%w: %s", data.ChatNotFoundErr, message.ChatId)
	}
	u, err := c.GetUser(message.UserId)
	if err != nil {
		return fmt.Errorf("%w: user %s, chat: %s", data.UserNotInChatErr, message.UserId, message.ChatId)
	}
	message.UserName = u.Name
	c.Content = append(c.Content, message)
	sort.Slice(c.Content, func(i, j int) bool {
		return c.Content[i].At.Before(c.Content[j].At)
	})

	s.chats[message.ChatId] = c
	s.sendLineUpdate(message)
	return nil
}

// RefreshUsers gets a list of users. It's trying to create new domain.Chat in the store with these.
// Will be generated one chat per user. Each chat object is requiring an id which is created as base64(join(sort({currentUser.id, users[n]}), "_"))
// If the users in the store are not in the received list of users, the chats are marked as offline.
func (s *store) RefreshUsers(users []domain.User) error {
	cu := s.CurrentUser()
	chats := s.GetChats()
	for _, u := range users {
		if u.Id == cu.Id {
			continue
		}

		chat, err := s.buildChat(u)
		if err != nil {
			return err
		}

		delete(chats, chat.Id)

		s.storeChat(*chat)
	}
	for _, c := range chats {
		c.Offline = true
		s.storeChat(c)
	}
	return nil
}

// GetChat gets a chat by the given ID. Error if not found.
func (s *store) GetChat(chatId string) (*domain.Chat, error) {
	s.m.Lock()
	defer s.m.Unlock()
	c, ok := s.chats[chatId]
	if !ok {
		return nil, fmt.Errorf("%w: %s", data.ChatNotFoundErr, chatId)
	}
	return &c, nil
}

// GetChats returns a map[string]domain.Chat where the key is the ID of the chat object.
func (s *store) GetChats() map[string]domain.Chat {
	s.m.Lock()
	defer s.m.Unlock()
	res := make(map[string]domain.Chat, len(s.chats))
	for k, v := range s.chats {
		res[k] = v
	}
	return res
}

// CurrentUser gets the current user, the one that was used to initiate the store with.
func (s *store) CurrentUser() domain.User {
	return s.currentUser
}

// RegisterMessageHandler registers a new data.MessageHandler that will be called every time a new message will be saved into the store.
func (s *store) RegisterMessageHandler(handler data.MessageHandler) {
	s.hm.Lock()
	defer s.hm.Unlock()
	s.chatLinesUpdatesListeners = append(s.chatLinesUpdatesListeners, handler)
}

// RegisterChatHandler registers a new data.ChatHandler that will be called every time a new chat will be saved into the store.
func (s *store) RegisterChatHandler(handler data.ChatHandler) {
	s.cm.Lock()
	defer s.cm.Unlock()
	s.chatUpdatesListeners = append(s.chatUpdatesListeners, handler)
}

func (s *store) sendLineUpdate(m domain.Message) {
	if s.chatLineUpdates == nil {
		return
	}
	select {
	case s.chatLineUpdates <- m:
	default:
		log.Printf("chat line update discarded because nobody is listening for it: %+v", m)
	}
}

func (s *store) sendChatUpdate(cId string) {
	if s.chatsUpdates == nil {
		return
	}
	select {
	case s.chatsUpdates <- cId:
	default:
		log.Printf("chat update discarded because nobody is listening for it: %s", cId)
	}
}

func (s *store) buildChat(users ...domain.User) (*domain.Chat, error) {
	userIds := make([]string, len(users)+1)
	var idx int
	userIds[idx] = s.currentUser.Id
	idx++
	for _, u := range users {
		if u.Id == s.currentUser.Id {
			return nil, data.WrongNewChatUsersErr
		}
		userIds[idx] = u.Id
		idx++
	}
	sort.Strings(userIds)
	chatId := base64.StdEncoding.EncodeToString([]byte(strings.Join(userIds, "_")))
	chat := domain.Chat{
		Id:        chatId,
		OwnerUser: s.currentUser,
		Users:     users,
		Content:   nil,
		Offline:   false,
	}
	return &chat, nil
}

func (s *store) storeChat(chat domain.Chat) {
	s.m.Lock()
	defer s.m.Unlock()
	if c, ok := s.chats[chat.Id]; ok {
		chat.Content = c.Content
	}
	s.chats[chat.Id] = chat
	s.sendChatUpdate(chat.Id)
}
