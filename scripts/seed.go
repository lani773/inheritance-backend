// scripts/seed.go — seeds the database with sample data
// Run: go run ./scripts/seed.go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"

	"github.com/inheritance-choir/backend/internal/config"
	"github.com/inheritance-choir/backend/internal/models"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Simple logger for seed
	fmt.Println("🌱 Connecting to MongoDB…")
	client, _ := mongo.Connect(context.Background(),
		options.Client().ApplyURI(cfg.MongoURI))
	defer client.Disconnect(context.Background())
	db := client.Database(cfg.MongoDB)
	ctx := context.Background()

	// ── Admin ──────────────────────────────────────────────
	adminEmail := cfg.AdminEmail
	count, _ := db.Collection("members").CountDocuments(ctx, bson.M{"email": adminEmail})
	if count == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), 12)
		now := time.Now()
		db.Collection("members").InsertOne(ctx, models.Member{
			FullName: "Choir Administrator", Email: adminEmail,
			PasswordHash: string(hash), VoicePart: "Soprano",
			Role: "president", Status: "active", IsAdmin: true,
			Permissions: []string{"all"}, JoinDate: now, CreatedAt: now, UpdatedAt: now,
		})
		fmt.Printf("✅ Admin: %s\n", adminEmail)
	} else {
		fmt.Printf("ℹ️  Admin already exists: %s\n", adminEmail)
	}

	// ── Sample members ─────────────────────────────────────
	sampleMembers := []struct{ Name, Email, Voice string }{
		{"Marie Claire Uwimana", "marie@choir.rw", "Soprano"},
		{"Diane Mukamana", "diane@choir.rw", "Alto"},
		{"Jean-Paul Habimana", "jean@choir.rw", "Tenor"},
		{"Emmanuel Ndayishimiye", "emma@choir.rw", "Bass"},
		{"Erica Ingabire", "erica@choir.rw", "Soprano"},
		{"Solange Nkurunziza", "solange@choir.rw", "Alto"},
		{"Patrick Nzabahimana", "pat@choir.rw", "Tenor"},
		{"Olivier Rukundo", "olivier@choir.rw", "Bass"},
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("Password123."), 12)
	for _, m := range sampleMembers {
		c, _ := db.Collection("members").CountDocuments(ctx, bson.M{"email": m.Email})
		if c == 0 {
			now := time.Now()
			db.Collection("members").InsertOne(ctx, models.Member{
				FullName: m.Name, Email: m.Email,
				PasswordHash: string(hash), VoicePart: m.Voice,
				Role: "member", Status: "active", JoinDate: now, CreatedAt: now, UpdatedAt: now,
			})
			fmt.Printf("   ✅ Member: %s (%s)\n", m.Name, m.Voice)
		}
	}

	// ── Sample events ──────────────────────────────────────
	events := []struct {
		Title, Type string
		Days        int
		Mandatory   bool
	}{
		{"Weekly Rehearsal", "rehearsal", 2, true},
		{"Sunday Service", "service", 5, true},
		{"Directors Meeting", "meeting", 7, false},
		{"Christmas Performance", "performance", 30, true},
		{"Voice Training", "workshop", 14, false},
	}
	for _, ev := range events {
		c, _ := db.Collection("events").CountDocuments(ctx, bson.M{"title": ev.Title})
		if c == 0 {
			date := time.Now().AddDate(0, 0, ev.Days).Format("2006-01-02")
			now := time.Now()
			db.Collection("events").InsertOne(ctx, models.Event{
				Title: ev.Title, Type: ev.Type, Date: date,
				Time: "09:00", EndTime: "11:00",
				Location:  "Kigali Main Church",
				Mandatory: ev.Mandatory, TargetVoices: []string{},
				CreatedAt: now, UpdatedAt: now,
			})
			fmt.Printf("   ✅ Event: %s\n", ev.Title)
		}
	}

	// ── App settings ───────────────────────────────────────
	c, _ := db.Collection("appsettings").CountDocuments(ctx, bson.M{"key": "global"})
	if c == 0 {
		db.Collection("appsettings").InsertOne(ctx, models.AppSettings{
			Key: "global", ChoirName: "INHERITANCE CHOIR",
			Tagline:      "Voices united in worship and excellence",
			ContactEmail: adminEmail, Currency: "RWF", Timezone: "Africa/Kigali",
			AttendanceTarget: 80, TitheSuggestion: 10,
			EmailNotifications: true, UpdatedAt: time.Now(),
		})
		fmt.Println("✅ App settings created")
	}

	fmt.Println("\n🎉 Seed complete!")
	fmt.Printf("   Admin:   %s\n", adminEmail)
	fmt.Println("   Members: 8 sample members (Password123.)")
}
