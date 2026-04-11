package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterUIRoutes registers the Web UI route.
func RegisterUIRoutes(r *gin.Engine, enable bool) {
	if !enable {
		return
	}
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})
}
