package model

import (
	"errors"
	"time"
)

// ErrNotFound is the shared sentinel for "no link matches this code".
// Repositories return it and services/handlers match it with errors.Is.
var ErrNotFound = errors.New("model: short link not found")

// Link is the persistent representation of one shortened URL.
type Link struct {
	ID          int64     `json:"id"`
	ShortCode   string    `json:"short_code"`
	OriginalURL string    `json:"original_url"`
	CreatedAt   time.Time `json:"created_at"`
}
