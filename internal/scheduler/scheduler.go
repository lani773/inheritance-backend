// Package scheduler provides cron-based background job scheduling.
package scheduler
import "go.mongodb.org/mongo-driver/v2/bson"

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/robfig/cron/v3"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"

	"github.com/inheritance-choir/backend/internal/config"
	"github.com/inheritance-choir/backend/internal/models"
	"github.com/inheritance-choir/backend/internal/notifications"
	"github.com/inheritance-choir/backend/internal/repository"
	"github.com/inheritance-choir/backend/internal/services"
	"github.com/inheritance-choir/backend/internal/workers"
)

// Scheduler wraps robfig/cron with our domain jobs.
type Scheduler struct {
	cron    *cron.Cron
	db      *repository.DB
	pool    *workers.Pool
	mailer  *notifications.Mailer
	cfg     *config.Config
	autoSvc *services.AutomationService
	log     *zap.Logger
}

// New creates and starts the Scheduler. Call Stop() on shutdown.
func New(db *repository.DB, pool *workers.Pool, mailer *notifications.Mailer, cfg *config.Config, autoSvc *services.AutomationService, log *zap.Logger) *Scheduler {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		loc = time.UTC
	}

	s := &Scheduler{
		cron:    cron.New(cron.WithLocation(loc), cron.WithSeconds()),
		db:      db,
		pool:    pool,
		mailer:  mailer,
		cfg:     cfg,
		autoSvc: autoSvc,
		log:     log,
	}
	s.registerJobs()
	return s
}

// Start begins the cron scheduler.
func (s *Scheduler) Start() {
	s.cron.Start()
	s.log.Info("✅ Scheduler started", zap.Int("jobs", len(s.cron.Entries())))
}

// Stop gracefully stops the scheduler.
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.log.Info("Scheduler stopped")
}

func (s *Scheduler) registerJobs() {
	day := s.cfg.MonthlyReminderDay

	// Monthly tithe reminder — 5th of each month @ 09:00
	s.cron.AddFunc(fmt.Sprintf("0 0 9 %d * *", day), func() {
		s.pool.Submit(workers.Task{
			Name:    "monthly-tithe-reminder",
			Retries: 2,
			Handler: s.monthlyTitheReminder,
		})
	})

	// Weekly at-risk alert — every Sunday @ 08:00
	s.cron.AddFunc("0 0 8 * * 0", func() {
		s.pool.Submit(workers.Task{
			Name:    "weekly-attendance-alert",
			Retries: 1,
			Handler: s.weeklyAttendanceAlert,
		})
	})

	// Event reminders — every hour
	s.cron.AddFunc("0 0 * * * *", func() {
		s.pool.Submit(workers.Task{
			Name:    "event-reminders",
			Retries: 1,
			Handler: s.eventReminders,
		})
	})

	// Nightly stats recalc — 02:00
	s.cron.AddFunc("0 0 2 * * *", func() {
		s.pool.Submit(workers.Task{
			Name:    "nightly-stats-recalc",
			Retries: 1,
			Handler: s.nightlyStatsRecalc,
		})
	})

	// Monthly finance report — 1st of month @ 07:00
	s.cron.AddFunc("0 0 7 1 * *", func() {
		s.pool.Submit(workers.Task{
			Name:    "monthly-finance-report",
			Retries: 1,
			Handler: s.monthlyFinanceReport,
		})
	})

	// Weekly choir digest — Friday @ 17:00
	s.cron.AddFunc("0 0 17 * * 5", func() {
		s.pool.Submit(workers.Task{
			Name:    "weekly-choir-digest",
			Retries: 1,
			Handler: s.weeklyChoirDigest,
		})
	})

	// Birthday greetings — daily @ 08:00
	s.cron.AddFunc("0 0 8 * * *", func() {
		s.pool.Submit(workers.Task{
			Name:    "birthday-greetings",
			Retries: 1,
			Handler: s.birthdayGreetings,
		})
	})

	// Token cleanup — every 6 hours
	s.cron.AddFunc("0 0 */6 * * *", func() {
		s.pool.Submit(workers.Task{
			Name:    "token-cleanup",
			Retries: 0,
			Handler: s.cleanupTokens,
		})
	})

	// Welcome sequences — every 5 minutes
	s.cron.AddFunc("0 */5 * * * *", func() {
		s.pool.Submit(workers.Task{
			Name:    "welcome-sequences",
			Retries: 0,
			Handler: s.welcomeSequences,
		})
	})

	// Run all active user-defined automation rules — every 30 minutes
	s.cron.AddFunc("0 */30 * * * *", func() {
		s.pool.Submit(workers.Task{
			Name:    "run-active-automations",
			Retries: 0,
			Handler: func(ctx context.Context, _ interface{}) error {
				s.autoSvc.RunAllActive(ctx)
				return nil
			},
		})
	})
}

