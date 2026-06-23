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

## 功能特性

- 支持网站、App、小程序、快应用备案查询
- 支持违规网站、违规 App、违规小程序、违规快应用查询
- 支持批量查询、并发控制、自动翻页和 JSON 结果落盘
- 本地 SQLite 记录批量查询历史
- 提供 MCP Server，供 Claude、Cursor 等 AI Agent 调用
- 支持配置文件、环境变量和代理配置
- 单二进制分发，无需外部运行时

## 安装

### 从 GitHub Release 下载

在 Releases 页面下载对应系统和架构的二进制文件：

```bash
# macOS / Linux
chmod +x icpcli-*
./icpcli-* version
```

Windows 用户下载 `.exe` 文件后可直接运行。

### 源码编译

```bash
git clone https://github.com/helGayhub233/icp-query-go.git
cd icp-query-go
go build -o icpcli .
./icpcli version
```

### Makefile

```bash
make build      # 编译当前平台
make build-all  # 编译 macOS/Linux/Windows 多平台产物到 dist/
make test       # 运行测试
```

### Docker

```bash
docker build -t icp-query .
docker run --rm icp-query version
docker run --rm icp-query query baidu.com
```

## 快速开始

### 单条查询

```bash
icpcli query baidu.com
icpcli query 微信 -t app
icpcli query "北京百度网讯科技有限公司" -t mapp
icpcli query baidu.com -t bweb
```

也可以使用 `-n` 指定查询内容：

```bash
icpcli query -n baidu.com -t web
```

查询结果默认以格式化 JSON 输出。

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

### 批量查询

准备一个文本文件，每行一个查询目标，空行和 `#` 开头的注释会被忽略：

```txt
baidu.com
qq.com
北京百度网讯科技有限公司
```

执行批量任务：

```bash
icpcli batch -f domains.txt -t web
icpcli batch -f domains.txt -t web --auto-page -j 5
icpcli batch -f domains.txt -t web --output-dir ./output
```

结果会写入 JSON 文件，并记录到本地 SQLite 数据库 `icp_history.db`。

## MCP Server

启动 MCP Server：

```bash
icpcli mcp
```

Claude Code 示例：

```bash
claude mcp add icp-query -- icpcli mcp
```

MCP 工具：

| 工具 | 说明 |
| --- | --- |
| `icp_query` | 备案查询，`type`: `web/app/mapp/kapp` |
| `icp_blacklist` | 违规查询，`type`: `bweb/bapp/bmapp/bkapp` |
| `config_show` | 查看当前配置 |

MCP 默认启用节流：

| 工具 | 默认速率 |
| --- | --- |
| `icp_query` | 5 次/分钟 |
| `icp_blacklist` | 3 次/分钟 |

连续调用时会排队等待，不会因为超过速率立即失败。

## 配置

默认读取当前目录的 `config.yml`，也可以通过 `-c` 指定配置文件：

```bash
icpcli -c config.yml query baidu.com
```

可从 `config.example.yml` 复制一份配置：

```bash
cp config.example.yml config.yml
```

示例：

```yaml
timeout: 30
concurrency: 5

rate_limit:
  enabled: true
  query_per_min: 5
  blacklist_per_min: 3

proxy:
  tunnel: ""
  pool:
    url: ""
    size: 100
    ipv6: false
```

环境变量使用 `ICP_` 前缀，例如：

```bash
ICP_RATE_LIMIT_QUERY_PER_MIN=10 icpcli mcp
```

## 版本信息

```bash
icpcli version
icpcli version -o json
```

Release 构建会通过 `-ldflags` 注入版本号、Git commit 和构建时间。

## 开发

```bash
go test ./...
go vet ./...
make build
```

发布前建议执行：

```bash
go test ./...
make build-all
```

## 免责声明

本项目仅供学习和技术研究使用，不得用于商业或非法用途。

本项目通过非官方方式调用工信部 ICP 备案查询接口，接口行为可能变化，使用风险由使用者自行承担。

## License

[MIT](LICENSE)
