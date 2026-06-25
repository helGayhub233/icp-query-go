package mcpserver

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"
)

// RateLimiter provides concurrency and token-bucket controls for MCP tool calls.
// Restricted tools share a concurrency gate and keep per-tool rate limiters.
type RateLimiter struct {
	queryLimiter     *rate.Limiter
	blacklistLimiter *rate.Limiter
	concurrencyGate  *semaphore.Weighted
}

// NewRateLimiter creates a RateLimiter with per-second limits derived from
// per-minute quotas and a shared MCP tool concurrency gate.
func NewRateLimiter(queryPerMin, blacklistPerMin, maxConcurrent int) *RateLimiter {
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}
	return &RateLimiter{
		// rate.Limit(n)/60 = n permits per second
		queryLimiter:     rate.NewLimiter(rate.Limit(queryPerMin)/60, 1),
		blacklistLimiter: rate.NewLimiter(rate.Limit(blacklistPerMin)/60, 1),
		concurrencyGate:  semaphore.NewWeighted(int64(maxConcurrent)),
	}
}

// Middleware returns an MCP receiving middleware that queues restricted tool calls.
// Non-tool methods (initialize, notifications, etc.) pass through unrestricted.
func (rl *RateLimiter) Middleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}

			toolName := extractToolName(req)
			limiter := rl.limiterFor(toolName)
			if limiter == nil {
				return next(ctx, method, req)
			}

			if err := rl.concurrencyGate.Acquire(ctx, 1); err != nil {
				slog.Warn("concurrency wait cancelled", "tool", toolName, "error", err)
				return nil, err
			}
			defer rl.concurrencyGate.Release(1)

			if err := limiter.Wait(ctx); err != nil {
				slog.Warn("rate limit wait cancelled", "tool", toolName, "error", err)
				return nil, err
			}

			return next(ctx, method, req)
		}
	}
}

// limiterFor returns the rate limiter for the given tool, or nil if unrestricted.
func (rl *RateLimiter) limiterFor(toolName string) *rate.Limiter {
	switch toolName {
	case "icp_query":
		return rl.queryLimiter
	case "icp_blacklist":
		return rl.blacklistLimiter
	default:
		return nil
	}
}

// extractToolName parses the tool name from a tools/call request.
func extractToolName(req mcp.Request) string {
	params := req.GetParams()
	if callParams, ok := params.(*mcp.CallToolParamsRaw); ok {
		return callParams.Name
	}
	return ""
}
