package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/fyodor/messenger/chat-service/internal/domain"
	"github.com/fyodor/messenger/chat-service/internal/repository"
)

type roomService struct {
	repo repository.RoomRepository
}

func NewRoomService(repo repository.RoomRepository) RoomService {
	return &roomService{repo: repo}
}

func (s *roomService) Create(ctx context.Context, name, createdByUserID string) (*domain.Room, error) {
	room, err := s.repo.Create(ctx, name, createdByUserID)
	if err != nil {
		if errors.Is(err, repository.ErrRoomNameTaken) {
			return nil, ErrRoomNameTaken
		}
		return nil, fmt.Errorf("roomService.Create: %w", err)
	}
	return room, nil
}

func (s *roomService) Join(ctx context.Context, roomID, userID string) error {
	room, err := s.repo.GetByID(ctx, roomID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("roomService.Join: %w", err)
	}
	if room.IsDM {
		return ErrIsDMRoom
	}
	if err := s.repo.AddMember(ctx, roomID, userID); err != nil {
		return fmt.Errorf("roomService.Join: %w", err)
	}
	return nil
}

func (s *roomService) Leave(ctx context.Context, roomID, userID string) error {
	err := s.repo.RemoveMember(ctx, roomID, userID)
	if err != nil && !errors.Is(err, repository.ErrNotMember) {
		return fmt.Errorf("roomService.Leave: %w", err)
	}
	return nil
}

func (s *roomService) ListPublic(ctx context.Context) ([]*domain.Room, error) {
	rooms, err := s.repo.ListPublic(ctx)
	if err != nil {
		return nil, fmt.Errorf("roomService.ListPublic: %w", err)
	}
	return rooms, nil
}

func (s *roomService) ListJoined(ctx context.Context, userID string) ([]*domain.Room, error) {
	rooms, err := s.repo.ListByMember(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("roomService.ListJoined: %w", err)
	}
	return rooms, nil
}

func (s *roomService) IsMember(ctx context.Context, roomID, userID string) (bool, error) {
	ok, err := s.repo.IsMember(ctx, roomID, userID)
	if err != nil {
		return false, fmt.Errorf("roomService.IsMember: %w", err)
	}
	return ok, nil
}
