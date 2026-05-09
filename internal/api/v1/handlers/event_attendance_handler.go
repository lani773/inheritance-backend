package handlers
import "go.mongodb.org/mongo-driver/v2/bson"

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"

	"github.com/inheritance-choir/backend/internal/api/v1/middleware"
	"github.com/inheritance-choir/backend/internal/models"
	"github.com/inheritance-choir/backend/internal/realtime"
	"github.com/inheritance-choir/backend/internal/repository"
)

// ═══════════════════════════════════════════════════════════════
// EVENT HANDLER
// ═══════════════════════════════════════════════════════════════

type EventHandler struct {
	db  *repository.DB
	log *zap.Logger
}

func NewEventHandler(db *repository.DB, log *zap.Logger) *EventHandler {
	return &EventHandler{db: db, log: log}
}

func (h *EventHandler) List(c *gin.Context) {
	page, pageSize, skip := paginationParams(c)
	ctx := c.Request.Context()
	today := time.Now().Format("2006-01-02")

	filter := bson.M{}
	switch c.Query("filter") {
	case "upcoming":
		filter["date"] = bson.M{"$gte": today}
	case "past":
		filter["date"] = bson.M{"$lt": today}
	case "today":
		filter["date"] = today
	}
	if v := c.Query("type"); v != "" {
		filter["type"] = v
	}
	if v := c.Query("search"); v != "" {
		filter["title"] = bson.M{"$regex": v, "$options": "i"}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "date", Value: 1}}).
		SetSkip(int64(skip)).SetLimit(int64(pageSize))

	total, _ := h.db.Events().CountDocuments(ctx, filter)
	cursor, err := h.db.Events().Find(ctx, filter, opts)
	if err != nil {
		respondError(c, err)
		return
	}
	defer cursor.Close(ctx)

	var events []models.Event
	cursor.All(ctx, &events)

	type enriched struct {
		models.Event
		AttendanceCount int64 `json:"attendanceCount"`
		PresentCount    int64 `json:"presentCount"`
	}
	result := make([]enriched, 0, len(events))
	for _, ev := range events {
		eid := ev.ID.Hex()
		att, _ := h.db.Attendances().CountDocuments(ctx, bson.M{"eventId": eid})
		present, _ := h.db.Attendances().CountDocuments(ctx, bson.M{
			"eventId": eid, "status": bson.M{"$in": bson.A{"present", "late"}},
		})
		result = append(result, enriched{ev, att, present})
	}
	sendPaginated(c, result, total, page, pageSize)
}

func (h *EventHandler) Upcoming(c *gin.Context) {
	ctx := c.Request.Context()
	today := time.Now().Format("2006-01-02")
	opts := options.Find().SetSort(bson.D{{Key: "date", Value: 1}}).SetLimit(10)
	cursor, _ := h.db.Events().Find(ctx, bson.M{"date": bson.M{"$gte": today}}, opts)
	var events []models.Event
	cursor.All(ctx, &events)
	cursor.Close(ctx)
	sendSuccess(c, events, "")
}

func (h *EventHandler) Calendar(c *gin.Context) {
	month := c.Query("month")
	if month == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "month required (YYYY-MM)"})
		return
	}
	ctx := c.Request.Context()
	cursor, _ := h.db.Events().Find(ctx,
		bson.M{"date": bson.M{"$regex": "^" + month}},
		options.Find().SetSort(bson.D{{Key: "date", Value: 1}}),
	)
	var events []models.Event
	cursor.All(ctx, &events)
	cursor.Close(ctx)
	sendSuccess(c, events, "")
}

func (h *EventHandler) Create(c *gin.Context) {
	var ev models.Event
	if !bindJSON(c, &ev) {
		return
	}
	curr := middleware.CurrentMember(c)
	now := time.Now()
	ev.CreatedAt = now
	ev.UpdatedAt = now
	cid := curr.ID
	ev.CreatedBy = cid.Hex()

	res, err := h.db.Events().InsertOne(c.Request.Context(), ev)
	if err != nil {
		respondError(c, err)
		return
	}
	ev.ID = res.InsertedID.(bson.ObjectID)
	realtime.Publish(realtime.EvtEventCreated, ev)
	sendCreated(c, ev, "Event created")
}

