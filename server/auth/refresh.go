package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

const refreshTokenTTL = 30 * 24 * time.Hour

var ErrRefreshTokenInvalid = errors.New("refresh token invalid or already used")

type RefreshToken struct {
	TokenHash string
	AppID     string
	UserID    string
	ExpiresAt time.Time
	Used      bool
}

type RefreshStore interface {
	CreateRefreshToken(ctx context.Context, rt RefreshToken) error
	GetAndInvalidateRefreshToken(ctx context.Context, tokenHash string) (*RefreshToken, error)
	RevokeAllForUser(ctx context.Context, appID, userID string) error
}

type RefreshService struct {
	store  RefreshStore
	jwtSvc *JWTService
}

func NewRefreshService(store RefreshStore, jwtSvc *JWTService) *RefreshService {
	return &RefreshService{store: store, jwtSvc: jwtSvc}
}

func (s *RefreshService) IssueRefresh(ctx context.Context, appID, userID string) (string, error) {
	raw, err := randomHex(32)
	if err != nil {
		return "", err
	}
	hash := sha256hex(raw)
	rt := RefreshToken{
		TokenHash: hash,
		AppID:     appID,
		UserID:    userID,
		ExpiresAt: time.Now().Add(refreshTokenTTL),
	}
	if err := s.store.CreateRefreshToken(ctx, rt); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *RefreshService) Rotate(ctx context.Context, rawToken string) (accessToken, newRefresh string, err error) {
	hash := sha256hex(rawToken)
	rt, err := s.store.GetAndInvalidateRefreshToken(ctx, hash)
	if err != nil {
		return "", "", ErrRefreshTokenInvalid
	}
	if time.Now().After(rt.ExpiresAt) {
		return "", "", ErrRefreshTokenInvalid
	}
	accessToken, err = s.jwtSvc.Issue(rt.AppID, rt.UserID)
	if err != nil {
		return "", "", err
	}
	newRefresh, err = s.IssueRefresh(ctx, rt.AppID, rt.UserID)
	return accessToken, newRefresh, err
}

func sha256hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
