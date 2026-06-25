<h1 align="center">ICP 备案查询工具</h1>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go&logoColor=white" alt="技术栈"/>
  <img src="https://img.shields.io/badge/Platform-macOS | Linux | Windows-lightgrey.svg" alt="支持平台"/>
  <img src="https://visitor-badge.laobi.icu/badge?page_id=helGayhub233.icp-query-go" alt="访问次数"/>
  <a href="https://github.com/helGayhub233/icp-query-go/releases"><img src="https://img.shields.io/github/downloads/helGayhub233/icp-query-go/total" alt="下载次数"/></a>
</p>

<p align="center">
  <code>icpcli</code> 是一个纯 Go 实现的 ICP 备案查询工具，用于网站、App、小程序、快应用备案信息以及违规黑名单查询。
</p>

## 安装

### 下载二进制

前往 [Releases](https://github.com/helGayhub233/icp-query-go/releases) 下载对应系统文件：

```bash
# macOS / Linux
chmod +x icpcli-*
./icpcli-* version
```

Windows 下载 `.exe` 后直接运行。

### 源码编译

```bash
git clone https://github.com/helGayhub233/icp-query-go.git
cd icp-query-go
go build -o icpcli .
./icpcli version
```

### Docker

```bash
docker build -t icp-query .
docker run --rm icp-query version
docker run --rm icp-query query baidu.com
```

## 快速使用

### 单条查询

```bash
icpcli query baidu.com
icpcli query -n baidu.com -t web
icpcli query 微信 -t app
icpcli query "北京百度网讯科技有限公司" -t mapp
icpcli query baidu.com -t bweb
```

查询结果默认输出格式化 JSON。

### 批量查询

```bash
cat > domains.txt <<'EOF'
baidu.com
qq.com
北京百度网讯科技有限公司
EOF

icpcli batch -f domains.txt -t web
icpcli batch -f domains.txt -t web --auto-page -j 5
icpcli batch -f domains.txt -t web --output-dir ./output
```

批量结果会写入 JSON 文件，并记录到本地 `icp_history.db`。

## 参数速查

### 查询类型

| 类型 | 说明 |
| --- | --- |
| `web` | 网站备案 |
| `app` | App 备案 |
| `mapp` | 小程序备案 |
| `kapp` | 快应用备案 |
| `bweb` | 违规网站 |
| `bapp` | 违规 App |
| `bmapp` | 违规小程序 |
| `bkapp` | 违规快应用 |

### 常用命令

| 命令 | 说明 |
| --- | --- |
| `icpcli query <关键词>` | 查询网站备案，默认 `type=web` |
| `icpcli query -n <关键词> -t <类型>` | 指定关键词和查询类型 |
| `icpcli batch -f <文件> -t <类型>` | 批量查询，每行一个关键词 |
| `icpcli batch --auto-page` | 批量查询时自动翻页 |
| `icpcli batch -j 5` | 批量查询并发数，最大 20 |
| `icpcli mcp` | 启动 MCP Server |
| `icpcli version` | 查看版本 |

### 配置项

| 配置 | 默认值 | 说明 |
| --- | --- | --- |
| `timeout` | `30` | HTTP 请求超时，单位秒 |
| `concurrency` | `5` | 单次查询详情拉取并发数，最大 20 |
| `rate_limit.enabled` | `true` | 是否启用 MCP 限流 |
| `rate_limit.query_per_min` | `5` | `icp_query` 每分钟最大调用次数 |
| `rate_limit.blacklist_per_min` | `3` | `icp_blacklist` 每分钟最大调用次数 |
| `rate_limit.max_concurrent` | `1` | MCP 查询工具最大并发数，`1` 表示串行执行 |
| `proxy.tunnel` | `""` | 固定代理地址，如 `socks5://127.0.0.1:1080` |
| `proxy.pool.url` | `""` | 代理池 API 地址 |
| `proxy.pool.size` | `100` | 代理池最大数量 |
| `proxy.pool.ipv6` | `false` | 是否启用本地 IPv6 出口 |

## MCP 快速配置

### 启动服务

```bash
icpcli mcp
```

### Claude Code

```bash
claude mcp add icp-query -- icpcli mcp
```

### Cursor / Qoder / 其他 IDE

将命令配置为：

```json
{
  "mcpServers": {
    "icp-query": {
      "command": "icpcli",
      "args": ["mcp"]
    }
  }
}
```

如需指定配置文件：

```json
{
  "mcpServers": {
    "icp-query": {
      "command": "icpcli",
      "args": ["-c", "/path/to/config.yml", "mcp"]
    }
  }
}
```

### MCP 工具

| 工具 | 说明 |
| --- | --- |
| `icp_query` | 备案查询，`type`: `web/app/mapp/kapp` |
| `icp_blacklist` | 违规黑名单查询，`type`: `bweb/bapp/bmapp/bkapp` |
| `config_show` | 查看当前配置 |

### config.yml 快速复制

```yaml
timeout: 30
concurrency: 5

rate_limit:
  enabled: true
  query_per_min: 5
  blacklist_per_min: 3
  max_concurrent: 1

proxy:
  tunnel: ""
  pool:
    url: ""
    size: 100
    ipv6: false
```

### 环境变量

环境变量使用 `ICP_` 前缀，配置层级用 `_` 连接：

```bash
ICP_RATE_LIMIT_QUERY_PER_MIN=10 icpcli mcp
ICP_RATE_LIMIT_MAX_CONCURRENT=1 icpcli mcp
ICP_PROXY_TUNNEL=socks5://127.0.0.1:1080 icpcli mcp
```

## 开发

```bash
make build
make test
make build-all
```

## 免责声明

本项目仅供学习和技术研究使用，不得用于商业或非法用途。项目通过非官方方式调用工信部 ICP 备案查询接口，接口行为可能变化，使用风险由使用者自行承担。

## License

[MIT](LICENSE)
