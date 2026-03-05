package users

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/users/usecases/domain"
)

type mockRepo struct {
	users   map[string]domain.User
	keys    map[uuid.UUID][]domain.APIKey
	members map[uuid.UUID][]domain.Member
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		users:   make(map[string]domain.User),
		keys:    make(map[uuid.UUID][]domain.APIKey),
		members: make(map[uuid.UUID][]domain.Member),
	}
}

func (m *mockRepo) GetUserByExternalID(externalID string) (domain.User, bool) {
	u, ok := m.users[externalID]
	return u, ok
}

func (m *mockRepo) UpsertUser(externalID, email, name, avatarURL string) domain.User {
	u := domain.User{ID: uuid.New(), ExternalID: externalID, Email: email, Name: name, AvatarURL: avatarURL}
	m.users[externalID] = u
	return u
}

func (m *mockRepo) SoftDeleteUser(externalID string) bool {
	if _, ok := m.users[externalID]; !ok {
		return false
	}
	delete(m.users, externalID)
	return true
}

func (m *mockRepo) AddMembership(orgID, userID uuid.UUID, role string) {
	m.members[orgID] = append(m.members[orgID], domain.Member{UserID: userID, Role: role})
}

func (m *mockRepo) RemoveMembership(orgID, userID uuid.UUID) {
	members := m.members[orgID]
	for i, mem := range members {
		if mem.UserID == userID {
			m.members[orgID] = append(members[:i], members[i+1:]...)
			return
		}
	}
}

func (m *mockRepo) ListMembers(orgID uuid.UUID) []domain.Member {
	return m.members[orgID]
}

func (m *mockRepo) ListAPIKeys(orgID uuid.UUID) []domain.APIKey {
	return m.keys[orgID]
}

func (m *mockRepo) CreateAPIKey(orgID uuid.UUID, name, createdBy string, scopes []string, rawKey string) domain.APIKey {
	key := domain.APIKey{
		ID:        uuid.New(),
		OrgID:     orgID,
		Name:      name,
		KeyPrefix: rawKey[:16],
		Scopes:    scopes,
		CreatedBy: createdBy,
	}
	m.keys[orgID] = append(m.keys[orgID], key)
	return key
}

func (m *mockRepo) DeleteAPIKey(orgID, keyID uuid.UUID) bool {
	keys := m.keys[orgID]
	for i, k := range keys {
		if k.ID == keyID {
			m.keys[orgID] = append(keys[:i], keys[i+1:]...)
			return true
		}
	}
	return false
}

func (m *mockRepo) RotateAPIKey(orgID, keyID uuid.UUID, rawKey string) (domain.APIKey, bool) {
	keys := m.keys[orgID]
	for i, k := range keys {
		if k.ID == keyID {
			k.KeyPrefix = rawKey[:16]
			m.keys[orgID][i] = k
			return k, true
		}
	}
	return domain.APIKey{}, false
}

func TestGetMe(t *testing.T) {
	repo := newMockRepo()
	uc := NewUsecases(repo)
	ctx := context.Background()

	t.Run("empty actor", func(t *testing.T) {
		_, err := uc.GetMe(ctx, "")
		if err == nil {
			t.Error("expected error for empty actor")
		}
	})

	t.Run("new user created", func(t *testing.T) {
		user, err := uc.GetMe(ctx, "user123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.ExternalID != "user123" {
			t.Errorf("ExternalID = %q; want %q", user.ExternalID, "user123")
		}
	})

	t.Run("existing user returned", func(t *testing.T) {
		user1, _ := uc.GetMe(ctx, "user123")
		user2, _ := uc.GetMe(ctx, "user123")
		if user1.ID != user2.ID {
			t.Error("expected same user to be returned")
		}
	})
}

func TestCreateAPIKey(t *testing.T) {
	repo := newMockRepo()
	uc := NewUsecases(repo)
	ctx := context.Background()
	orgID := uuid.New().String()

	t.Run("invalid org_id", func(t *testing.T) {
		_, _, err := uc.CreateAPIKey(ctx, "bad-uuid", "test", "actor", nil)
		if err == nil {
			t.Error("expected error for invalid org_id")
		}
	})

	t.Run("empty name", func(t *testing.T) {
		_, _, err := uc.CreateAPIKey(ctx, orgID, "", "actor", nil)
		if err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("success", func(t *testing.T) {
		key, raw, err := uc.CreateAPIKey(ctx, orgID, "my-key", "actor", []string{"read"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key.Name != "my-key" {
			t.Errorf("Name = %q; want %q", key.Name, "my-key")
		}
		if raw == "" {
			t.Error("raw key should not be empty")
		}
	})
}

func TestDeleteAPIKey(t *testing.T) {
	repo := newMockRepo()
	uc := NewUsecases(repo)
	ctx := context.Background()
	orgID := uuid.New()

	key := repo.CreateAPIKey(orgID, "test", "actor", []string{"read"}, "psk_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")

	t.Run("not found", func(t *testing.T) {
		err := uc.DeleteAPIKey(ctx, orgID.String(), uuid.New().String())
		if err == nil {
			t.Error("expected error for non-existent key")
		}
	})

	t.Run("success", func(t *testing.T) {
		err := uc.DeleteAPIKey(ctx, orgID.String(), key.ID.String())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestUpsertClerkUser(t *testing.T) {
	repo := newMockRepo()
	uc := NewUsecases(repo)
	ctx := context.Background()

	t.Run("empty external_id", func(t *testing.T) {
		err := uc.UpsertClerkUser(ctx, "", "email", "name", "")
		if err == nil {
			t.Error("expected error for empty external_id")
		}
	})

	t.Run("success", func(t *testing.T) {
		err := uc.UpsertClerkUser(ctx, "ext123", "test@example.com", "Test", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		user, ok := repo.GetUserByExternalID("ext123")
		if !ok {
			t.Fatal("user not found in repo")
		}
		if user.Email != "test@example.com" {
			t.Errorf("Email = %q; want %q", user.Email, "test@example.com")
		}
	})
}

func TestDeleteClerkUser(t *testing.T) {
	repo := newMockRepo()
	uc := NewUsecases(repo)
	ctx := context.Background()

	t.Run("empty external_id", func(t *testing.T) {
		err := uc.DeleteClerkUser(ctx, "")
		if err == nil {
			t.Error("expected error for empty external_id")
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := uc.DeleteClerkUser(ctx, "nonexistent")
		if err == nil {
			t.Error("expected error for non-existent user")
		}
	})

	t.Run("success", func(t *testing.T) {
		repo.UpsertUser("ext456", "test@test.com", "Test", "")
		err := uc.DeleteClerkUser(ctx, "ext456")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
