# ICP备案查询工具

纯 Go 实现的 ICP 备案查询工具，支持网站、App、小程序、快应用备案查询，也支持违规黑名单查询。

支持两种用法：

- 命令行直接查询
- MCP Server，供 Claude、Cursor 等 AI Agent 调用

## 安装

### 源码编译

```bash
git clone https://github.com/helGayhub233/icp-query-go.git
cd icp-query-go
go build -o icpcli .
```

### Docker

```bash
docker build -t icp-query .
docker run icp-query
docker run icp-query query baidu.com
```

## 单条查询

```bash
icpcli query baidu.com
icpcli query 微信 -t app
icpcli query "北京百度网讯科技有限公司" -t mapp
icpcli query baidu.com -t bweb
```

查询类型：

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

## 批量查询

准备一个文本文件，每行一个查询目标：

```txt
baidu.com
qq.com
北京百度网讯科技有限公司
```

执行：

```bash
icpcli batch -f domains.txt -t web
icpcli batch -f domains.txt -t web --auto-page -j 5
icpcli batch -f domains.txt -t web --output-dir ./output
```

结果会写入 JSON 文件，并记录到本地 SQLite 数据库 `icp_history.db`。

## MCP Server

启动：

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

默认读取当前目录的 `config.yml`，也可以指定配置文件：

```bash
icpcli -c config.yml query baidu.com
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

## 版本

```bash
icpcli version
icpcli version -o json
```

## 免责声明

本项目仅供学习和技术研究使用，不得用于商业或非法用途。

本项目通过非官方方式调用工信部 ICP 备案查询接口，接口行为可能变化，使用风险由使用者自行承担。

## License

[MIT](LICENSE)
