package config

// Config 配置结构，所有字段均可选，有合理默认值。
// 支持 config.yml / CLI flag / 环境变量 (ICP_ 前缀) 三层覆盖。
type Config struct {
	Timeout     int       `mapstructure:"timeout"`     // HTTP 请求超时(秒)
	Concurrency int       `mapstructure:"concurrency"` // 并发详情获取协程数
	RateLimit   RateLimit `mapstructure:"rate_limit"`
	Proxy       Proxy     `mapstructure:"proxy"`
}

// RateLimit controls request throttling for MCP tools.
type RateLimit struct {
	Enabled         bool `mapstructure:"enabled"`           // 是否启用限流
	QueryPerMin     int  `mapstructure:"query_per_min"`     // icp_query 每分钟上限
	BlacklistPerMin int  `mapstructure:"blacklist_per_min"` // icp_blacklist 每分钟上限
	MaxConcurrent   int  `mapstructure:"max_concurrent"`    // MCP 查询工具最大并发数
}

type Proxy struct {
	Tunnel string `mapstructure:"tunnel"` // 隧道代理 URL
	Pool   Pool   `mapstructure:"pool"`
}

type Pool struct {
	URL  string `mapstructure:"url"`  // 代理池 API 地址
	Size int    `mapstructure:"size"` // 最大代理数
	IPv6 bool   `mapstructure:"ipv6"` // 启用本地 IPv6
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Timeout:     30,
		Concurrency: 5,
		RateLimit: RateLimit{
			Enabled:         true,
			QueryPerMin:     5,
			BlacklistPerMin: 3,
			MaxConcurrent:   1,
		},
		Proxy: Proxy{
			Pool: Pool{
				Size: 100,
			},
		},
	}
}

// Validate checks required fields and applies defaults where zero.
func (c *Config) Validate() error {
	if c.Timeout <= 0 {
		c.Timeout = 30
	}
	if c.Concurrency <= 0 {
		c.Concurrency = 5
	}
	if c.Proxy.Pool.Size <= 0 {
		c.Proxy.Pool.Size = 100
	}
	if c.RateLimit.QueryPerMin <= 0 {
		c.RateLimit.QueryPerMin = 5
	}
	if c.RateLimit.BlacklistPerMin <= 0 {
		c.RateLimit.BlacklistPerMin = 3
	}
	if c.RateLimit.MaxConcurrent <= 0 {
		c.RateLimit.MaxConcurrent = 1
	}
	return nil
}
