package models

import (
	"time"
)

type Chat struct {
	ID        string
	UserID1   string
	UserID2   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Message struct {
	ID        string
	ChatID    string
	SenderID  string
	Content   string
	CreatedAt time.Time
	ReadAt    *time.Time
}

