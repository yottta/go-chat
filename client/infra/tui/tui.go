package tui

import (
	"context"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/yottta/chat/client/domain"
	"github.com/yottta/chat/client/infra/data"
	"log"
	"strings"
	"time"
)

type Handler interface {
	Start(ctx context.Context) error
}

type handler struct {
	users        *CList[*domain.Chat]
	chat         *tview.List
	messageField *tview.InputField
	app          *tview.Application

	currentChat *domain.Chat
	s           data.Store
}

func New(store data.Store) Handler {
	users := NewCustomList[*domain.Chat](func(chat *domain.Chat) (string, string) {
		users := chat.GetOtherUsers()

		userNames := make([]string, len(users))
		var idx int
		for _, u := range users {
			userNames[idx] = u.Name
			idx++
		}
		var offlineTag string
		if chat.Offline {
			offlineTag = " (offline)"
		}
		return strings.Join(userNames, ",") + offlineTag, chat.Id
	})
	users.SetTitle(fmt.Sprintf("Users(%s)", store.CurrentUser().Name))
	users.SetBorder(true)
	users.ShowSecondaryText(false)

	chat := tview.NewList()
	chat.SetCurrentItem(chat.GetItemCount() - 1)
	chat.SetBorder(true).SetTitle("Chat")
	chat.ShowSecondaryText(false)

	messageField := tview.NewInputField().
		SetPlaceholder("message")
	messageField.SetBorder(true).SetTitle("Message")

	application := tview.NewApplication()

	return &handler{
		users:        users,
		chat:         chat,
		messageField: messageField,
		app:          application,
		s:            store,
	}
}

func (h *handler) Start(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		h.app.Stop()
	}()
	h.bindActions()
	h.bindStoreListeners()

	flex := tview.NewFlex().
		AddItem(h.users, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(h.chat, 0, 5, false).
			AddItem(h.messageField, 0, 1, false),
			0, 5, false)

	h.app.SetFocus(h.users)
	if err := h.app.SetRoot(flex, true).EnableMouse(false).Run(); err != nil {
		return err
	}
	return nil
}

func (h *handler) bindActions() {
	h.users.SetSelectedFunc(func(i int, s string, s2 string, r rune) {
		h.chat.Clear()
		chat, err := h.s.GetChat(s2)
		if err != nil {
			h.chat.AddItem("ERROR, TRY AGAIN", "", 0, nil)
			return
		}
		for _, m := range chat.Content {
			h.addChatMessage(m)
		}

		//users := chat.GetOtherUsers()
		//userNames := make([]string, len(users))
		//var idx int
		//for _, u := range users {
		//	userNames[idx] = u.Name
		//	idx++
		//}
		//h.chat.SetTitle(strings.Join(userNames, ","))
		h.chat.SetTitle(s)
		h.chat.SetCurrentItem(h.chat.GetItemCount() - 1)
		h.app.SetFocus(h.messageField)
		h.users.SetUnreadChat(chat.Id, false)
		h.currentChat = chat
	})

	h.messageField.SetDoneFunc(func(key tcell.Key) {
		txt := strings.TrimSpace(h.messageField.GetText())
		if len(txt) > 0 {
			h.s.AddChatLine(domain.Message{
				ChatId: h.currentChat.Id,
				UserId: h.s.CurrentUser().Id,
				Text:   txt,
				At:     time.Now(),
			})
		}
		h.messageField.SetText("")
	})

	focusChain := []tview.Primitive{h.messageField, h.chat, h.users}
	focusNext := func(focused tview.Primitive) tview.Primitive {
		focusedIdx := -1
		for i := range focusChain {
			if focused == focusChain[i] {
				focusedIdx = i
				break
			}
		}
		switch focusedIdx {
		case -1:
			return focusChain[0]
		case len(focusChain) - 1:
			return focusChain[0]
		default:
			return focusChain[focusedIdx+1]
		}
	}

	h.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == 9 {
			h.app.SetFocus(focusNext(h.app.GetFocus()))
			return nil
		}

		//// Ctrl+C
		//if event.Key() == 3 && event.Modifiers() == tcell.ModCtrl {
		//	return nil
		//}
		return event
	})
	h.app.SetFocus(h.messageField)
}

func (h *handler) bindStoreListeners() {
	h.s.RegisterChatHandler(func(ctx context.Context, cu string) {
		chat, err := h.s.GetChat(cu)
		if err != nil {
			log.Printf("something wrong with the store as it sent update for %s chat but GetChat returned error %s", cu, err)
			return
		}
		h.users.AddItem(chat.Id, chat)
		h.app.QueueUpdateDraw(func() {})
	})

	h.s.RegisterMessageHandler(func(ctx context.Context, msg domain.Message) {
		if h.currentChat == nil || msg.ChatId != h.currentChat.Id {
			h.users.SetUnreadChat(msg.ChatId, true)
			h.app.QueueUpdateDraw(func() {})
			return
		}
		h.addChatMessage(msg)
		h.chat.SetCurrentItem(h.chat.GetItemCount() - 1)
		h.app.QueueUpdateDraw(func() {})
	})
}

func (h *handler) addChatMessage(msg domain.Message) {
	if msg.ErrorMessage {
		h.chat.AddItem(msg.Text, "", 0, nil)
	} else {
		h.chat.AddItem(formatChatText(msg.Text, msg.UserName, msg.At), "", 0, nil)
	}
}

func formatChatText(text, userName string, at time.Time) string {
	formatted := at.Format(time.Stamp)
	return fmt.Sprintf("%s (%s): %s", userName, formatted, text)
}
