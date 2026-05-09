package handlers
import "go.mongodb.org/mongo-driver/v2/bson"

import (
	"context"
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
	"github.com/inheritance-choir/backend/pkg/jwt"
)

type ContributionHandler struct {
	db  *repository.DB
	log *zap.Logger
}

func NewContributionHandler(db *repository.DB, log *zap.Logger) *ContributionHandler {
	return &ContributionHandler{db: db, log: log}
}

// GET /contributions
func (h *ContributionHandler) List(c *gin.Context) {
	page, pageSize, skip := paginationParams(c)
	ctx := c.Request.Context()

	filter := bson.M{}
	if v := c.Query("memberId"); v != "" {
		filter["memberId"] = v
	}
	if v := c.Query("type"); v != "" {
		filter["type"] = v
	}
	if v := c.Query("method"); v != "" {
		filter["method"] = v
	}
	if v := c.Query("month"); v != "" {
		filter["date"] = bson.M{"$regex": "^" + v}
	}
	if v := c.Query("verified"); v != "" {
		filter["verified"] = v == "true"
	}

	sortBy := c.DefaultQuery("sortBy", "date")
	sortDir := -1
	if c.Query("sortDir") == "asc" {
		sortDir = 1
	}

	opts := options.Find().
		SetSort(bson.D{{Key: sortBy, Value: sortDir}}).
		SetSkip(int64(skip)).SetLimit(int64(pageSize))

	total, _ := h.db.Contributions().CountDocuments(ctx, filter)

	// Total amount via aggregation
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.M{"_id": nil, "total": bson.M{"$sum": "$amount"}}}},
	}
	cur, _ := h.db.Contributions().Aggregate(ctx, pipeline)
	var agg []struct {
		Total float64 `bson:"total"`
	}
	cur.All(ctx, &agg)
	totalAmount := 0.0
	if len(agg) > 0 {
		totalAmount = agg[0].Total
	}

	cursor, _ := h.db.Contributions().Find(ctx, filter, opts)
	var items []models.Contribution
	cursor.All(ctx, &items)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    items,
		"meta": gin.H{
			"total": total, "totalAmount": totalAmount,
			"page": page, "pageSize": pageSize,
		},
	})
}

// GET /contributions/stats
func (h *ContributionHandler) Stats(c *gin.Context) {
	ctx := c.Request.Context()
	now := time.Now()
	thisM := now.Format("2006-01")
	lastM := now.AddDate(0, -1, 0).Format("2006-01")

	pipeline := mongo.Pipeline{
		{{Key: "$facet", Value: bson.M{
			"total": mongo.Pipeline{{{Key: "$group", Value: bson.M{"_id": nil, "s": bson.M{"$sum": "$amount"}}}}},
			"thisMonth": mongo.Pipeline{
				{{Key: "$match", Value: bson.M{"date": bson.M{"$regex": "^" + thisM}}}},
				{{Key: "$group", Value: bson.M{"_id": nil, "s": bson.M{"$sum": "$amount"}}}},
			},
			"lastMonth": mongo.Pipeline{
				{{Key: "$match", Value: bson.M{"date": bson.M{"$regex": "^" + lastM}}}},
				{{Key: "$group", Value: bson.M{"_id": nil, "s": bson.M{"$sum": "$amount"}}}},
			},
			"byType": mongo.Pipeline{
				{{Key: "$group", Value: bson.M{"_id": "$type", "total": bson.M{"$sum": "$amount"}}}},
			},
			"unverified": mongo.Pipeline{
				{{Key: "$match", Value: bson.M{"verified": false}}},
				{{Key: "$count", Value: "count"}},
			},
		}}},
	}
	cur, _ := h.db.Contributions().Aggregate(ctx, pipeline)
	var result []map[string]interface{}
	cur.All(ctx, &result)

	sendSuccess(c, result, "")
}

// GET /contributions/trend
func (h *ContributionHandler) Trend(c *gin.Context) {
	ctx := c.Request.Context()
	months := 12

	pipeline := mongo.Pipeline{
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
	}
	cur, _ := h.db.Contributions().Aggregate(ctx, pipeline)
	var raw []map[string]interface{}
	cur.All(ctx, &raw)
	sendSuccess(c, raw, "")
}

