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
   templates.go   线路模板注册表：vless-reality / vless-xhttp / vless-tls / trojan / hysteria2 / tuic / shadowsocks / vless-cdn / vless-cdn-xhttp / vless-tunnel
   vless_reality.go / xhttp_line.go / quic_lines.go / tcp_lines.go   按上游表单默认值生成入站
   password.go    共用密码升级为 Shadowsocks 2022 密钥（第一次建 SS 线路时）
   cloudflare.go  Cloudflare v4 API：找域名、增删 DNS 记录、读写 SSL/TLS 模式、签发和吊销源站证书
   cdn_line.go / cdn_store.go   CDN 线路：Cloudflare 设置、建线路时的解析和证书、失败或删除时清理
   tunnel.go      隧道线路：127.0.0.1 上的 VLESS + WS，cloudflared 作为独立 systemd 单元 easybox-tunnel 运行（systemd-run，DynamicUser）；从 cloudflared 的 /quicktunnel、/ready 读地址和连接状态；StartTunnelWatch 每分钟检查（重启后拉起、地址变了更新订阅、没有线路时停掉）
   settings.go    easybox_settings 的读写
   tier.go        订阅节点按快速创建的分级标注和排序：sub.InboundTiers 按入站查线路模板的 tier，remark 变量 {{TIER}} 输出「🟢推荐|」「🔵备用|」「🟠易识别|」（设置键 subTierLabels，没设置 = 开），订阅里按推荐、备用、易识别排序（同级保持 sub_sort_index 顺序），不是 EasyBox 建的入站不加标签、排在最后；分级不存进线路名
   names.go       线路命名：地区代码（设置键 regionCode，第一次用时按服务器 IP 识别，可修改）+ 模板的 nameStyle，如 HK-VLESS-Reality，重名加 -2、-3；改名同时改 easybox_endpoints.name 和入站 remark
   subcdn.go      订阅走 CDN：sub-<label>.<域名> 橙色云记录（设置键 subCdnHost / subCdnRecordId / subCdnZoneId / subCdnOrigin）；经这个域名来的订阅请求，默认节点地址换回服务器 IP（sub.SubRequestHostOverride）
   cdn_xhttp.go   XHTTP + TLS 的 CDN 入站（模板的 cdnInbound 决定入站形状，Cloudflare 部分两种 CDN 线路共用）
   cdn_preferred.go   优选地址（最多 10 个，各带备注）：写成上游 Host 组 easybox-cdn-0（原地址）和 easybox-cdn-N，订阅多出「线路名-备注」
   reality.go     REALITY 参数：xray x25519 + 上游 ScanRealityTargets 扫描后，在最快的一批可用目标里随机选一个（优先选面板上还没有入站在用的）
   cert.go        面板证书 / 自签证书（内嵌 + GetCertHash 指纹）
   port.go        端口范围 20000–20099，交给上游 CheckPortConflict 判断；REALITY / Hysteria2 先试 443（模板的 preferredPorts）
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
| `easybox_settings` | EasyBox 自己的键值设置。`cdnPreferredAddresses`：优选地址（JSON：地址 + 备注），和 Cloudflare 令牌无关，没有域名的线路也用。`regionCode`：自动命名用的地区代码（HK、JP…）。 |
| `easybox_cloudflare` | CDN 线路用的 Cloudflare 账号（一行）：域名、zone ID、API 令牌。（eb.10–eb.12 的优选地址也存在这里，升级时搬到 `easybox_settings`。）令牌不出现在任何接口返回和日志里。 |
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

1. `EasyBoxSubscriptionDefaults`：remark 模板仍为上游默认时改为 `{{TIER}}{{INBOUND}}{{HOST}}`；开启 Clash 订阅。
   `EasyBoxCdnXhttpPacketUp`：eb.12–eb.14 建的 XHTTP + CDN 线路，XHTTP 模式从 auto 改成 packet-up（Cloudflare 只转发得了分包上传；Shadowrocket 在 auto 下不选它）。
   `EasyBoxPreferredStandalone`：把 eb.10–eb.12 存在 Cloudflare 那一行里的优选地址搬到 `easybox_settings`（断开 Cloudflare 不再丢失）。
   `EasyBoxRemarkHostToken`：模板仍是旧的 `{{INBOUND}}` 时改为 `{{INBOUND}}{{HOST}}`（优选节点靠 `{{HOST}}` 带上「-优选N」）；管理员自己改过的不动。
   `EasyBoxRemarkTierToken`：模板是 eb.10–eb.17 的 `{{INBOUND}}{{HOST}}` 时改为 `{{TIER}}{{INBOUND}}{{HOST}}`（节点名带分级）；管理员自己改过的不动。
   `EasyBoxRealityTargets`：设置里存的 REALITY 候选列表还是上游旧的 10 个时，换成 EasyBox 的 50 个；管理员自己改过的不动。
2. `EasyBoxSubTLSFromPanel`：面板有证书、订阅没有证书时，订阅服务复用面板证书（等到证书出现后才执行）。

## 对上游文件的改动

尽量只新增文件。必须改动的上游文件：`main.go`（调用 `easybox.Init`）、`internal/web/controller/api.go`
（注册控制器）、`internal/sub/controller.go`（sing-box 分支）、`internal/database/db.go` 与 `migrate_data.go`、
`tools/openapigen/main.go`、`internal/web/service/reality_scan.go` 与 `frontend/src/models/setting.ts`
（REALITY 候选列表换成 EasyBox 的）、`internal/web/service/setting.go`（`SetRealityScanCandidates`）、`frontend/src/{routes.tsx,layouts/AppSidebar.tsx,api/queryKeys.ts,pages/index/IndexPage.tsx,
pages/api-docs/endpoints.ts,hooks/usePageTitle.ts,pages/login/LoginPage.tsx}`、翻译文件，以及安装 / 更新地址。
