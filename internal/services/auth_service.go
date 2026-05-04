// Package services contains all business logic for the Inheritance Choir system.
package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/inheritance-choir/backend/internal/config"
	"github.com/inheritance-choir/backend/internal/models"
	"github.com/inheritance-choir/backend/internal/notifications"
	"github.com/inheritance-choir/backend/internal/repository"
	"github.com/inheritance-choir/backend/pkg/crypto"
	"github.com/inheritance-choir/backend/pkg/jwt"
)

// ── API error ─────────────────────────────────────────────────

type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string { return e.Message }

func errUnauthorized(msg string) *APIError { return &APIError{http.StatusUnauthorized, msg} }
func errForbidden(msg string)   *APIError  { return &APIError{http.StatusForbidden, msg} }
func errNotFound(msg string)    *APIError  { return &APIError{http.StatusNotFound, msg} }
func errConflict(msg string)    *APIError  { return &APIError{http.StatusConflict, msg} }
func errBadRequest(msg string)  *APIError  { return &APIError{http.StatusBadRequest, msg} }
func errTooMany(msg string)     *APIError  { return &APIError{http.StatusTooManyRequests, msg} }

// ── Auth Service ──────────────────────────────────────────────

type AuthService struct {
	db     *repository.DB
	cfg    *config.Config
	jwtMgr *jwt.Manager
	mailer *notifications.Mailer
	log    *zap.Logger
}

func NewAuthService(db *repository.DB, cfg *config.Config, jwtMgr *jwt.Manager, mailer *notifications.Mailer, log *zap.Logger) *AuthService {
	return &AuthService{db: db, cfg: cfg, jwtMgr: jwtMgr, mailer: mailer, log: log}
}

// LoginResult is returned from Login on success.
type LoginResult struct {
	Member       *models.Member
	AccessToken  string
	RefreshToken string
}

// Login authenticates a member and returns tokens.
func (s *AuthService) Login(ctx context.Context, email, password string, rememberMe bool, ip, ua string) (*LoginResult, error) {
	var member models.Member
	err := s.db.Members().FindOne(ctx, bson.M{"email": strings.ToLower(email)}).Decode(&member)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			s.audit(ctx, "auth.login_failed", "", email, ip, ua)
			return nil, errUnauthorized("Invalid email or password")
		}
		return nil, err
	}

	if !crypto.VerifyPassword(password, member.PasswordHash) {
		s.audit(ctx, "auth.login_failed", member.ID.Hex(), email, ip, ua)
		return nil, errUnauthorized("Invalid email or password")
	}

	switch member.Status {
	case "pending":
		return nil, errForbidden("Account is pending admin approval")
	case "inactive":
		return nil, errForbidden("Account has been deactivated")
	}

	// Generate tokens
	accessToken, err := s.jwtMgr.CreateAccessToken(
		member.ID.Hex(), member.Email, member.Role, member.IsAdmin,
	)
	if err != nil {
		return nil, err
	}

	refreshStr, err := jwt.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	expiry := s.cfg.JWTRefreshExpiry
	if !rememberMe {
		expiry = 24 * time.Hour
	}

	_, err = s.db.RefreshTokens().InsertOne(ctx, models.RefreshToken{
		MemberID:  member.ID.Hex(),
		Token:     refreshStr,
		ExpiresAt: time.Now().Add(expiry),
		UserAgent: ua,
		IPAddress: ip,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return nil, err
	}

	// Mark online
	now := time.Now()
	s.db.Members().UpdateByID(ctx, member.ID, bson.M{
		"$set": bson.M{"online": true, "lastSeen": now, "updatedAt": now},
	})

	s.audit(ctx, "auth.login", member.ID.Hex(), member.Email, ip, ua)
	s.log.Info("Login success", zap.String("email", member.Email), zap.String("ip", ip))

	return &LoginResult{Member: &member, AccessToken: accessToken, RefreshToken: refreshStr}, nil
}

