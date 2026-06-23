package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/imxw/icp-query-go/internal/beian"
	"github.com/imxw/icp-query-go/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP server for ICP query tools.
type Server struct {
	cfg   *config.Config
	beian *beian.Beian
}

// New creates a new MCP server wrapper.
func New(cfg *config.Config, b *beian.Beian) *Server {
	return &Server{cfg: cfg, beian: b}
}

// RegisterTools registers all ICP query tools with the MCP server.
func (s *Server) RegisterTools(srv *mcp.Server) {
	srv.AddTool(&mcp.Tool{
		Name:        "icp_query",
		Description: "查询工信部 ICP 备案信息。支持网站、APP、小程序、快应用的备案查询。",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name":      {"type": "string", "description": "查询关键词（公司名/域名/APP名）"},
				"type":      {"type": "string", "enum": ["web","app","mapp","kapp"], "description": "查询类型: web=网站, app=APP, mapp=小程序, kapp=快应用", "default": "web"},
				"page":      {"type": "integer", "description": "页码", "default": 1},
				"page_size": {"type": "integer", "description": "每页条数(最大26)", "default": 26}
			},
			"required": ["name"]
		}`),
	}, s.handleICPQuery)

	srv.AddTool(&mcp.Tool{
		Name:        "icp_blacklist",
		Description: "查询工信部违法违规应用黑名单。支持网站、APP、小程序、快应用。",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {"type": "string", "description": "查询关键词（域名/APP名）"},
				"type": {"type": "string", "enum": ["bweb","bapp","bmapp","bkapp"], "description": "查询类型: bweb=违规域名, bapp=违规App, bmapp=违规小程序, bkapp=违规快应用", "default": "bweb"}
			},
			"required": ["name"]
		}`),
	}, s.handleICPBlacklist)

	srv.AddTool(&mcp.Tool{
		Name:        "config_show",
		Description: "查看当前 ICP 查询服务的配置信息。",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, s.handleConfigShow)
}

func (s *Server) handleICPQuery(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Page     int    `json:"page"`
		PageSize int    `json:"page_size"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Type == "" {
		args.Type = "web"
	}
	if args.PageSize == 0 {
		args.PageSize = 26
	}

	slog.Info("MCP icp_query", "name", args.Name, "type", args.Type)

	serviceType, ok := beian.ParseServiceType(args.Type)
	if !ok {
		return nil, fmt.Errorf("unsupported type: %s", args.Type)
	}
	data, err := s.beian.Query(ctx, beian.QueryRequest{
		Name:        args.Name,
		ServiceType: serviceType,
		PageNum:     args.Page,
		PageSize:    args.PageSize,
	})

	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("查询失败: %v", err)}},
			IsError: true,
		}, nil
	}

	resultJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(resultJSON)}},
	}, nil
}

func (s *Server) handleICPBlacklist(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Type == "" {
		args.Type = "bweb"
	}

	slog.Info("MCP icp_blacklist", "name", args.Name, "type", args.Type)

	serviceType, ok := beian.ParseBlacklistServiceType(args.Type)
	if !ok {
		return nil, fmt.Errorf("unsupported type: %s", args.Type)
	}
	data, err := s.beian.QueryBlacklist(ctx, beian.BlacklistRequest{
		Name:        args.Name,
		ServiceType: serviceType,
	})

	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("查询失败: %v", err)}},
			IsError: true,
		}, nil
	}

	resultJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(resultJSON)}},
	}, nil
}

func (s *Server) handleConfigShow(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfgData := map[string]any{
		"timeout":     s.cfg.Timeout,
		"concurrency": s.cfg.Concurrency,
		"rate_limit": map[string]any{
			"enabled":           s.cfg.RateLimit.Enabled,
			"query_per_min":     s.cfg.RateLimit.QueryPerMin,
			"blacklist_per_min": s.cfg.RateLimit.BlacklistPerMin,
		},
		"proxy": map[string]any{
			"tunnel": s.cfg.Proxy.Tunnel,
			"pool": map[string]any{
				"url":  s.cfg.Proxy.Pool.URL,
				"size": s.cfg.Proxy.Pool.Size,
				"ipv6": s.cfg.Proxy.Pool.IPv6,
			},
		},
	}
	resultJSON, err := json.MarshalIndent(cfgData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(resultJSON)}},
	}, nil
}
