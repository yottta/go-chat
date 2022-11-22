package domain

import (
	"fmt"
	"strings"
)

type Client struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IP   string `json:"address"`
	Port int    `json:"port"`
}

func (c Client) Validate() error {
	if len(strings.TrimSpace(c.ID)) == 0 {
		return fmt.Errorf("client id empty")
	}
	if len(strings.TrimSpace(c.Name)) == 0 {
		return fmt.Errorf("client name empty")
	}
	if len(strings.TrimSpace(c.IP)) == 0 {
		return fmt.Errorf("client ip empty")
	}
	if c.Port < 1000 {
		return fmt.Errorf("invalid client port")
	}
	return nil
}
