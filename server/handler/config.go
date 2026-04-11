package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) handleGetConfig(c *gin.Context) {
	cfg := h.cfg

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"port":        cfg.Port,
		"host":        cfg.Host,
		"timeout":     cfg.Timeout,
		"retry_times": cfg.RetryTimes,
		"concurrency": cfg.Concurrency,
		"proxy": gin.H{
			"tunnel": cfg.Proxy.Tunnel,
			"pool": gin.H{
				"url":     cfg.Proxy.Pool.URL,
				"size":    cfg.Proxy.Pool.Size,
				"ipv6":    cfg.Proxy.Pool.IPv6,
				"ipv6Num": cfg.Proxy.Pool.IPv6Num,
			},
		},
	}})
}
