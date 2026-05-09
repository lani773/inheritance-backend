// Package repository handles all MongoDB interactions.
package repository

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"

	"github.com/inheritance-choir/backend/internal/config"
)

// DB wraps the MongoDB client and database handle.
type DB struct {
	Client *mongo.Client
	DB     *mongo.Database
	log    *zap.Logger
}

// Collections returns named collection handles.
func (d *DB) Members() *mongo.Collection         { return d.DB.Collection("members") }
func (d *DB) Events() *mongo.Collection          { return d.DB.Collection("events") }
func (d *DB) Contributions() *mongo.Collection   { return d.DB.Collection("contributions") }
func (d *DB) Attendances() *mongo.Collection     { return d.DB.Collection("attendances") }
func (d *DB) Excuses() *mongo.Collection         { return d.DB.Collection("excuses") }
func (d *DB) Messages() *mongo.Collection        { return d.DB.Collection("messages") }
func (d *DB) OTPs() *mongo.Collection            { return d.DB.Collection("otps") }
func (d *DB) RefreshTokens() *mongo.Collection   { return d.DB.Collection("refreshtokens") }
func (d *DB) AuditLogs() *mongo.Collection       { return d.DB.Collection("auditlogs") }
func (d *DB) Posts() *mongo.Collection           { return d.DB.Collection("posts") }
func (d *DB) Songs() *mongo.Collection           { return d.DB.Collection("songs") }
func (d *DB) WelfareCases() *mongo.Collection    { return d.DB.Collection("welfarecases") }
func (d *DB) Notifications() *mongo.Collection   { return d.DB.Collection("notifications") }
func (d *DB) AppSettings() *mongo.Collection     { return d.DB.Collection("appsettings") }
func (d *DB) AutomationRules() *mongo.Collection { return d.DB.Collection("automationrules") }
func (d *DB) Setlists() *mongo.Collection        { return d.DB.Collection("setlists") }
func (d *DB) BudgetGoals() *mongo.Collection     { return d.DB.Collection("budgetgoals") }
func (d *DB) PledgeCampaigns() *mongo.Collection { return d.DB.Collection("pledgecampaigns") }
func (d *DB) Pledges() *mongo.Collection         { return d.DB.Collection("pledges") }
func (d *DB) APIKeys() *mongo.Collection         { return d.DB.Collection("apikeys") }
func (d *DB) Webhooks() *mongo.Collection        { return d.DB.Collection("webhooks") }
func (d *DB) UploadAssets() *mongo.Collection    { return d.DB.Collection("uploadassets") }
func (d *DB) PrayerRequests() *mongo.Collection  { return d.DB.Collection("prayerrequests") }
func (d *DB) ChatMessages() *mongo.Collection    { return d.DB.Collection("chatmessages") }

// Connect establishes a connection to MongoDB and ensures all indexes.
func Connect(cfg *config.Config, log *zap.Logger) (*DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().
		ApplyURI(cfg.MongoURI).
		SetServerAPIOptions(serverAPI).
		SetMinPoolSize(cfg.MongoPoolMin).
		SetMaxPoolSize(cfg.MongoPoolMax).
		SetConnectTimeout(5 * time.Second).
		SetServerSelectionTimeout(5 * time.Second).
		SetCompressors([]string{"snappy", "zlib"})

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}

	if err = client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	db := &DB{Client: client, DB: client.Database(cfg.MongoDB), log: log}

	if err = db.ensureIndexes(ctx); err != nil {
		return nil, fmt.Errorf("mongo indexes: %w", err)
	}

	log.Info("✅ MongoDB connected with Stable API v1",
		zap.String("uri", cfg.MongoURI),
		zap.String("db", cfg.MongoDB),
	)
	return db, nil
}

// Disconnect closes the MongoDB connection gracefully.
func (d *DB) Disconnect(ctx context.Context) error {
	return d.Client.Disconnect(ctx)
}

func (d *DB) ensureIndexes(ctx context.Context) error {
	type indexSpec struct {
		collection *mongo.Collection
		indexes    []mongo.IndexModel
	}

	specs := []indexSpec{
		{d.Members(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "status", Value: 1}}},
			{Keys: bson.D{{Key: "voicePart", Value: 1}}},
			{Keys: bson.D{{Key: "role", Value: 1}}},
			{Keys: bson.D{{Key: "fullName", Value: "text"}, {Key: "email", Value: "text"}}},
		}},
		{d.Events(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "date", Value: 1}}},
			{Keys: bson.D{{Key: "type", Value: 1}}},
		}},
		{d.Contributions(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "memberId", Value: 1}}},
			{Keys: bson.D{{Key: "date", Value: -1}}},
			{Keys: bson.D{{Key: "verified", Value: 1}}},
			{Keys: bson.D{{Key: "type", Value: 1}}},
		}},
		{d.Attendances(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "eventId", Value: 1}, {Key: "memberId", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "memberId", Value: 1}}},
			{Keys: bson.D{{Key: "date", Value: 1}}},
		}},
		{d.Messages(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "senderId", Value: 1}}},
			{Keys: bson.D{{Key: "recipientId", Value: 1}}},
			{Keys: bson.D{{Key: "sentAt", Value: -1}}},
		}},
		{d.OTPs(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			{Keys: bson.D{{Key: "email", Value: 1}}},
		}},
		{d.RefreshTokens(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "token", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			{Keys: bson.D{{Key: "memberId", Value: 1}}},
		}},
		{d.AuditLogs(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "timestamp", Value: -1}}},
			{Keys: bson.D{{Key: "userId", Value: 1}}},
			{Keys: bson.D{{Key: "action", Value: 1}}},
		}},
		{d.Notifications(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "recipientId", Value: 1}, {Key: "isRead", Value: 1}}},
			{Keys: bson.D{{Key: "createdAt", Value: -1}}},
		}},
		{d.Songs(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "title", Value: "text"}, {Key: "artist", Value: "text"}, {Key: "lyrics", Value: "text"}}},
		}},
		{d.Posts(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "title", Value: "text"}, {Key: "content", Value: "text"}, {Key: "tags", Value: "text"}}},
		}},
		{d.Setlists(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "eventId", Value: 1}}},
			{Keys: bson.D{{Key: "title", Value: "text"}, {Key: "notes", Value: "text"}}},
		}},
		{d.BudgetGoals(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "year", Value: 1}, {Key: "type", Value: 1}}, Options: options.Index().SetUnique(true)},
		}},
		{d.Pledges(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "campaignId", Value: 1}, {Key: "memberId", Value: 1}}},
			{Keys: bson.D{{Key: "status", Value: 1}}},
		}},
		{d.APIKeys(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "prefix", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "status", Value: 1}}},
		}},
		{d.Webhooks(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "active", Value: 1}}},
		}},
		{d.PrayerRequests(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "memberId", Value: 1}}},
			{Keys: bson.D{{Key: "status", Value: 1}}},
			{Keys: bson.D{{Key: "title", Value: "text"}, {Key: "description", Value: "text"}}},
		}},
		{d.ChatMessages(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "channelId", Value: 1}, {Key: "createdAt", Value: -1}}},
		}},
	}

	for _, spec := range specs {
		if _, err := spec.collection.Indexes().CreateMany(ctx, spec.indexes); err != nil {
			d.log.Warn("index creation warning", zap.Error(err))
		}
	}

	d.log.Info("✅ MongoDB indexes ensured")
	return nil
}