func (h *EventHandler) GetByID(c *gin.Context) {
	oid, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid ID"})
		return
	}
	var ev models.Event
	if err := h.db.Events().FindOne(c.Request.Context(), bson.M{"_id": oid}).Decode(&ev); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Event not found"})
		return
	}
	sendSuccess(c, ev, "")
}

func (h *EventHandler) Update(c *gin.Context) {
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	body["updatedAt"] = time.Now()
	h.db.Events().UpdateByID(c.Request.Context(), oid, bson.M{"$set": body})
	var ev models.Event
	h.db.Events().FindOne(c.Request.Context(), bson.M{"_id": oid}).Decode(&ev)
	realtime.Publish(realtime.EvtEventUpdated, ev)
	sendSuccess(c, ev, "Updated")
}

func (h *EventHandler) Delete(c *gin.Context) {
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	h.db.Events().DeleteOne(c.Request.Context(), bson.M{"_id": oid})
	realtime.Publish(realtime.EvtEventDeleted, gin.H{"id": oid.Hex()})
	c.Status(http.StatusNoContent)
}

func (h *EventHandler) GetAttendance(c *gin.Context) {
	eid := c.Param("id")
	ctx := c.Request.Context()
	cursor, _ := h.db.Attendances().Find(ctx, bson.M{"eventId": eid})
	var recs []models.Attendance
	cursor.All(ctx, &recs)
	cursor.Close(ctx)

	for i, r := range recs {
		oid, err := bson.ObjectIDFromHex(r.MemberID)
		if err != nil {
			continue
		}
		var m models.Member
		h.db.Members().FindOne(ctx, bson.M{"_id": oid},
			options.FindOne().SetProjection(bson.M{"fullName": 1, "voicePart": 1})).Decode(&m)
		recs[i].MemberName = m.FullName
		recs[i].VoicePart = m.VoicePart
	}
	sendSuccess(c, recs, "")
}

// ═══════════════════════════════════════════════════════════════
// ATTENDANCE HANDLER
// ═══════════════════════════════════════════════════════════════

type AttendanceHandler struct {
	db  *repository.DB
	log *zap.Logger
}

func NewAttendanceHandler(db *repository.DB, log *zap.Logger) *AttendanceHandler {
	return &AttendanceHandler{db: db, log: log}
}

