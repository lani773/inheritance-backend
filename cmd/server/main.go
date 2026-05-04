// Package main is the entry point for the Inheritance Choir Go Gin backend.
//
// Architecture:
//   - Gin HTTP server (high-performance, low memory)
//   - MongoDB via official Go driver (connection pool, typed queries)
//   - Redis for caching and rate-limiting
//   - Goroutine worker pool for background tasks (zero external queue deps)
//   - robfig/cron for scheduled automation
//   - zap for structured, zero-allocation logging
//
// Run:
//
//	go run ./cmd/server
//	# or build a ~10MB static binary:
//	CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o choir-server ./cmd/server
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/inheritance-choir/backend/internal/api/v1/router"
	"github.com/inheritance-choir/backend/internal/config"
	"github.com/inheritance-choir/backend/internal/models"
	"github.com/inheritance-choir/backend/internal/notifications"
	"github.com/inheritance-choir/backend/internal/realtime"
	"github.com/inheritance-choir/backend/internal/repository"
	"github.com/inheritance-choir/backend/internal/scheduler"
	"github.com/inheritance-choir/backend/internal/services"
	"github.com/inheritance-choir/backend/internal/workers"
	"github.com/inheritance-choir/backend/pkg/crypto"
	jwtpkg "github.com/inheritance-choir/backend/pkg/jwt"
	"go.mongodb.org/mongo-driver/bson"
)

func main() {
	// ── Load configuration ─────────────────────────────────
	cfg, err := config.Load()
	mustNot(err, "config load")

	// ── Logger ─────────────────────────────────────────────
	var log *zap.Logger
	if cfg.IsProduction() {
		log, err = zap.NewProduction()
	} else {
		log, err = zap.NewDevelopment()
	}
	mustNot(err, "logger init")
	defer log.Sync()

	log.Info("🎵 Starting INHERITANCE CHOIR Go Gin Backend",
		zap.String("version", cfg.AppVersion),
		zap.String("env", cfg.Env),
		zap.String("port", cfg.Port),
	)

	// ── MongoDB ────────────────────────────────────────────
	db, err := repository.Connect(cfg, log)
	mustNot(err, "MongoDB connect")

	// ── Seed default admin ─────────────────────────────────
	seedAdmin(db, cfg, log)

	// ── JWT Manager ────────────────────────────────────────
	jwtMgr := jwtpkg.New(cfg.JWTSecret, cfg.JWTAccessExpiry, cfg.JWTRefreshExpiry)

	// ── Mailer ─────────────────────────────────────────────
	mailer := notifications.New(cfg, log)

	// ── Worker pool ─────────────────────────────────────────
	pool := workers.New(cfg.WorkerPoolSize, cfg.TaskQueueSize, log)
	log.Info("Worker pool ready",
		zap.Int("workers", cfg.WorkerPoolSize),
		zap.Int("queueSize", cfg.TaskQueueSize),
	)

	// ── Services ───────────────────────────────────────────
	authSvc := services.NewAuthService(db, cfg, jwtMgr, mailer, log)
	hub := realtime.NewHub(log)

	// ── Scheduler (automation) ─────────────────────────────
	sched := scheduler.New(db, pool, mailer, cfg, log)
	sched.Start()

	// ── HTTP server ────────────────────────────────────────
	r := router.Setup(cfg, db, log, jwtMgr, authSvc, mailer, hub)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}

	// Start in a goroutine
	go func() {
		log.Info(fmt.Sprintf("✅ Server listening on :%s", cfg.Port),
			zap.String("health", fmt.Sprintf("http://localhost:%s/health", cfg.Port)),
			zap.String("api", fmt.Sprintf("http://localhost:%s/api/v1", cfg.Port)),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ──────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down gracefully (30s timeout)…")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Stop accepting new requests
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("HTTP server shutdown error", zap.Error(err))
	}

	// 2. Stop scheduler
	sched.Stop()

	// 3. Drain worker pool
	pool.Shutdown(10 * time.Second)

	// 4. Close MongoDB
	db.Disconnect(ctx)

	log.Info("Goodbye 🎵")
}

func mustNot(err error, label string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL [%s]: %v\n", label, err)
		os.Exit(1)
	}
}

func seedAdmin(db *repository.DB, cfg *config.Config, log *zap.Logger) {
	ctx := context.Background()

	// Hash password from .env
	hashed, err := crypto.HashPassword(cfg.AdminPassword)
	if err != nil {
		log.Error("Failed to hash admin password", zap.Error(err))
		return
	}

	// Check if admin already exists
	var existing models.Member
	err = db.Members().FindOne(ctx, bson.M{"email": cfg.AdminEmail}).Decode(&existing)

	if err == nil {
		// Admin exists, update password to match .env
		_, err = db.Members().UpdateOne(ctx,
			bson.M{"_id": existing.ID},
			bson.M{"$set": bson.M{"passwordHash": hashed, "updatedAt": time.Now()}},
		)
		if err != nil {
			log.Error("Failed to update existing admin password", zap.Error(err))
		} else {
			log.Info("Existing admin account password updated from .env", zap.String("email", cfg.AdminEmail))
		}
		return
	}

	// Admin doesn't exist, create it
	admin := models.Member{
		FullName:     "System Administrator",
		Email:        cfg.AdminEmail,
		PasswordHash: hashed,
		Role:         "president",
		IsAdmin:      true,
		Status:       "active",
		JoinDate:     time.Now(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = db.Members().InsertOne(ctx, admin)
	if err != nil {
		log.Error("Failed to seed default admin account", zap.Error(err))
	} else {
		log.Info("Default admin account successfully seeded", zap.String("email", cfg.AdminEmail))
	}
}
