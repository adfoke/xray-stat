# xray-stat

`xray-stat` 是一个用于终端环境的 Xray 运行状态监控工具。  
启动即用，不常驻后台，按 `Ctrl+C` 退出。

## 功能

- 实时流量监控：上/下行速率（自动单位）与累计流量
- 出站状态监控：连接状态（正常/断开）与延迟（ms）
- 实时日志窗口：滚动显示最新日志并按级别高亮
- 日志模式切换：`error-only` / `merge` / `access-only`
- 终端仪表盘：ANSI 清屏刷新，适配 iTerm2 / WezTerm / macOS Terminal

## 环境要求

- Go `1.25.7` 或更高版本（见 `go.mod`）
- 可访问 Xray gRPC API（默认 `127.0.0.1:10085`）
- Xray 已开启 `StatsService` 与 `ObservatoryService`

## Xray 配置前置要求

为避免改错配置导致 Xray 无法启动，建议按下面顺序操作。

### 1) 修改前先备份

```bash
cp /path/to/config.json /path/to/config.backup.json
```

### 2) 必填项清单（缺一项都可能导致功能不完整）

- 顶层 `stats: {}`
- 顶层 `policy.system.statsOutboundUplink` 与 `statsOutboundDownlink` 为 `true`
- 顶层 `api.tag` 和 `api.services`（必须包含 `StatsService`、`ObservatoryService`）
- 顶层 `observatory.subjectSelector`（必须命中真实 outbound tag，例如 `proxy`）
- `inbounds` 中新增 `tag: "api"` 的 dokodemo-door 入站（默认 `127.0.0.1:10085`）
- `outbounds` 中新增 `tag: "api"`（通常用 `freedom`）
- `routing.rules` 中新增 `inboundTag: ["api"] -> outboundTag: "api"` 规则
- 顶层 `log.access` / `log.error` 使用绝对路径

### 3) 推荐配置模板（可直接对照）

说明：下面是监控相关字段模板，`outbounds` 里的代理节点配置请保留你自己的原始内容。

```json
{
  "log": {
    "loglevel": "warning",
    "access": "/Users/yourname/.xray/access.log",
    "error": "/Users/yourname/.xray/error.log",
    "dnsLog": false
  },
  "stats": {},
  "policy": {
    "system": {
      "statsOutboundUplink": true,
      "statsOutboundDownlink": true
    }
  },
  "api": {
    "tag": "api",
    "services": ["StatsService", "ObservatoryService"]
  },
  "observatory": {
    "subjectSelector": ["proxy"],
    "probeUrl": "https://www.google.com/generate_204",
    "probeInterval": "5s"
  },
  "inbounds": [
    {
      "tag": "api",
      "listen": "127.0.0.1",
      "port": 10085,
      "protocol": "dokodemo-door",
      "settings": {
        "address": "127.0.0.1"
      }
    }
  ],
  "outbounds": [
    {
      "protocol": "freedom",
      "tag": "api"
    }
  ],
  "routing": {
    "rules": [
      {
        "type": "field",
        "inboundTag": ["api"],
        "outboundTag": "api"
      }
    ]
  }
}
```

### 4) 启动前自检（强烈建议）

```bash
xray run -test -config /path/to/config.json
```

看到 `Configuration OK.` 再重启 Xray。

### 5) 常见错误对照

| 现象 | 常见原因 | 处理方式 |
| --- | --- | --- |
| Xray 启动失败，提示 JSON 解析错误 | 配置里带注释、缺逗号、括号不匹配 | 用严格 JSON，先 `xray run -test` |
| 启动失败，提示找不到 `api` outbound | 加了 `routing` 的 `outboundTag: "api"`，但没加 `outbounds` 的 `tag: "api"` | 在 `outbounds` 新增 `{"protocol":"freedom","tag":"api"}` |
| `xray-stat` 显示 API 连接超时 | API inbound 未监听 `127.0.0.1:10085` 或端口冲突 | 检查 `inbounds` 的 `api` 配置和端口占用 |
| 有连接但没有流量统计 | 少了 `policy.system.statsOutbound*` 或 `stats` | 补齐 `policy.system` 和 `stats: {}` |
| 有流量但没有延迟 | `subjectSelector` 与真实 outbound tag 不一致 | 改成真实 tag，例如 `proxy` |
| 没有日志显示 | 日志路径错误、目录未创建或权限不足 | 执行 `mkdir -p /Users/yourname/.xray`，检查路径与权限 |

## 快速开始

```bash
# 在项目根目录
mkdir -p /Users/yourname/.xray
go run . 
```

自定义示例：

```bash
go run . -tag=proxy -i=1 -loglines=15 -logmode=merge
```

构建二进制：

```bash
go build -o xray-stat .
./xray-stat
```

## 参数说明

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-addr` | `127.0.0.1:10085` | Xray API gRPC 地址 |
| `-tag` | `proxy` | 监控的 outbound tag |
| `-i` | `1` | 仪表盘刷新间隔（秒） |
| `-loglines` | `10` | 日志窗口初始行数 |
| `-error-log` | `~/.xray/error.log` | error 日志路径 |
| `-access-log` | `~/.xray/access.log` | access 日志路径 |
| `-logmode` | `error-only` | 日志模式：`error-only` / `merge` / `access-only` |

## 日志高亮规则

- 红色：`ERROR` / `Failed` / `panic` / `Fatal`
- 黄色：`WARNING` / `warning`
- 绿色：`INFO` / `Debug`

## 常见问题

### 1. API 连接失败（`context deadline exceeded`）

请检查：
- Xray 是否已启动
- API inbound 是否监听 `127.0.0.1:10085`
- `-addr` 参数是否与实际端口一致

### 2. 看不到流量或延迟

请检查：
- `stats`、`api.services`、`observatory` 是否启用
- `-tag` 是否与实际 outbound tag 一致

### 3. 看不到日志

请检查：
- Xray `log.access` / `log.error` 路径是否正确
- 当前运行用户是否有读取权限
- `-logmode` 是否选择了包含该日志源的模式

## 项目结构

```text
.
├── main.go
├── go.mod
├── internal
│   ├── config      # 参数解析与校验
│   ├── xrayapi     # StatsService / ObservatoryService 客户端
│   ├── logs        # 日志跟随与轮转/截断处理
│   └── ui          # 终端仪表盘渲染
└── README.md
```

## 说明

当前版本为可用 MVP，后续可继续扩展：
- 多 tag 同时监控
- 关键字过滤
- 流量曲线与导出