func (h *AttendanceHandler) BulkMark(c *gin.Context) {
	var req struct {
		EventID string `json:"eventId" binding:"required"`
		Records []struct {
			MemberID string `json:"memberId" binding:"required"`
			Status   string `json:"status"   binding:"required"`
		} `json:"records" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}

	ctx := c.Request.Context()
	curr := middleware.CurrentMember(c)
	eid, _ := bson.ObjectIDFromHex(req.EventID)

	var ev models.Event
	if err := h.db.Events().FindOne(ctx, bson.M{"_id": eid}).Decode(&ev); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Event not found"})
		return
	}

	saved := 0
	errs := []gin.H{}
	mids := make([]string, 0, len(req.Records))

	for _, rec := range req.Records {
		_, err := h.db.Attendances().UpdateOne(ctx,
			bson.M{"eventId": req.EventID, "memberId": rec.MemberID},
			bson.M{"$set": bson.M{
				"status": rec.Status, "date": ev.Date,
				"markedBy": curr.ID.Hex(), "markedAt": time.Now(),
				"eventId": req.EventID, "memberId": rec.MemberID,
			}},
			options.UpdateOne().SetUpsert(true),
		)
		if err != nil {
			errs = append(errs, gin.H{"memberId": rec.MemberID, "error": err.Error()})
		} else {
			saved++
			mids = append(mids, rec.MemberID)
		}
	}

	for _, mid := range mids {
		h.recalcAttendance(ctx, mid)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true, "saved": saved, "errors": errs,
		"message": fmt.Sprintf("Attendance saved: %d/%d", saved, len(req.Records)),
	})
	realtime.Publish(realtime.EvtAttendanceMarked, gin.H{"eventId": req.EventID, "saved": saved})
}

func (h *AttendanceHandler) List(c *gin.Context) {
	page, pageSize, skip := paginationParams(c)
	ctx := c.Request.Context()
	filter := bson.M{}
	if v := c.Query("eventId"); v != "" {
		filter["eventId"] = v
	}
	if v := c.Query("memberId"); v != "" {
		filter["memberId"] = v
	}
	if v := c.Query("status"); v != "" {
		filter["status"] = v
	}
	if v := c.Query("month"); v != "" {
		filter["date"] = bson.M{"$regex": "^" + v}
	}

	total, _ := h.db.Attendances().CountDocuments(ctx, filter)
	cursor, _ := h.db.Attendances().Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "markedAt", Value: -1}}).
			SetSkip(int64(skip)).SetLimit(int64(pageSize)))
	var recs []models.Attendance
	cursor.All(ctx, &recs)
	cursor.Close(ctx)
	sendPaginated(c, recs, total, page, pageSize)
}

func (h *AttendanceHandler) Stats(c *gin.Context) {
	ctx := c.Request.Context()

	overall := []struct {
		Total    int64 `bson:"total"`
		Attended int64 `bson:"attended"`
	}{}
	cur, _ := h.db.Attendances().Aggregate(ctx, mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":   nil,
			"total": bson.M{"$sum": 1},
			"attended": bson.M{"$sum": bson.M{"$cond": bson.A{
				bson.M{"$in": bson.A{"$status", bson.A{"present", "late"}}}, 1, 0,
			}}},
		}}},
	})
	cur.All(ctx, &overall)
	cur.Close(ctx)

	rate := 0.0
	if len(overall) > 0 && overall[0].Total > 0 {
		rate = math.Round(float64(overall[0].Attended)/float64(overall[0].Total)*1000) / 10
	}

	cursor, _ := h.db.Members().Find(ctx,
		bson.M{"status": "active", "attendance": bson.M{"$lt": 70}},
		options.Find().SetProjection(bson.M{"fullName": 1, "voicePart": 1, "attendance": 1}))
	var atRisk []models.Member
	cursor.All(ctx, &atRisk)
	cursor.Close(ctx)

	tCur, _ := h.db.Attendances().Aggregate(ctx, mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":   bson.M{"$substr": bson.A{"$date", 0, 7}},
			"total": bson.M{"$sum": 1},
			"attended": bson.M{"$sum": bson.M{"$cond": bson.A{
				bson.M{"$in": bson.A{"$status", bson.A{"present", "late"}}}, 1, 0,
			}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
		{{Key: "$limit", Value: 12}},
	})
	var trend []map[string]interface{}
	tCur.All(ctx, &trend)
	tCur.Close(ctx)

	sendSuccess(c, gin.H{
		"overall": rate, "atRiskCount": len(atRisk),
		"atRiskMembers": atRisk, "trend": trend,
	}, "")
}

func (h *AttendanceHandler) QRCheckin(c *gin.Context) {
	var req struct {
		EventID  string `json:"eventId"  binding:"required"`
		MemberID string `json:"memberId" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	ctx := c.Request.Context()
	curr := middleware.CurrentMember(c)
	eid, _ := bson.ObjectIDFromHex(req.EventID)
	mid, _ := bson.ObjectIDFromHex(req.MemberID)

	var ev models.Event
	var member models.Member
	if err := h.db.Events().FindOne(ctx, bson.M{"_id": eid}).Decode(&ev); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Event not found"})
		return
	}
	if err := h.db.Members().FindOne(ctx, bson.M{"_id": mid}).Decode(&member); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Member not found"})
		return
	}

	now := time.Now()
	h.db.Attendances().UpdateOne(ctx,
		bson.M{"eventId": req.EventID, "memberId": req.MemberID},
		bson.M{"$set": bson.M{
			"status": "present", "date": ev.Date,
			"markedBy": curr.ID.Hex(), "markedAt": now,
			"eventId": req.EventID, "memberId": req.MemberID,
		}},
		options.UpdateOne().SetUpsert(true),
	)
	h.recalcAttendance(ctx, req.MemberID)

	realtime.Publish(realtime.EvtAttendanceMarked, gin.H{"eventId": req.EventID, "memberId": req.MemberID, "status": "present", "markedAt": now})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"member":  gin.H{"id": member.ID.Hex(), "fullName": member.FullName, "voicePart": member.VoicePart},
		"event":   ev.Title, "status": "present",
		"checkedInAt": now.Format(time.RFC3339),
	})
}

