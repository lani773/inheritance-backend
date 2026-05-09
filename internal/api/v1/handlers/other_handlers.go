package handlers
import "go.mongodb.org/mongo-driver/v2/bson"

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"

	"github.com/inheritance-choir/backend/internal/api/v1/middleware"
	"github.com/inheritance-choir/backend/internal/models"
	"github.com/inheritance-choir/backend/internal/notifications"
	"github.com/inheritance-choir/backend/internal/realtime"
	"github.com/inheritance-choir/backend/internal/repository"
	"github.com/inheritance-choir/backend/internal/services"
)

// ═══════════════════════════════════════════════════════════════
// MESSAGE HANDLER
// ═══════════════════════════════════════════════════════════════

type MessageHandler struct {
	db  *repository.DB
	log *zap.Logger
}

func NewMessageHandler(db *repository.DB, log *zap.Logger) *MessageHandler {
	return &MessageHandler{db: db, log: log}
}

func (h *MessageHandler) List(c *gin.Context) {
	page, pageSize, skip := paginationParams(c)
	ctx := c.Request.Context()
	curr := middleware.CurrentMember(c)
	myID := curr.ID.Hex()
	folder := c.DefaultQuery("folder", "inbox")

	filter := bson.M{"deletedBy": bson.M{"$nin": bson.A{myID}}}
	switch folder {
	case "inbox":
		filter["$or"] = bson.A{
			bson.M{"recipientId": myID},
			bson.M{"isBroadcast": true},
			bson.M{"toVoicePart": curr.VoicePart},
		}
		filter["senderId"] = bson.M{"$ne": myID}
	case "sent":
		filter["senderId"] = myID
	case "broadcasts":
		filter["isBroadcast"] = true
	}
	if v := c.Query("search"); v != "" {
		filter["$and"] = bson.A{bson.M{"$or": bson.A{
			bson.M{"subject": bson.M{"$regex": v, "$options": "i"}},
			bson.M{"body": bson.M{"$regex": v, "$options": "i"}},
		}}}
	}

	total, _ := h.db.Messages().CountDocuments(ctx, filter)
	cursor, _ := h.db.Messages().Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "sentAt", Value: -1}}).
			SetSkip(int64(skip)).SetLimit(int64(pageSize)))
	var msgs []models.Message
	cursor.All(ctx, &msgs)
	cursor.Close(ctx)

	unread := int64(0)
	if folder == "inbox" {
		uFilter := bson.M{
			"$or":       filter["$or"],
			"senderId":  bson.M{"$ne": myID},
			"readBy":    bson.M{"$nin": bson.A{myID}},
			"deletedBy": bson.M{"$nin": bson.A{myID}},
		}
		unread, _ = h.db.Messages().CountDocuments(ctx, uFilter)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true, "data": msgs,
		"meta": gin.H{"total": total, "unreadCount": unread, "page": page, "pageSize": pageSize},
	})
}

