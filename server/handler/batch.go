package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/imxw/icp-query-go/internal/task"
)

func (h *Handler) handleGetBatchTasks(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "数据库未初始化"})
		return
	}

	limit := parseInt(c.DefaultQuery("limit", "20"), 20)
	offset := parseInt(c.DefaultQuery("offset", "0"), 0)
	status := c.Query("status")

	tasks, err := h.db.GetBatchTasks(c.Request.Context(), limit, offset, status)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	total, err := h.db.GetBatchTasksCount(c.Request.Context(), status)
	if err != nil {
		slog.Error("get batch tasks count failed", "error", err)
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": tasks, "total": total})
}

func (h *Handler) handleGetBatchTaskDetail(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "数据库未初始化"})
		return
	}

	taskName := c.Param("name")
	detail, err := h.db.GetBatchTaskDetail(c.Request.Context(), taskName)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "查询失败"})
		return
	}
	if detail == nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "message": "任务不存在"})
		return
	}

	result := gin.H{"code": 200, "data": detail}

	// Attach live progress if task is still running
	if detail.Status == "running" && h.tm != nil {
		if p, ok := h.tm.GetProgress(taskName); ok {
			result["progress"] = p
		}
	}

	c.JSON(http.StatusOK, result)
}

func (h *Handler) handleDeleteBatchTask(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "数据库未初始化"})
		return
	}

	taskName := c.Param("name")

	// Cancel running task if exists
	if h.tm != nil {
		if err := h.tm.Cancel(taskName); err != nil {
			slog.Warn("cancel task failed", "name", taskName, "error", err)
		}
	}

	if err := h.db.DeleteBatchTask(c.Request.Context(), taskName); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *Handler) handleCreateTask(c *gin.Context) {
	if h.tm == nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "任务管理器未初始化"})
		return
	}

	var req task.CreateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请求参数无效"})
		return
	}

	if req.Name == "" {
		req.Name = "task_" + time.Now().Format("20060102_150405")
	}

	err := h.tm.Create(c.Request.Context(), req)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "任务创建成功", "data": gin.H{"task_name": req.Name}})
}
