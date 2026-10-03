package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid or expired access token")

type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

type Principal struct {
	UserID         string
	OrganizationID string
}

type AccessClaims struct {
	OrganizationID string `json:"organization_id"`
	jwt.RegisteredClaims
}

func NewTokenManager(secret string, ttl time.Duration) (*TokenManager, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT secret must contain at least 32 characters")
	}
	if ttl <= 0 {
		return nil, errors.New("JWT lifetime must be positive")
	}
	return &TokenManager{secret: []byte(secret), ttl: ttl}, nil
}

func (manager *TokenManager) Issue(userID, organizationID string) (string, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(organizationID) == "" {
		return "", errors.New("user and organization IDs are required for an access token")
	}
	now := time.Now()
	claims := AccessClaims{
		OrganizationID: organizationID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(manager.ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(manager.secret)
}

func (manager *TokenManager) Verify(tokenString string) (Principal, error) {
	claims := &AccessClaims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, ErrInvalidToken
			}
			return manager.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil || token == nil || !token.Valid || strings.TrimSpace(claims.Subject) == "" {
		return Principal{}, ErrInvalidToken
	}
	if strings.TrimSpace(claims.OrganizationID) == "" {
		return Principal{}, ErrInvalidToken
	}
	return Principal{UserID: claims.Subject, OrganizationID: claims.OrganizationID}, nil
}

type contextKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(contextKey{}).(Principal)
	return principal, ok && principal.UserID != "" && principal.OrganizationID != ""
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	principal, ok := PrincipalFromContext(ctx)
	return principal.UserID, ok
}

func OrganizationIDFromContext(ctx context.Context) (string, bool) {
	principal, ok := PrincipalFromContext(ctx)
	return principal.OrganizationID, ok
}
