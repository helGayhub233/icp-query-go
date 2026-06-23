# ICP备案查询工具 (Go)

纯 Go 实现的工信部 ICP 备案查询工具，支持域名、App、小程序、快应用的备案查询与违规查询。

原生支持 AI 集成：内置 MCP Server 可被 Claude、Cursor 等 AI Agent 直接调用。

## 安装

### 从 GitHub Release 下载

前往 [Releases](https://github.com/helGayhub233/icp-query-go/releases) 下载对应平台的压缩包，解压后将 `icpcli` 放入 PATH 即可。

### 从源码编译

```bash
git clone https://github.com/helGayhub233/icp-query-go.git
cd icp-query-go
go build -o icpcli .
```

### Docker

```bash
docker build -t icp-query .
docker run icp-query              # 默认启动 MCP Server
docker run icp-query query baidu.com  # CLI 模式
```

## 使用

### CLI 查询

```bash
# 查询域名备案
icpcli query baidu.com

# 查询 App 备案
icpcli query 微信 -t app

# 查询小程序备案
icpcli query "北京百度网讯科技有限公司" -t mapp

# 查询违规域名
icpcli query baidu.com -t bweb
```

支持的查询类型：

| 类型 | 说明 |
|------|------|
| `web` | 域名备案 |
| `app` | App 备案 |
| `mapp` | 小程序备案 |
| `kapp` | 快应用备案 |
| `bweb` | 违规域名 |
| `bapp` | 违规 App |
| `bmapp` | 违规小程序 |
| `bkapp` | 违规快应用 |

### 批量查询

```bash
# 从文件批量查询
icpcli batch -f domains.txt -t web

# 指定并发数和自动翻页
icpcli batch -f domains.txt -t web -j 10 --auto-page

# 指定结果输出目录
icpcli batch -f domains.txt -t web --output-dir ./output
```

批量查询会创建本地 SQLite 历史库 `icp_history.db`，并将结果写入 `<output-dir>/<task-name>.json`。如果数据库无法初始化，任务会直接失败并返回错误，避免后台任务在无存储状态下运行。

## 配置

默认读取当前目录下的 `config.yml`，也可以通过 `-c` 指定配置文件：

```bash
icpcli -c /path/to/config.yml query baidu.com
```

所有配置项都有默认值，示例见 [config.example.yml](config.example.yml)。

```yaml
timeout: 30
concurrency: 5

rate_limit:
  enabled: true
  query_per_min: 10
  blacklist_per_min: 5

proxy:
  tunnel: ""
  pool:
    url: ""
    size: 100
    ipv6: false
```

也可以使用 `ICP_` 前缀环境变量覆盖配置，例如：

```bash
ICP_RATE_LIMIT_QUERY_PER_MIN=20 icpcli mcp
```

### 版本信息

```bash
icpcli version
icpcli version -o json
```

### MCP Server

```bash
# 启动 MCP Server
icpcli mcp
```

在 Claude Code 中配置：

```bash
# 方式一：命令行添加（icpcli 已在 PATH 中）
claude mcp add icp-query -- icpcli mcp

# 方式二：命令行添加（指定完整路径）
claude mcp add icp-query -- /path/to/icpcli mcp
```

或在项目 `.mcp.json` / `~/.claude/settings.json` 中配置：

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

MCP 提供以下工具：

| 工具 | 说明 |
|------|------|
| `icp_query` | 备案查询，type: web/app/mapp/kapp |
| `icp_blacklist` | 违规查询，type: bweb/bapp/bmapp/bkapp |
| `config_show` | 查看当前配置 |

### MCP 节流设计

MCP Server 默认启用按工具维度的节流，避免 AI Agent 连续调用时过快触发上游限制：

| 工具 | 默认限制 |
|------|----------|
| `icp_query` | 10 次/分钟 |
| `icp_blacklist` | 5 次/分钟 |

实现方式是 `golang.org/x/time/rate` token bucket。每个受限工具有独立 limiter，`burst=1`，因此不会允许瞬时批量请求；达到限制时会返回 MCP 调用错误，请稍后重试。这个设计适合当前 stdio 单进程 MCP Server 场景。它不是跨进程/跨机器的全局限流，也不是严格滑动窗口限流。

如需关闭或调整：

```yaml
rate_limit:
  enabled: true
  query_per_min: 10
  blacklist_per_min: 5
```

### 代码结构

核心查询逻辑位于 `internal/beian`：

- `QueryRequest` / `BlacklistRequest` 描述查询入参。
- `ServiceType` 显式表达网站、App、小程序、快应用类型，避免散落的 magic number。
- 响应解析集中在 `internal/beian/response.go`，对 `code`、`success`、`params.list`、`params.total` 等字段做显式校验。

## 免责声明

本项目仅供学习和技术研究使用，不得用于任何商业或非法用途。

本项目通过非官方方式调用工信部 ICP 备案查询接口，可能违反相关服务条款，使用者需自行承担全部风险和责任。作者不对因使用本项目造成的任何直接或间接损失负责。

请遵守相关法律法规，合理使用。

## License

[MIT](LICENSE)
