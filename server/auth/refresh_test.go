package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/federicoserini/mobile-db/server/auth"
)

type stubRefreshStore struct {
	tokens map[string]*auth.RefreshToken
}

func newRefreshStore() *stubRefreshStore {
	return &stubRefreshStore{tokens: map[string]*auth.RefreshToken{}}
}

func (s *stubRefreshStore) CreateRefreshToken(_ context.Context, rt auth.RefreshToken) error {
	s.tokens[rt.TokenHash] = &rt
	return nil
}

func (s *stubRefreshStore) GetAndInvalidateRefreshToken(_ context.Context, hash string) (*auth.RefreshToken, error) {
	rt, ok := s.tokens[hash]
	if !ok || rt.Used {
		return nil, auth.ErrRefreshTokenInvalid
	}
	rt.Used = true
	return rt, nil
}

func (s *stubRefreshStore) RevokeAllForUser(_ context.Context, appID, userID string) error {
	for _, rt := range s.tokens {
		if rt.AppID == appID && rt.UserID == userID {
			rt.Used = true
		}
	}
	return nil
}

func TestRefreshIssueAndRotate(t *testing.T) {
	jwtSvc := auth.NewJWTService([]byte("32-byte-secret-for-testing-1234!"))
	svc := auth.NewRefreshService(newRefreshStore(), jwtSvc)
	rt, err := svc.IssueRefresh(context.Background(), "app1", "user1")
	if err != nil {
		t.Fatalf("IssueRefresh: %v", err)
	}
	newAccess, newRT, err := svc.Rotate(context.Background(), rt)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if newAccess == "" || newRT == "" {
		t.Fatal("must return new access token and refresh token")
	}
}

func TestRefreshTokenReuse(t *testing.T) {
	jwtSvc := auth.NewJWTService([]byte("32-byte-secret-for-testing-1234!"))
	svc := auth.NewRefreshService(newRefreshStore(), jwtSvc)
	rt, _ := svc.IssueRefresh(context.Background(), "app1", "user1")
	svc.Rotate(context.Background(), rt)
	_, _, err := svc.Rotate(context.Background(), rt)
	if err == nil {
		t.Fatal("reusing refresh token must error")
	}
}

// Compile-time check that stubRefreshStore satisfies the interface.
var _ interface {
	CreateRefreshToken(context.Context, auth.RefreshToken) error
	GetAndInvalidateRefreshToken(context.Context, string) (*auth.RefreshToken, error)
	RevokeAllForUser(context.Context, string, string) error
} = (*stubRefreshStore)(nil)

// Ensure time import is used.
var _ = time.Hour
