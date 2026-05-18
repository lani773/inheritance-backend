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
	"github.com/inheritance-choir/backend/internal/notifications"
	"github.com/inheritance-choir/backend/internal/realtime"
	"github.com/inheritance-choir/backend/internal/repository"
	"github.com/inheritance-choir/backend/internal/scheduler"
	"github.com/inheritance-choir/backend/internal/services"
	"github.com/inheritance-choir/backend/internal/workers"
	"github.com/inheritance-choir/backend/pkg/crypto"
	jwtpkg "github.com/inheritance-choir/backend/pkg/jwt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	autoSvc := services.NewAutomationService(db, mailer, log)

	// ── Scheduler (automation) ─────────────────────────────
	sched := scheduler.New(db, pool, mailer, cfg, autoSvc, log)
	sched.Start()

	// ── HTTP server ────────────────────────────────────────
	r := router.Setup(cfg, db, log, jwtMgr, authSvc, mailer, hub, autoSvc)

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

// mustNot panics if an error is not nil.
func mustNot(err error, label string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL [%s]: %v\n", label, err)
		os.Exit(1)
	}
}

// seedAdmin creates or updates the default administrator account from .env configuration.
func seedAdmin(db *repository.DB, cfg *config.Config, log *zap.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hashedPassword, err := crypto.HashPassword(cfg.AdminPassword)
	if err != nil {
		log.Error("Failed to hash admin password during seeding", zap.Error(err))
		return
	}

	filter := bson.M{"email": cfg.AdminEmail}
	update := bson.M{
		"$set": bson.M{
			"passwordHash": hashedPassword,
			"updatedAt":    time.Now(),
		},
		"$setOnInsert": bson.M{
			"fullName":  "System Administrator",
			"email":     cfg.AdminEmail,
			"role":      "president",
			"isAdmin":   true,
			"status":    "active",
			"voicePart": "Bass",
			"joinDate":  time.Now(),
			"createdAt": time.Now(),
		},
	}

	opts := options.Update().SetUpsert(true)
	res, err := db.Members().UpdateOne(ctx, filter, update, opts)
	if err != nil {
		log.Error("Failed to seed default admin account",
			zap.String("email", cfg.AdminEmail),
			zap.Error(err),
		)
		return
	}

	if res.UpsertedID != nil {
		log.Info("✅ Default admin account successfully seeded", zap.String("email", cfg.AdminEmail))
	} else if res.ModifiedCount > 0 {
		log.Info("✅ Existing admin account password updated from .env", zap.String("email", cfg.AdminEmail))
	} else {
		log.Info("Admin account already up-to-date", zap.String("email", cfg.AdminEmail))
	}
}
