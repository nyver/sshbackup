package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// NewID returns a new opaque, globally unique identifier suitable for any
// entity primary key. It is not ordered and carries no embedded timestamp.
func NewID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
