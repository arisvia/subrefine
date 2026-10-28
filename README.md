# subrefine

单二进制、低内存的代理订阅聚合与分发服务：用 Go 重写 [Sub-Store](https://github.com/sub-store-org/Sub-Store) 的核心能力，并补上它缺失的管理鉴权。

- 解析并归一化 SS / SS2022 / VMess / VLESS (Reality·Vision·gRPC·WS) / Trojan / Hysteria2 / WireGuard
- 按客户端下发对应格式：sing-box JSON、Clash.Meta (Mihomo) YAML、Base64 URI
- WireGuard 说明：sing-box 1.13.0 起移除了 wireguard 出站（新式 `endpoints` 是启动隧道，非按节点可选代理），因此 **sing-box 格式不携带 WG 节点**，并在响应头 `X-Subrefine-Skipped: wireguard=N` 与服务日志中显式声明；Clash.Meta 与 URI 格式仍完整下发 WG。产物均以 sing-box 1.14.4 / Mihomo v1.19.33 的官方解析器实测通过。
- 管线算子：过滤、正则重命名、特征去重、属性修饰（uTLS 指纹 / detour）、排序
- 内嵌 Web 控制台（SolidJS + Tailwind，`//go:embed` 打进同一个二进制）
- 双层鉴权：管理端 Bearer 令牌 + 每条订阅独立 token，均为恒定时间比较

| | subrefine | Node.js Sub-Store |
|---|---|---|
| 交付物 | 单个静态二进制 | Node 运行时 + 包目录 |
| 常驻内存 | ~15MB（活跃堆 ~1MB） | ~140MB |
| 管理接口鉴权 | 有 | 无 |

## 快速开始

```bash
SUBREFINE_ADMIN_TOKEN=change-me ./subrefine -host 0.0.0.0 -port 3000
```

打开 `http://<host>:3000/`，用管理员令牌登录控制台，新建订阅：填入机场订阅地址（或直接粘贴节点 URI / Base64 / Clash / sing-box 内容），配置管线算子，保存即可。

客户端链接形如：

```
http://<host>:3000/s/<id>?token=<订阅 token>            # 按 User-Agent 自动协商
http://<host>:3000/s/<id>?token=<订阅 token>&target=sing-box
```

`target` 可取 `sing-box` / `clash` / `uri`；未显式指定时按 User-Agent 识别 sing-box 与 Clash/Mihomo，其余回落到 Base64 URI。

### 配置

命令行参数优先于环境变量，环境变量优先于默认值。

| 环境变量 | 参数 | 默认值 | 说明 |
|---|---|---|---|
| `SUBREFINE_HOST` | `-host` | `0.0.0.0` | 监听地址 |
| `SUBREFINE_PORT` | `-port` | `3000` | 监听端口 |
| `SUBREFINE_ADMIN_TOKEN` | | 无（必填） | 管理令牌；未设置时进程拒绝启动，不再内置默认值 |
| `SUBREFINE_DATA_DIR` | `-data` | `./data` | 订阅定义持久化目录 |
| `SUBREFINE_DASHBOARD` | | 空 | 用磁盘目录替换内嵌控制台（前端开发用） |
| `SUBREFINE_TRUST_PROXY` | | `false` | 仅在可信反代之后开启：以 `X-Forwarded-For` 识别限流客户端地址 |

运行时默认 `GOGC=50`、`GOMEMLIMIT=12MiB`；若已设置同名环境变量则不覆盖。

安全行为：

- **管理接口限流**：同一客户端地址 5 分钟内失败 20 次触发 `429 + Retry-After`；限流期间即使令牌正确也会被拒绝，窗口结束自动恢复。未被限流时，一次成功登录即清零计数。`SUBREFINE_TRUST_PROXY=true` 时按 `X-Forwarded-For` 分桶（仅当反代会重写该头，否则客户端可自选限流桶）。`/s/{id}` 客户端分发不参与限流，避免误伤机场用户。
- **控制台安全头**：响应带 CSP（脚本仅同源）、`X-Content-Type-Options: nosniff`、`Referrer-Policy: same-origin`。管理令牌存于浏览器 `localStorage`，注入脚本可读，故 CSP 必须收紧。
- **保存即校验**：未知算子、非法正则、无法识别的协议白名单在保存时被 `400` 拒绝，不会静默放行错误配置。

## HTTP API

| 方法 | 路径 | 鉴权 | 用途 |
|---|---|---|---|
| `GET` | `/healthz` | 无 | 存活检查 |
| `GET` | `/s/{id}` | 订阅 token | 分发订阅（`Authorization: Bearer` 或 `?token=`） |
| `GET` | `/api/admin/info` | 管理令牌 | 版本、监听地址、订阅统计 |
| `GET` | `/api/admin/subscriptions` | 管理令牌 | 订阅列表 |
| `POST` | `/api/admin/subscriptions` | 管理令牌 | 新建或覆盖订阅 |
| `DELETE` | `/api/admin/subscriptions/{id}` | 管理令牌 | 删除订阅 |
| `GET` | `/api/admin/subscriptions/{id}/nodes` | 管理令牌 | 拉取并执行管线，返回归一化节点与对应分享 URI |
| `GET` | `/` | 无（前端自行登录） | 内嵌控制台 |

## 开发

```bash
go build ./... && go test -race ./...

cd webui
npm ci
npm run dev        # http://localhost:20137，代理到 SR_PORT 指向的后端
npm run typecheck && npm run test && npm run build
```

`webui/dist` 随仓库跟踪，因此只做后端改动时无需 Node 环境；前端有改动请重新 `npm run build`，CI 会重建并嵌入。

`localtest/` 是本地端到端验证工具集（不参与构建）：

```bash
python localtest/serve_fixture.py &      # 本地订阅源
go build -o subrefine-test.exe ./cmd/subrefine
bash localtest/run_e2e.sh                # 75 项功能 + 分发 + 失败路径 + 控制台断言 + 内存门槛
python localtest/mem_plateau.py          # 内存平台/泄漏观察（SR_PASSES/SR_ROUNDS/SR_TRAFFIC 可调）
```

## 部署

```ini
[Service]
Environment=SUBREFINE_ADMIN_TOKEN=change-me
Environment=SUBREFINE_DATA_DIR=/var/lib/subrefine
ExecStart=/usr/local/bin/subrefine -host 127.0.0.1 -port 3000
Restart=always
```

建议置于反向代理之后并强制 HTTPS：订阅 token 与管理员令牌都以 Bearer 形式随请求传输。
