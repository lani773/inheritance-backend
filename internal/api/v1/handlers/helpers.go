package handlers

import (
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/inheritance-choir/backend/internal/services"
)

// bindJSON binds and validates JSON body. Returns false and writes error on failure.
func bindJSON(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Validation failed",
			"error":   err.Error(),
		})
		return false
	}
	return true
}

// respondError maps service errors to HTTP responses.
func respondError(c *gin.Context, err error) {
	if apiErr, ok := err.(*services.APIError); ok {
		c.JSON(apiErr.Code, gin.H{"success": false, "message": apiErr.Message})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Internal server error"})
}

// paginationParams extracts page/pageSize from query string.
func paginationParams(c *gin.Context) (page, pageSize, skip int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	skip = (page - 1) * pageSize
	return
}

// sendSuccess sends a standard success response.
func sendSuccess(c *gin.Context, data interface{}, message string) {
	body := gin.H{"success": true, "message": message}
	if data != nil {
		body["data"] = data
	}
	c.JSON(http.StatusOK, body)
}

// sendCreated sends a 201 Created response.
func sendCreated(c *gin.Context, data interface{}, message string) {
	body := gin.H{"success": true, "message": message}
	if data != nil {
		body["data"] = data
	}
	c.JSON(http.StatusCreated, body)
}

// sendPaginated sends a paginated list response.
func sendPaginated(c *gin.Context, items interface{}, total int64, page, pageSize int) {
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    items,
		"meta": gin.H{
			"total":      total,
			"page":       page,
			"pageSize":   pageSize,
			"totalPages": totalPages,
			"hasNext":    int64(page*pageSize) < total,
			"hasPrev":    page > 1,
		},
	})
}
