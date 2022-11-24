package inmemory

import (
	"context"
	"github.com/yottta/chat/client/domain"
	"testing"
	"time"
)

func TestStore_RefreshUsers(t *testing.T) {
	currentUser := domain.User{
		Id:      "current_user_id",
		Name:    "current_user_name",
		Address: "192.168.0.1",
		Port:    1000,
	}
	testUser1 := domain.User{
		Id:      "user1",
		Name:    "user1",
		Address: "192.168.0.1",
		Port:    1001,
	}
	testUser2 := domain.User{
		Id:      "user2",
		Name:    "user2",
		Address: "192.168.0.1",
		Port:    1002,
	}
	t.Run(`Given a store, 
	When RefreshUsers is called with two users but one is the current user, 
	Then just one chat is added and the chat handler is called`, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		s := NewStore(ctx, currentUser)
		chatHandlerRequests := make(chan string, 1)

		s.RegisterChatHandler(func(ctx context.Context, chatId string) {
			chatHandlerRequests <- chatId
		})
		err := s.RefreshUsers([]domain.User{
			testUser1,
			currentUser,
		})
		if err != nil {
			t.Fatalf("expected to receive no error but received %s", err)
		}

		var checksDone int
	beforeCheck:
		for {
			select {
			case chatId := <-chatHandlerRequests:
				checksDone++
				chat, err := s.GetChat(chatId)
				if err != nil {
					t.Fatalf("chat not found in store %s", err)
				}
				users := chat.GetOtherUsers()
				if len(users) != 1 {
					t.Fatalf("wrong number of users in chat %s. expected %d but received %d", chat.Id, 1, len(chat.Users))
				}

				if testUser1.Id != users[0].Id {
					t.Fatalf("expected to find user with id %s in chat", testUser1.Id)
				}
				break beforeCheck
			case <-time.After(1 * time.Second):
				break beforeCheck
			}
		}
		if checksDone != 1 {
			t.Fatalf("expected just one chat events. received %d", checksDone)
		}

	})

	t.Run(`Given a store, 
	When RefreshUsers is called with two users and afterwards with just one, 
	Then the user missing from the second request is marked as offline`, func(t *testing.T) {
		// Given
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		s := NewStore(ctx, currentUser)

		err := s.RefreshUsers([]domain.User{
			testUser1,
			testUser2,
		})
		if err != nil {
			t.Fatalf("expected to receive no error but received %s", err)
		}
		chats := s.GetChats()
		if len(chats) != 2 {
			t.Fatalf("expected to have 2 chats in the store but there are %d", len(chats))
		}
		for _, c := range chats {
			if c.Offline {
				t.Fatalf("all chats should be online but %s is offline", c.Id)
			}
		}

		// When
		err = s.RefreshUsers([]domain.User{
			testUser1,
		})
		if err != nil {
			t.Fatalf("expected to receive no error but received %s", err)
		}
		chats = s.GetChats()
		if len(chats) != 2 {
			t.Fatalf("expected to have 2 chats in the store but there are %d", len(chats))
		}
		for _, c := range chats {
			if c.Offline == (c.Users[0].Id == testUser2.Id) {
				continue
			}
			t.Fatalf("chat %s should have been offline=%t", c.Id, c.Users[0].Id == testUser2.Id)
		}
	})
}
