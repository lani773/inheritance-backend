package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"github.com/inheritance-choir/backend/internal/api/v1/middleware"
	"github.com/inheritance-choir/backend/internal/models"
	"github.com/inheritance-choir/backend/internal/notifications"
	"github.com/inheritance-choir/backend/internal/realtime"
	"github.com/inheritance-choir/backend/internal/repository"
	"github.com/inheritance-choir/backend/internal/utils"
	"github.com/inheritance-choir/backend/pkg/crypto"
)

// ═══════════════════════════════════════════════════════════════
// MEMBER HANDLER
// ═══════════════════════════════════════════════════════════════

type MemberHandler struct {
	db     *repository.DB
	mailer *notifications.Mailer
	log    *zap.Logger
}

func NewMemberHandler(db *repository.DB, mailer *notifications.Mailer, log *zap.Logger) *MemberHandler {
	return &MemberHandler{db: db, mailer: mailer, log: log}
}

// GET /members
func (h *MemberHandler) List(c *gin.Context) {
	page, pageSize, skip := paginationParams(c)
	ctx := c.Request.Context()

	filter := bson.M{}
	if v := c.Query("voicePart"); v != "" {
		filter["voicePart"] = v
	}
	if v := c.Query("status"); v != "" {
		filter["status"] = v
	}
	if v := c.Query("role"); v != "" {
		filter["role"] = v
	}
	if v := c.Query("search"); v != "" {
		filter["$text"] = bson.M{"$search": v}
	}

	sortField := c.DefaultQuery("sortBy", "fullName")
	sortDir := 1
	if c.Query("sortDir") == "desc" {
		sortDir = -1
	}

	opts := options.Find().
		SetSort(bson.D{{Key: sortField, Value: sortDir}}).
		SetSkip(int64(skip)).
		SetLimit(int64(pageSize)).
		SetProjection(bson.M{"passwordHash": 0})

	total, _ := h.db.Members().CountDocuments(ctx, filter)
	cursor, err := h.db.Members().Find(ctx, filter, opts)
	if err != nil {
		respondError(c, err)
		return
	}
	defer cursor.Close(ctx)

	var members []models.Member
	cursor.All(ctx, &members)
	sendPaginated(c, members, total, page, pageSize)
}

// GET /members/stats
func (h *MemberHandler) Stats(c *gin.Context) {
	ctx := c.Request.Context()
	total, _ := h.db.Members().CountDocuments(ctx, bson.M{})
	active, _ := h.db.Members().CountDocuments(ctx, bson.M{"status": "active"})
	pending, _ := h.db.Members().CountDocuments(ctx, bson.M{"status": "pending"})
	inactive, _ := h.db.Members().CountDocuments(ctx, bson.M{"status": "inactive"})

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"status": "active"}}},
		{{Key: "$group", Value: bson.M{"_id": "$voicePart", "count": bson.M{"$sum": 1}}}},
	}
	cursor, _ := h.db.Members().Aggregate(ctx, pipeline)
	var vpRaw []struct {
		ID    string `bson:"_id"`
		Count int    `bson:"count"`
	}
	cursor.All(ctx, &vpRaw)

	byVoice := make(map[string]int)
	for _, v := range vpRaw {
		byVoice[v.ID] = v.Count
	}

	// Avg attendance
	attCursor, _ := h.db.Members().Find(ctx, bson.M{"status": "active"},
		options.Find().SetProjection(bson.M{"attendance": 1}))
	var attList []struct {
		Attendance float64 `bson:"attendance"`
	}
	attCursor.All(ctx, &attList)
	var total_att float64
	atRisk := 0
	for _, m := range attList {
		total_att += m.Attendance
		if m.Attendance < 70 {
			atRisk++
		}
	}
	avgAtt := 0.0
	if len(attList) > 0 {
		avgAtt = total_att / float64(len(attList))
	}

	sendSuccess(c, gin.H{
		"total": total, "active": active, "pending": pending, "inactive": inactive,
		"byVoicePart": byVoice, "avgAttendance": avgAtt, "atRiskCount": atRisk,
	}, "")
}

