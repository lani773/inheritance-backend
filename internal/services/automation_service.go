// Package services — automation rule execution engine.
// Reads all active rules from MongoDB and executes their configured actions.
package services

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"

	"github.com/inheritance-choir/backend/internal/models"
	"github.com/inheritance-choir/backend/internal/notifications"
	"github.com/inheritance-choir/backend/internal/realtime"
	"github.com/inheritance-choir/backend/internal/repository"
)

// AutomationService executes automation rules stored in MongoDB.
type AutomationService struct {
	db     *repository.DB
	mailer *notifications.Mailer
	log    *zap.Logger
}

// NewAutomationService creates a new AutomationService.
func NewAutomationService(db *repository.DB, mailer *notifications.Mailer, log *zap.Logger) *AutomationService {
	return &AutomationService{db: db, mailer: mailer, log: log}
}

// RunRule executes a single automation rule by ID and returns an execution summary.
func (s *AutomationService) RunRule(ctx context.Context, rule *models.AutomationRule) (string, error) {
	s.log.Info("executing automation rule", zap.String("id", rule.ID.Hex()), zap.String("action", rule.Action))

	var result string
	var err error

	switch rule.Action {
	case "send_notification":
		result, err = s.actionSendNotification(ctx, rule)
	case "send_email":
		result, err = s.actionSendEmail(ctx, rule)
	case "send_broadcast":
		result, err = s.actionSendBroadcast(rule)
	case "flag_absent_members":
		result, err = s.actionFlagAbsentMembers(ctx, rule)
	case "generate_report":
		result, err = s.actionGenerateReport(ctx, rule)
	case "send_birthday_wishes":
		result, err = s.actionBirthdayWishes(ctx, rule)
	default:
		result = fmt.Sprintf("unknown action: %s (no-op)", rule.Action)
	}

	// Update run metadata
	now := time.Now()
	s.db.AutomationRules().UpdateByID(ctx, rule.ID, bson.M{
		"$inc": bson.M{"runCount": 1},
		"$set": bson.M{"lastRun": now, "updatedAt": now},
	})

	if err != nil {
		s.log.Error("automation rule failed", zap.String("id", rule.ID.Hex()), zap.Error(err))
	}

	return result, err
}

// RunAllActive fetches and executes all active rules — called by the scheduler.
func (s *AutomationService) RunAllActive(ctx context.Context) {
	cursor, err := s.db.AutomationRules().Find(ctx, bson.M{"active": true})
	if err != nil {
		s.log.Error("failed to load automation rules", zap.Error(err))
		return
	}
	defer cursor.Close(ctx)

	var rules []models.AutomationRule
	cursor.All(ctx, &rules)

	for _, rule := range rules {
		r := rule // capture loop var
		go func() {
			result, err := s.RunRule(ctx, &r)
			if err != nil {
				s.log.Error("automation error", zap.String("rule", r.Name), zap.Error(err))
			} else {
				s.log.Info("automation ok", zap.String("rule", r.Name), zap.String("result", result))
			}
		}()
	}
}

// ── Action Implementations ────────────────────────────────────────

// actionSendNotification creates an in-app notification for all members or a specific one.
func (s *AutomationService) actionSendNotification(ctx context.Context, rule *models.AutomationRule) (string, error) {
	title, _ := rule.ActionConfig["title"].(string)
	message, _ := rule.ActionConfig["message"].(string)
	recipientID, _ := rule.ActionConfig["recipientId"].(string)

	if title == "" {
		title = rule.Name
	}
	if message == "" {
		message = fmt.Sprintf("Automated notification from rule: %s", rule.Name)
	}

	if recipientID != "" {
		// Targeted notification
		notif := models.Notification{
			RecipientID: recipientID,
			Type:        "automation",
			Title:       title,
			Message:     message,
			IsRead:      false,
			CreatedAt:   time.Now(),
		}
		s.db.Notifications().InsertOne(ctx, notif)
		realtime.PublishTo(recipientID, realtime.EvtNotificationNew, notif)
		return fmt.Sprintf("notification sent to member %s", recipientID), nil
	}

	// Broadcast to all active members
	cursor, err := s.db.Members().Find(ctx, bson.M{"status": "active"})
	if err != nil {
		return "", err
	}
	defer cursor.Close(ctx)

	var members []models.Member
	cursor.All(ctx, &members)

	count := 0
	for _, m := range members {
		notif := models.Notification{
			RecipientID: m.ID.Hex(),
			Type:        "automation",
			Title:       title,
			Message:     message,
			IsRead:      false,
			CreatedAt:   time.Now(),
		}
		s.db.Notifications().InsertOne(ctx, notif)
		realtime.PublishTo(m.ID.Hex(), realtime.EvtNotificationNew, notif)
		count++
	}
	return fmt.Sprintf("notifications sent to %d members", count), nil
}

