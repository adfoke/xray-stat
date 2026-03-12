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

请在 Xray 配置中确保开启以下能力（示例）：

```json
{
  "log": {
    "loglevel": "warning",
    "access": "/Users/yourname/.xray/access.log",
    "error": "/Users/yourname/.xray/error.log",
    "dnsLog": false
  },
  "stats": {},
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
  ]
}
```

说明：
- `subjectSelector` 要与监控目标出站 `tag` 对应（默认 `proxy`）
- 日志路径建议放在 `~/.xray/`，便于权限管理
- 首次使用请手动创建日志目录：`mkdir -p /Users/yourname/.xray`

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
