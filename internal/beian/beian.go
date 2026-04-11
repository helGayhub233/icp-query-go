package beian

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/imxw/icp-query-go/internal/config"
	"github.com/imxw/icp-query-go/internal/netutil"
	"github.com/imxw/icp-query-go/internal/proxy"
	"github.com/spf13/cast"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const (
	queryByConditionURL = "https://hlwicpfwc.miit.gov.cn/icpproject_query/api/icpAbbreviateInfo/queryByCondition"
	blackQueryURL       = "https://hlwicpfwc.miit.gov.cn/icpproject_query/api/blackListDomain/queryByCondition"
	blackAppMiniURL     = "https://hlwicpfwc.miit.gov.cn/icpproject_query/api/blackListDomain/queryByCondition_appAndMini"
	detailByAppMiniURL  = "https://hlwicpfwc.miit.gov.cn/icpproject_query/api/icpAbbreviateInfo/queryDetailByAppAndMiniId"

	msgChuangyuDun = "当前访问已被创宇盾拦截"
)

// queryType maps service type index to serviceType value.
var queryTypes = map[int]int{
	0: 1, // 网站
	1: 6, // APP
	2: 7, // 小程序
	3: 8, // 快应用
}

// Beian handles ICP registration queries.
type Beian struct {
	cfg *config.Config

	token       string
	tokenExpire int64 // unix milliseconds

	localIPv6Addresses []string
	ipv6Index          int
	ipv6Mu             sync.Mutex
	blockedIPs         sync.Map

	proxyPool *proxy.Pool
}

// New creates a new Beian instance.
func New(cfg *config.Config) *Beian {
	b := &Beian{cfg: cfg}

	if cfg.Proxy.Pool.IPv6 {
		b.localIPv6Addresses = netutil.GetLocalIPv6Addresses()
	}

	// Initialize proxy pool if external API is configured
	if pool := proxy.NewPool(&cfg.Proxy.Pool); pool != nil {
		b.proxyPool = pool
		pool.Start()
	}

	return b
}

// resolveProxy picks a proxy using priority: caller-provided > local IPv6 > tunnel > pool > direct.
func (b *Beian) resolveProxy(proxy string) string {
	if proxy != "" {
		return proxy
	}
	if len(b.localIPv6Addresses) > 0 {
		return "" // IPv6 binding handled in makeHTTPClient
	}
	if b.cfg.Proxy.Tunnel != "" {
		return b.cfg.Proxy.Tunnel
	}
	if b.proxyPool != nil {
		if p := b.proxyPool.GetProxy(); p != "" {
			return p
		}
	}
	return ""
}

// authContext bundles the authentication parameters passed between query methods.
type authContext struct {
	pUUID   string
	token   string
	sign    string
	headers map[string]string
}

// Public query methods — all return (map[string]any, error) for now.
// P5 note: these will later return typed structs, but the API response
// from MIIT is dynamic, so map[string]any is pragmatic here.

func (b *Beian) QueryWeb(ctx context.Context, name string, pageNum, pageSize int, proxy string) (map[string]any, error) {
	return b.autoGet(ctx, name, 0, pageNum, pageSize, proxy, true)
}

func (b *Beian) QueryApp(ctx context.Context, name string, pageNum, pageSize int, proxy string) (map[string]any, error) {
	return b.autoGet(ctx, name, 1, pageNum, pageSize, proxy, true)
}

func (b *Beian) QueryMiniApp(ctx context.Context, name string, pageNum, pageSize int, proxy string) (map[string]any, error) {
	return b.autoGet(ctx, name, 2, pageNum, pageSize, proxy, true)
}

func (b *Beian) QueryKuaiApp(ctx context.Context, name string, pageNum, pageSize int, proxy string) (map[string]any, error) {
	return b.autoGet(ctx, name, 3, pageNum, pageSize, proxy, true)
}

func (b *Beian) QueryBlackWeb(ctx context.Context, name string, proxy string) (map[string]any, error) {
	return b.autoGet(ctx, name, 0, 0, 0, proxy, false)
}

func (b *Beian) QueryBlackApp(ctx context.Context, name string, proxy string) (map[string]any, error) {
	return b.autoGet(ctx, name, 1, 0, 0, proxy, false)
}

func (b *Beian) QueryBlackMiniApp(ctx context.Context, name string, proxy string) (map[string]any, error) {
	return b.autoGet(ctx, name, 2, 0, 0, proxy, false)
}

func (b *Beian) QueryBlackKuaiApp(ctx context.Context, name string, proxy string) (map[string]any, error) {
	return b.autoGet(ctx, name, 3, 0, 0, proxy, false)
}

// --- Core logic ---

func (b *Beian) autoGet(ctx context.Context, name string, sp, pageNum, pageSize int, proxy string, normal bool) (map[string]any, error) {
	proxy = b.resolveProxy(proxy)

	var data map[string]any
	var err error

	if normal {
		data, err = b.getBeian(ctx, name, sp, pageNum, pageSize, proxy)
	} else {
		data, err = b.getBlackBeian(ctx, name, sp, proxy)
	}

	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}

	code, ok := data["code"].(float64)
	if !ok {
		return data, nil
	}
	if code == 500 {
		return map[string]any{"code": 122, "message": "工信部服务器异常"}, nil
	}

	return data, nil
}

