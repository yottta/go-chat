package socket

import (
	"context"
	"errors"
	"fmt"
	"github.com/yottta/chat/client/domain"
	"github.com/yottta/chat/client/infra/data"
	"github.com/yottta/chat/client/infra/socket/conn"
	"log"
	"net"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// Socket handles the connections that are coming to the opened port and also is handling the outgoing connections
// whenever a new message is received from the data.Store.
// In order for it to work properly, call Listen with a context and be sure that the context is cancellable or initialized with a timeout.
type Socket interface {
	Listen(ctx context.Context) error
	AllocatedPort() int
	LocalIP() string
	RegisterStore(store data.Store)
}

type socket struct {
	port  int
	ip    string
	store data.Store

	cm          *sync.Mutex
	connections map[string]conn.Conn
}

func NewSocket() (Socket, error) {
	ip, err := findIp()
	if err != nil {
		return nil, err
	}
	return &socket{
		ip: ip,

		cm:          &sync.Mutex{},
		connections: map[string]conn.Conn{},
	}, nil
}

func (s *socket) RegisterStore(store data.Store) {
	s.store = store
	s.store.RegisterMessageHandler(func(ctx context.Context, m domain.Message) {
		if m.UserId != s.store.CurrentUser().Id {
			return
		}
		s.handleOutgoingMessages(ctx, m)
	})
}

const portSeed = 1000

func (s *socket) listenOnAvailablePort() (net.Listener, int, error) {
	for i := portSeed; i < 65535; i++ {
		l, err := net.Listen("tcp", ":"+strconv.Itoa(i))
		if err != nil {
			if errors.Is(err, syscall.EADDRINUSE) {
				continue
			}
			return nil, 0, err
		}
		return l, i, nil
	}
	return nil, 0, fmt.Errorf("no available port")
}

func (s *socket) Listen(ctx context.Context) error {
	l, port, err := s.listenOnAvailablePort()
	if err != nil {
		return err
	}
	s.port = port
	go func() {
		<-ctx.Done()
		log.Println("closing socket client")
		if err := l.Close(); err != nil {
			fmt.Printf("error closing the socket listener when context was closed: %s", err)
		}
	}()

	go func() {
		s.listenIncomingConns(ctx, l)
	}()

	return nil
}

func (s *socket) listenIncomingConns(ctx context.Context, l net.Listener) {
	defer log.Println("closing incoming conns")
	for {
		newCon, err := l.Accept()
		if err != nil {
			log.Printf("error accepting connection %s", err)
			if errors.Is(err, net.ErrClosed) {
				break
			}
			continue
		}

		go s.handleNewConn(ctx, newCon)
	}
}

func (s *socket) AllocatedPort() int {
	return s.port
}

func (s *socket) handleNewConn(ctx context.Context, establishedConn net.Conn) {
	_ = establishedConn.SetReadDeadline(time.Now().Add(5 * time.Second))

	m, err := conn.ReadNetworkMessage(establishedConn)
	if err != nil {
		fmt.Printf("error reading network message: %s", err)
		return
	}
	_ = establishedConn.SetReadDeadline(time.Time{})

	chat, err := s.store.GetChat(m.ChatId)
	if err != nil {
		_ = establishedConn.Close()
		log.Printf("failed to ack the connection as the chat id received is not found in the store. received %s", m.ChatId)
		return
	}
	user, err := chat.GetUser(m.UserId)
	if err != nil {
		_ = establishedConn.Close()
		log.Printf("failed to ack the connection as the chat id (%s) does not contain the received user id %s", m.ChatId, m.UserId)
		return
	}
	c := conn.NewConnection(
		*user,
		*chat,
		establishedConn,
		s.removeConn,
		addReceivedMessageToStore(s.store),
	)
	go c.Start(ctx)
	s.storeConn(user.Id, c)
	if err := s.store.AddChatLine(domain.Message{
		ChatId: m.ChatId,
		UserId: m.UserId,
		Text:   m.Message,
		At:     m.At,
	}); err != nil {
		log.Printf("failed to add the chat line to the store for user %s", user.Id)
	}
}

func (s *socket) handleOutgoingMessages(ctx context.Context, msg domain.Message) {
	conns, err := s.getConns(ctx, msg.ChatId)
	if err != nil {
		log.Printf("failed to send message '%s': %s", msg.Text, err)
		return
	}
	for _, c := range conns {
		c.SendMessage(msg)
	}
}

func (s *socket) getConns(ctx context.Context, chatId string) ([]conn.Conn, error) {
	chat, err := s.store.GetChat(chatId)
	if err != nil {
		return nil, err
	}
	var res []conn.Conn
	users := chat.GetOtherUsers()
	s.cm.Lock()
	for _, u := range users {
		c, ok := s.connections[u.Id]
		if !ok {
			c = conn.NewConnection(u, *chat, nil, s.removeConn, addReceivedMessageToStore(s.store))
			s.storeConnNoLock(u.Id, c)
			go c.Start(ctx)
		}
		res = append(res, c)
	}
	s.cm.Unlock()
	return res, nil
}

func (s *socket) storeConn(userId string, conn conn.Conn) {
	s.cm.Lock()
	defer s.cm.Unlock()
	chatConn, ok := s.connections[userId]
	if ok {
		if err := chatConn.Close(); err != nil {
			log.Printf("failed to close the already existing connection: %s", err)
		}
	}
	s.connections[userId] = conn
}

func (s *socket) storeConnNoLock(userId string, conn conn.Conn) {
	chatConn, ok := s.connections[userId]
	if ok {
		if err := chatConn.Close(); err != nil {
			log.Printf("failed to close the already existing connection: %s", err)
		}
	}
	s.connections[userId] = conn
}

func (s *socket) removeConn(u domain.User, c domain.Chat) {
	s.cm.Lock()
	defer s.cm.Unlock()
	chatConn, ok := s.connections[u.Id]
	if ok {
		if err := chatConn.Close(); err != nil {
			log.Printf("failed to close the already existing connection: %s", err)
		}
	}
	delete(s.connections, u.Id)
	if err := s.store.AddChatLine(domain.Message{
		ChatId:       c.Id,
		UserId:       u.Id,
		UserName:     u.Name,
		Text:         "Disconnected",
		At:           time.Now(),
		ErrorMessage: true,
	}); err != nil {
		log.Printf("failed to add the disconnected chat line to the store for user %s and chat %s", u.Id, c.Id)
	}
}

func (s *socket) LocalIP() string {
	return s.ip
}

func findIp() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ip, ok := address.(*net.IPNet); ok && !ip.IP.IsLoopback() {
			if ip.IP.To4() != nil {
				return ip.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("could not figure out the IP of your machine")
}

func addReceivedMessageToStore(store data.Store) func(m domain.Message) {
	return func(m domain.Message) {
		if err := store.AddChatLine(m); err != nil {
			log.Printf("error adding chat line to store: %s", err)
		}
	}
}