// POST /members
func (h *MemberHandler) Create(c *gin.Context) {
	var req struct {
		FullName  string `json:"fullName"  binding:"required"`
		Email     string `json:"email"     binding:"required,email"`
		Password  string `json:"password"`
		Phone     string `json:"phone"`
		VoicePart string `json:"voicePart" binding:"required,oneof=Soprano Alto Tenor Bass"`
		Role      string `json:"role"`
		Status    string `json:"status"`
	}
	if !bindJSON(c, &req) {
		return
	}

	ctx := c.Request.Context()
	count, _ := h.db.Members().CountDocuments(ctx, bson.M{"email": strings.ToLower(req.Email)})
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "Email already exists"})
		return
	}

	pwd := req.Password
	if pwd == "" {
		pwd = "ChangeMe123!"
	}
	hash, _ := crypto.HashPassword(pwd)

	role := req.Role
	if role == "" {
		role = "member"
	}
	status := req.Status
	if status == "" {
		status = "active"
	}

	now := time.Now()
	member := models.Member{
		FullName: req.FullName, Email: strings.ToLower(req.Email),
		PasswordHash: hash, Phone: req.Phone,
		VoicePart: req.VoicePart, Role: role, Status: status,
		IsAdmin: role == "president", Permissions: []string{},
		JoinDate: now, CreatedAt: now, UpdatedAt: now,
	}
	res, err := h.db.Members().InsertOne(ctx, member)
	if err != nil {
		respondError(c, err)
		return
	}

	member.ID = res.InsertedID.(primitive.ObjectID)
	realtime.Publish("member:new", utils.FormatMember(&member))
	c.JSON(http.StatusCreated, gin.H{"success": true, "message": "Member created", "data": utils.FormatMember(&member)})
}

// GET /members/:id
func (h *MemberHandler) GetByID(c *gin.Context) {
	ctx := c.Request.Context()
	oid, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid ID"})
		return
	}
	var member models.Member
	if err := h.db.Members().FindOne(ctx, bson.M{"_id": oid},
		options.FindOne().SetProjection(bson.M{"passwordHash": 0})).Decode(&member); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Member not found"})
		return
	}
	sendSuccess(c, utils.FormatMember(&member), "")
}

// PUT /members/:id
func (h *MemberHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	current := middleware.CurrentMember(c)
	if !current.IsAdmin && current.ID.Hex() != id {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Cannot update another member"})
		return
	}

	var body map[string]interface{}
	c.ShouldBindJSON(&body)

	allowed := map[string]bool{"fullName": true, "phone": true, "dateOfBirth": true, "gender": true,
		"maritalStatus": true, "voicePart": true, "role": true, "status": true, "bio": true, "avatarUrl": true, "permissions": true}
	update := bson.M{"updatedAt": time.Now()}
	for k, v := range body {
		if allowed[k] {
			update[k] = v
		}
	}
	if r, ok := update["role"].(string); ok {
		update["isAdmin"] = r == "president"
	}

	oid, _ := primitive.ObjectIDFromHex(id)
	h.db.Members().UpdateByID(ctx, oid, bson.M{"$set": update})

	var updated models.Member
	h.db.Members().FindOne(ctx, bson.M{"_id": oid},
		options.FindOne().SetProjection(bson.M{"passwordHash": 0})).Decode(&updated)
	realtime.Publish("member:updated", utils.FormatMember(&updated))
	sendSuccess(c, utils.FormatMember(&updated), "Member updated")
}

// DELETE /members/:id
func (h *MemberHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	curr := middleware.CurrentMember(c)

	oid, _ := primitive.ObjectIDFromHex(id)
	var member models.Member
	h.db.Members().FindOne(ctx, bson.M{"_id": oid}).Decode(&member)
	if member.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Cannot delete admin"})
		return
	}
	if curr.ID.Hex() == id {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Cannot delete yourself"})
		return
	}
	h.db.Members().DeleteOne(ctx, bson.M{"_id": oid})
	realtime.Publish("member:deleted", gin.H{"id": id})
	c.Status(http.StatusNoContent)
}

// POST /members/:id/approve
func (h *MemberHandler) Approve(c *gin.Context) {
	ctx := c.Request.Context()
	oid, _ := primitive.ObjectIDFromHex(c.Param("id"))
	now := time.Now()
	h.db.Members().UpdateByID(ctx, oid, bson.M{
		"$set": bson.M{"status": "active", "updatedAt": now},
	})
	var m models.Member
	h.db.Members().FindOne(ctx, bson.M{"_id": oid},
		options.FindOne().SetProjection(bson.M{"passwordHash": 0})).Decode(&m)

	go h.mailer.SendApprovalEmail(m.Email, m.FullName)

	realtime.Publish("member:approved", utils.FormatMember(&m))
	sendSuccess(c, utils.FormatMember(&m), m.FullName+" approved")
}

// GET /members/:id/qr — returns PNG
func (h *MemberHandler) GetQR(c *gin.Context) {
	id := c.Param("id")
	qr, err := qrcode.Encode(id, qrcode.Medium, 300)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "QR generation failed"})
		return
	}
	c.Header("Content-Disposition", "attachment; filename=member-qr.png")
	c.Data(http.StatusOK, "image/png", qr)
}

// GET /members/export
func (h *MemberHandler) Export(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"success": false, "message": "Not implemented"})
}

// POST /members/import
func (h *MemberHandler) Import(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"success": false, "message": "Not implemented"})
}
