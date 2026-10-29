# subrefine

单二进制、低内存的代理订阅聚合与分发服务：用 Go 重写 [Sub-Store](https://github.com/sub-store-org/Sub-Store) 的核心能力，并补上它缺失的管理鉴权。

- 解析并归一化 SS / SS2022 / VMess / VLESS (Reality·Vision·gRPC·WS) / Trojan / Hysteria2 / WireGuard
- 按客户端下发对应格式：sing-box JSON、Clash.Meta (Mihomo) YAML、Base64 URI
- 完整配置分发：以官方 schema 分流模板为骨架、节点原地注入（`@subrefine` 标记 + 正则分组），官方 sing-box / mihomo 客户端可直接加载订阅 URL；tun / tproxy / mixed 多模式，订阅级开关 + URL 覆盖
- WireGuard 说明：sing-box 1.13.0 起移除了 wireguard 出站（新式 `endpoints` 是启动隧道，非按节点可选代理），因此 **sing-box 格式不携带 WG 节点**，并在响应头 `X-Subrefine-Skipped: wireguard=N` 与服务日志中显式声明；Clash.Meta 与 URI 格式仍完整下发 WG。产物均以 sing-box 1.14.4 / Mihomo v1.19.33 的官方解析器实测通过。
- 管线算子：过滤、正则重命名、特征去重、属性修饰（uTLS 指纹 / detour）、排序
- 内嵌 Web 控制台（SolidJS + Tailwind，`//go:embed` 打进同一个二进制）
- 双层鉴权：管理端 Bearer 令牌 + 全局下发密钥池，均为恒定时间比较
- 账号状态透传：抓取上游 `subscription-userinfo` / `profile-update-interval` 响应头，下发时原样回吐，客户端即可显示到期时间与剩余流量；多账号合并时用量累加、到期取最早、刷新间隔取最短，控制台同时按源展示各自数字（与节点管线解耦，过滤节点不影响用量显示）
- 全局密钥下发：密钥独立管理（「密钥」页新建/重命名/轮换/删除），订阅只挂接引用；同一密钥可服务多个订阅（每人/每设备一条凭据），屏蔽节点按「订阅 × 密钥」对独立生效（按连接指纹的稳定 ID，重命名不掉、上游真变了自动失效）；旧版内嵌密钥与单 token 数据启动时自动迁移为全局密钥，token 原样保留、已发出的链接不失效

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
http://<host>:3000/s/<id>?token=<订阅 token>&target=sing-box&mode=tun   # 完整配置
```

`target` 可取 `sing-box` / `clash` / `uri`；未显式指定时按 User-Agent 识别 sing-box 与 Clash/Mihomo，其余回落到 Base64 URI。

下发凭据是全局密钥：在控制台「密钥」页新建（token 由服务器 `crypto/rand` 生成），再到订阅编辑器里挂接；一个密钥挂到几个订阅，就有几条能用它的链接。轮换在「密钥」页一键完成（所有引用它的订阅同步换新）；删除被引用的密钥需级联确认，失去唯一密钥的订阅会自动补发新的默认密钥。节点屏蔽在订阅编辑器的「节点开关」里勾选，仅作用于该订阅中该密钥的下发结果，其余密钥与订阅不受影响。

### 完整配置模板（官方客户端直连订阅 URL）

官方 sing-box 没有订阅/provider 概念，客户端加载 URL 必须拿到**整套可运行配置**。配置 `SUBREFINE_TEMPLATES=<目录>` 后，每个订阅可为两个目标各选一个模板模式（控制台开关，或 URL `&mode=<模式>` 临时覆盖，`&mode=off` 强制回退片段）：

- 目录内命名即注册：`singbox.<模式>.json`、`clash.<模式>.yaml`（示例见 `templates.example/`，常用模式命名 `tun` / `tproxy` / `mixed`，实际不限）；
- 模板是纯官方 schema 骨架（log/dns/inbounds/route/分组），`@subrefine` 为注入标记：`outbounds`/`proxies` 里 `{"tag":"@subrefine"}` 处原地替换为节点出站；分组引用 `@subrefine` 展开为全部节点，`@subrefine:(?i)(HK|港)` 按正则挑选（无匹配回退全量，避免空分组被客户端拒载）；
- 渲染产物不含任何标记、经 sing-box 1.14.4 `check` 与 mihomo `-t` 实测加载；WG 在 sing-box 目标仍按 1.13.0 规则剔除并声明，Clash 目标完整保留；
- 模板损坏拒绝启动；模式不存在/节点为空时回退片段并带 `X-Subrefine-Full: unknown-mode|empty-nodes` 响应头，绝不静默半生效。

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
| `SUBREFINE_TEMPLATES` | | 空 | 完整配置模板目录（`singbox.<模式>.json` / `clash.<模式>.yaml`）；未设置则完整模式不可用 |

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
| `GET` | `/api/admin/templates` | 管理令牌 | 已加载的完整配置模式列表（供控制台选择器） |
| `GET` | `/api/admin/keys` | 管理令牌 | 全局密钥列表（含每个密钥被哪些订阅挂接） |
| `POST` | `/api/admin/keys` | 管理令牌 | 新建密钥（token 留空则自动生成） |
| `PUT` | `/api/admin/keys/{id}` | 管理令牌 | 重命名 / 轮换 token（令牌冲突返回 409） |
| `DELETE` | `/api/admin/keys/{id}` | 管理令牌 | 删除密钥（被挂接时 409；`?cascade=1` 强制解除挂接） |
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

## 部署

```ini
[Service]
Environment=SUBREFINE_ADMIN_TOKEN=change-me
Environment=SUBREFINE_DATA_DIR=/var/lib/subrefine
ExecStart=/usr/local/bin/subrefine -host 127.0.0.1 -port 3000
Restart=always
```

建议置于反向代理之后并强制 HTTPS：订阅 token 与管理员令牌都以 Bearer 形式随请求传输。