func (h *AttendanceHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	var rec models.Attendance
	h.db.Attendances().FindOne(ctx, bson.M{"_id": oid}).Decode(&rec)
	h.db.Attendances().DeleteOne(ctx, bson.M{"_id": oid})
	if rec.MemberID != "" {
		h.recalcAttendance(ctx, rec.MemberID)
	}
	c.Status(http.StatusNoContent)
}

func (h *AttendanceHandler) ListExcuses(c *gin.Context) {
	ctx := c.Request.Context()
	curr := middleware.CurrentMember(c)
	filter := bson.M{}
	if v := c.Query("status"); v != "" {
		filter["status"] = v
	}
	if !curr.IsAdmin {
		filter["memberId"] = curr.ID.Hex()
	} else if v := c.Query("memberId"); v != "" {
		filter["memberId"] = v
	}
	cursor, _ := h.db.Excuses().Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	var excuses []models.Excuse
	cursor.All(ctx, &excuses)
	cursor.Close(ctx)
	sendSuccess(c, excuses, "")
}

func (h *AttendanceHandler) SubmitExcuse(c *gin.Context) {
	var req struct {
		EventID string `json:"eventId" binding:"required"`
		Reason  string `json:"reason"  binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	curr := middleware.CurrentMember(c)
	now := time.Now()
	excuse := models.Excuse{
		EventID: req.EventID, MemberID: curr.ID.Hex(),
		Reason: req.Reason, Status: "pending",
		CreatedAt: now, UpdatedAt: now,
	}
	res, _ := h.db.Excuses().InsertOne(c.Request.Context(), excuse)
	excuse.ID = res.InsertedID.(bson.ObjectID)
	sendCreated(c, gin.H{"id": excuse.ID.Hex()}, "Excuse submitted for review")
}

func (h *AttendanceHandler) ReviewExcuse(c *gin.Context) {
	var req struct {
		Status     string `json:"status"     binding:"required,oneof=approved rejected"`
		ReviewNote string `json:"reviewNote"`
	}
	if !bindJSON(c, &req) {
		return
	}
	ctx := c.Request.Context()
	curr := middleware.CurrentMember(c)
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	now := time.Now()

	h.db.Excuses().UpdateByID(ctx, oid, bson.M{"$set": bson.M{
		"status": req.Status, "reviewedBy": curr.ID.Hex(),
		"reviewedAt": now, "reviewNote": req.ReviewNote, "updatedAt": now,
	}})

	if req.Status == "approved" {
		var excuse models.Excuse
		h.db.Excuses().FindOne(ctx, bson.M{"_id": oid}).Decode(&excuse)
		h.db.Attendances().UpdateOne(ctx,
			bson.M{"eventId": excuse.EventID, "memberId": excuse.MemberID},
			bson.M{"$set": bson.M{"status": "excused"}},
			options.UpdateOne().SetUpsert(true),
		)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "status": req.Status, "message": "Excuse " + req.Status})
}

func (h *AttendanceHandler) recalcAttendance(ctx context.Context, memberID string) {
	total, _ := h.db.Attendances().CountDocuments(ctx, bson.M{"memberId": memberID})
	attended, _ := h.db.Attendances().CountDocuments(ctx, bson.M{
		"memberId": memberID, "status": bson.M{"$in": bson.A{"present", "late"}},
	})
	rate := 0.0
	if total > 0 {
		rate = math.Round(float64(attended)/float64(total)*1000) / 10
	}
	oid, err := bson.ObjectIDFromHex(memberID)
	if err != nil {
		return
	}
	h.db.Members().UpdateByID(ctx, oid, bson.M{
		"$set": bson.M{"attendance": rate, "updatedAt": time.Now()},
	})
}
