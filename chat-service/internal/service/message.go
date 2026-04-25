package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fyodor/messenger/chat-service/internal/domain"
	"github.com/fyodor/messenger/chat-service/internal/repository"
	"github.com/google/uuid"
)

type messageService struct {
	repo        repository.MessageRepository
	roomRepo    repository.RoomRepository
	broadcaster Broadcaster
}

func NewMessageService(repo repository.MessageRepository, roomRepo repository.RoomRepository, broadcaster Broadcaster) MessageService {
	return &messageService{repo: repo, roomRepo: roomRepo, broadcaster: broadcaster}
}

func (s *messageService) Send(ctx context.Context, roomID, senderID, senderUsername, content string) (*domain.Message, error) {
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("content cannot be empty")
	}

	member, err := s.roomRepo.IsMember(ctx, roomID, senderID)
	if err != nil {
		return nil, fmt.Errorf("messageService.Send: %w", err)
	}
	if !member {
		return nil, ErrForbidden
	}

	msg, err := s.repo.Create(ctx, uuid.NewString(), roomID, senderID, senderUsername, content)
	if err != nil {
		return nil, fmt.Errorf("messageService.Send: %w", err)
	}

	s.broadcastSafe(roomID, msg)
	return msg, nil
}

func (s *messageService) History(ctx context.Context, roomID, userID string, limit, offset int) ([]*domain.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	member, err := s.roomRepo.IsMember(ctx, roomID, userID)
	if err != nil {
		return nil, fmt.Errorf("messageService.History: %w", err)
	}
	if !member {
		return nil, ErrForbidden
	}

	msgs, err := s.repo.ListByRoom(ctx, roomID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("messageService.History: %w", err)
	}
	return msgs, nil
}

// broadcastSafe calls Broadcast and recovers from any panic so a broadcaster
// bug never causes a persisted message to be lost from the caller's perspective.
func (s *messageService) broadcastSafe(roomID string, msg *domain.Message) {
	defer func() {
		recover() // a real impl would log the panic here
	}()
	s.broadcaster.Broadcast(roomID, msg)
}
