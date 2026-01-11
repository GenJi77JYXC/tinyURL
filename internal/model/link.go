package model

import "time"

type Link struct {
	ID          int64      `json:"id"`
	OriginalURL string     `json:"original_url"`
	ShortCode   string     `json:"short_code"`
	CreatedAt   time.Time  `json:"created_at"`
	UserID      int64      `json:"user_id"`
	Clicks      int64      `json:"clicks"`
	ExpireAt    *time.Time `json:"expire_at,omitempty"`
}