// ── Job implementations ───────────────────────────────────────

func (s *Scheduler) monthlyTitheReminder(ctx context.Context, _ interface{}) error {
	s.log.Info("[JOB] monthly-tithe-reminder started")
	thisMonth := time.Now().Format("2006-01")

	cursor, err := s.db.Members().Find(ctx, bson.M{"status": "active"})
	if err != nil { return err }
	defer cursor.Close(ctx)

	var members []models.Member
	cursor.All(ctx, &members)

	sent := 0
	for _, m := range members {
		count, _ := s.db.Contributions().CountDocuments(ctx, bson.M{
			"memberId": m.ID.Hex(),
			"date":     bson.M{"$regex": "^" + thisMonth},
		})
		if count == 0 {
			s.mailer.SendMonthlyTitheReminder(m.Email, m.FullName, time.Now().Format("January 2006"))
			s.createNotification(ctx, m.ID.Hex(), "info",
				"💰 Monthly Contribution Reminder",
				"Your contribution for "+time.Now().Format("January 2006")+" has not been recorded.",
				"/dashboard/contributions",
			)
			sent++
		}
	}

	s.auditLog(ctx, "automation.monthly_tithe_reminder", map[string]interface{}{
		"month": thisMonth, "sent": sent, "total": len(members),
	})
	s.log.Info("[JOB] monthly-tithe-reminder done", zap.Int("sent", sent))
	return nil
}

func (s *Scheduler) weeklyAttendanceAlert(ctx context.Context, _ interface{}) error {
	s.log.Info("[JOB] weekly-attendance-alert started")
	threshold := s.cfg.AttendanceThreshold

	cursor, err := s.db.Members().Find(ctx, bson.M{
		"status": "active", "attendance": bson.M{"$lt": threshold},
	})
	if err != nil { return err }
	defer cursor.Close(ctx)

	var atRisk []models.Member
	cursor.All(ctx, &atRisk)

	for _, m := range atRisk {
		s.mailer.SendAttendanceAlert(m.Email, m.FullName, m.Attendance)
		s.createNotification(ctx, m.ID.Hex(), "warning",
			"⚠️ Attendance Alert",
			fmt.Sprintf("Your attendance is %.1f%% — below the %.0f%% target.", m.Attendance, threshold),
			"/dashboard/attendance",
		)
	}

	// Admin digest
	s.sendAdminDigest(ctx)

	s.log.Info("[JOB] weekly-attendance-alert done", zap.Int("atRisk", len(atRisk)))
	return nil
}

func (s *Scheduler) eventReminders(ctx context.Context, _ interface{}) error {
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	cursor, err := s.db.Events().Find(ctx, bson.M{
		"date": tomorrow, "mandatory": true,
	})
	if err != nil { return err }
	defer cursor.Close(ctx)

	var events []models.Event
	cursor.All(ctx, &events)

	for _, ev := range events {
		// Skip if reminder already sent
		count, _ := s.db.AuditLogs().CountDocuments(ctx, bson.M{
			"action":     "automation.event_reminder_sent",
			"resourceId": ev.ID.Hex(),
		})
		if count > 0 { continue }

		memberFilter := bson.M{"status": "active"}
		if len(ev.TargetVoices) > 0 {
			memberFilter["voicePart"] = bson.M{"$in": ev.TargetVoices}
		}

		mCur, _ := s.db.Members().Find(ctx, memberFilter)
		var members []models.Member
		mCur.All(ctx, &members)
		mCur.Close(ctx)

		for _, m := range members {
			s.mailer.SendEventReminder(m.Email, m.FullName, ev.Title, ev.Date, ev.Time, ev.Location)
			s.createNotification(ctx, m.ID.Hex(), "info",
				"📅 Reminder: "+ev.Title+" Tomorrow",
				ev.Date+" at "+ev.Time+" — "+ev.Location,
				"/dashboard/events",
			)
		}

		s.auditLog(ctx, "automation.event_reminder_sent", map[string]interface{}{
			"eventId": ev.ID.Hex(), "eventTitle": ev.Title, "recipientCount": len(members),
		})
	}
	return nil
}

