package cmd

import (
	"fmt"
	"os"

	"github.com/imxw/icp-query-go/internal/beian"
	mcpserver "github.com/imxw/icp-query-go/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "启动 MCP server (stdio transport)",
	Long: `启动 Model Context Protocol (MCP) server，通过 stdio 与 AI agent 通信。

AI agent 可通过 MCP 直接调用 ICP 查询能力，无需启动 HTTP 服务。

在 Claude Code 中配置:
  claude mcp add icp-query -- /path/to/icpcli mcp

示例:
  icpcli mcp`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "MCP server started (stdio transport)")

		b := beian.New(cfg)
		mcpSrv := mcpserver.New(cfg, b)

		srv := mcp.NewServer(&mcp.Implementation{
			Name:    "icp-query",
			Version: Version,
		}, nil)

		mcpSrv.RegisterTools(srv)

		transport := &mcp.StdioTransport{}
		ctx := cmd.Context()
		if err := srv.Run(ctx, transport); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
