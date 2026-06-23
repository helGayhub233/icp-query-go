package beian

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/imxw/icp-query-go/internal/config"
	"github.com/imxw/icp-query-go/internal/netutil"
	"github.com/imxw/icp-query-go/internal/proxy"
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

// ServiceType is the MIIT serviceType value used by ICP query APIs.
type ServiceType int

const (
	ServiceTypeWeb     ServiceType = 1
	ServiceTypeApp     ServiceType = 6
	ServiceTypeMiniApp ServiceType = 7
	ServiceTypeKuaiApp ServiceType = 8
)

func (t ServiceType) valid() bool {
	switch t {
	case ServiceTypeWeb, ServiceTypeApp, ServiceTypeMiniApp, ServiceTypeKuaiApp:
		return true
	default:
		return false
	}
}

// QueryRequest is the input for a normal ICP query.
type QueryRequest struct {
	Name        string
	ServiceType ServiceType
	PageNum     int
	PageSize    int
	Proxy       string
}

// BlacklistRequest is the input for a blacklist query.
type BlacklistRequest struct {
	Name        string
	ServiceType ServiceType
	Proxy       string
}

// ParseServiceType converts CLI/MCP type names into serviceType values.
func ParseServiceType(value string) (ServiceType, bool) {
	switch value {
	case "web":
		return ServiceTypeWeb, true
	case "app":
		return ServiceTypeApp, true
	case "mapp":
		return ServiceTypeMiniApp, true
	case "kapp":
		return ServiceTypeKuaiApp, true
	default:
		return 0, false
	}
}

// ParseBlacklistServiceType converts blacklist type names into serviceType values.
func ParseBlacklistServiceType(value string) (ServiceType, bool) {
	switch value {
	case "bweb":
		return ServiceTypeWeb, true
	case "bapp":
		return ServiceTypeApp, true
	case "bmapp":
		return ServiceTypeMiniApp, true
	case "bkapp":
		return ServiceTypeKuaiApp, true
	default:
		return 0, false
	}
}

