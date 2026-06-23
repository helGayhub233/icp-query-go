package mcpserver

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/time/rate"
)

// RateLimiter provides token-bucket rate limiting for MCP tool calls.
// Each restricted tool type has its own limiter to prevent excessive calls.
type RateLimiter struct {
	queryLimiter     *rate.Limiter
	blacklistLimiter *rate.Limiter
}

// NewRateLimiter creates a RateLimiter with per-second limits derived from
// per-minute quotas.
func NewRateLimiter(queryPerMin, blacklistPerMin int) *RateLimiter {
	return &RateLimiter{
		// rate.Limit(n)/60 = n permits per second
		queryLimiter:     rate.NewLimiter(rate.Limit(queryPerMin)/60, 1),
		blacklistLimiter: rate.NewLimiter(rate.Limit(blacklistPerMin)/60, 1),
	}
}

// Middleware returns an MCP receiving middleware that rate-limits tool calls.
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

			if !limiter.Allow() {
				slog.Warn("rate limit exceeded", "tool", toolName)
				return nil, fmt.Errorf("rate limit exceeded for %s, please retry later", toolName)
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
