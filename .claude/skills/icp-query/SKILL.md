---
name: icp-query
description: |
  查询中国工信部 ICP 备案信息和违法违规黑名单。支持域名、App、小程序、快应用的备案查询与违规查询。
  当用户提到以下任何话题时触发此 skill：ICP备案、域名备案、网站备案、App备案、小程序备案、快应用备案、
  备案号查询、备案信息、违规域名、违规App、违法网站、工信部黑名单、ICP查询、beian、
  website registration China、domain compliance China、ICP lookup、
  以及用户想查某个域名/公司/App 是否有备案、是否在黑名单中。
  即使用户没有明确说"备案"，只要语境涉及中国网站/App 的注册登记信息、合规状态，也应使用此 skill。
---

# ICP 备案查询

通过 MCP 工具或 `icpcli` CLI 查询工信部 ICP 备案信息。优先使用 MCP，不可用时回退到 CLI。

## 常见陷阱

- **公司名必须用全称**，简称或部分名称查不到结果
- **违规查询返回单个对象**，不是数组——与正常查询的 `params.list` 结构不同
- **web 查询没有 `serviceName` 字段**，app/mapp/kapp 查询没有 `domain` 字段——按类型取对应字段
- **app/mapp/kapp 查询比 web 慢**——会自动并发获取每条记录的详情
- **验证码破解有失败率**，内部会自动重试，偶尔仍可能超时失败
- **创宇盾拦截**是服务端限流，需要等待后重试，不是代码问题
- **备案信息以 API 实际返回为准**，不要猜测或编造字段和值

## 查询类型

| 类型 | 说明 | 类别 |
|------|------|------|
| `web` | 网站域名备案 | 正常 |
| `app` | App 备案 | 正常 |
| `mapp` | 小程序备案 | 正常 |
| `kapp` | 快应用备案 | 正常 |
| `bweb` | 违规域名 | 黑名单 |
| `bapp` | 违规 App | 黑名单 |
| `bmapp` | 违规小程序 | 黑名单 |
| `bkapp` | 违规快应用 | 黑名单 |

## 使用方式

### 方式一：MCP 工具（优先）

检查 MCP 工具 `icp_query` 和 `icp_blacklist` 是否可用，可用则直接调用。

**正常备案查询 — `icp_query`**
- `name`（必填）：域名、公司名、备案号、App 名
- `type`（可选）：`web` | `app` | `mapp` | `kapp`，默认 `web`
- `page`（可选）：页码，默认 1
- `page_size`（可选）：每页条数，最大 26

**违规查询 — `icp_blacklist`**
- `name`（必填）：域名或 App 名
- `type`（可选）：`bweb` | `bapp` | `bmapp` | `bkapp`，默认 `bweb`

### 方式二：CLI 回退

MCP 不可用时，使用 `icpcli` 命令行工具：

```bash
icpcli query <域名/公司名/备案号/App名> -t <类型>
```

### 查询示例

```bash
icpcli query baidu.com                           # 域名备案
icpcli query "深圳市腾讯计算机系统有限公司" -t web  # 公司名查域名
icpcli query 微信 -t app                          # App 备案
icpcli query "百度" -t mapp                       # 小程序备案
icpcli query example.com -t bweb                  # 违规域名
```

## 常见查询场景

| 用户意图 | 推荐类型 |
|---------|---------|
| 查某个域名备案 | `web` |
| 查公司有哪些备案 | `web` |
| 查 App 备案信息 | `app` |
| 查公司的小程序 | `mapp` |
| 查域名是否违规 | `bweb` |
| 查 App 是否违规 | `bapp` |

## 返回结构

所有查询返回：`{code, msg, success, params}`，`code` 为 `200` 或 `0` 表示成功。

### 正常查询（web/app/mapp/kapp）

`params` 是分页对象，结果在 `params.list` 数组中，分页字段：`total`、`pages`、`pageNum`、`pageSize`。

app/mapp/kapp 查询会自动并发获取详情，返回的 `list` 中每项已经是完整详情。

**通用字段：**

| 字段 | 说明 |
|------|------|
| `unitName` | 主办单位 |
| `mainLicence` | 主体备案号 |
| `serviceLicence` | 网站备案号 |
| `natureName` | 单位性质（企业/个人等） |
| `leaderName` | 负责人 |
| `contentTypeName` | 内容类型 |
| `updateRecordTime` | 更新时间 |

**类型特有字段：**

| 字段 | 适用类型 | 说明 |
|------|---------|------|
| `domain` | web | 域名 |
| `serviceName` | app/mapp/kapp | 服务名称 |

### 违规查询（bweb/bapp/bmapp/bkapp）

`params` 始终是单个对象（不是数组）。无论是否有违规记录都会返回，区别在于字段是否为空。

**bweb 返回字段：**

| 字段 | 说明 |
|------|------|
| `domainName` | 域名 |
| `blackListLevel` | 违规级别 |
| `title` | 标题 |
| `accessLicence` | 接入许可证 |
| `cancelLicence` | 注销许可证 |

**bapp/bmapp/bkapp 返回字段：**

| 字段 | 说明 |
|------|------|
| `serviceName` | 服务名称 |
| `unitName` | 主办单位 |
| `blacklistLevel` | 违规级别 |
| `blacklistReasonContent` | 违规原因 |
| `remark` | 备注 |
| `serviceType` | 服务类型 |

## 回答用户时的建议

- 用表格展示：域名/名称、主办单位、备案号、更新时间
- 结果为空：告知未查到备案信息
- 用户问"某公司备案了哪些网站/小程序"：用公司全称查询
- 违规查询的结果直接展示即可，不要对违规级别做主观判断
