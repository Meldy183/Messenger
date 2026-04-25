package repository

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrRoomNameTaken = errors.New("room name already taken")
	ErrNotMember    = errors.New("not a member of this room")
)
