// Package handlers contains all Gin HTTP handlers for the Inheritance Choir API.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/inheritance-choir/backend/internal/api/v1/middleware"
	"github.com/inheritance-choir/backend/internal/services"
	"github.com/inheritance-choir/backend/internal/utils"
)

// AuthHandler groups all authentication-related handlers.
type AuthHandler struct {
	svc *services.AuthService
}

func NewAuthHandler(svc *services.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// ── POST /auth/login ──────────────────────────────────────────

type loginRequest struct {
	Email      string `json:"email"      binding:"required,email"`
	Password   string `json:"password"   binding:"required,min=6"`
	RememberMe bool   `json:"rememberMe"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if !bindJSON(c, &req) {
		return
	}

	result, err := h.svc.Login(
		c.Request.Context(),
		req.Email, req.Password, req.RememberMe,
		c.ClientIP(), c.GetHeader("User-Agent"),
	)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Login successful",
		"accessToken":  result.AccessToken,
		"refreshToken": result.RefreshToken,
		"tokenType":    "Bearer",
		"expiresIn":    3600,
		"member":       utils.FormatMember(result.Member),
	})
}

// ── POST /auth/register ───────────────────────────────────────

type registerRequest struct {
	FullName      string `json:"fullName"      binding:"required,min=2,max=100"`
	Email         string `json:"email"         binding:"required,email"`
	Password      string `json:"password"      binding:"required,min=8"`
	Phone         string `json:"phone"`
	DateOfBirth   string `json:"dateOfBirth"`
	Gender        string `json:"gender"`
	MaritalStatus string `json:"maritalStatus"`
	VoicePart     string `json:"voicePart"     binding:"required,oneof=Soprano Alto Tenor Bass"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if !bindJSON(c, &req) {
		return
	}

	member, err := h.svc.Register(c.Request.Context(), services.RegisterInput{
		FullName:      req.FullName,
		Email:         req.Email,
		Password:      req.Password,
		Phone:         req.Phone,
		DateOfBirth:   req.DateOfBirth,
		Gender:        req.Gender,
		MaritalStatus: req.MaritalStatus,
		VoicePart:     req.VoicePart,
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":  true,
		"message":  "Registration successful. Check your email for the verification code.",
		"memberId": member.ID.Hex(),
		"email":    member.Email,
	})
}

// ── POST /auth/verify-otp ─────────────────────────────────────

type otpRequest struct {
	Email   string `json:"email"   binding:"required,email"`
	Code    string `json:"code"    binding:"required,len=6"`
	Purpose string `json:"purpose"`
}

func (h *AuthHandler) VerifyOTP(c *gin.Context) {
	var req otpRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Purpose == "" {
		req.Purpose = "verify"
	}

	if err := h.svc.VerifyOTP(c.Request.Context(), req.Email, req.Code, req.Purpose); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Email verified successfully", "verified": true})
}

// ── POST /auth/refresh ────────────────────────────────────────

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refreshToken" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}

	access, refresh, err := h.svc.RefreshTokens(c.Request.Context(), req.RefreshToken)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"accessToken":  access,
		"refreshToken": refresh,
		"tokenType":    "Bearer",
		"expiresIn":    3600,
	})
}

// ── POST /auth/logout ─────────────────────────────────────────

func (h *AuthHandler) Logout(c *gin.Context) {
	member := middleware.CurrentMember(c)
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	_ = c.ShouldBindJSON(&req)

	h.svc.Logout(c.Request.Context(), member.ID.Hex(), req.RefreshToken)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Logged out successfully"})
}

// ── POST /auth/forgot-password ────────────────────────────────

func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if !bindJSON(c, &req) {
		return
	}
	h.svc.SendResetOTP(c.Request.Context(), req.Email)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "If that email is registered, a reset code has been sent.",
	})
}

// ── POST /auth/reset-password ─────────────────────────────────

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req struct {
		Email       string `json:"email"       binding:"required,email"`
		Code        string `json:"code"        binding:"required,len=6"`
		NewPassword string `json:"newPassword" binding:"required,min=8"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := h.svc.ResetPassword(c.Request.Context(), req.Email, req.Code, req.NewPassword); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Password reset successfully."})
}

// ── POST /auth/change-password ────────────────────────────────

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req struct {
		CurrentPassword string `json:"currentPassword" binding:"required"`
		NewPassword     string `json:"newPassword"     binding:"required,min=8"`
	}
	if !bindJSON(c, &req) {
		return
	}
	member := middleware.CurrentMember(c)
	if err := h.svc.ChangePassword(c.Request.Context(), member, req.CurrentPassword, req.NewPassword); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Password changed. Please log in again."})
}

// ── GET /auth/me ──────────────────────────────────────────────

func (h *AuthHandler) GetMe(c *gin.Context) {
	member := middleware.CurrentMember(c)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": utils.FormatMember(member)})
}