func (h *MessageHandler) Send(c *gin.Context) {
	var req struct {
		RecipientID string `json:"recipientId"`
		ToVoicePart string `json:"toVoicePart"`
		IsBroadcast bool   `json:"isBroadcast"`
		Subject     string `json:"subject" binding:"required"`
		Body        string `json:"body"    binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	curr := middleware.CurrentMember(c)
	if !req.IsBroadcast && req.RecipientID == "" && req.ToVoicePart == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Provide recipientId, toVoicePart, or isBroadcast=true"})
		return
	}
	msg := models.Message{
		SenderID: curr.ID.Hex(), RecipientID: req.RecipientID,
		ToVoicePart: req.ToVoicePart, IsBroadcast: req.IsBroadcast,
		Subject: req.Subject, Body: req.Body,
		ReadBy: []string{curr.ID.Hex()}, Reactions: map[string][]string{},
		DeletedBy: []string{}, SentAt: time.Now(),
	}
	res, _ := h.db.Messages().InsertOne(c.Request.Context(), msg)
	msg.ID = res.InsertedID.(bson.ObjectID)
	realtime.Publish(realtime.EvtMessageSent, msg)
	sendCreated(c, gin.H{"id": msg.ID.Hex(), "subject": msg.Subject}, "Message sent")
}

func (h *MessageHandler) GetByID(c *gin.Context) {
	ctx := c.Request.Context()
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	curr := middleware.CurrentMember(c)
	var msg models.Message
	if err := h.db.Messages().FindOne(ctx, bson.M{"_id": oid}).Decode(&msg); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Not found"})
		return
	}
	myID := curr.ID.Hex()
	found := false
	for _, id := range msg.ReadBy {
		if id == myID {
			found = true
			break
		}
	}
	if !found {
		h.db.Messages().UpdateByID(ctx, oid, bson.M{"$addToSet": bson.M{"readBy": myID}})
		msg.ReadBy = append(msg.ReadBy, myID)
	}
	sendSuccess(c, msg, "")
}

func (h *MessageHandler) React(c *gin.Context) {
	var req struct {
		Emoji string `json:"emoji" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	ctx := c.Request.Context()
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	curr := middleware.CurrentMember(c)
	var msg models.Message
	if err := h.db.Messages().FindOne(ctx, bson.M{"_id": oid}).Decode(&msg); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Not found"})
		return
	}
	myID := curr.ID.Hex()
	if msg.Reactions == nil {
		msg.Reactions = map[string][]string{}
	}
	users := msg.Reactions[req.Emoji]
	reacted := false
	newUsers := []string{}
	for _, u := range users {
		if u == myID {
			reacted = true
		} else {
			newUsers = append(newUsers, u)
		}
	}
	if !reacted {
		newUsers = append(newUsers, myID)
	}
	msg.Reactions[req.Emoji] = newUsers
	h.db.Messages().UpdateByID(ctx, oid, bson.M{"$set": bson.M{"reactions": msg.Reactions}})
	realtime.Publish(realtime.EvtChatMessage, gin.H{"id": oid.Hex(), "emoji": req.Emoji, "count": len(newUsers), "reacted": !reacted})
	c.JSON(http.StatusOK, gin.H{"success": true, "emoji": req.Emoji, "count": len(newUsers), "reacted": !reacted})
}

func (h *MessageHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	curr := middleware.CurrentMember(c)
	h.db.Messages().UpdateByID(ctx, oid, bson.M{"$addToSet": bson.M{"deletedBy": curr.ID.Hex()}})
	c.Status(http.StatusNoContent)
}

// ═══════════════════════════════════════════════════════════════
// ANALYTICS HANDLER
// ═══════════════════════════════════════════════════════════════

type AnalyticsHandler struct {
	db  *repository.DB
	log *zap.Logger
}

func NewAnalyticsHandler(db *repository.DB, log *zap.Logger) *AnalyticsHandler {
	return &AnalyticsHandler{db: db, log: log}
}

func (h *AnalyticsHandler) Dashboard(c *gin.Context) {
	ctx := c.Request.Context()
	today := time.Now().Format("2006-01-02")
	now := time.Now()
	thisM := now.Format("2006-01")
	lastM := now.AddDate(0, -1, 0).Format("2006-01")

	totalM, _ := h.db.Members().CountDocuments(ctx, bson.M{})
	activeM, _ := h.db.Members().CountDocuments(ctx, bson.M{"status": "active"})
	pendingM, _ := h.db.Members().CountDocuments(ctx, bson.M{"status": "pending"})

	cr := []struct{ Total, ThisM, LastM float64 }{}
	cur, _ := h.db.Contributions().Aggregate(ctx, mongo.Pipeline{
		{{Key: "$facet", Value: bson.M{
			"total":     mongo.Pipeline{{{Key: "$group", Value: bson.M{"_id": nil, "s": bson.M{"$sum": "$amount"}}}}},
			"thisMonth": mongo.Pipeline{{{Key: "$match", Value: bson.M{"date": bson.M{"$regex": "^" + thisM}}}}, {{Key: "$group", Value: bson.M{"_id": nil, "s": bson.M{"$sum": "$amount"}}}}},
			"lastMonth": mongo.Pipeline{{{Key: "$match", Value: bson.M{"date": bson.M{"$regex": "^" + lastM}}}}, {{Key: "$group", Value: bson.M{"_id": nil, "s": bson.M{"$sum": "$amount"}}}}},
		}}},
	})
	var facet []map[string]interface{}
	cur.All(ctx, &facet)
	cur.Close(ctx)

	upcoming, _ := h.db.Events().CountDocuments(ctx, bson.M{"date": bson.M{"$gte": today}})
	atRisk, _ := h.db.Members().CountDocuments(ctx, bson.M{"status": "active", "attendance": bson.M{"$lt": 70}})

	_ = cr
	sendSuccess(c, gin.H{
		"activeMembers": activeM, "totalMembers": totalM, "pendingMembers": pendingM,
		"upcomingEvents": upcoming, "atRiskCount": atRisk,
		"contributionData": facet,
	}, "")
}

