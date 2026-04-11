package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (h *Handler) handleQuery(c *gin.Context) {
	search := c.Query("search")
	queryType := c.DefaultQuery("type", "web")
	pageNum := c.DefaultQuery("pageNum", "0")
	pageSize := c.DefaultQuery("pageSize", "0")
	proxy := c.Query("proxy")

	if search == "" {
		c.JSON(http.StatusOK, gin.H{"code": 101, "msg": "参数错误,请指定search参数"})
		return
	}

	// Validate query type
	validTypes := map[string]bool{
		"web": true, "app": true, "mapp": true, "kapp": true,
		"bweb": true, "bapp": true, "bmapp": true, "bkapp": true,
	}
	if !validTypes[queryType] {
		c.JSON(http.StatusOK, gin.H{"code": 102, "msg": "不是支持的查询类型"})
		return
	}


	p := formatProxy(proxy)
	normalTypes := map[string]bool{"web": true, "app": true, "mapp": true, "kapp": true}

	var data map[string]any
	var err error

	for i := 0; i < h.cfg.RetryTimes; i++ {
		if normalTypes[queryType] {
			data, err = h.queryNormal(c, queryType, search, pageNum, pageSize, p)
		} else {
			data, err = h.queryBlack(c, queryType, search, p)
		}
		if err != nil {
			continue
		}
		if code, _ := data["code"].(float64); code == 200 {
			c.JSON(http.StatusOK, data)
			return
		}
		if msg, _ := data["message"].(string); msg == "当前访问已被创宇盾拦截" || msg == "创宇盾拦截" {
			c.JSON(http.StatusOK, data)
			return
		}
	}

	if data != nil {
		c.JSON(http.StatusOK, data)
	} else if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
	}
}

func (h *Handler) queryNormal(c *gin.Context, path, search, pageNum, pageSize, proxy string) (map[string]any, error) {
	pn := parseInt(pageNum, 0)
	ps := parseInt(pageSize, 0)
	ctx := c.Request.Context()

	switch path {
	case "web":
		return h.beian.QueryWeb(ctx, search, pn, ps, proxy)
	case "app":
		return h.beian.QueryApp(ctx, search, pn, ps, proxy)
	case "mapp":
		return h.beian.QueryMiniApp(ctx, search, pn, ps, proxy)
	case "kapp":
		return h.beian.QueryKuaiApp(ctx, search, pn, ps, proxy)
	default:
		return map[string]any{"code": 102, "msg": "不支持的查询类型"}, nil
	}
}

func (h *Handler) queryBlack(c *gin.Context, path, search, proxy string) (map[string]any, error) {
	ctx := c.Request.Context()

	switch path {
	case "bweb":
		return h.beian.QueryBlackWeb(ctx, search, proxy)
	case "bapp":
		return h.beian.QueryBlackApp(ctx, search, proxy)
	case "bmapp":
		return h.beian.QueryBlackMiniApp(ctx, search, proxy)
	case "bkapp":
		return h.beian.QueryBlackKuaiApp(ctx, search, proxy)
	default:
		return map[string]any{"code": 102, "msg": "不支持的查询类型"}, nil
	}
}

func formatProxy(proxy string) string {
	if proxy == "" {
		return ""
	}
	if !strings.HasPrefix(proxy, "http://") && !strings.HasPrefix(proxy, "https://") {
		return "http://" + proxy
	}
	return proxy
}
