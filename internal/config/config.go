package config

import "fmt"

// Config 配置结构，所有字段均可选，有合理默认值。
// 支持 config.yml / CLI flag / 环境变量 (ICP_ 前缀) 三层覆盖。
type Config struct {
	Port        int    `mapstructure:"port"`
	Host        string `mapstructure:"host"`
	Timeout     int    `mapstructure:"timeout"`       // HTTP 请求超时(秒)
	RetryTimes  int    `mapstructure:"retry_times"`   // 验证码重试次数
	Concurrency int    `mapstructure:"concurrency"`   // 并发详情获取协程数
	Proxy       Proxy  `mapstructure:"proxy"`
}

type Proxy struct {
	Tunnel string `mapstructure:"tunnel"` // 隧道代理 URL
	Pool   Pool   `mapstructure:"pool"`
}

type Pool struct {
	URL     string `mapstructure:"url"`      // 代理池 API 地址
	Size    int    `mapstructure:"size"`     // 最大代理数
	IPv6    bool   `mapstructure:"ipv6"`     // 启用本地 IPv6
	IPv6Num int    `mapstructure:"ipv6_num"` // IPv6 池大小
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Port:        8080,
		Host:        "0.0.0.0",
		Timeout:     30,
		RetryTimes:  10,
		Concurrency: 5,
		Proxy: Proxy{
			Pool: Pool{
				Size:    100,
				IPv6Num: 88,
			},
		},
	}
}

// Validate checks required fields and applies defaults where zero.
func (c *Config) Validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	if c.Timeout <= 0 {
		c.Timeout = 30
	}
	if c.RetryTimes <= 0 {
		c.RetryTimes = 10
	}
	if c.Concurrency <= 0 {
		c.Concurrency = 5
	}
	if c.Proxy.Pool.Size <= 0 {
		c.Proxy.Pool.Size = 100
	}
	if c.Proxy.Pool.IPv6Num <= 0 {
		c.Proxy.Pool.IPv6Num = 88
	}
	return nil
}
