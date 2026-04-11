package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/imxw/icp-query-go/internal/beian"
	"github.com/imxw/icp-query-go/internal/config"
	"github.com/imxw/icp-query-go/internal/store"
	"github.com/imxw/icp-query-go/internal/task"
)

// Handler holds all dependencies for HTTP handlers.
type Handler struct {
	beian *beian.Beian
	db    *store.Store
	cfg   *config.Config
	tm    *task.Manager
}

// New creates a Handler with the given dependencies.
func New(b *beian.Beian, db *store.Store, cfg *config.Config, tm *task.Manager) *Handler {
	return &Handler{beian: b, db: db, cfg: cfg, tm: tm}
}

// RegisterRoutes registers all API routes on the given gin engine.
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.GET("/query", h.handleQuery)
		api.GET("/history", h.handleGetHistory)
		api.GET("/history/:id", h.handleGetHistoryDetail)
		api.DELETE("/history/:id", h.handleDeleteHistory)
		api.DELETE("/history", h.handleClearHistory)
		api.GET("/config", h.handleGetConfig)

		// Batch tasks
		api.GET("/batch", h.handleGetBatchTasks)
		api.GET("/batch/:name", h.handleGetBatchTaskDetail)
		api.POST("/batch", h.handleCreateTask)
		api.DELETE("/batch/:name", h.handleDeleteBatchTask)
	}
}
