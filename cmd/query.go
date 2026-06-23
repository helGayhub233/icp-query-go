package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/imxw/icp-query-go/internal/beian"
	"github.com/spf13/cobra"
)

var (
	queryName  string
	queryType  string
	queryPage  int
	querySize  int
	queryProxy string
)

var queryCmd = &cobra.Command{
	Use:   "query [查询内容]",
	Short: "单条 ICP 备案查询 (CLI 直接查)",
	Long: `直接在命令行查询 ICP 备案信息，默认输出 JSON 格式。
退出码: 0=成功, 10=查询失败, 20=网络错误

示例:
  icpcli query baidu.com
  icpcli query "深圳市腾讯计算机系统有限公司" -t web
  icpcli query 微信 -t app
  icpcli query -n "baidu.com" -t web`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Accept query content from positional arg or -n flag
		keyword := queryName
		if len(args) > 0 {
			keyword = args[0]
		}
		if keyword == "" {
			return ExitCodeError(2, "请提供查询内容，例如: icpcli query baidu.com")
		}

		// CLI mode: suppress slog noise, only show errors
		handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})
		slog.SetDefault(slog.New(handler))

		b := beian.New(cfg)
		var data map[string]any
		var err error

		ctx := cmd.Context()

		if serviceType, ok := beian.ParseServiceType(queryType); ok {
			data, err = b.Query(ctx, beian.QueryRequest{
				Name:        keyword,
				ServiceType: serviceType,
				PageNum:     queryPage,
				PageSize:    querySize,
				Proxy:       queryProxy,
			})
		} else if serviceType, ok := beian.ParseBlacklistServiceType(queryType); ok {
			data, err = b.QueryBlacklist(ctx, beian.BlacklistRequest{
				Name:        keyword,
				ServiceType: serviceType,
				Proxy:       queryProxy,
			})
		} else {
			return fmt.Errorf("不支持的查询类型: %s (可选: web, app, mapp, kapp, bweb, bapp, bmapp, bkapp)", queryType)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "查询错误: %v\n", err)
			return ExitCodeError(20, "网络错误: %w", err)
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(data)

		code, ok := beian.ResponseCode(data)
		if ok && code != 0 && code != 200 {
			return ExitCodeError(10, "查询失败: code=%v", code)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(queryCmd)

	queryCmd.Flags().StringVarP(&queryName, "name", "n", "", "查询内容 (公司名/域名/备案号/App名)")
	queryCmd.Flags().StringVarP(&queryType, "type", "t", "web", "查询类型: web|app|mapp|kapp|bweb|bapp|bmapp|bkapp")
	queryCmd.Flags().IntVar(&queryPage, "page", 0, "页码")
	queryCmd.Flags().IntVar(&querySize, "size", 26, "每页条数 (最大26)")
	queryCmd.Flags().StringVar(&queryProxy, "proxy", "", "代理地址")
}
