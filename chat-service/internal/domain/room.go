package domain

import "time"

type Room struct {
	ID        string
	Name      *string // NULL for DMs
	IsDM      bool
	CreatedBy string
	CreatedAt time.Time
}

// DmRoom is a view type returned by the DM service.
// It enriches Room with the other participant's profile.
type DmRoom struct {
	Room      *Room
	OtherUser *User
}
