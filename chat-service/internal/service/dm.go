package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/fyodor/messenger/chat-service/internal/domain"
	"github.com/fyodor/messenger/chat-service/internal/repository"
)

type dmService struct {
	repo     repository.DMRepository
	userRepo repository.UserRepository
}

func NewDMService(repo repository.DMRepository, userRepo repository.UserRepository) DMService {
	return &dmService{repo: repo, userRepo: userRepo}
}

func (s *dmService) CreateOrGet(ctx context.Context, requesterID, targetUserID string) (*domain.DmRoom, error) {
	if requesterID == targetUserID {
		return nil, ErrSelfDM
	}

	_, err := s.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("dmService.CreateOrGet: %w", err)
	}

	room, err := s.repo.GetByUsers(ctx, requesterID, targetUserID)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("dmService.CreateOrGet: %w", err)
		}
		room, err = s.repo.CreateDM(ctx, requesterID, targetUserID)
		if err != nil {
			return nil, fmt.Errorf("dmService.CreateOrGet: %w", err)
		}
	}

	otherUser, err := s.repo.GetOtherMember(ctx, room.ID, requesterID)
	if err != nil {
		return nil, fmt.Errorf("dmService.CreateOrGet: %w", err)
	}

	return &domain.DmRoom{Room: room, OtherUser: otherUser}, nil
}

func (s *dmService) List(ctx context.Context, userID string) ([]*domain.DmRoom, error) {
	rooms, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("dmService.List: %w", err)
	}

	// TODO(perf): N+1 — replace with a JOIN-based repo method before scaling beyond ~20 users.
	result := make([]*domain.DmRoom, 0, len(rooms))
	for _, room := range rooms {
		otherUser, err := s.repo.GetOtherMember(ctx, room.ID, userID)
		if err != nil {
			return nil, fmt.Errorf("dmService.List: %w", err)
		}
		result = append(result, &domain.DmRoom{Room: room, OtherUser: otherUser})
	}

	return result, nil
}