// Register creates a new pending member account.
func (s *AuthService) Register(ctx context.Context, data RegisterInput) (*models.Member, error) {
	email := strings.ToLower(data.Email)

	count, _ := s.db.Members().CountDocuments(ctx, bson.M{"email": email})
	if count > 0 {
		return nil, errConflict("Email already registered")
	}

	hash, err := crypto.HashPassword(data.Password)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	member := models.Member{
		FullName:     data.FullName,
		Email:        email,
		PasswordHash: hash,
		Phone:        data.Phone,
		DateOfBirth:  data.DateOfBirth,
		Gender:       data.Gender,
		MaritalStatus: data.MaritalStatus,
		VoicePart:    data.VoicePart,
		Role:         "member",
		Status:       "pending",
		IsAdmin:      false,
		Permissions:  []string{},
		Attendance:   0,
		ContribTotal: 0,
		JoinDate:     now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	res, err := s.db.Members().InsertOne(ctx, member)
	if err != nil {
		return nil, err
	}
	member.ID = res.InsertedID.(primitive.ObjectID)

	// Create OTP for email verification
	if err = s.createOTP(ctx, email, "verify"); err != nil {
		s.log.Warn("OTP creation failed", zap.String("email", email), zap.Error(err))
	}

	go s.mailer.SendWelcomeEmail(email, data.FullName)

	s.log.Info("New registration", zap.String("email", email))
	return &member, nil
}

// VerifyOTP checks a 6-digit code against the stored OTP hash.
func (s *AuthService) VerifyOTP(ctx context.Context, email, code, purpose string) error {
	if s.cfg.IsDevelopment() {
		return nil // accept any code in dev
	}

	var otp models.OTP
	err := s.db.OTPs().FindOne(ctx, bson.M{
		"email": strings.ToLower(email), "purpose": purpose,
	}).Decode(&otp)
	if err != nil {
		return errBadRequest("OTP not found or expired")
	}

	// Increment attempts
	s.db.OTPs().UpdateByID(ctx, otp.ID, bson.M{"$inc": bson.M{"attempts": 1}})
	otp.Attempts++

	if otp.Attempts > s.cfg.OTPMaxAttempts {
		s.db.OTPs().DeleteOne(ctx, bson.M{"_id": otp.ID})
		return errTooMany("Too many OTP attempts")
	}
	if time.Now().After(otp.ExpiresAt) {
		s.db.OTPs().DeleteOne(ctx, bson.M{"_id": otp.ID})
		return errBadRequest("OTP has expired")
	}
	if !crypto.VerifyPassword(code, otp.Code) {
		remaining := s.cfg.OTPMaxAttempts - otp.Attempts
		return errBadRequest(fmt.Sprintf("Invalid OTP. %d attempts remaining", remaining))
	}

	s.db.OTPs().DeleteOne(ctx, bson.M{"_id": otp.ID})
	return nil
}

// RefreshTokens rotates the refresh token and returns a new pair.
func (s *AuthService) RefreshTokens(ctx context.Context, refreshToken string) (string, string, error) {
	var rt models.RefreshToken
	err := s.db.RefreshTokens().FindOne(ctx, bson.M{"token": refreshToken, "revoked": false}).Decode(&rt)
	if err != nil {
		return "", "", errUnauthorized("Invalid refresh token")
	}
	if time.Now().After(rt.ExpiresAt) {
		s.db.RefreshTokens().DeleteOne(ctx, bson.M{"_id": rt.ID})
		return "", "", errUnauthorized("Refresh token expired")
	}

	var member models.Member
	oid, _ := primitive.ObjectIDFromHex(rt.MemberID)
	if err = s.db.Members().FindOne(ctx, bson.M{"_id": oid}).Decode(&member); err != nil {
		return "", "", errUnauthorized("Member not found")
	}

	// Rotate: delete old, create new
	s.db.RefreshTokens().DeleteOne(ctx, bson.M{"_id": rt.ID})

	newAccess, err := s.jwtMgr.CreateAccessToken(member.ID.Hex(), member.Email, member.Role, member.IsAdmin)
	if err != nil {
		return "", "", err
	}
	newRefresh, err := jwt.GenerateRefreshToken()
	if err != nil {
		return "", "", err
	}

	s.db.RefreshTokens().InsertOne(ctx, models.RefreshToken{
		MemberID:  member.ID.Hex(),
		Token:     newRefresh,
		ExpiresAt: time.Now().Add(s.cfg.JWTRefreshExpiry),
		CreatedAt: time.Now(),
	})

	return newAccess, newRefresh, nil
}

// Logout revokes the refresh token and marks member offline.
func (s *AuthService) Logout(ctx context.Context, memberID, refreshToken string) error {
	if refreshToken != "" {
		s.db.RefreshTokens().DeleteOne(ctx, bson.M{"token": refreshToken})
	}
	oid, _ := primitive.ObjectIDFromHex(memberID)
	now := time.Now()
	s.db.Members().UpdateByID(ctx, oid, bson.M{
		"$set": bson.M{"online": false, "lastSeen": now, "updatedAt": now},
	})
	return nil
}

// SendResetOTP creates and stores a password-reset OTP.
func (s *AuthService) SendResetOTP(ctx context.Context, email string) error {
	count, _ := s.db.Members().CountDocuments(ctx, bson.M{"email": strings.ToLower(email)})
	if count == 0 {
		return nil // silent — no enumeration
	}
	return s.createOTP(ctx, strings.ToLower(email), "reset")
}

// ResetPassword verifies OTP and sets a new password.
func (s *AuthService) ResetPassword(ctx context.Context, email, code, newPassword string) error {
	if err := s.VerifyOTP(ctx, email, code, "reset"); err != nil {
		return err
	}
	hash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return err
	}
	now := time.Now()
	res, err := s.db.Members().UpdateOne(ctx,
		bson.M{"email": strings.ToLower(email)},
		bson.M{"$set": bson.M{"passwordHash": hash, "updatedAt": now}},
	)
	if err != nil || res.MatchedCount == 0 {
		return errNotFound("Member not found")
	}
	// Revoke all refresh tokens
	s.db.RefreshTokens().DeleteMany(ctx, bson.M{"memberId": email})
	return nil
}

