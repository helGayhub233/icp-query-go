package beian

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"time"
)

const (
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.41 Safari/537.36 Edg/101.0.1210.32"
)

// --- HTTP helpers ---

func (b *Beian) doPost(ctx context.Context, url string, body []byte, headers map[string]string, proxy string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request for %s: %w", url, err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := b.makeHTTPClient(proxy)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute POST to %s: %w", url, err)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (b *Beian) makeHTTPClient(proxy string) *http.Client {
	// Bind to local IPv6 if available and no proxy
	if proxy == "" && len(b.localIPv6Addresses) > 0 {
		if ipv6 := b.getNextIPv6(); ipv6 != "" {
			// Clone transport with custom dialer for IPv6 binding
			ipv6Transport := b.httpTransport.Clone()
			ipv6Transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				d := net.Dialer{
					LocalAddr: &net.TCPAddr{IP: net.ParseIP(ipv6)},
					Timeout:   30 * time.Second,
				}
				slog.Info("using local IPv6", "ip", ipv6)
				return d.DialContext(ctx, network, addr)
			}
			return &http.Client{Transport: ipv6Transport, Timeout: b.httpClient.Timeout}
		}
	}

	// Return the shared client for normal requests
	return b.httpClient
}

// --- IPv6 rotation ---

func (b *Beian) getNextIPv6() string {
	b.ipv6Mu.Lock()
	defer b.ipv6Mu.Unlock()

	if len(b.localIPv6Addresses) == 0 {
		return ""
	}

	for attempts := 0; attempts < len(b.localIPv6Addresses)*2; attempts++ {
		if b.ipv6Index >= len(b.localIPv6Addresses) {
			b.ipv6Index = 0
		}
		ipv6 := b.localIPv6Addresses[b.ipv6Index]
		b.ipv6Index++

		if !b.isIPBlocked(ipv6) {
			return ipv6
		}
	}

	slog.Warn("all IPv6 addresses blocked")
	return ""
}

func (b *Beian) isIPBlocked(ip string) bool {
	t, ok := b.blockedIPs.Load(ip)
	if !ok {
		return false
	}
	blockTime, ok := t.(time.Time)
	if !ok {
		b.blockedIPs.Delete(ip)
		return false
	}
	if time.Since(blockTime) > 5*time.Minute {
		b.blockedIPs.Delete(ip)
		return false
	}
	return true
}

// --- Utility ---

func randomHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(rand.IntN(256))
	}
	return hex.EncodeToString(b)
}

func generateClientUID() string {
	chars := "0123456789abcdef"
	id := make([]byte, 36)
	for i := range id {
		id[i] = chars[rand.IntN(16)]
	}
	id[14] = '4'
	v := 3 & (int(id[19]) & 0xf)
	id[19] = chars[v|8]
	id[8] = '-'
	id[13] = '-'
	id[18] = '-'
	id[23] = '-'
	return "point-" + string(id)
}

func isBlocked(body []byte) bool {
	return bytes.Contains(body, []byte("当前访问疑似黑客攻击"))
}
