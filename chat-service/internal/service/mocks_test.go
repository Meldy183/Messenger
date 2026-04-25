package service

import (
	"context"

	"github.com/fyodor/messenger/chat-service/internal/domain"
)

// mockUserRepo

type mockUserRepo struct {
	listFn    func(ctx context.Context) ([]*domain.User, error)
	getByIDFn func(ctx context.Context, id string) (*domain.User, error)
}

func (m *mockUserRepo) List(ctx context.Context) ([]*domain.User, error) {
	return m.listFn(ctx)
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return m.getByIDFn(ctx, id)
}

// mockRoomRepo

type mockRoomRepo struct {
	createFn       func(ctx context.Context, name, createdBy string) (*domain.Room, error)
	getByIDFn      func(ctx context.Context, id string) (*domain.Room, error)
	listPublicFn   func(ctx context.Context) ([]*domain.Room, error)
	listByMemberFn func(ctx context.Context, userID string) ([]*domain.Room, error)
	addMemberFn    func(ctx context.Context, roomID, userID string) error
	removeMemberFn func(ctx context.Context, roomID, userID string) error
	isMemberFn     func(ctx context.Context, roomID, userID string) (bool, error)
}

func (m *mockRoomRepo) Create(ctx context.Context, name, createdBy string) (*domain.Room, error) {
	return m.createFn(ctx, name, createdBy)
}

func (m *mockRoomRepo) GetByID(ctx context.Context, id string) (*domain.Room, error) {
	return m.getByIDFn(ctx, id)
}

func (m *mockRoomRepo) ListPublic(ctx context.Context) ([]*domain.Room, error) {
	return m.listPublicFn(ctx)
}

func (m *mockRoomRepo) ListByMember(ctx context.Context, userID string) ([]*domain.Room, error) {
	return m.listByMemberFn(ctx, userID)
}

func (m *mockRoomRepo) AddMember(ctx context.Context, roomID, userID string) error {
	return m.addMemberFn(ctx, roomID, userID)
}

func (m *mockRoomRepo) RemoveMember(ctx context.Context, roomID, userID string) error {
	return m.removeMemberFn(ctx, roomID, userID)
}

func (m *mockRoomRepo) IsMember(ctx context.Context, roomID, userID string) (bool, error) {
	return m.isMemberFn(ctx, roomID, userID)
}

// mockDMRepo

type mockDMRepo struct {
	createDMFn       func(ctx context.Context, user1ID, user2ID string) (*domain.Room, error)
	getByUsersFn     func(ctx context.Context, user1ID, user2ID string) (*domain.Room, error)
	listByUserFn     func(ctx context.Context, userID string) ([]*domain.Room, error)
	getOtherMemberFn func(ctx context.Context, roomID, myUserID string) (*domain.User, error)
}

func (m *mockDMRepo) CreateDM(ctx context.Context, user1ID, user2ID string) (*domain.Room, error) {
	return m.createDMFn(ctx, user1ID, user2ID)
}

func (m *mockDMRepo) GetByUsers(ctx context.Context, user1ID, user2ID string) (*domain.Room, error) {
	return m.getByUsersFn(ctx, user1ID, user2ID)
}

func (m *mockDMRepo) ListByUser(ctx context.Context, userID string) ([]*domain.Room, error) {
	return m.listByUserFn(ctx, userID)
}

func (m *mockDMRepo) GetOtherMember(ctx context.Context, roomID, myUserID string) (*domain.User, error) {
	return m.getOtherMemberFn(ctx, roomID, myUserID)
}

// mockMessageRepo

type mockMessageRepo struct {
	createFn     func(ctx context.Context, id, roomID, senderID, senderUsername, content string) (*domain.Message, error)
	listByRoomFn func(ctx context.Context, roomID string, limit, offset int) ([]*domain.Message, error)
}

func (m *mockMessageRepo) Create(ctx context.Context, id, roomID, senderID, senderUsername, content string) (*domain.Message, error) {
	return m.createFn(ctx, id, roomID, senderID, senderUsername, content)
}

func (m *mockMessageRepo) ListByRoom(ctx context.Context, roomID string, limit, offset int) ([]*domain.Message, error) {
	return m.listByRoomFn(ctx, roomID, limit, offset)
}

// mockBroadcaster

type mockBroadcaster struct {
	broadcastFn func(roomID string, msg *domain.Message)
}

func (m *mockBroadcaster) Broadcast(roomID string, msg *domain.Message) {
	if m.broadcastFn != nil {
		m.broadcastFn(roomID, msg)
	}
}
