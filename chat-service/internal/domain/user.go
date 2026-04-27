package domain

import "time"

type User struct {
	ID        string
	Username  string
	CreatedAt time.Time
}
