// Package jwt provides JWT access/refresh token creation and verification.
package jwt

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

// Claims represents the payload embedded in a JWT access token.
type Claims struct {
	MemberID string `json:"sub"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	IsAdmin  bool   `json:"isAdmin"`
	Type     string `json:"type"` // "access" or "refresh"
	jwtlib.RegisteredClaims
}

// Manager handles token creation and verification.
type Manager struct {
	secret        []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

// New creates a new JWT Manager.
func New(secret string, accessExpiry, refreshExpiry time.Duration) *Manager {
	return &Manager{
		secret:        []byte(secret),
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
}

// CreateAccessToken generates a signed JWT access token for a member.
func (m *Manager) CreateAccessToken(memberID, email, role string, isAdmin bool) (string, error) {
	now := time.Now()
	claims := Claims{
		MemberID: memberID,
		Email:    email,
		Role:     role,
		IsAdmin:  isAdmin,
		Type:     "access",
		RegisteredClaims: jwtlib.RegisteredClaims{
			Subject:   memberID,
			IssuedAt:  jwtlib.NewNumericDate(now),
			ExpiresAt: jwtlib.NewNumericDate(now.Add(m.accessExpiry)),
		},
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// VerifyAccessToken parses and validates a JWT access token.
func (m *Manager) VerifyAccessToken(tokenStr string) (*Claims, error) {
	token, err := jwtlib.ParseWithClaims(tokenStr, &Claims{}, func(t *jwtlib.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.Type != "access" {
		return nil, errors.New("wrong token type")
	}
	return claims, nil
}

// GenerateRefreshToken creates a cryptographically secure random refresh token string.
func GenerateRefreshToken() (string, error) {
	b := make([]byte, 48)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GenerateOTP creates a random numeric OTP of the specified length.
func GenerateOTP(length int) string {
	const digits = "0123456789"
	b := make([]byte, length)
	rand.Read(b)
	for i := range b {
		b[i] = digits[b[i]%10]
	}
	return string(b)
}

// GenerateReceiptNo creates a unique receipt number like RC-20260401-A3X2.
func GenerateReceiptNo(prefix string) string {
	b := make([]byte, 2)
	rand.Read(b)
	return fmt.Sprintf("%s-%s-%X", prefix, time.Now().Format("20060102"), b)
}