// actionSendEmail sends an email to all active members (or targeted).
func (s *AutomationService) actionSendEmail(ctx context.Context, rule *models.AutomationRule) (string, error) {
	subject, _ := rule.ActionConfig["subject"].(string)
	body, _ := rule.ActionConfig["body"].(string)
	toEmail, _ := rule.ActionConfig["toEmail"].(string)

	if subject == "" {
		subject = fmt.Sprintf("[Inheritance Choir] %s", rule.Name)
	}
	if body == "" {
		body = fmt.Sprintf("<p>This is an automated message from rule: <strong>%s</strong></p>", rule.Name)
	}

	if toEmail != "" {
		s.mailer.SendGeneric(toEmail, "Member", subject, body)
		return fmt.Sprintf("email sent to %s", toEmail), nil
	}

	// Blast to all active members
	cursor, err := s.db.Members().Find(ctx, bson.M{"status": "active"})
	if err != nil {
		return "", err
	}
	defer cursor.Close(ctx)

	var members []models.Member
	cursor.All(ctx, &members)

	count := 0
	for _, m := range members {
		s.mailer.SendGeneric(m.Email, m.FullName, subject, body)
		count++
	}
	return fmt.Sprintf("emails sent to %d members", count), nil
}

// actionSendBroadcast publishes a real-time broadcast to all connected clients.
func (s *AutomationService) actionSendBroadcast(rule *models.AutomationRule) (string, error) {
	message, _ := rule.ActionConfig["message"].(string)
	if message == "" {
		message = fmt.Sprintf("Automated broadcast: %s", rule.Name)
	}
	realtime.Publish(realtime.EvtBroadcast, map[string]interface{}{
		"source":  "automation",
		"ruleId":  rule.ID.Hex(),
		"message": message,
		"sentAt":  time.Now(),
	})
	return "broadcast sent to all connected clients", nil
}

// actionFlagAbsentMembers creates notifications for members with attendance < threshold.
func (s *AutomationService) actionFlagAbsentMembers(ctx context.Context, rule *models.AutomationRule) (string, error) {
	threshold := 70.0
	if v, ok := rule.ActionConfig["threshold"].(float64); ok {
		threshold = v
	}

	cursor, err := s.db.Members().Find(ctx, bson.M{
		"status":     "active",
		"attendance": bson.M{"$lt": threshold},
	})
	if err != nil {
		return "", err
	}
	defer cursor.Close(ctx)

	var members []models.Member
	cursor.All(ctx, &members)

	count := 0
	for _, m := range members {
		notif := models.Notification{
			RecipientID: m.ID.Hex(),
			Type:        "attendance_warning",
			Title:       "Attendance Warning",
			Message:     fmt.Sprintf("Your attendance rate of %.0f%% is below the %.0f%% target. Please attend upcoming events.", m.Attendance, threshold),
			IsRead:      false,
			CreatedAt:   time.Now(),
		}
		s.db.Notifications().InsertOne(ctx, notif)
		realtime.PublishTo(m.ID.Hex(), realtime.EvtNotificationNew, notif)
		count++
	}
	return fmt.Sprintf("flagged %d members with low attendance", count), nil
}

// actionGenerateReport publishes a stats snapshot to admins.
func (s *AutomationService) actionGenerateReport(ctx context.Context, rule *models.AutomationRule) (string, error) {
	memberCount, _ := s.db.Members().CountDocuments(ctx, bson.M{"status": "active"})
	contribCount, _ := s.db.Contributions().CountDocuments(ctx, bson.M{})

	realtime.PublishToAdmins(realtime.EvtLiveStats, map[string]interface{}{
		"source":          "automation",
		"activeMembers":   memberCount,
		"totalContributions": contribCount,
		"onlineNow":       realtime.OnlineCount(),
		"generatedAt":     time.Now(),
	})
	return fmt.Sprintf("report generated: %d members, %d contributions", memberCount, contribCount), nil
}

// actionBirthdayWishes sends birthday greetings to members born today.
func (s *AutomationService) actionBirthdayWishes(ctx context.Context, rule *models.AutomationRule) (string, error) {
	today := time.Now().Format("--01-02") // --MM-DD suffix match

	cursor, err := s.db.Members().Find(ctx, bson.M{
		"status":      "active",
		"dateOfBirth": bson.M{"$regex": today + "$"},
	})
	if err != nil {
		return "", err
	}
	defer cursor.Close(ctx)

	var members []models.Member
	cursor.All(ctx, &members)

	count := 0
	for _, m := range members {
		// Send email
		s.mailer.SendGeneric(
			m.Email,
			m.FullName,
			"🎂 Happy Birthday from Inheritance Choir!",
			fmt.Sprintf(`<h2>Happy Birthday, %s! 🎉</h2>
			<p>The entire Inheritance Choir family wishes you a wonderful birthday filled with joy and blessings!</p>
			<p style="font-style:italic;">With love, Inheritance Choir</p>`, m.FullName),
		)
		// In-app notification
		notif := models.Notification{
			RecipientID: m.ID.Hex(),
			Type:        "birthday",
			Title:       "🎂 Happy Birthday!",
			Message:     "The choir wishes you a blessed birthday! 🎉",
			IsRead:      false,
			CreatedAt:   time.Now(),
		}
		s.db.Notifications().InsertOne(ctx, notif)
		realtime.PublishTo(m.ID.Hex(), realtime.EvtNotificationNew, notif)
		count++
	}
	return fmt.Sprintf("birthday wishes sent to %d members", count), nil
}
