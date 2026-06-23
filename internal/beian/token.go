package beian

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

const (
	authURL = "https://hlwicpfwc.miit.gov.cn/icpproject_query/api/auth"
)

// getToken returns (token, headers, error). Uses cached token if still valid.
func (b *Beian) getToken(ctx context.Context, proxy string) (string, map[string]string, error) {
	baseHeader := map[string]string{
		"User-Agent": userAgent,
		"Origin":     "https://beian.miit.gov.cn",
		"Referer":    "https://beian.miit.gov.cn/",
		"Accept":     "application/json, text/plain, */*",
	}

	b.tokenMu.RLock()
	if b.tokenExpire > time.Now().UnixMilli() {
		token := b.token
		b.tokenMu.RUnlock()
		baseHeader["Cookie"] = fmt.Sprintf("__jsluid_s=%s", randomHex(32))
		return token, baseHeader, nil
	}
	b.tokenMu.RUnlock()

	ts := time.Now().UnixMilli()
	authSecret := "testtest" + fmt.Sprintf("%d", ts)
	hash := md5.Sum([]byte(authSecret))
	authKey := hex.EncodeToString(hash[:])

	baseHeader["Cookie"] = fmt.Sprintf("__jsluid_s=%s", randomHex(32))
	baseHeader["Content-Type"] = "application/x-www-form-urlencoded"

	formBody := []byte(fmt.Sprintf("authKey=%s&timeStamp=%d", authKey, ts))

	res, err := b.doPost(ctx, authURL, formBody, baseHeader, proxy)
	if err != nil {
		slog.Warn("getToken failed", "error", err)
		return "", nil, fmt.Errorf("请求auth接口失败: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(res, &result); err != nil {
		return "", nil, fmt.Errorf("解析token响应失败: %w", err)
	}

	params, ok := result["params"].(map[string]any)
	if !ok {
		return "", nil, fmt.Errorf("invalid token response")
	}

	token, ok := stringValue(params["bussiness"])
	if !ok {
		return "", nil, fmt.Errorf("invalid token response: missing bussiness")
	}
	expire, ok := numberValue(params["expire"])
	if !ok {
		return "", nil, fmt.Errorf("invalid token response: missing expire")
	}

	b.tokenMu.Lock()
	b.token = token
	b.tokenExpire = time.Now().UnixMilli() + int64(expire)
	b.tokenMu.Unlock()

	return token, baseHeader, nil
}