// Beian handles ICP registration queries.
type Beian struct {
	cfg *config.Config

	tokenMu     sync.RWMutex
	token       string
	tokenExpire int64 // unix milliseconds

	localIPv6Addresses []string
	ipv6Index          int
	ipv6Mu             sync.Mutex
	blockedIPs         sync.Map

	proxyPool *proxy.Pool

	// Reusable HTTP transport for non-IPv6 requests.
	// IPv6 requests create a cloned transport with a custom dialer.
	httpTransport *http.Transport
	httpClient    *http.Client
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

	// Shared HTTP transport for connection reuse
	b.httpTransport = &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 30,
		IdleConnTimeout:     90 * time.Second,
	}
	b.httpClient = &http.Client{
		Transport: b.httpTransport,
		Timeout:   time.Duration(cfg.Timeout) * time.Second,
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

func (b *Beian) QueryWeb(ctx context.Context, name string, pageNum, pageSize int, proxy string) (map[string]any, error) {
	return b.Query(ctx, QueryRequest{Name: name, ServiceType: ServiceTypeWeb, PageNum: pageNum, PageSize: pageSize, Proxy: proxy})
}

func (b *Beian) QueryApp(ctx context.Context, name string, pageNum, pageSize int, proxy string) (map[string]any, error) {
	return b.Query(ctx, QueryRequest{Name: name, ServiceType: ServiceTypeApp, PageNum: pageNum, PageSize: pageSize, Proxy: proxy})
}

func (b *Beian) QueryMiniApp(ctx context.Context, name string, pageNum, pageSize int, proxy string) (map[string]any, error) {
	return b.Query(ctx, QueryRequest{Name: name, ServiceType: ServiceTypeMiniApp, PageNum: pageNum, PageSize: pageSize, Proxy: proxy})
}

func (b *Beian) QueryKuaiApp(ctx context.Context, name string, pageNum, pageSize int, proxy string) (map[string]any, error) {
	return b.Query(ctx, QueryRequest{Name: name, ServiceType: ServiceTypeKuaiApp, PageNum: pageNum, PageSize: pageSize, Proxy: proxy})
}

func (b *Beian) QueryBlackWeb(ctx context.Context, name string, proxy string) (map[string]any, error) {
	return b.QueryBlacklist(ctx, BlacklistRequest{Name: name, ServiceType: ServiceTypeWeb, Proxy: proxy})
}

func (b *Beian) QueryBlackApp(ctx context.Context, name string, proxy string) (map[string]any, error) {
	return b.QueryBlacklist(ctx, BlacklistRequest{Name: name, ServiceType: ServiceTypeApp, Proxy: proxy})
}

func (b *Beian) QueryBlackMiniApp(ctx context.Context, name string, proxy string) (map[string]any, error) {
	return b.QueryBlacklist(ctx, BlacklistRequest{Name: name, ServiceType: ServiceTypeMiniApp, Proxy: proxy})
}

func (b *Beian) QueryBlackKuaiApp(ctx context.Context, name string, proxy string) (map[string]any, error) {
	return b.QueryBlacklist(ctx, BlacklistRequest{Name: name, ServiceType: ServiceTypeKuaiApp, Proxy: proxy})
}

// --- Core logic ---

func (b *Beian) Query(ctx context.Context, req QueryRequest) (map[string]any, error) {
	if !req.ServiceType.valid() {
		return nil, fmt.Errorf("不支持的服务类型: %d", req.ServiceType)
	}
	data, err := b.getBeian(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}
	return normalizeResponse(data), nil
}

func (b *Beian) QueryBlacklist(ctx context.Context, req BlacklistRequest) (map[string]any, error) {
	if !req.ServiceType.valid() {
		return nil, fmt.Errorf("不支持的服务类型: %d", req.ServiceType)
	}
	data, err := b.getBlackBeian(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}
	return normalizeResponse(data), nil
}

func (b *Beian) getBeian(ctx context.Context, req QueryRequest) (map[string]any, error) {
	proxy := b.resolveProxy(req.Proxy)
	info := map[string]any{
		"pageNum":     req.PageNum,
		"pageSize":    req.PageSize,
		"unitName":    req.Name,
		"serviceType": req.ServiceType,
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

	if req.ServiceType != ServiceTypeWeb {
		b.fetchDetails(ctx, result, req.ServiceType, ac, proxy)
	}
	return result, nil
}

func (b *Beian) getBlackBeian(ctx context.Context, req BlacklistRequest) (map[string]any, error) {
	proxy := b.resolveProxy(req.Proxy)
	info := map[string]any{}
	if req.ServiceType == ServiceTypeWeb {
		info["domainName"] = req.Name
	} else {
		info["serviceName"] = req.Name
		info["serviceType"] = req.ServiceType
	}

	targetURL := blackQueryURL
	if req.ServiceType != ServiceTypeWeb {
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

func (b *Beian) fetchDetails(ctx context.Context, result map[string]any, serviceType ServiceType, ac *authContext, proxy string) {
	params, ok := responseParams(result)
	if !ok {
		return
	}
	items, ok := params["list"].([]any)
	if !ok || len(items) == 0 {
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

			dataID, ok := stringValue(itemMap["dataId"])
			if !ok {
				detailedList[i] = item
				return nil
			}
			detail, err := b.getAppAndMiniDetail(gctx, dataID, serviceType, ac, proxy)
			if err != nil || !ResponseSuccess(detail) {
				slog.Warn("detail fetch failed", "dataId", itemMap["dataId"], "error", err, "response", detail)
				detailedList[i] = item
				return nil
			}
			if dParams, ok := responseParams(detail); ok {
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

func (b *Beian) getAppAndMiniDetail(ctx context.Context, dataID string, serviceType ServiceType, ac *authContext, proxy string) (map[string]any, error) {
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
