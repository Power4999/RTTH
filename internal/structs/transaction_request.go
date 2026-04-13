package structs

import (
	"errors"
	"strings"
)

type ClientTransaction struct {
	ClientID  int    `json:"clientid"`
	Payload   string `json:"payload"`
	Timestamp int64  `json:"timestamp"`
}

// Validate performs client transaction validation and returns an error when validation fails.
func (t *ClientTransaction) Validate() error {
	if t.ClientID == 0 {
		return errors.New("transaction ID is required")
	}
	if strings.TrimSpace(t.Payload) == "" {
		return errors.New("payload cannot be empty")
	}
	if t.Timestamp == 0 {
		return errors.New("timestamp is required")
	}
	return nil
}

type GetRequest struct {
	ClientID int `json:"clientid"`
}
