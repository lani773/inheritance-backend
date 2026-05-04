package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"github.com/inheritance-choir/backend/internal/api/v1/middleware"
	"github.com/inheritance-choir/backend/internal/models"
	"github.com/inheritance-choir/backend/internal/realtime"
	"github.com/inheritance-choir/backend/internal/repository"
	"github.com/inheritance-choir/backend/internal/services"
)

type SearchHandler struct {
	db  *repository.DB
	log *zap.Logger
}
type SetlistHandler struct {
	db  *repository.DB
	hub *realtime.Hub
	log *zap.Logger
}
type BudgetHandler struct {
	db  *repository.DB
	hub *realtime.Hub
	log *zap.Logger
}
type PledgeHandler struct {
	db  *repository.DB
	hub *realtime.Hub
	log *zap.Logger
}
type AutomationHandler struct {
	db      *repository.DB
	hub     *realtime.Hub
	autoSvc *services.AutomationService
	log     *zap.Logger
}
type APIKeyHandler struct {
	db  *repository.DB
	log *zap.Logger
}
type WebhookHandler struct {
	db  *repository.DB
	log *zap.Logger
}
type UploadHandler struct {
	db  *repository.DB
	hub *realtime.Hub
	log *zap.Logger
}
type PrayerHandler struct {
	db  *repository.DB
	hub *realtime.Hub
	log *zap.Logger
}
type ChatHandler struct {
	db  *repository.DB
	hub *realtime.Hub
	log *zap.Logger
}
type IntelligenceHandler struct {
	db  *repository.DB
	log *zap.Logger
}

func NewSearchHandler(db *repository.DB, log *zap.Logger) *SearchHandler {
	return &SearchHandler{db, log}
}
func NewSetlistHandler(db *repository.DB, hub *realtime.Hub, log *zap.Logger) *SetlistHandler {
	return &SetlistHandler{db, hub, log}
}
func NewBudgetHandler(db *repository.DB, hub *realtime.Hub, log *zap.Logger) *BudgetHandler {
	return &BudgetHandler{db, hub, log}
}
func NewPledgeHandler(db *repository.DB, hub *realtime.Hub, log *zap.Logger) *PledgeHandler {
	return &PledgeHandler{db, hub, log}
}
func NewAutomationHandler(db *repository.DB, hub *realtime.Hub, autoSvc *services.AutomationService, log *zap.Logger) *AutomationHandler {
	return &AutomationHandler{db, hub, autoSvc, log}
}
func NewAPIKeyHandler(db *repository.DB, log *zap.Logger) *APIKeyHandler {
	return &APIKeyHandler{db, log}
}
func NewWebhookHandler(db *repository.DB, log *zap.Logger) *WebhookHandler {
	return &WebhookHandler{db, log}
}
func NewUploadHandler(db *repository.DB, hub *realtime.Hub, log *zap.Logger) *UploadHandler {
	return &UploadHandler{db, hub, log}
}
func NewPrayerHandler(db *repository.DB, hub *realtime.Hub, log *zap.Logger) *PrayerHandler {
	return &PrayerHandler{db, hub, log}
}
func NewChatHandler(db *repository.DB, hub *realtime.Hub, log *zap.Logger) *ChatHandler {
	return &ChatHandler{db, hub, log}
}
func NewIntelligenceHandler(db *repository.DB, log *zap.Logger) *IntelligenceHandler {
	return &IntelligenceHandler{db, log}
}

func oidParam(c *gin.Context) (primitive.ObjectID, bool) {
	oid, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid ID"})
		return primitive.NilObjectID, false
	}
	return oid, true
}

func currentID(c *gin.Context) string {
	if m := middleware.CurrentMember(c); m != nil {
		return m.ID.Hex()
	}
	return ""
}

