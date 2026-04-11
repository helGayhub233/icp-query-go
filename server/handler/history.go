package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (h *Handler) handleGetHistory(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "数据库未初始化"})
		return
	}

	limit := parseInt(c.DefaultQuery("limit", "50"), 50)
	offset := parseInt(c.DefaultQuery("offset", "0"), 0)
	searchType := c.Query("type")

	ctx := c.Request.Context()
	history, err := h.db.GetHistory(ctx, limit, offset, searchType)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	total, err := h.db.GetHistoryCount(ctx, searchType)
	if err != nil {
		slog.Error("get history count failed", "error", err)
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": history, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) handleGetHistoryDetail(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "数据库未初始化"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的ID"})
		return
	}

	detail, err := h.db.GetHistoryDetail(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "查询失败"})
		return
	}
	if detail == nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "message": "历史记录不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": detail})
}

func (h *Handler) handleDeleteHistory(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "数据库未初始化"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的ID"})
		return
	}

	if err := h.db.DeleteHistory(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *Handler) handleClearHistory(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "数据库未初始化"})
		return
	}

	searchType := c.Query("type")

	if err := h.db.ClearHistory(c.Request.Context(), searchType); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "清空失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "清空成功"})
}
