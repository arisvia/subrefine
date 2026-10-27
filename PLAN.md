# subrefine 项目架构规划与交接文档

## 1. 项目定位与背景
- **替换目标**：替代当前服务器上内存占用大 (~140MB RSS)、由 Node.js 编写且鉴权不完善的 `substore` 服务（端口 3000）。
- **目标性能**：使用 Go 语言开发，单静态二进制部署，运行时常驻物理内存控制在 **< 15MB**，启动毫秒级。
- **与基础设施协同**：支持直接输出 sing-box 1.15+ (inbounds/outbounds/route rulesets)、Clash.Meta (Mihomo) 及标准 URI 格式。支持集成本地 WARP (127.0.0.1:40000) 与节点健康检查。

---

## 2. 核心系统模块设计

```
subrefine
├── cmd/
│   └── subrefine/          # 主程序入口
├── internal/
│   ├── api/                # HTTP 路由与中间件（原生鉴权、限流）
│   ├── auth/               # 认证模块（Admin Token 与 Per-Subscription Secret）
│   ├── config/             # 配置管理（YAML / 环境变量）
│   ├── core/
│   │   ├── model/          # 统一通用节点结构 (ProxyNode)
│   │   ├── parser/         # 订阅源解析器 (Base64, Clash YAML, sing-box JSON, SIP002, VLESS, Hy2)
│   │   ├── emitter/        # 格式生成器 (sing-box, clash.meta, uri)
│   │   └── pipeline/       # 节点处理器 (重命名、去重、排序、协议属性修饰)
│   └── storage/            # 存储层 (SQLite 单文件 / JSON 持久化)
├── web/                    # 现代化内嵌 WebUI (静态资源 embed)
└── docs/                   # API 与设计文档
```

---

## 3. 关键特性细节

### (1) 原生无缝双层鉴权
- **管理后台 (Admin API)**：
  - Header: `Authorization: Bearer <ADMIN_TOKEN>`
  - 控制所有节点源、订阅编排流、用户管理的增删改查。
- **订阅分发端 (Public Subscribe Endpoint)**：
  - 路径：`/s/:id?token=<SUB_TOKEN>` 或 `/s/:token`
  - 每个订阅链接独享专属 Token 与过期机制。
  - 支持绑定 User-Agent 白名单或根据 User-Agent 自动协商下发格式（遇到 sing-box 客户端下发 JSON，遇到 Clash 下发 YAML）。

### (2) 统一节点模型 (Universal Proxy Model)
支持各协议双向无损转换：
- `VLESS` (含 XTLS Reality, Vision, gRPC, WebSocket)
- `Shadowsocks` (AEAD, 2022)
- `Hysteria2`
- `Trojan`
- `WireGuard / Cloudflare WARP`

### (3) 处理管线 (Pipeline Operators)
- `RegexRename`：节点重命名（如提取国旗 Emoji，替换杂质文本）。
- `Filter`：按协议、地区代码、关键字过滤。
- `Deduplicate`：按协议核心特征（服务器+端口+配置）去除重复节点。
- `Decorator`：动态注入统一配置（如强制开启客户端特定 uTLS、注入 detour WARP 出站）。

---

## 4. 阶段实施计划 (Roadmap)

- [x] **Phase 1: 基础设施与核心数据结构**
  - 初始化 `go.mod`，配置加载器。
  - 定义 `ProxyNode` 数据规范。
  - 实现各主流协议的 URI 解析与生成 (SS, VLESS Reality/Vision, Trojan, Hy2)。
- [x] **Phase 2: 订阅解析与转换引擎**
  - 实现从远端 URL 拉取并解析主流格式（Base64 节点列表、Clash YAML、sing-box JSON）。
  - 实现针对客户端 Target 的格式发射器（生成符合 sing-box 1.15 标准的配置片段与 Clash.Meta 配置）。
- [x] **Phase 3: HTTP API 与鉴权网关**
  - 接入 Go 原生轻量路由 (Go 1.22+ ServeMux Pattern)，实现 Admin Auth 与 Sub Token 恒定时间校验。
  - 订阅链接分发端点 (`/s/{id}`)，支持 `target` 参数与 User-Agent 自动内容协商 (sing-box JSON, Clash YAML, Base64)。
- [x] **Phase 4: 持久化存储与管理**
  - 实现原子写入安全持久化存储 (JSON Storage) 与订阅管线模型。
  - 实现核心节点处理管线算子 (RegexRename, Filter, Deduplicate, Decorator)。
  - 实现 `service.Manager` 抓取调度、管道编排与内存缓存。
- [ ] **Phase 5: 服务端部署与迁移**
  - 打包编译轻量二进制。
  - 替换服务器上的 Node.js substore，验证内存与功能。
