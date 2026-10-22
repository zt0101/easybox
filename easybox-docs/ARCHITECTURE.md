# EasyBox 架构

EasyBox 是 3X-UI 之上的一层产品逻辑，和面板编译进同一个 x-ui 进程，没有新进程、新端口或新数据库。
上游自身的架构见仓库 `docs/architecture.md`；本文只描述 EasyBox 新增的部分。

```
 浏览器（React SPA，frontend/src/pages/easybox/*）
        │  /panel/api/easybox/*（上游的登录、CSRF、API Token 中间件）
        ▼
 internal/web/controller/easybox.go      薄控制器
        ▼
 internal/easybox/                       EasyBox 业务层（只调用上游 Service）
   templates.go   线路模板注册表：vless-reality / vless-xhttp / vless-tls / trojan / hysteria2 / tuic / shadowsocks / vless-cdn
   vless_reality.go / xhttp_line.go / quic_lines.go / tcp_lines.go   按上游表单默认值生成入站
   password.go    共用密码升级为 Shadowsocks 2022 密钥（第一次建 SS 线路时）
   cloudflare.go  Cloudflare v4 API：找域名、增删 DNS 记录、读写 SSL/TLS 模式、签发和吊销源站证书
   cdn_line.go / cdn_store.go   CDN 线路：Cloudflare 设置、建线路时的解析和证书、失败或删除时清理
   cdn_preferred.go   优选地址：写成上游 Host 组 easybox-cdn-0（原地址）和 easybox-cdn-N（优选 N），订阅多出「线路名-优选N」
   reality.go     REALITY 参数：xray x25519 + 上游 ScanRealityTargets 的第一个可用结果
   cert.go        面板证书 / 自签证书（内嵌 + GetCertHash 指纹）
   port.go        端口范围 20000–20099，交给上游 CheckPortConflict 判断
   endpoint.go    快速创建：prepare → AddInbound → apply → health check，失败补偿删除
   regenerate.go  重新生成（替换 / 保留旧线路）、订阅轮换
   subscription.go / subinfo.go / link.go / preview.go / init.go
        ▼
 上游 Service 层：InboundService / ClientService / SettingService / ServerService / XrayService
        ▼
 runtime.Runtime → Xray-core（VLESS、Trojan、Shadowsocks、Hysteria2） / 进程内 TUIC 服务

 订阅服务器（上游 internal/sub，订阅端口）
   controller.subs()  ── EasyBox 一个分支 ──▶ easybox_singbox.go（sing-box 完整配置）
                      └─ 其余沿用上游：浏览器 → HTML，Clash UA → YAML，其他 → raw base64
```

## 数据

| 表 | 内容 |
|---|---|
| `easybox_cloudflare` | CDN 线路用的 Cloudflare 账号（一行）：域名、zone ID、API 令牌、优选地址（换行分隔）。令牌不出现在任何接口返回和日志里。 |
| `easybox_subscriptions` | 订阅用户：Email、SubID（订阅地址 token）、UUID、Auth（Hysteria2）、Password（TUIC、Trojan、Shadowsocks 共用；16 字节密钥的 base64，因为 Shadowsocks 2022 只接受这种格式）。上游在删除最后一个入站时会删掉 ClientRecord，这张表保证订阅地址不变 |
| `easybox_endpoints` | 线路：名称、模板、对应的上游入站、状态、失败步骤与原因。端口、启用状态、流量实时读取上游入站 |
| `easybox_audit_logs` | 审计日志（创建、删除、重新生成、启用 / 禁用、订阅轮换），不记录凭据 |

三张表都同时登记在 `internal/database/db.go`（AutoMigrate）和 `migrate_data.go`（SQLite → PostgreSQL 迁移）。
同一个订阅用户的 Email + SubID 写进每条线路的入站，上游因此只保留一条 ClientRecord，一个订阅地址返回全部线路。

## 订阅：Canonical Node Model

sing-box 渲染器不直接读入站 JSON，而是先把上游订阅服务**已经生成的链接**解析成 `CanonicalNode`，
再转换为 sing-box 出站；唯一例外是自签 Hysteria2 线路的证书（sing-box 需要证书本身而不是指纹）。
无法表达的线路被跳过并记录原因（能力过滤）；预览接口会显示这些原因。

## 启动时的一次性默认（init.go）

在订阅服务器读取设置之前执行，用 `history_of_seeders` 保证只执行一次：

1. `EasyBoxSubscriptionDefaults`：remark 模板仍为上游默认时改为 `{{INBOUND}}{{HOST}}`；开启 Clash 订阅。
   `EasyBoxRemarkHostToken`：模板仍是旧的 `{{INBOUND}}` 时改为 `{{INBOUND}}{{HOST}}`（优选节点靠 `{{HOST}}` 带上「-优选N」）；管理员自己改过的不动。
2. `EasyBoxSubTLSFromPanel`：面板有证书、订阅没有证书时，订阅服务复用面板证书（等到证书出现后才执行）。

## 对上游文件的改动

尽量只新增文件。必须改动的上游文件：`main.go`（调用 `easybox.Init`）、`internal/web/controller/api.go`
（注册控制器）、`internal/sub/controller.go`（sing-box 分支）、`internal/database/db.go` 与 `migrate_data.go`、
`tools/openapigen/main.go`、`frontend/src/{routes.tsx,layouts/AppSidebar.tsx,api/queryKeys.ts,pages/index/IndexPage.tsx,
pages/api-docs/endpoints.ts,hooks/usePageTitle.ts,pages/login/LoginPage.tsx}`、翻译文件，以及安装 / 更新地址。
