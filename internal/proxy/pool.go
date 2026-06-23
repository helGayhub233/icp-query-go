package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/imxw/icp-query-go/internal/config"
)

const (
	poolRefreshInterval  = 3 * time.Second
	poolTTL              = 92 * time.Second // (100 - 8) seconds
	poolCheckTimeout     = 500 * time.Millisecond
	poolCheckConcurrency = 20
)

// Pool manages external HTTP proxy addresses.
type Pool struct {
	cfg           *config.Pool
	cache         *TTLCache[string, time.Time]
	mu            sync.Mutex
	client        *http.Client
	stopCh        chan struct{}
	cleanupStopCh chan struct{}
	stopOnce      sync.Once
}

// NewPool creates a proxy pool from configuration.
func NewPool(cfg *config.Pool) *Pool {
	if cfg == nil || cfg.URL == "" {
		return nil
	}

	return &Pool{
		cfg:   cfg,
		cache: NewTTLCache[string, time.Time](),
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
		stopCh: make(chan struct{}),
	}
}

// Start begins the background refresh goroutine.
func (p *Pool) Start() {
	p.cleanupStopCh = p.cache.StartCleanupGoroutine(poolRefreshInterval)
	go p.refreshLoop(poolRefreshInterval)
	slog.Info("proxy pool started", "interval", poolRefreshInterval, "max", p.cfg.Size)
}

// Stop signals the background goroutine to stop.
func (p *Pool) Stop() {
	p.stopOnce.Do(func() {
		close(p.stopCh)
		if p.cleanupStopCh != nil {
			close(p.cleanupStopCh)
		}
	})
}

// GetProxy returns a random proxy URL from the pool, or empty string if none available.
func (p *Pool) GetProxy() string {
	keys := p.cache.Keys()
	if len(keys) == 0 {
		return ""
	}
	addr := keys[rand.IntN(len(keys))]
	return "http://" + addr
}

// Size returns the number of proxies currently in the pool.
func (p *Pool) Size() int {
	return p.cache.Len()
}

// --- internal ---

func (p *Pool) refreshLoop(interval time.Duration) {
	p.refresh(context.Background())

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.refresh(context.Background())
		case <-p.stopCh:
			return
		}
	}
}

func (p *Pool) refresh(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cache.Len() >= p.cfg.Size {
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.URL, nil)
	if err != nil {
		slog.Error("proxy pool request failed", "error", err)
		return
	}

	resp, err := p.client.Do(req)
	if err != nil {
		slog.Error("proxy pool fetch failed", "error", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("proxy pool read failed", "error", err)
		return
	}

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	var proxies []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			proxies = append(proxies, line)
		}
	}

	if len(proxies) == 0 {
		slog.Error("proxy pool extracted 0 IPs")
		return
	}

	now := time.Now()
	p.validateAndAdd(proxies, now, poolTTL)
	slog.Info("proxy pool refreshed", "count", p.cache.Len())
}

func (p *Pool) validateAndAdd(proxies []string, now time.Time, ttl time.Duration) {
	sem := make(chan struct{}, poolCheckConcurrency)
	var wg sync.WaitGroup

	for _, addr := range proxies {
		if p.cache.Len() >= p.cfg.Size {
			break
		}

		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if p.checkProxy(address) {
				p.cache.Set(address, now, ttl)
				slog.Info("proxy validated", "addr", address)
			}
		}(addr)
	}
	wg.Wait()
}

func (p *Pool) checkProxy(addr string) bool {
	client := &http.Client{
		Timeout: poolCheckTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	proxyURL := fmt.Sprintf("http://%s", addr)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://ifconfig.me/ip", nil)
	if err != nil {
		return false
	}

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		slog.Debug("proxy URL parse failed", "addr", addr, "error", err)
		return false
	}
	client.Transport.(*http.Transport).Proxy = http.ProxyURL(parsedURL)

	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("proxy validation failed", "addr", addr, "error", err)
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}