func (h *AnalyticsHandler) Contributions(c *gin.Context) {
	ctx := c.Request.Context()
	months := 12
	cur, _ := h.db.Contributions().Aggregate(ctx, mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":   bson.M{"month": bson.M{"$substr": bson.A{"$date", 0, 7}}, "type": "$type"},
			"total": bson.M{"$sum": "$amount"},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id.month", Value: 1}}}},
		{{Key: "$group", Value: bson.M{
			"_id":       "$_id.month",
			"breakdown": bson.M{"$push": bson.M{"type": "$_id.type", "amount": "$total"}},
			"total":     bson.M{"$sum": "$total"},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
		{{Key: "$limit", Value: months}},
	})
	var trend []map[string]interface{}
	cur.All(ctx, &trend)
	cur.Close(ctx)
	sendSuccess(c, gin.H{"monthlyTrend": trend}, "")
}

func (h *AnalyticsHandler) Attendance(c *gin.Context) {
	ctx := c.Request.Context()
	cur, _ := h.db.Attendances().Aggregate(ctx, mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":      bson.M{"$substr": bson.A{"$date", 0, 7}},
			"total":    bson.M{"$sum": 1},
			"attended": bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$in": bson.A{"$status", bson.A{"present", "late"}}}, 1, 0}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
		{{Key: "$limit", Value: 12}},
	})
	var trend []map[string]interface{}
	cur.All(ctx, &trend)
	cur.Close(ctx)
	sendSuccess(c, gin.H{"monthlyTrend": trend}, "")
}

func (h *AnalyticsHandler) Members(c *gin.Context) {
	ctx := c.Request.Context()
	vpCur, _ := h.db.Members().Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"status": "active"}}},
		{{Key: "$group", Value: bson.M{"_id": "$voicePart", "count": bson.M{"$sum": 1}}}},
	})
	var voiceDist []map[string]interface{}
	vpCur.All(ctx, &voiceDist)
	vpCur.Close(ctx)
	sendSuccess(c, gin.H{"voiceDistribution": voiceDist}, "")
}

func (h *AnalyticsHandler) YoY(c *gin.Context) {
	// unused ctx removed
	year := time.Now().Year()
	thisY := bson.Regex{Pattern: "^" + time.Now().Format("2006")}
	lastY := bson.Regex{Pattern: "^" + time.Date(year-1, 1, 1, 0, 0, 0, 0, time.UTC).Format("2006")}
	_ = thisY
	_ = lastY
	sendSuccess(c, gin.H{"thisYear": year, "lastYear": year - 1}, "")
}

func (h *AnalyticsHandler) AuditLog(c *gin.Context) {
	page, pageSize, skip := paginationParams(c)
	ctx := c.Request.Context()
	filter := bson.M{}
	if v := c.Query("action"); v != "" {
		filter["action"] = bson.M{"$regex": v, "$options": "i"}
	}
	total, _ := h.db.AuditLogs().CountDocuments(ctx, filter)
	cursor, _ := h.db.AuditLogs().Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}}).
			SetSkip(int64(skip)).SetLimit(int64(pageSize)))
	var logs []models.AuditLog
	cursor.All(ctx, &logs)
	cursor.Close(ctx)
	sendPaginated(c, logs, total, page, pageSize)
}

// ═══════════════════════════════════════════════════════════════
// WELFARE / POSTS / SONGS / NOTIFICATIONS / SETTINGS / ADMIN
// ═══════════════════════════════════════════════════════════════

type WelfareHandler struct {
	db  *repository.DB
	log *zap.Logger
}
type PostHandler struct {
	db  *repository.DB
	log *zap.Logger
}
type SongHandler struct {
	db  *repository.DB
	log *zap.Logger
}
type NotificationHandler struct {
	db  *repository.DB
	log *zap.Logger
}
type SettingsHandler struct {
	db  *repository.DB
	log *zap.Logger
}
type AdminHandler struct {
	db     *repository.DB
	svc    *services.AuthService
	mailer *notifications.Mailer
	log    *zap.Logger
}