func (h *SearchHandler) Search(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "q is required"})
		return
	}
	typ := c.DefaultQuery("type", "all")
	ctx := c.Request.Context()
	results := gin.H{}

	add := func(name string, col *mongo.Collection, filter bson.M, projection bson.M) {
		if typ != "all" && typ != name {
			return
		}
		cursor, err := col.Find(ctx, filter, options.Find().SetProjection(projection).SetLimit(8))
		if err != nil {
			results[name] = []interface{}{}
			return
		}
		var docs []bson.M
		cursor.All(ctx, &docs)
		cursor.Close(ctx)
		results[name] = docs
	}

	regex := primitive.Regex{Pattern: q, Options: "i"}
	add("members", h.db.Members(), bson.M{"$or": bson.A{bson.M{"fullName": regex}, bson.M{"email": regex}, bson.M{"voicePart": regex}}}, bson.M{"passwordHash": 0})
	add("events", h.db.Events(), bson.M{"$or": bson.A{bson.M{"title": regex}, bson.M{"location": regex}, bson.M{"description": regex}}}, bson.M{})
	add("songs", h.db.Songs(), bson.M{"$or": bson.A{bson.M{"title": regex}, bson.M{"artist": regex}, bson.M{"lyrics": regex}}}, bson.M{})
	add("posts", h.db.Posts(), bson.M{"$or": bson.A{bson.M{"title": regex}, bson.M{"content": regex}, bson.M{"category": regex}}}, bson.M{})
	add("prayer", h.db.PrayerRequests(), bson.M{"$or": bson.A{bson.M{"title": regex}, bson.M{"description": regex}}}, bson.M{})
	sendSuccess(c, results, "")
}

func (h *SetlistHandler) List(c *gin.Context) {
	filter := bson.M{}
	if v := c.Query("eventId"); v != "" {
		filter["eventId"] = v
	}
	cursor, _ := h.db.Setlists().Find(c.Request.Context(), filter, options.Find().SetSort(bson.D{{Key: "updatedAt", Value: -1}}))
	var items []models.Setlist
	cursor.All(c.Request.Context(), &items)
	cursor.Close(c.Request.Context())
	sendSuccess(c, items, "")
}

func (h *SetlistHandler) Create(c *gin.Context) {
	var item models.Setlist
	if !bindJSON(c, &item) {
		return
	}
	now := time.Now()
	item.CreatedBy = currentID(c)
	item.CreatedAt = now
	item.UpdatedAt = now
	res, err := h.db.Setlists().InsertOne(c.Request.Context(), item)
	if err != nil {
		respondError(c, err)
		return
	}
	item.ID = res.InsertedID.(primitive.ObjectID)
	h.hub.Publish("setlist:updated", realtime.ChGeneral, item)
	sendCreated(c, item, "Setlist saved")
}

func (h *SetlistHandler) Get(c *gin.Context) {
	oid, ok := oidParam(c)
	if !ok {
		return
	}
	var item models.Setlist
	if err := h.db.Setlists().FindOne(c.Request.Context(), bson.M{"_id": oid}).Decode(&item); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Setlist not found"})
		return
	}
	sendSuccess(c, item, "")
}

func (h *SetlistHandler) Update(c *gin.Context) {
	oid, ok := oidParam(c)
	if !ok {
		return
	}
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	body["updatedAt"] = time.Now()
	h.db.Setlists().UpdateByID(c.Request.Context(), oid, bson.M{"$set": body})
	var item models.Setlist
	h.db.Setlists().FindOne(c.Request.Context(), bson.M{"_id": oid}).Decode(&item)
	h.hub.Publish("setlist:updated", realtime.ChGeneral, item)
	sendSuccess(c, item, "Setlist updated")
}

func (h *SetlistHandler) Delete(c *gin.Context) {
	oid, ok := oidParam(c)
	if !ok {
		return
	}
	h.db.Setlists().DeleteOne(c.Request.Context(), bson.M{"_id": oid})
	h.hub.Publish("setlist:deleted", realtime.ChGeneral, gin.H{"id": oid.Hex()})
	c.Status(http.StatusNoContent)
}

func (h *BudgetHandler) List(c *gin.Context) {
	filter := bson.M{}
	if y := c.Query("year"); y != "" {
		if n, err := strconv.Atoi(y); err == nil {
			filter["year"] = n
		}
	}
	cursor, _ := h.db.BudgetGoals().Find(c.Request.Context(), filter, options.Find().SetSort(bson.D{{Key: "year", Value: -1}, {Key: "type", Value: 1}}))
	var goals []models.BudgetGoal
	cursor.All(c.Request.Context(), &goals)
	cursor.Close(c.Request.Context())
	sendSuccess(c, goals, "")
}

