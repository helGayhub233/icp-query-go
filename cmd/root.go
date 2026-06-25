package cmd

import (
	"fmt"

	"github.com/imxw/icp-query-go/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	cfg     *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "icpcli",
	Short: "ICP备案查询工具",
	Long: `ICP备案查询工具 - 支持网站、APP、小程序、快应用备案查询。
	支持 CLI 查询和 MCP Server 两种运行模式。`,
	SilenceUsage:  true,
	SilenceErrors: false,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return loadConfig()
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "config.yml", "配置文件路径")
}

// envKeys maps viper config keys to their env var names.
// viper requires explicit BindEnv for reliable env var → Unmarshal behavior.
var envKeys = []string{
	"timeout",
	"concurrency",
	"rate_limit.enabled",
	"rate_limit.query_per_min",
	"rate_limit.blacklist_per_min",
	"rate_limit.max_concurrent",
	"proxy.tunnel",
	"proxy.pool.url",
	"proxy.pool.size",
	"proxy.pool.ipv6",
}

// loadConfig reads and validates the configuration. Returns error instead of calling os.Exit.
func loadConfig() error {
	cfg = config.DefaultConfig()

	v := viper.New()
	v.SetConfigFile(cfgFile)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("ICP")

	// Explicitly bind env vars so AutomaticEnv works with Unmarshal
	for _, key := range envKeys {
		if err := v.BindEnv(key); err != nil {
			return fmt.Errorf("bind env %s: %w", key, err)
		}
	}

	v.AutomaticEnv()

	if err := v.ReadInConfig(); err == nil {
		if err := v.Unmarshal(cfg); err != nil {
			return fmt.Errorf("解析配置文件失败: %w", err)
		}
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("配置校验失败: %w", err)
	}
	return nil
}
