// Package middleware provides Gin middleware for the Inheritance Choir API.
package middleware
import "go.mongodb.org/mongo-driver/v2/bson"

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/inheritance-choir/backend/internal/models"
	"github.com/inheritance-choir/backend/internal/repository"
	jwtpkg "github.com/inheritance-choir/backend/pkg/jwt"
)

// ─── Auth Middleware ──────────────────────────────────────────

// Authenticate verifies the Bearer JWT and attaches the member to the context.
func Authenticate(db *repository.DB, jwtMgr *jwtpkg.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false, "message": "No token provided",
			})
			return
		}

		claims, err := jwtMgr.VerifyAccessToken(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false, "message": "Invalid or expired token",
			})
			return
		}

		ctx := c.Request.Context()
		var member models.Member
		err = db.Members().FindOne(ctx, bson.M{"_id": mustObjectID(claims.MemberID)}).Decode(&member)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false, "message": "Member not found",
			})
			return
		}

		switch member.Status {
		case "inactive":
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false, "message": "Account is inactive",
			})
			return
		case "pending":
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false, "message": "Account pending approval",
			})
			return
		}

		c.Set("member", &member)
		c.Set("memberID", member.ID.Hex())
		c.Next()
	}
}

// RequireAdmin checks that the current user has admin privileges.
func RequireAdmin() gin.HandlerFunc {
	adminRoles := map[string]bool{
		"president": true, "secretary": true,
		"treasurer": true, "choir_director": true,
	}
	return func(c *gin.Context) {
		member := CurrentMember(c)
		if member == nil || (!member.IsAdmin && !adminRoles[member.Role]) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false, "message": "Admin privileges required",
			})
			return
		}
		c.Next()
	}
}

// RequireRole allows access only to members with one of the specified roles.
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		member := CurrentMember(c)
		if member == nil || (!member.IsAdmin && !allowed[member.Role]) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false, "message": "Insufficient role",
			})
			return
		}
		c.Next()
	}
}

// CurrentMember extracts the authenticated member from Gin context.
func CurrentMember(c *gin.Context) *models.Member {
	val, exists := c.Get("member")
	if !exists {
		return nil
	}
	m, _ := val.(*models.Member)
	return m
}

// ─── Request Logger ───────────────────────────────────────────

func RequestLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start  := time.Now()
		path   := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		if path == "/health" || path == "/favicon.ico" {
			return
		}

		log.Info("HTTP",
			zap.String("method",  method),
			zap.String("path",    path),
			zap.Int("status",     c.Writer.Status()),
			zap.Duration("dur",   time.Since(start)),
			zap.String("ip",      c.ClientIP()),
			zap.String("reqID",   c.GetHeader("X-Request-ID")),
		)
	}
}

// ─── Recovery ─────────────────────────────────────────────────

func Recovery(log *zap.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		log.Error("panic recovered",
			zap.Any("error", recovered),
			zap.String("path", c.Request.URL.Path),
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false, "message": "Internal server error",
		})
	})
}

// ─── Rate limiter (in-memory, per IP) ─────────────────────────

// SimpleRateLimiter uses a token-bucket approach per IP via sync.Map.
// For production, use Redis-backed limiter (see workers/ratelimit.go).
func SimpleRateLimiter(requestsPerMin int) gin.HandlerFunc {
	type entry struct {
		count     int
		resetAt   time.Time
	}
	var mu sync.Map

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		val, _ := mu.LoadOrStore(ip, &entry{count: 0, resetAt: now.Add(time.Minute)})
		e := val.(*entry)

		if now.After(e.resetAt) {
			e.count = 0
			e.resetAt = now.Add(time.Minute)
		}

		e.count++
		if e.count > requestsPerMin {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "Rate limit exceeded. Please try again later.",
			})
			return
		}
		c.Next()
	}
}

// ─── Helpers ──────────────────────────────────────────────────

func mustObjectID(id string) interface{} {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return id
	}
	return oid
}


