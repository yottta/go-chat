package directory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/yottta/chat/client/domain"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	pingHTTPContext = "ping"
	pingHTTPMethod  = http.MethodPut

	clientsHTTPContext = "clients"
	clientsHTTPMethod  = http.MethodGet
)

type Client interface {
	Ping(ctx context.Context, user domain.User) error
	Users(ctx context.Context) ([]domain.User, error)
}

func WithClient(httpClient *http.Client) func(c *client) {
	return func(c *client) {
		c.h = httpClient
	}
}

func WithTimeout(t time.Duration) func(c *client) {
	return func(c *client) {
		c.t = t
	}
}

// NewClient returns a new object that you can use to communicate with the Directory server.
func NewClient(serverURL string, opts ...func(c *client)) Client {
	c := &client{
		h: http.DefaultClient,
		s: serverURL,
		t: 2 * time.Second,
	}

	for _, o := range opts {
		o(c)
	}
	return c
}

type client struct {
	h *http.Client
	s string
	t time.Duration
}

func (c *client) Ping(ctx context.Context, user domain.User) error {
	marshal, err := json.Marshal(user)
	if err != nil {
		return err
	}
	ctx, cancelFunc := context.WithTimeout(ctx, c.t)
	defer cancelFunc()

	req, err := http.NewRequestWithContext(ctx, pingHTTPMethod, strings.Join([]string{c.s, pingHTTPContext}, "/"), bytes.NewReader(marshal))
	if err != nil {
		return err
	}

	resp, err := c.h.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	return fmt.Errorf("non 2xx http status: %d", resp.StatusCode)
}

func (c *client) Users(ctx context.Context) ([]domain.User, error) {
	ctx, cancelFunc := context.WithTimeout(ctx, c.t)
	defer cancelFunc()
	request, err := http.NewRequestWithContext(ctx, clientsHTTPMethod, strings.Join([]string{c.s, clientsHTTPContext}, "/"), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.h.Do(request)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non 2xx http status: %d", resp.StatusCode)
	}

	if resp.Body == nil {
		return nil, fmt.Errorf("no response from the server")
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("error trying to close the body of the request to get the clients from the directory server: %s", err)
		}
	}()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request response")
	}
	res := struct {
		Clients []domain.User `json:"clients"`
	}{
		Clients: []domain.User{},
	}
	if err := json.Unmarshal(b, &res); err != nil {
		return nil, err
	}
	return res.Clients, nil
}
