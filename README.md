# xray-stat

终端里的 Xray 状态面板。

显示这些东西：

- 连接状态和延迟
- 实时上下行和累计流量
- Xray 日志

日志区固定在一个框里。最新日志在最上面。可滚动查看旧日志。

## 依赖

- Go 1.25.7+
- 可访问的 Xray gRPC API，默认 `127.0.0.1:10085`
- Xray 已启用 `StatsService` 和 `ObservatoryService`

## Xray 最少配置

这些项缺了就不完整：

- `stats: {}`
- `policy.system.statsOutboundUplink: true`
- `policy.system.statsOutboundDownlink: true`
- `api.tag`
- `api.services` 包含 `StatsService` 和 `ObservatoryService`
- `observatory.subjectSelector` 命中真实 outbound tag，比如 `proxy`
- 一个给 API 用的 inbound，默认监听 `127.0.0.1:10085`
- 一个 `tag: "api"` 的 outbound
- 一条把 `inboundTag: ["api"]` 指到 `outboundTag: "api"` 的路由
- `log.access` 和 `log.error` 用绝对路径

完整示例：

`outbounds` 里的代理节点按你自己的来。下面用 `jsonc` 写，注释只是为了说明，实际配置时删掉注释。

```jsonc
{
  "log": {
    "loglevel": "warning",
    "access": "/Users/yourname/.xray/access.log", // 需要确认或新增
    "error": "/Users/yourname/.xray/error.log"    // 需要确认或新增
  },
  "stats": {}, // 需要新增
  "policy": {
    "system": {
      "statsOutboundUplink": true,   // 需要新增
      "statsOutboundDownlink": true  // 需要新增
    }
  },
  "api": {
    "tag": "api", // 需要新增
    "services": [
      "StatsService",        // 需要新增
      "ObservatoryService"   // 需要新增
    ]
  },
  "observatory": {
    "subjectSelector": ["proxy"], // 改成你的真实 outbound tag
    "probeUrl": "https://www.google.com/generate_204",
    "probeInterval": "5s"
  },
  "inbounds": [
    {
      "tag": "socks-in",
      "port": 10808,
      "listen": "127.0.0.1",
      "protocol": "socks",
      "settings": {
        "udp": true
      }
    },
    {
      "tag": "api", // 需要新增
      "listen": "127.0.0.1", // 需要新增
      "port": 10085, // 需要新增
      "protocol": "dokodemo-door", // 需要新增
      "settings": {
        "address": "127.0.0.1" // 需要新增
      }
    }
  ],
  "outbounds": [
    {
      "tag": "proxy",
      "protocol": "vmess",
      "settings": {
        "vnext": [
          {
            "address": "your-server.com",
            "port": 443,
            "users": [
              {
                "id": "00000000-0000-0000-0000-000000000000",
                "alterId": 0
              }
            ]
          }
        ]
      },
      "streamSettings": {
        "network": "ws",
        "security": "tls"
      }
    },
    {
      "protocol": "freedom",
      "tag": "direct"
    },
    {
      "protocol": "blackhole",
      "tag": "block"
    },
    {
      "protocol": "freedom",
      "tag": "api" // 需要新增
    }
  ],
  "routing": {
    "domainStrategy": "AsIs",
    "rules": [
      {
        "type": "field",
        "inboundTag": ["api"], // 需要新增
        "outboundTag": "api"   // 需要新增
      }
    ]
  }
}
```

改完先验配置：

```bash
xray run -test -config /path/to/config.json
```

## 运行

```bash
go run .
```

常用示例：

```bash
go run . -tag=proxy -i=1 -loglines=15 -logmode=merge
```

## 构建

构建脚本只打包 macOS arm64：

```bash
sh scripts/build.sh
```

也可以直接本地编译：

```bash
go build -o xray-stat .
./xray-stat
```

## 参数

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-addr` | `127.0.0.1:10085` | Xray API 地址 |
| `-tag` | `proxy` | 要监控的 outbound tag |
| `-i` | `1` | 刷新间隔，秒 |
| `-loglines` | `10` | 日志区最少显示行数 |
| `-error-log` | `~/.xray/error.log` | error 日志路径 |
| `-access-log` | `~/.xray/access.log` | access 日志路径 |
| `-logmode` | `error-only` | `error-only` / `merge` / `access-only` |

## 日志操作

- `j` / `↓`: 看更旧的日志
- `k` / `↑`: 回到更新的日志
- `PgDn`: 向后翻一页
- `PgUp`: 向前翻一页
- `g`: 回到最新
- `G`: 跳到最旧
- `Ctrl+C`: 退出

## 常见问题

API 连不上：

- 确认 Xray 已启动
- 确认 API 监听的是 `127.0.0.1:10085`
- 确认 `-addr` 对得上

没有流量或延迟：

- 检查 `stats`
- 检查 `api.services`
- 检查 `observatory.subjectSelector`
- 检查 `-tag`

没有日志：

- 检查 `log.access` 和 `log.error` 路径
- 检查当前用户是否有读权限
- 检查 `-logmode`