func NewWelfareHandler(db *repository.DB, log *zap.Logger) *WelfareHandler {
	return &WelfareHandler{db, log}
}
func NewPostHandler(db *repository.DB, log *zap.Logger) *PostHandler { return &PostHandler{db, log} }
func NewSongHandler(db *repository.DB, log *zap.Logger) *SongHandler { return &SongHandler{db, log} }
func NewNotificationHandler(db *repository.DB, log *zap.Logger) *NotificationHandler {
	return &NotificationHandler{db, log}
}
func NewSettingsHandler(db *repository.DB, log *zap.Logger) *SettingsHandler {
	return &SettingsHandler{db, log}
}
func NewAdminHandler(db *repository.DB, svc *services.AuthService, mailer *notifications.Mailer, log *zap.Logger) *AdminHandler {
	return &AdminHandler{db, svc, mailer, log}
}

// ── Welfare ───────────────────────────────────────────────────
func (h *WelfareHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	cursor, _ := h.db.WelfareCases().Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	var cases []models.WelfareCase
	cursor.All(ctx, &cases)
	cursor.Close(ctx)
	sendSuccess(c, cases, "")
}
func (h *WelfareHandler) Create(c *gin.Context) {
	var wc models.WelfareCase
	if !bindJSON(c, &wc) {
		return
	}
	curr := middleware.CurrentMember(c)
	now := time.Now()
	wc.CreatedBy = curr.ID.Hex()
	wc.CreatedAt = now
	wc.UpdatedAt = now
	wc.Status = "open"
	if wc.Timeline == nil {
		wc.Timeline = []interface{}{}
	}
	res, _ := h.db.WelfareCases().InsertOne(c.Request.Context(), wc)
	wc.ID = res.InsertedID.(bson.ObjectID)
	realtime.Publish(realtime.EvtWelfareCreated, wc)
	sendCreated(c, gin.H{"id": wc.ID.Hex()}, "Welfare case created")
}
func (h *WelfareHandler) GetByID(c *gin.Context) {
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	var wc models.WelfareCase
	if err := h.db.WelfareCases().FindOne(c.Request.Context(), bson.M{"_id": oid}).Decode(&wc); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Not found"})
		return
	}
	sendSuccess(c, wc, "")
}
func (h *WelfareHandler) Update(c *gin.Context) {
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	body["updatedAt"] = time.Now()
	h.db.WelfareCases().UpdateByID(c.Request.Context(), oid, bson.M{"$set": body})
	realtime.Publish(realtime.EvtWelfareUpdated, gin.H{"id": oid.Hex()})
	sendSuccess(c, nil, "Updated")
}
func (h *WelfareHandler) AddTimeline(c *gin.Context) {
	var req struct {
		Note string `json:"note" binding:"required"`
		Type string `json:"type"`
	}
	if !bindJSON(c, &req) {
		return
	}
	curr := middleware.CurrentMember(c)
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	entry := map[string]interface{}{"note": req.Note, "type": req.Type, "by": curr.ID.Hex(), "at": time.Now().Format(time.RFC3339)}
	h.db.WelfareCases().UpdateByID(c.Request.Context(), oid, bson.M{"$push": bson.M{"timeline": entry}, "$set": bson.M{"updatedAt": time.Now()}})
	sendSuccess(c, entry, "Timeline entry added")
}

