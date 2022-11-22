package conn

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"github.com/yottta/chat/client/domain"
	"log"
	"net"
	"sync"
	"time"
)

type Conn interface {
	Start(ctx context.Context)
	SendMessage(m domain.Message)
	Close() error
}

// connection is holding the actual socket conn to a specific address of a specific user bound to a specific chat.
// It's handling the communication on both directions.
type connection struct {
	u domain.User
	c domain.Chat

	conn      net.Conn
	cm        *sync.Mutex
	writeChan chan domain.Message

	closeChan chan struct{}

	closeCallback      func(u domain.User, c domain.Chat)
	receiveMsgCallback func(m domain.Message)
}

// NewConnection creates a new connection object. In order to start using it, #start needs to be called in a new goroutine.
// The function requires 4 parameters:
// * u: a domain.User object describing the user. Important because it's using the IP and the Port from it
// * c: a domain.Chat object describing the chat object. This is mostly important for the ID inside because it's needed for sending it over to the connected user.
// * closeCallback: a function that receives the user and the chat given in the constructor whenever the connection with the other party is closed. This is really useful for cleaning up the connection from a pool or something similar.
// * messageReceiveCallback: a function that is going to handle the received information from the other party.
func NewConnection(u domain.User, c domain.Chat, conn net.Conn, closeCallback func(user domain.User, chat domain.Chat), messageReceiveCallback func(m domain.Message)) Conn {
	return &connection{
		u:         u,
		c:         c,
		conn:      conn,
		cm:        &sync.Mutex{},
		writeChan: make(chan domain.Message, 5),

		closeChan: make(chan struct{}, 1),

		closeCallback:      closeCallback,
		receiveMsgCallback: messageReceiveCallback,
	}
}

func (c *connection) Start(ctx context.Context) {
	defer func() {
		if c.conn != nil {
			c.cm.Lock()
			defer c.cm.Unlock()
			if err := c.conn.Close(); err != nil {
				log.Printf("error trying to close a socket connection: %s", err)
			}
		}
		close(c.writeChan)
		c.closeCallback(c.u, c.c)
	}()
	if c.conn == nil {
		if err := c.initializeConn(); err != nil {
			return
		}
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case m, ok := <-c.writeChan:
				if !ok {
					return
				}
				c.writeToConn(m)
			case <-c.closeChan:
				return
			}
		}
	}()
	// TODO, needs to test this for bigger messages
	b := make([]byte, 1024)
	for {
		n, err := c.conn.Read(b)
		if err != nil {
			return
		}

		var m NetworkMsg
		if err := gob.NewDecoder(bytes.NewReader(b[:n])).Decode(&m); err != nil {
			log.Printf("failed to decode network message")
			continue
		}

		c.receiveMsgCallback(domain.Message{
			ChatId: m.ChatId,
			UserId: m.UserId,
			Text:   m.Message,
			At:     m.At,
		})
	}
}

// SendMessage is scheduling the given message to be sent through the socket to the other party
func (c *connection) SendMessage(m domain.Message) {
	c.writeChan <- m
}

// Close is closing the connection created if any.
func (c *connection) Close() error {
	c.closeChan <- struct{}{}
	return nil
}

// initializeConn creates a new connection with the info from the user object.
func (c *connection) initializeConn() error {
	c.cm.Lock()
	defer c.cm.Unlock()

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			log.Printf("error closing existing connection: %s", err)
		}
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", c.u.Address, c.u.Port), 4*time.Second)
	if err != nil {
		return err
	}
	c.conn = conn

	return nil
}

// writeToConn writes the message to the actual socket.
func (c *connection) writeToConn(m domain.Message) {
	if c.conn == nil {
		if err := c.initializeConn(); err != nil {
			log.Printf("error initializing connection for connection on userId %s and chatId %s: %s. message discarded", c.u.Id, c.c.Id, err)
			return
		}
	}
	c.cm.Lock()
	defer c.cm.Unlock()
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(NetworkMsg{
		UserId:  m.UserId,
		ChatId:  m.ChatId,
		Message: m.Text,
		At:      m.At,
	}); err != nil {
		log.Printf("failed to encode message to send it over network: %s", err)
		return
	}
	if _, err := c.conn.Write(b.Bytes()); err != nil {
		log.Printf("failed to write the message into the socket: %s", err)
	}
}

type NetworkMsg struct {
	UserId  string
	ChatId  string
	Message string
	At      time.Time
}