// ChangePassword verifies current password and sets a new one.
func (s *AuthService) ChangePassword(ctx context.Context, member *models.Member, current, newPwd string) error {
	if !crypto.VerifyPassword(current, member.PasswordHash) {
		return errBadRequest("Current password is incorrect")
	}
	hash, err := crypto.HashPassword(newPwd)
	if err != nil {
		return err
	}
	now := time.Now()
	s.db.Members().UpdateByID(ctx, member.ID, bson.M{
		"$set": bson.M{"passwordHash": hash, "updatedAt": now},
	})
	s.db.RefreshTokens().DeleteMany(ctx, bson.M{"memberId": member.ID.Hex()})
	return nil
}

// ── Helpers ───────────────────────────────────────────────────

func (s *AuthService) createOTP(ctx context.Context, email, purpose string) error {
	s.db.OTPs().DeleteMany(ctx, bson.M{"email": email, "purpose": purpose})

	code := jwt.GenerateOTP(6)
	hash, err := crypto.HashPassword(code)
	if err != nil {
		return err
	}

	s.db.OTPs().InsertOne(ctx, models.OTP{
		Email:     email,
		Code:      hash,
		Purpose:   purpose,
		Attempts:  0,
		ExpiresAt: time.Now().Add(s.cfg.OTPExpiry),
		CreatedAt: time.Now(),
	})

	if s.cfg.IsDevelopment() {
		s.log.Warn("[DEV OTP]", zap.String("email", email), zap.String("code", code))
	}

	go s.mailer.SendOTP(email, code, purpose)

	return nil
}

func (s *AuthService) audit(ctx context.Context, action, userID, email, ip, ua string) {
	s.db.AuditLogs().InsertOne(ctx, models.AuditLog{
		UserID:    userID,
		UserEmail: email,
		Action:    action,
		IPAddress: ip,
		UserAgent: ua,
		Timestamp: time.Now(),
	})
}

// RegisterInput carries registration request data.
type RegisterInput struct {
	FullName      string
	Email         string
	Password      string
	Phone         string
	DateOfBirth   string
	Gender        string
	MaritalStatus string
	VoicePart     string
}