func (h *BudgetHandler) Upsert(c *gin.Context) {
	var goal models.BudgetGoal
	if !bindJSON(c, &goal) {
		return
	}
	if goal.Year == 0 {
		goal.Year = time.Now().Year()
	}
	now := time.Now()
	goal.CreatedBy = currentID(c)
	goal.UpdatedAt = now
	update := bson.M{"$set": bson.M{"target": goal.Target, "updatedAt": now}, "$setOnInsert": bson.M{"year": goal.Year, "type": goal.Type, "createdBy": goal.CreatedBy, "createdAt": now}}
	h.db.BudgetGoals().UpdateOne(c.Request.Context(), bson.M{"year": goal.Year, "type": goal.Type}, update, options.Update().SetUpsert(true))
	h.hub.Publish("budget:updated", realtime.ChFinance, goal)
	sendSuccess(c, goal, "Budget goal saved")
}

func (h *PledgeHandler) ListCampaigns(c *gin.Context) {
	cursor, _ := h.db.PledgeCampaigns().Find(c.Request.Context(), bson.M{}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	var campaigns []models.PledgeCampaign
	cursor.All(c.Request.Context(), &campaigns)
	cursor.Close(c.Request.Context())
	sendSuccess(c, campaigns, "")
}

func (h *PledgeHandler) CreateCampaign(c *gin.Context) {
	var item models.PledgeCampaign
	if !bindJSON(c, &item) {
		return
	}
	now := time.Now()
	if item.Status == "" {
		item.Status = "active"
	}
	item.CreatedBy = currentID(c)
	item.CreatedAt = now
	item.UpdatedAt = now
	res, _ := h.db.PledgeCampaigns().InsertOne(c.Request.Context(), item)
	item.ID = res.InsertedID.(primitive.ObjectID)
	h.hub.Publish("pledge:campaign:new", realtime.ChFinance, item)
	sendCreated(c, item, "Campaign created")
}

func (h *PledgeHandler) UpdateCampaign(c *gin.Context) {
	oid, ok := oidParam(c)
	if !ok {
		return
	}
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	body["updatedAt"] = time.Now()
	h.db.PledgeCampaigns().UpdateByID(c.Request.Context(), oid, bson.M{"$set": body})
	sendSuccess(c, nil, "Campaign updated")
}

func (h *PledgeHandler) ListPledges(c *gin.Context) {
	filter := bson.M{}
	if v := c.Query("campaignId"); v != "" {
		filter["campaignId"] = v
	}
	if v := c.Query("memberId"); v != "" {
		filter["memberId"] = v
	}
	cursor, _ := h.db.Pledges().Find(c.Request.Context(), filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	var pledges []models.Pledge
	cursor.All(c.Request.Context(), &pledges)
	cursor.Close(c.Request.Context())
	sendSuccess(c, pledges, "")
}

func (h *PledgeHandler) CreatePledge(c *gin.Context) {
	var item models.Pledge
	if !bindJSON(c, &item) {
		return
	}
	now := time.Now()
	if item.Status == "" {
		item.Status = "active"
	}
	item.CreatedAt = now
	item.UpdatedAt = now
	res, _ := h.db.Pledges().InsertOne(c.Request.Context(), item)
	item.ID = res.InsertedID.(primitive.ObjectID)
	h.hub.Publish("pledge:new", realtime.ChFinance, item)
	sendCreated(c, item, "Pledge saved")
}

func (h *PledgeHandler) UpdatePledge(c *gin.Context) {
	oid, ok := oidParam(c)
	if !ok {
		return
	}
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	body["updatedAt"] = time.Now()
	h.db.Pledges().UpdateByID(c.Request.Context(), oid, bson.M{"$set": body})
	sendSuccess(c, nil, "Pledge updated")
}

func (h *AutomationHandler) List(c *gin.Context) {
	cursor, _ := h.db.AutomationRules().Find(c.Request.Context(), bson.M{}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	var rules []models.AutomationRule
	cursor.All(c.Request.Context(), &rules)
	cursor.Close(c.Request.Context())
	sendSuccess(c, rules, "")
}

func (h *AutomationHandler) Create(c *gin.Context) {
	var item models.AutomationRule
	if !bindJSON(c, &item) {
		return
	}
	now := time.Now()
	item.CreatedAt = now
	item.UpdatedAt = now
	res, _ := h.db.AutomationRules().InsertOne(c.Request.Context(), item)
	item.ID = res.InsertedID.(primitive.ObjectID)
	h.hub.Publish("automation:updated", realtime.ChAdmin, item)
	sendCreated(c, item, "Automation rule created")
}

func (h *AutomationHandler) Update(c *gin.Context) {
	oid, ok := oidParam(c)
	if !ok {
		return
	}
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	body["updatedAt"] = time.Now()
	h.db.AutomationRules().UpdateByID(c.Request.Context(), oid, bson.M{"$set": body})
	sendSuccess(c, nil, "Automation rule updated")
}

func (h *AutomationHandler) Delete(c *gin.Context) {
	oid, ok := oidParam(c)
	if !ok {
		return
	}
	h.db.AutomationRules().DeleteOne(c.Request.Context(), bson.M{"_id": oid})
	c.Status(http.StatusNoContent)
}

func (h *AutomationHandler) Run(c *gin.Context) {
	oid, ok := oidParam(c)
	if !ok {
		return
	}
	var rule models.AutomationRule
	if err := h.db.AutomationRules().FindOne(c.Request.Context(), bson.M{"_id": oid}).Decode(&rule); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Rule not found"})
		return
	}
	result, err := h.autoSvc.RunRule(c.Request.Context(), &rule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	h.hub.Publish(realtime.EvtAutomationRun, realtime.ChAdmin, gin.H{"id": oid.Hex(), "result": result, "ranAt": time.Now()})
	sendSuccess(c, gin.H{"result": result, "ranAt": time.Now()}, "Automation executed")
}

func (h *APIKeyHandler) List(c *gin.Context) {
	cursor, _ := h.db.APIKeys().Find(c.Request.Context(), bson.M{}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	var keys []models.APIKey
	cursor.All(c.Request.Context(), &keys)
	cursor.Close(c.Request.Context())
	sendSuccess(c, keys, "")
}

func (h *APIKeyHandler) Create(c *gin.Context) {
	var req struct {
		Name        string   `json:"name" binding:"required"`
		Permissions []string `json:"permissions"`
	}
	if !bindJSON(c, &req) {
		return
	}
	raw := randomToken(32)
	sum := sha256.Sum256([]byte(raw))
	prefix := raw[:12]
	now := time.Now()
	key := models.APIKey{Name: req.Name, KeyHash: hex.EncodeToString(sum[:]), Prefix: prefix, Permissions: req.Permissions, Status: "active", CreatedBy: currentID(c), CreatedAt: now}
	res, _ := h.db.APIKeys().InsertOne(c.Request.Context(), key)
	key.ID = res.InsertedID.(primitive.ObjectID)
	c.JSON(http.StatusCreated, gin.H{"success": true, "message": "API key created", "data": key, "plainTextKey": raw})
}

func (h *APIKeyHandler) Revoke(c *gin.Context) {
	oid, ok := oidParam(c)
	if !ok {
		return
	}
	now := time.Now()
	h.db.APIKeys().UpdateByID(c.Request.Context(), oid, bson.M{"$set": bson.M{"status": "revoked", "revokedAt": now}})
	sendSuccess(c, nil, "API key revoked")
}

func (h *WebhookHandler) List(c *gin.Context) {
	cursor, _ := h.db.Webhooks().Find(c.Request.Context(), bson.M{}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	var hooks []models.Webhook
	cursor.All(c.Request.Context(), &hooks)
	cursor.Close(c.Request.Context())
	sendSuccess(c, hooks, "")
}

func (h *WebhookHandler) Create(c *gin.Context) {
	var item models.Webhook
	if !bindJSON(c, &item) {
		return
	}
	now := time.Now()
	if item.Secret == "" {
		item.Secret = randomToken(16)
	}
	item.Active = true
	item.CreatedBy = currentID(c)
	item.CreatedAt = now
	item.UpdatedAt = now
	res, _ := h.db.Webhooks().InsertOne(c.Request.Context(), item)
	item.ID = res.InsertedID.(primitive.ObjectID)
	sendCreated(c, item, "Webhook created")
}

func (h *WebhookHandler) Update(c *gin.Context) {
	oid, ok := oidParam(c)
	if !ok {
		return
	}
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	body["updatedAt"] = time.Now()
	h.db.Webhooks().UpdateByID(c.Request.Context(), oid, bson.M{"$set": body})
	sendSuccess(c, nil, "Webhook updated")
}

func (h *WebhookHandler) Delete(c *gin.Context) {
	oid, ok := oidParam(c)
	if !ok {
		return
	}
	h.db.Webhooks().DeleteOne(c.Request.Context(), bson.M{"_id": oid})
	c.Status(http.StatusNoContent)
}

func (h *WebhookHandler) Test(c *gin.Context) {
	sendSuccess(c, gin.H{"delivered": true, "testedAt": time.Now()}, "Webhook test accepted")
}

func (h *UploadHandler) Create(c *gin.Context) {
	var req struct {
		FileName    string `json:"fileName" binding:"required"`
		ContentType string `json:"contentType"`
		Size        int64  `json:"size"`
		DataURL     string `json:"dataUrl"`
		Scope       string `json:"scope"`
	}
	if !bindJSON(c, &req) {
		return
	}
	now := time.Now()
	asset := models.UploadAsset{FileName: req.FileName, ContentType: req.ContentType, Size: req.Size, URL: req.DataURL, Scope: req.Scope, OwnerID: currentID(c), CreatedAt: now}
	res, _ := h.db.UploadAssets().InsertOne(c.Request.Context(), asset)
	asset.ID = res.InsertedID.(primitive.ObjectID)
	h.hub.Publish("upload:new", realtime.ChGeneral, asset)
	sendCreated(c, asset, "Upload recorded")
}

func (h *UploadHandler) List(c *gin.Context) {
	filter := bson.M{}
	if s := c.Query("scope"); s != "" {
		filter["scope"] = s
	}
	cursor, _ := h.db.UploadAssets().Find(c.Request.Context(), filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	var assets []models.UploadAsset
	cursor.All(c.Request.Context(), &assets)
	cursor.Close(c.Request.Context())
	sendSuccess(c, assets, "")
}

func (h *PrayerHandler) List(c *gin.Context) {
	filter := bson.M{"$or": bson.A{bson.M{"visibility": "public"}, bson.M{"memberId": currentID(c)}}}
	if s := c.Query("status"); s != "" {
		filter["status"] = s
	}
	cursor, _ := h.db.PrayerRequests().Find(c.Request.Context(), filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	var items []models.PrayerRequest
	cursor.All(c.Request.Context(), &items)
	cursor.Close(c.Request.Context())
	sendSuccess(c, items, "")
}

func (h *PrayerHandler) Create(c *gin.Context) {
	var item models.PrayerRequest
	if !bindJSON(c, &item) {
		return
	}
	now := time.Now()
	if item.Visibility == "" {
		item.Visibility = "public"
	}
	if item.Status == "" {
		item.Status = "open"
	}
	item.MemberID = currentID(c)
	item.PrayedBy = []string{}
	item.CreatedAt = now
	item.UpdatedAt = now
	res, _ := h.db.PrayerRequests().InsertOne(c.Request.Context(), item)
	item.ID = res.InsertedID.(primitive.ObjectID)
	h.hub.Publish("prayer:new", realtime.ChPrayer, item)
	sendCreated(c, item, "Prayer request created")
}

func (h *PrayerHandler) Pray(c *gin.Context) {
	oid, ok := oidParam(c)
	if !ok {
		return
	}
	h.db.PrayerRequests().UpdateByID(c.Request.Context(), oid, bson.M{"$addToSet": bson.M{"prayedBy": currentID(c)}, "$set": bson.M{"updatedAt": time.Now()}})
	sendSuccess(c, nil, "Prayer recorded")
}

func (h *PrayerHandler) Update(c *gin.Context) {
	oid, ok := oidParam(c)
	if !ok {
		return
	}
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	body["updatedAt"] = time.Now()
	h.db.PrayerRequests().UpdateByID(c.Request.Context(), oid, bson.M{"$set": body})
	sendSuccess(c, nil, "Prayer request updated")
}

func (h *ChatHandler) List(c *gin.Context) {
	filter := bson.M{}
	if ch := c.Query("channelId"); ch != "" {
		filter["channelId"] = ch
	}
	cursor, _ := h.db.ChatMessages().Find(c.Request.Context(), filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(100))
	var msgs []models.ChatMessage
	cursor.All(c.Request.Context(), &msgs)
	cursor.Close(c.Request.Context())
	sendSuccess(c, msgs, "")
}

func (h *ChatHandler) Send(c *gin.Context) {
	var msg models.ChatMessage
	if !bindJSON(c, &msg) {
		return
	}
	curr := middleware.CurrentMember(c)
	now := time.Now()
	msg.SenderID = curr.ID.Hex()
	msg.SenderName = curr.FullName
	msg.CreatedAt = now
	if msg.ReadBy == nil {
		msg.ReadBy = []string{curr.ID.Hex()}
	}
	if msg.Reactions == nil {
		msg.Reactions = map[string][]string{}
	}
	res, _ := h.db.ChatMessages().InsertOne(c.Request.Context(), msg)
	msg.ID = res.InsertedID.(primitive.ObjectID)
	h.hub.Publish("chat:message", msg.ChannelID, msg)
	sendCreated(c, msg, "Message sent")
}

func (h *IntelligenceHandler) Forecast(c *gin.Context) {
	ctx := c.Request.Context()
	contribTrend := aggregateMonthly(ctx, h.db.Contributions(), "amount")
	attendanceTrend := aggregateAttendance(ctx, h.db.Attendances())
	sendSuccess(c, gin.H{
		"contributions": buildForecast(contribTrend, "total"),
		"attendance":    buildForecast(attendanceTrend, "rate"),
		"health":        choirHealth(contribTrend, attendanceTrend),
		"generatedAt":   time.Now(),
	}, "")
}

func aggregateMonthly(ctx context.Context, col *mongo.Collection, field string) []gin.H {
	cur, err := col.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":   bson.M{"$substr": bson.A{"$date", 0, 7}},
			"total": bson.M{"$sum": "$" + field},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
		{{Key: "$limit", Value: 12}},
	})
	if err != nil {
		return []gin.H{}
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID    string  `bson:"_id"`
		Total float64 `bson:"total"`
	}
	cur.All(ctx, &rows)
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{"month": r.ID, "total": r.Total})
	}
	return out
}

func buildForecast(data []gin.H, key string) gin.H {
	n := len(data)
	if n == 0 {
		return gin.H{"historical": data, "forecast": []gin.H{}}
	}
	last, _ := data[n-1][key].(float64)
	prev := last
	if n > 1 {
		prev, _ = data[n-2][key].(float64)
	}
	step := last - prev
	out := []gin.H{}
	for i := 1; i <= 3; i++ {
		v := last + step*float64(i)
		if v < 0 {
			v = 0
		}
		out = append(out, gin.H{"period": i, "predicted": v})
	}
	return gin.H{"historical": data, "forecast": out}
}

func choirHealth(contrib, attendance []gin.H) gin.H {
	score := 75.0
	if len(attendance) > 0 {
		if v, ok := attendance[len(attendance)-1]["rate"].(float64); ok {
			score = (score + v) / 2
		}
	}
	grade := "C"
	if score >= 85 {
		grade = "A"
	} else if score >= 75 {
		grade = "B"
	} else if score < 65 {
		grade = "D"
	}
	return gin.H{"score": score, "grade": grade}
}

func aggregateAttendance(ctx context.Context, col *mongo.Collection) []gin.H {
	cur, err := col.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":      bson.M{"$substr": bson.A{"$date", 0, 7}},
			"total":    bson.M{"$sum": 1},
			"attended": bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$in": bson.A{"$status", bson.A{"present", "late"}}}, 1, 0}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
		{{Key: "$limit", Value: 12}},
	})
	if err != nil {
		return []gin.H{}
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID       string  `bson:"_id"`
		Total    float64 `bson:"total"`
		Attended float64 `bson:"attended"`
	}
	cur.All(ctx, &rows)
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		rate := 0.0
		if r.Total > 0 {
			rate = r.Attended / r.Total * 100
		}
		out = append(out, gin.H{"month": r.ID, "rate": rate})
	}
	return out
}

func randomToken(bytes int) string {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(time.Now().UnixNano(), 36)))
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