func (s *Scheduler) nightlyStatsRecalc(ctx context.Context, _ interface{}) error {
	s.log.Info("[JOB] nightly-stats-recalc started")

	cursor, err := s.db.Members().Find(ctx, bson.M{"status": "active"})
	if err != nil { return err }
	defer cursor.Close(ctx)

	var members []models.Member
	cursor.All(ctx, &members)

	for _, m := range members {
		mid := m.ID.Hex()

		// Attendance
		total, _    := s.db.Attendances().CountDocuments(ctx, bson.M{"memberId": mid})
		attended, _ := s.db.Attendances().CountDocuments(ctx, bson.M{
			"memberId": mid, "status": bson.M{"$in": []string{"present", "late"}},
		})
		attRate := 0.0
		if total > 0 { attRate = float64(attended) / float64(total) * 100 }

		// Contributions
		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: bson.M{"memberId": mid}}},
			{{Key: "$group", Value: bson.M{"_id": nil, "total": bson.M{"$sum": "$amount"}}}},
		}
		cur, _ := s.db.Contributions().Aggregate(ctx, pipeline)
		var agg []struct{ Total float64 `bson:"total"` }
		cur.All(ctx, &agg)
		total_contrib := 0.0
		if len(agg) > 0 { total_contrib = agg[0].Total }

		now := time.Now()
		s.db.Members().UpdateByID(ctx, m.ID, bson.M{
			"$set": bson.M{
				"attendance": math.Round(attRate*10) / 10,
				"contributionTotal": math.Round(total_contrib*100) / 100,
				"updatedAt": now,
			},
		})
	}

	s.auditLog(ctx, "automation.nightly_stats_recalc", map[string]interface{}{
		"membersUpdated": len(members),
	})
	s.log.Info("[JOB] nightly-stats-recalc done", zap.Int("updated", len(members)))
	return nil
}

func (s *Scheduler) monthlyFinanceReport(ctx context.Context, _ interface{}) error {
	s.log.Info("[JOB] monthly-finance-report started")
	// Fetch contributions for last month and email Excel to admins
	lastMonth := time.Now().AddDate(0, -1, 0).Format("2006-01")

	cursor, _ := s.db.Contributions().Find(ctx, bson.M{"date": bson.M{"$regex": "^" + lastMonth}})
	var contribs []models.Contribution
	cursor.All(ctx, &contribs)
	cursor.Close(ctx)

	admins, _ := s.getAdmins(ctx)
	for _, admin := range admins {
		s.mailer.SendFinanceReport(admin.Email, admin.FullName, lastMonth, len(contribs))
	}

	s.log.Info("[JOB] monthly-finance-report done", zap.Int("admins", len(admins)))
	return nil
}

func (s *Scheduler) weeklyChoirDigest(ctx context.Context, _ interface{}) error {
	stats := s.getChoirStats(ctx)
	admins, _ := s.getAdmins(ctx)
	for _, a := range admins {
		s.mailer.SendChoirDigest(a.Email, a.FullName, stats)
	}
	s.log.Info("[JOB] weekly-choir-digest done", zap.Int("admins", len(admins)))
	return nil
}