// POST /contributions
func (h *ContributionHandler) Create(c *gin.Context) {
	var req models.Contribution
	if !bindJSON(c, &req) {
		return
	}

	ctx := c.Request.Context()
	curr := middleware.CurrentMember(c)
	now := time.Now()
	creBy := curr.ID.Hex()
	req.ReceiptNo = jwt.GenerateReceiptNo("RC")
	req.CreatedBy = &creBy
	req.CreatedAt = now
	req.UpdatedAt = now

	res, err := h.db.Contributions().InsertOne(ctx, req)
	if err != nil {
		respondError(c, err)
		return
	}

	req.ID = res.InsertedID.(bson.ObjectID)
	h.recalcMemberTotal(ctx, req.MemberID)
	realtime.Publish(realtime.EvtContributionCreated, req)
	c.JSON(http.StatusCreated, gin.H{"success": true, "message": "Contribution recorded", "data": req})
}

// GET /contributions/:id
func (h *ContributionHandler) GetByID(c *gin.Context) {
	ctx := c.Request.Context()
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	var item models.Contribution
	if err := h.db.Contributions().FindOne(ctx, bson.M{"_id": oid}).Decode(&item); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Not found"})
		return
	}
	sendSuccess(c, item, "")
}

// PUT /contributions/:id
func (h *ContributionHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	body["updatedAt"] = time.Now()
	h.db.Contributions().UpdateByID(ctx, oid, bson.M{"$set": body})
	var item models.Contribution
	h.db.Contributions().FindOne(ctx, bson.M{"_id": oid}).Decode(&item)
	h.recalcMemberTotal(ctx, item.MemberID)
	realtime.Publish(realtime.EvtContributionCreated, item)
	sendSuccess(c, item, "Updated")
}

// DELETE /contributions/:id
func (h *ContributionHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	var item models.Contribution
	h.db.Contributions().FindOne(ctx, bson.M{"_id": oid}).Decode(&item)
	h.db.Contributions().DeleteOne(ctx, bson.M{"_id": oid})
	h.recalcMemberTotal(ctx, item.MemberID)
	realtime.Publish(realtime.EvtContributionDeleted, gin.H{"id": oid.Hex(), "memberId": item.MemberID})
	c.Status(http.StatusNoContent)
}

// POST /contributions/:id/verify
func (h *ContributionHandler) Verify(c *gin.Context) {
	ctx := c.Request.Context()
	oid, _ := bson.ObjectIDFromHex(c.Param("id"))
	curr := middleware.CurrentMember(c)
	now := time.Now()
	verBy := curr.ID.Hex()
	h.db.Contributions().UpdateByID(ctx, oid, bson.M{
		"$set": bson.M{"verified": true, "verifiedBy": &verBy, "verifiedAt": now, "updatedAt": now},
	})
	var item models.Contribution
	h.db.Contributions().FindOne(ctx, bson.M{"_id": oid}).Decode(&item)
	realtime.Publish(realtime.EvtContributionVerified, item)
	sendSuccess(c, item, "Contribution verified")
}

// POST /contributions/bulk-verify
func (h *ContributionHandler) BulkVerify(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}

	ctx := c.Request.Context()
	curr := middleware.CurrentMember(c)
	now := time.Now()
	verBy := curr.ID.Hex()

	var oids []bson.ObjectID
	for _, id := range req.IDs {
		if oid, err := bson.ObjectIDFromHex(id); err == nil {
			oids = append(oids, oid)
		}
	}

	res, _ := h.db.Contributions().UpdateMany(ctx, bson.M{"_id": bson.M{"$in": oids}}, bson.M{
		"$set": bson.M{"verified": true, "verifiedBy": &verBy, "verifiedAt": now, "updatedAt": now},
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "verified": res.ModifiedCount})
}

// ── Helper ────────────────────────────────────────────────────

func (h *ContributionHandler) recalcMemberTotal(ctx context.Context, memberID string) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"memberId": memberID}}},
		{{Key: "$group", Value: bson.M{"_id": nil, "total": bson.M{"$sum": "$amount"}}}},
	}
	cur, _ := h.db.Contributions().Aggregate(ctx, pipeline)
	var agg []struct {
		Total float64 `bson:"total"`
	}
	cur.All(ctx, &agg)
	total := 0.0
	if len(agg) > 0 {
		total = agg[0].Total
	}

	oid, err := bson.ObjectIDFromHex(memberID)
	if err != nil {
		return
	}
	h.db.Members().UpdateByID(ctx, oid, bson.M{
		"$set": bson.M{"contributionTotal": total, "updatedAt": time.Now()},
	})
}

// GET /contributions/export
func (h *ContributionHandler) Export(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"success": false, "message": "Not implemented"})
}