// ── Posts ─────────────────────────────────────────────────────
func (h *PostHandler) List(c *gin.Context) {
	page, pageSize, skip := paginationParams(c)
	ctx := c.Request.Context()
	filter := bson.M{}
	if v := c.Query("category"); v != "" {
		filter["category"] = v
	}
	total, _ := h.db.Posts().CountDocuments(ctx, filter)
	cursor, _ := h.db.Posts().Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "pinned", Value: -1}, {Key: "createdAt", Value: -1}}).
			SetSkip(int64(skip)).SetLimit(int64(pageSize)))
	var posts []models.Post
	cursor.All(ctx, &posts)
	cursor.Close(ctx)
	sendPaginated(c, posts, total, page, pageSize)
}
func (h *PostHandler) Create(c *gin.Context) {
	var p models.Post
	if !bindJSON(c, &p) {
		return
	}
	curr := middleware.CurrentMember(c)
	now := time.Now()
	p.AuthorID = curr.ID.Hex()
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.Likes == nil {
		p.Likes = []string{}
	}
	if p.Comments == nil {
		p.Comments = []interface{}{}
	}
	res, _ := h.db.Posts().InsertOne(c.Request.Context(), p)
	p.ID = res.InsertedID.(bson.ObjectID)
	realtime.Publish(realtime.EvtPostCreated, p)
	sendCreated(c, gin.H{"id": p.ID.Hex()}, "Post created")
}
func (h *PostHandler) GetByID(c *gin.Context) {
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	h.db.Posts().UpdateByID(c.Request.Context(), oid, bson.M{"$inc": bson.M{"viewCount": 1}})
	var p models.Post
	h.db.Posts().FindOne(c.Request.Context(), bson.M{"_id": oid}).Decode(&p)
	sendSuccess(c, p, "")
}
func (h *PostHandler) Update(c *gin.Context) {
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	body["updatedAt"] = time.Now()
	h.db.Posts().UpdateByID(c.Request.Context(), oid, bson.M{"$set": body})
	realtime.Publish(realtime.EvtPostUpdated, gin.H{"id": oid.Hex()})
	sendSuccess(c, nil, "Updated")
}
func (h *PostHandler) Delete(c *gin.Context) {
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	h.db.Posts().DeleteOne(c.Request.Context(), bson.M{"_id": oid})
	realtime.Publish(realtime.EvtPostDeleted, gin.H{"id": oid.Hex()})
	c.Status(http.StatusNoContent)
}
func (h *PostHandler) Like(c *gin.Context) {
	ctx := c.Request.Context()
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	curr := middleware.CurrentMember(c)
	var p models.Post
	h.db.Posts().FindOne(ctx, bson.M{"_id": oid}).Decode(&p)
	myID := curr.ID.Hex()
	liked := false
	newLikes := []string{}
	for _, l := range p.Likes {
		if l == myID {
			liked = true
		} else {
			newLikes = append(newLikes, l)
		}
	}
	if !liked {
		newLikes = append(newLikes, myID)
	}
	h.db.Posts().UpdateByID(ctx, oid, bson.M{"$set": bson.M{"likes": newLikes}})
	realtime.Publish(realtime.EvtPostLiked, gin.H{"id": oid.Hex(), "total": len(newLikes)})
	c.JSON(http.StatusOK, gin.H{"success": true, "liked": !liked, "total": len(newLikes)})
}
func (h *PostHandler) AddComment(c *gin.Context) {
	var req struct {
		Text string `json:"text" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	curr := middleware.CurrentMember(c)
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	comment := map[string]interface{}{
		"authorId": curr.ID.Hex(), "authorName": curr.FullName,
		"text": req.Text, "at": time.Now().Format(time.RFC3339),
	}
	h.db.Posts().UpdateByID(c.Request.Context(), oid, bson.M{"$push": bson.M{"comments": comment}})
	realtime.Publish(realtime.EvtPostComment, gin.H{"id": oid.Hex(), "comment": comment})
	sendCreated(c, comment, "Comment added")
}

// ── Songs ─────────────────────────────────────────────────────
func (h *SongHandler) List(c *gin.Context) {
	page, pageSize, skip := paginationParams(c)
	ctx := c.Request.Context()
	filter := bson.M{}
	if v := c.Query("language"); v != "" {
		filter["language"] = v
	}
	if v := c.Query("genre"); v != "" {
		filter["genre"] = v
	}
	if v := c.Query("difficulty"); v != "" {
		filter["difficulty"] = v
	}
	if v := c.Query("search"); v != "" {
		filter["$text"] = bson.M{"$search": v}
	}
	total, _ := h.db.Songs().CountDocuments(ctx, filter)
	cursor, _ := h.db.Songs().Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "title", Value: 1}}).
			SetSkip(int64(skip)).SetLimit(int64(pageSize)))
	var songs []models.Song
	cursor.All(ctx, &songs)
	cursor.Close(ctx)
	sendPaginated(c, songs, total, page, pageSize)
}
func (h *SongHandler) Create(c *gin.Context) {
	var s models.Song
	if !bindJSON(c, &s) {
		return
	}
	curr := middleware.CurrentMember(c)
	now := time.Now()
	s.AddedBy = curr.ID.Hex()
	s.CreatedAt = now
	s.UpdatedAt = now
	res, _ := h.db.Songs().InsertOne(c.Request.Context(), s)
	s.ID = res.InsertedID.(bson.ObjectID)
	realtime.Publish(realtime.EvtSongCreated, s)
	sendCreated(c, gin.H{"id": s.ID.Hex()}, "Song added")
}
func (h *SongHandler) GetByID(c *gin.Context) {
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	var s models.Song
	h.db.Songs().FindOne(c.Request.Context(), bson.M{"_id": oid}).Decode(&s)
	sendSuccess(c, s, "")
}
func (h *SongHandler) Update(c *gin.Context) {
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	body["updatedAt"] = time.Now()
	h.db.Songs().UpdateByID(c.Request.Context(), oid, bson.M{"$set": body})
	realtime.Publish(realtime.EvtSongUpdated, gin.H{"id": oid.Hex()})
	sendSuccess(c, nil, "Updated")
}
func (h *SongHandler) Delete(c *gin.Context) {
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	h.db.Songs().DeleteOne(c.Request.Context(), bson.M{"_id": oid})
	c.Status(http.StatusNoContent)
	realtime.Publish(realtime.EvtSongDeleted, gin.H{"id": oid.Hex()})
}

// ── Notifications ─────────────────────────────────────────────
func (h *NotificationHandler) List(c *gin.Context) {
	_, pageSize, skip := paginationParams(c)
	ctx := c.Request.Context()
	curr := middleware.CurrentMember(c)
	myID := curr.ID.Hex()
	filter := bson.M{"$or": bson.A{bson.M{"recipientId": myID}, bson.M{"recipientId": ""}}}
	if c.Query("unreadOnly") == "true" {
		filter["isRead"] = false
	}
	total, _ := h.db.Notifications().CountDocuments(ctx, filter)
	unread, _ := h.db.Notifications().CountDocuments(ctx, bson.M{
		"$or": bson.A{bson.M{"recipientId": myID}, bson.M{"recipientId": ""}}, "isRead": false,
	})
	cursor, _ := h.db.Notifications().Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).
			SetSkip(int64(skip)).SetLimit(int64(pageSize)))
	var notifs []models.Notification
	cursor.All(ctx, &notifs)
	cursor.Close(ctx)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": notifs, "meta": gin.H{"total": total, "unreadCount": unread}})
}
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	h.db.Notifications().UpdateByID(c.Request.Context(), oid, bson.M{"$set": bson.M{"isRead": true}})
	realtime.Publish(realtime.EvtNotificationNew, gin.H{"id": oid.Hex()})
	sendSuccess(c, nil, "Marked as read")
}
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	ctx := c.Request.Context()
	curr := middleware.CurrentMember(c)
	myID := curr.ID.Hex()
	res, _ := h.db.Notifications().UpdateMany(ctx,
		bson.M{"$or": bson.A{bson.M{"recipientId": myID}, bson.M{"recipientId": ""}}, "isRead": false},
		bson.M{"$set": bson.M{"isRead": true}})
	c.JSON(http.StatusOK, gin.H{"success": true, "updated": res.ModifiedCount})
}
func (h *NotificationHandler) Delete(c *gin.Context) {
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	h.db.Notifications().DeleteOne(c.Request.Context(), bson.M{"_id": oid})
	c.Status(http.StatusNoContent)
}

// ── Settings ──────────────────────────────────────────────────
func (h *SettingsHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	var s models.AppSettings
	err := h.db.AppSettings().FindOne(ctx, bson.M{"key": "global"}).Decode(&s)
	if err != nil {
		s = models.AppSettings{Key: "global", ChoirName: "INHERITANCE CHOIR", Currency: "RWF"}
		res, _ := h.db.AppSettings().InsertOne(ctx, s)
		s.ID = res.InsertedID.(bson.ObjectID)
	}
	sendSuccess(c, s, "")
}
func (h *SettingsHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	body["updatedAt"] = time.Now()
	h.db.AppSettings().UpdateOne(ctx, bson.M{"key": "global"}, bson.M{"$set": body}, options.UpdateOne().SetUpsert(true))
	realtime.Publish(realtime.EvtSystemAlert, body)
	sendSuccess(c, nil, "Settings updated")
}

// ── Admin ─────────────────────────────────────────────────────
func (h *AdminHandler) ListRegistrations(c *gin.Context) {
	page, pageSize, skip := paginationParams(c)
	ctx := c.Request.Context()
	total, _ := h.db.Members().CountDocuments(ctx, bson.M{"status": "pending"})
	cursor, _ := h.db.Members().Find(ctx, bson.M{"status": "pending"},
		options.Find().SetProjection(bson.M{"passwordHash": 0}).
			SetSort(bson.D{{Key: "createdAt", Value: -1}}).
			SetSkip(int64(skip)).SetLimit(int64(pageSize)))
	var members []models.Member
	cursor.All(ctx, &members)
	cursor.Close(ctx)
	sendPaginated(c, members, total, page, pageSize)
}
func (h *AdminHandler) ApproveRegistration(c *gin.Context) {
	ctx := c.Request.Context()
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	now := time.Now()
	res, _ := h.db.Members().UpdateByID(ctx, oid, bson.M{"$set": bson.M{"status": "active", "updatedAt": now}})
	if res.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Not found"})
		return
	}
	h.db.Notifications().InsertOne(ctx, models.Notification{
		RecipientID: oid.Hex(), Type: "success",
		Title: "Account Approved! 🎉", Message: "Your account is now active.",
		ActionURL: "/dashboard", CreatedAt: now,
	})

	var m models.Member
	h.db.Members().FindOne(ctx, bson.M{"_id": oid}).Decode(&m)
	go h.mailer.SendApprovalEmail(m.Email, m.FullName)

	realtime.Publish(realtime.EvtMemberApproved, m)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Member approved"})
}
func (h *AdminHandler) RejectRegistration(c *gin.Context) {
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	h.db.Members().DeleteOne(c.Request.Context(), bson.M{"_id": oid})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Registration rejected"})
}
func (h *AdminHandler) SystemStats(c *gin.Context) {
	ctx := c.Request.Context()
	colls := []string{"members", "events", "contributions", "attendances", "messages", "posts", "songs", "notifications", "auditlogs"}
	stats := gin.H{}
	for _, col := range colls {
		n, _ := h.db.DB.Collection(col).CountDocuments(ctx, bson.M{})
		stats[col] = n
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"collections": stats, "serverTime": time.Now().Format(time.RFC3339)}})
}
func (h *AdminHandler) Broadcast(c *gin.Context) {
	var req struct {
		Subject     string `json:"subject"     binding:"required"`
		Body        string `json:"body"        binding:"required"`
		ToVoicePart string `json:"toVoicePart"`
	}
	if !bindJSON(c, &req) {
		return
	}
	curr := middleware.CurrentMember(c)
	ctx := c.Request.Context()
	msg := models.Message{
		SenderID: curr.ID.Hex(), IsBroadcast: true, ToVoicePart: req.ToVoicePart,
		Subject: req.Subject, Body: req.Body, ReadBy: []string{curr.ID.Hex()},
		Reactions: map[string][]string{}, DeletedBy: []string{}, SentAt: time.Now(),
	}
	res, _ := h.db.Messages().InsertOne(ctx, msg)
	msg.ID = res.InsertedID.(bson.ObjectID)
	realtime.Publish(realtime.EvtMessageSent, msg)

	filter := bson.M{"status": "active"}
	if req.ToVoicePart != "" {
		filter["voicePart"] = req.ToVoicePart
	}
	cursor, _ := h.db.Members().Find(ctx, filter, options.Find().SetProjection(bson.M{"_id": 1}))
	var members []models.Member
	cursor.All(ctx, &members)
	cursor.Close(ctx)

	now := time.Now()
	for _, m := range members {
		if m.ID.Hex() == curr.ID.Hex() {
			continue
		}
		h.db.Notifications().InsertOne(ctx, models.Notification{
			RecipientID: m.ID.Hex(), Type: "info",
			Title: "📢 " + req.Subject, Message: req.Body,
			ActionURL: "/dashboard/messages", CreatedAt: now,
		})
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "messageId": msg.ID.Hex(), "recipientCount": len(members)})
}
func (h *AdminHandler) RevokeSessions(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	h.db.RefreshTokens().DeleteMany(ctx, bson.M{"memberId": id})
	oid, _ := bson.ObjectIDFromHex(id)
	h.db.Members().UpdateByID(ctx, oid, bson.M{"$set": bson.M{"online": false}})
	c.Status(http.StatusNoContent)
}
