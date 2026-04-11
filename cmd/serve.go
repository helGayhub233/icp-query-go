package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/imxw/icp-query-go/internal/beian"
	"github.com/imxw/icp-query-go/internal/store"
	"github.com/imxw/icp-query-go/server"
	"github.com/spf13/cobra"
)

var (
	serveHost string
	servePort int
	noUI      bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "启动 HTTP API 服务",
	Long: `启动 ICP 备案查询 HTTP 服务，提供 Web UI 和 REST API。

示例:
  icpcli serve
  icpcli serve -p 9090 --no-ui`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create a copy so CLI flags don't mutate the shared config
		serveCfg := *cfg
		if cmd.Flags().Changed("host") {
			serveCfg.Host = serveHost
		}
		if cmd.Flags().Changed("port") {
			serveCfg.Port = servePort
		}

		handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
		slog.SetDefault(slog.New(handler))

		slog.Info("server starting",
			"host", serveCfg.Host,
			"port", serveCfg.Port,
		)

		b := beian.New(&serveCfg)

		db, err := store.New("icp_history.db")
		if err != nil {
			return fmt.Errorf("database init failed: %w", err)
		}
		defer db.Close()

		srv := server.New(&serveCfg, b, db, !noUI)

		webUI := !noUI
		if webUI {
			displayHost := serveCfg.Host
			if displayHost == "0.0.0.0" {
				displayHost = "127.0.0.1"
			}
			fmt.Fprintf(os.Stderr, "\nweb ui: http://%s:%d\n\n", displayHost, serveCfg.Port)
		}

		return srv.Run(server.WebFS)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVarP(&serveHost, "host", "H", "0.0.0.0", "监听地址")
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 8080, "监听端口")
	serveCmd.Flags().BoolVar(&noUI, "no-ui", false, "禁用 Web UI")
}