func (s *Scheduler) birthdayGreetings(ctx context.Context, _ interface{}) error {
	today := "-" + time.Now().Format("01-02") // -MM-DD suffix
	cursor, err := s.db.Members().Find(ctx, bson.M{
		"status": "active",
		"dateOfBirth": bson.M{"$regex": today + "$"},
	})
	if err != nil { return err }
	defer cursor.Close(ctx)

	var members []models.Member
	cursor.All(ctx, &members)

	for _, m := range members {
		// Check not already sent today
		count, _ := s.db.AuditLogs().CountDocuments(ctx, bson.M{
			"action":     "automation.birthday_greeting",
			"resourceId": m.ID.Hex(),
			"timestamp":  bson.M{"$gte": time.Now().Truncate(24 * time.Hour)},
		})
		if count > 0 { continue }

		s.mailer.SendBirthdayGreeting(m.Email, m.FullName)
		s.createNotification(ctx, m.ID.Hex(), "success",
			"🎂 Happy Birthday!", "The Inheritance Choir family wishes you a wonderful birthday!", "")
		s.auditLog(ctx, "automation.birthday_greeting", map[string]interface{}{
			"memberId": m.ID.Hex(), "name": m.FullName,
		})
	}
	return nil
}

func (s *Scheduler) cleanupTokens(ctx context.Context, _ interface{}) error {
	now := time.Now()
	r1, _ := s.db.OTPs().DeleteMany(ctx, bson.M{"expiresAt": bson.M{"$lt": now}})
	r2, _ := s.db.RefreshTokens().DeleteMany(ctx, bson.M{"expiresAt": bson.M{"$lt": now}})
	s.log.Info("[JOB] token-cleanup done",
		zap.Int64("otps", r1.DeletedCount),
		zap.Int64("refreshTokens", r2.DeletedCount),
	)
	return nil
}

func (s *Scheduler) welcomeSequences(ctx context.Context, _ interface{}) error {
	window := time.Now().Add(-10 * time.Minute)
	cursor, err := s.db.Members().Find(ctx, bson.M{
		"status": "active", "updatedAt": bson.M{"$gte": window},
	})
	if err != nil { return err }
	defer cursor.Close(ctx)

	var members []models.Member
	cursor.All(ctx, &members)

	for _, m := range members {
		count, _ := s.db.AuditLogs().CountDocuments(ctx, bson.M{
			"action": "automation.welcome_email_sent", "resourceId": m.ID.Hex(),
		})
		if count == 0 {
			s.mailer.SendWelcomeEmail(m.Email, m.FullName)
			s.auditLog(ctx, "automation.welcome_email_sent", map[string]interface{}{
				"memberId": m.ID.Hex(),
			})
		}
	}
	return nil
}

// ── Internal helpers ──────────────────────────────────────────

func (s *Scheduler) createNotification(ctx context.Context, recipientID, nType, title, message, actionURL string) {
	s.db.Notifications().InsertOne(ctx, models.Notification{
		RecipientID: recipientID,
		Type:        nType,
		Title:       title,
		Message:     message,
		IsRead:      false,
		ActionURL:   actionURL,
		CreatedAt:   time.Now(),
	})
}

func (s *Scheduler) auditLog(ctx context.Context, action string, vals map[string]interface{}) {
	s.db.AuditLogs().InsertOne(ctx, models.AuditLog{
		Action:    action,
		UserEmail: "system@automation",
		NewValues: vals,
		Timestamp: time.Now(),
	})
}

func (s *Scheduler) getAdmins(ctx context.Context) ([]models.Member, error) {
	cursor, err := s.db.Members().Find(ctx, bson.M{"isAdmin": true, "status": "active"})
	if err != nil { return nil, err }
	defer cursor.Close(ctx)
	var admins []models.Member
	cursor.All(ctx, &admins)
	return admins, nil
}

func (s *Scheduler) getChoirStats(ctx context.Context) map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	active, _   := s.db.Members().CountDocuments(ctx, bson.M{"status": "active"})
	pending, _  := s.db.Members().CountDocuments(ctx, bson.M{"status": "pending"})
	upcoming, _ := s.db.Events().CountDocuments(ctx, bson.M{"date": bson.M{"$gte": today}})
	atRisk, _   := s.db.Members().CountDocuments(ctx, bson.M{
		"status": "active", "attendance": bson.M{"$lt": s.cfg.AttendanceThreshold},
	})
	return map[string]interface{}{
		"activeMembers": active, "pendingMembers": pending,
		"upcomingEvents": upcoming, "atRiskCount": atRisk,
	}
}

func (s *Scheduler) sendAdminDigest(ctx context.Context) {
	stats := s.getChoirStats(ctx)
	admins, _ := s.getAdmins(ctx)
	for _, a := range admins {
		s.mailer.SendChoirDigest(a.Email, a.FullName, stats)
	}
}


