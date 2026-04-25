package domain

import "time"

type Message struct {
	ID             string
	RoomID         string
	SenderID       string
	SenderUsername string
	Content        string
	CreatedAt      time.Time
}