func (b *Beian) getBeian(ctx context.Context, name string, sp, pageNum, pageSize int, proxy string) (map[string]any, error) {
	serviceType := queryTypes[sp]
	info := map[string]any{
		"pageNum":     pageNum,
		"pageSize":    pageSize,
		"unitName":    name,
		"serviceType": serviceType,
	}

	cr, err := b.checkImg(ctx, proxy)
	if err != nil {
		return nil, fmt.Errorf("打码失败: %w", err)
	}

	body, err := json.Marshal(info)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	ac := &authContext{
		pUUID:   cr.PUUID,
		token:   cr.Token,
		sign:    cr.Sign,
		headers: cr.Headers,
	}
	ac.headers["Content-Length"] = fmt.Sprintf("%d", len(body))
	ac.headers["uuid"] = ac.pUUID
	ac.headers["token"] = ac.token
	ac.headers["sign"] = ac.sign

	res, err := b.doPost(ctx, queryByConditionURL, body, ac.headers, proxy)
	if err != nil {
		return nil, fmt.Errorf("query by condition: %w", err)
	}
	if isBlocked(res) {
		return nil, fmt.Errorf(msgChuangyuDun)
	}

	var result map[string]any
	if err := json.Unmarshal(res, &result); err != nil {
		return nil, fmt.Errorf("parse query response: %w", err)
	}

	if sp >= 1 && sp <= 3 {
		b.fetchDetails(ctx, result, sp, ac, proxy)
	}
	return result, nil
}

func (b *Beian) getBlackBeian(ctx context.Context, name string, sp int, proxy string) (map[string]any, error) {
	info := map[string]any{}
	if sp == 0 {
		info["domainName"] = name
	} else {
		info["serviceName"] = name
		info["serviceType"] = queryTypes[sp]
	}

	targetURL := blackQueryURL
	if sp != 0 {
		targetURL = blackAppMiniURL
	}

	cr, err := b.checkImg(ctx, proxy)
	if err != nil {
		return nil, fmt.Errorf("打码失败: %w", err)
	}

	body, err := json.Marshal(info)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	ac := &authContext{
		pUUID:   cr.PUUID,
		token:   cr.Token,
		sign:    cr.Sign,
		headers: cr.Headers,
	}
	ac.headers["Content-Length"] = fmt.Sprintf("%d", len(body))
	ac.headers["uuid"] = ac.pUUID
	ac.headers["token"] = ac.token
	ac.headers["sign"] = ac.sign

	res, err := b.doPost(ctx, targetURL, body, ac.headers, proxy)
	if err != nil {
		return nil, fmt.Errorf("blacklist query: %w", err)
	}
	if isBlocked(res) {
		return nil, fmt.Errorf(msgChuangyuDun)
	}

	var result map[string]any
	if err := json.Unmarshal(res, &result); err != nil {
		return nil, fmt.Errorf("parse blacklist response: %w", err)
	}
	return result, nil
}

// --- Detail fetching ---

func (b *Beian) fetchDetails(ctx context.Context, result map[string]any, sp int, ac *authContext, proxy string) {
	params, _ := result["params"].(map[string]any)
	if params == nil {
		return
	}
	items, _ := params["list"].([]any)
	if len(items) == 0 {
		return
	}

	slog.Info("fetching details concurrently", "count", len(items))

	maxCon := b.cfg.Concurrency
	if maxCon > 20 {
		maxCon = 20
	}
	if maxCon > len(items) {
		maxCon = len(items)
	}

	sem := semaphore.NewWeighted(int64(maxCon))
	g, gctx := errgroup.WithContext(ctx)

	serviceType := queryTypes[sp]
	detailedList := make([]any, len(items))

	for i, item := range items {
		i, item := i, item
		itemMap, _ := item.(map[string]any)
		if itemMap == nil || itemMap["dataId"] == nil {
			detailedList[i] = item
			continue
		}

		g.Go(func() error {
			if err := sem.Acquire(gctx, 1); err != nil {
				detailedList[i] = item
				return nil
			}
			defer sem.Release(1)

			detail, err := b.getAppAndMiniDetail(gctx, cast.ToString(itemMap["dataId"]), serviceType, ac, proxy)
			if err != nil || !cast.ToBool(detail["success"]) {
				slog.Warn("detail fetch failed", "dataId", itemMap["dataId"], "error", err, "response", detail)
				detailedList[i] = item
				return nil
			}
			dParams, _ := detail["params"].(map[string]any)
			if dParams != nil {
				detailedList[i] = dParams
			} else {
				detailedList[i] = item
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		slog.Error("detail fetch error", "error", err)
	}

	params["list"] = detailedList
	slog.Info("details fetch done", "total", len(detailedList))
}

func (b *Beian) getAppAndMiniDetail(ctx context.Context, dataID string, serviceType int, ac *authContext, proxy string) (map[string]any, error) {
	info := map[string]any{"dataId": dataID, "serviceType": serviceType}
	body, err := json.Marshal(info)
	if err != nil {
		return nil, fmt.Errorf("marshal detail request: %w", err)
	}

	headers := make(map[string]string, len(ac.headers)+2)
	for k, v := range ac.headers {
		headers[k] = v
	}
	headers["Content-Length"] = fmt.Sprintf("%d", len(body))
	headers["uuid"] = ac.pUUID
	headers["token"] = ac.token
	headers["sign"] = ac.sign

	res, err := b.doPost(ctx, detailByAppMiniURL, body, headers, proxy)
	if err != nil {
		return nil, fmt.Errorf("fetch detail for %v: %w", dataID, err)
	}

	var result map[string]any
	if err := json.Unmarshal(res, &result); err != nil {
		return nil, fmt.Errorf("parse detail response for %v: %w", dataID, err)
	}
	return result, nil
}
