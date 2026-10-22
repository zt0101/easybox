# SSR

# 3X-UI EasyBox

## 产品需求与开发规格

版本：

```
v0.2 MVP（基于上游 MHSanaei/3x-ui main a8d0700 / v3.10.2 核对修订，变更见文末）
```

项目性质：

```
基于 3X-UI 的二次开发
```

项目代号：

```
EasyBox
```

本文档是本项目当前阶段的核心需求文档。

AI Coding 工具在开始开发前必须完整阅读本文档。

## 术语约定

上游 3X-UI 已有：

```
model.Node
/nodes 页面
/panel/api/nodes
```

它们指被管理的**远程 3X-UI 服务器**（master / node 架构）。

为避免冲突：

```
本文中的「节点 / Node / EasyBox Node」
=
EasyBox 线路（Endpoint）
=
一个上游 Inbound + 挂载其上的订阅用户（ClientRecord）
```

代码与 UI 统一命名：

```
包：internal/easybox
表：easybox_*（如 easybox_endpoints）
接口：/panel/api/easybox/...
UI：快速创建（Quick Create）/ 线路（Endpoints）/ 订阅（Subscriptions）
```

「服务器节点」专指上游 model.Node。

本文中「订阅用户」指上游 ClientRecord，其 SubID 即订阅 token。

---

# 1. 项目背景

现有 3X-UI 功能非常强大，但它本质仍然是一个面向熟悉 Xray、代理协议和网络配置用户的专业管理面板。

创建一个节点通常需要理解很多概念：

```
Inbound
Protocol
Port
Transport
Security
TLS
Client
UUID
Password
Flow
SNI
Stream Settings
Subscription
Share Link
```

对于普通使用者而言操作成本过高。

本项目不是重新开发代理核心，也不是重新开发一个完整 Xray 面板。

项目目标是：

> 以 3X-UI 为底层管理平台，将复杂的协议配置封装成简单的“一键生成节点”产品。

最终使用者不应该需要理解大量 Xray 参数。

用户只需要：

```
选择协议
↓
创建节点
↓
复制订阅
↓
导入客户端
↓
使用
```

---

# 2. 核心产品目标

最终用户体验必须尽可能接近：

```
打开后台
↓
Quick Node
↓
选择协议
↓
点击「创建」
↓
完成
```

系统自动完成：

```
生成节点名称（没填时：<地区>-<协议>-<传输/安全>[-CDN|-Tunnel]，如 HK-VLESS-Reality、HK-VLESS-WS-CDN；重名加 -2）
生成端口
生成用户凭据
生成 UUID / Password
创建 Inbound
创建 Client
生成客户端配置
生成二维码
更新订阅
```

用户不需要手工填写复杂协议参数。

---

# 3. 最终产品形态

系统：

```
             EasyBox Web Panel
                    │
                    ▼
               3X-UI Core
                    │
          ┌─────────┴─────────┐
          │                   │
       Database            Xray Core
          │                   │
          │                   ▼
          │               Node Traffic
          │
          ▼
 Universal Subscription
          │
          ├── v2rayN
          ├── Shadowrocket
          ├── PassWall
          ├── sing-box
          ├── Clash/Mihomo
          └── Future Clients
```

原则：

```
3X-UI
=
底层管理能力

EasyBox
=
产品体验层
```

---

# 4. 不重新开发的能力

优先复用 3X-UI 已经存在的：

```
Xray Core 管理
Inbound 管理
Client 管理
数据库
用户系统
流量统计
节点在线状态
Share Link
QR Code
Subscription
多节点管理
REST API
证书管理
系统状态
```

核对上游后的修正：

```
多节点管理：上游为「远程服务器」管理（model.Node），见术语约定
证书管理：签发在 install.sh / x-ui.sh（acme.sh），面板只读取证书与计算指纹
Subscription：同一 {subPath}/{subId} 已按 UA 分流 HTML / Clash / JSON / raw
新建入站的默认参数生成：在上游前端，后端没有（见 §10 例外）
安全：2FA、随机面板路径、session 过期、LDAP、API Token（含 scope）均已有
```

禁止为了“代码看起来更干净”而重新实现已有成熟能力。

原则：

> 能调用现有 3X-UI Service / API 的地方优先复用。

---

# 5. MVP 核心功能

MVP 必须完成以下五个模块：

```
1. Quick Node

2. Protocol Templates

3. Node Manager

4. Universal Subscription

5. Client Compatibility
```

---

# 6. Quick Node

新增菜单：

```
Dashboard（仪表盘）
Quick Create（快速创建）
Endpoints（线路）
Subscriptions（订阅）
Advanced（上游原有功能，见 §48）
System
```

其中：

```
Quick Node
```

是本项目最核心功能。

---

# 7. Quick Node 页面

页面默认只显示少量内容。

例如：

```
快速创建节点

协议

[ VLESS          ]
[ Hysteria2      ]
[ TUIC           ]

节点名称

[ US-Oracle-01 ]

服务器地址

[ node.example.com ]

                [ 创建节点 ]
```

默认情况下：

用户不应该看到：

```
UUID
Password
Port
Flow
Transport JSON
StreamSettings
Sniffing JSON
Client JSON
Xray JSON
```

这些由系统生成。

---

# 8. Advanced Mode

Quick Node 提供：

```
高级设置 ▼
```

展开以后才允许高级用户修改部分参数。

例如：

```
端口

节点名称

服务器地址

凭据

协议模板

备注
```

MVP 不应该把所有 Xray 参数全部暴露。

否则会重新变回原来的 3X-UI。

---

# 9. MVP 协议

第一阶段只重点支持：

```
VLESS

Hysteria2

TUIC
```

原因：

不要在第一版一次支持十几个协议。

优先完成：

```
创建
↓
配置
↓
分享
↓
订阅
↓
客户端导入
↓
删除
↓
重新生成
```

完整闭环。

后续再增加：

```
Trojan

Shadowsocks

VMess

WireGuard

其他 3X-UI 已支持协议
```

上游支持情况（已核对源码）：

```
VLESS      Xray 原生入站，支持 REALITY / TLS
Hysteria2  Xray 原生入站（model.Hysteria），订阅输出 hysteria2:// 与 Mihomo hysteria2
TUIC v5    上游进程内 Go 服务（internal/tuic），经本地 SOCKS 中转进 Xray，订阅输出 tuic://
```

三个协议均不需要引入新的代理内核。

---

# 9.1 证书策略

```
VLESS      默认 REALITY，不需要域名和证书（见 §89.1）
           可选 TLS：需要域名和证书

Hysteria2  基于 QUIC，必须有 TLS 证书
TUIC       基于 QUIC，必须有 TLS 证书
```

Hysteria2 / TUIC 证书来源，按优先级：

```
1. 复用面板证书（上游 getWebCertFiles）
   install.sh 安装时默认通过 acme.sh 为服务器 IP 签发
   Let's Encrypt shortlived 证书（需开放 80 端口，约 6 天有效，自动续期）

2. 用户已有域名证书

3. 自签证书 + 证书指纹锁定
   订阅输出 pinSHA256 / pinnedPeerCertSha256
   禁止默认使用 insecure / allowInsecure
```

上游 validateInboundTLSCertificates 会拒绝没有证书的 TLS 入站。

因此 Quick Create 在没有可用证书时：

```
先引导用户配置证书
而不是直接创建失败
```

必须测试：

```
证书续期后 Xray 与 TUIC 是否无需重启即加载新证书
```

订阅地址必须走 HTTPS：

```
订阅内容包含全部线路凭据（UUID、密码、REALITY 公钥）

上游订阅服务支持单独证书（subCertFile / subKeyFile），默认为空 = 明文 HTTP

EasyBox 首次初始化时：
把面板证书同时配置给订阅服务
无可用证书时，在订阅页面明确警告「订阅地址未加密」
```

---

# 9.2 端口与防火墙

上游面板代码不处理防火墙。

MVP：

```
不自动修改 ufw / iptables
不自动修改云厂商安全组
```

线路创建成功后必须明确提示：

```
需要放行的端口
协议（VLESS = TCP；Hysteria2 / TUIC = UDP）
```

线路端口范围：

```
默认 20000–20099（easybox.DefaultPortRange），用户在防火墙 / 安全组中一次性放行
从随机起点开始顺序扫描（带回绕），保证范围内有空闲端口时一定能找到
范围用尽时创建失败并明确提示（ErrNoFreePort）
```

端口生成必须避开：

```
已有入站端口（复用上游 port_conflict）
TUIC 本地 SOCKS 中转端口（上游 checkTuicSocks*）
面板端口 / 订阅端口 / 80
```

---

# 10. Protocol Template

禁止把 Quick Node 的协议逻辑全部硬编码在前端。

必须创建：

```
Protocol Template
```

抽象层。

上游现状：

```
新建入站的随机 UUID / Password / Auth / email / subId
由前端 frontend/src/lib/xray/inbound-defaults.ts 生成

REALITY 组装（密钥、目标站、SNI）
在前端 frontend/src/pages/inbounds/form/useSecurityActions.ts

后端只提供基础能力：
getNewX25519Cert
scanRealityTargets
port_conflict
```

因此（作为 §4 的例外）：

```
允许在 Go 后端 internal/easybox 重新实现
VLESS / Hysteria2 / TUIC 三个协议的默认参数生成
```

约束：

```
上游后端已有的基础能力必须调用，不得另写

必须有一致性测试：
EasyBox 生成的入站
→ 上游订阅服务输出链接
→ 能被上游 internal/util/link 正确解析
```

用于在上游修改格式时及时发现漂移。

建议模型：

```
ProtocolTemplate
```

包含：

```
id

name

protocol

enabled

defaultPortPolicy

credentialsPolicy

inboundDefaults

clientDefaults

streamDefaults

subscriptionSupport

supportedClients
```

---

# 11. Protocol Template 示例

逻辑结构示例：

```
{
  "id": "hysteria2-default",
  "name": "Hysteria2 Default",
  "protocol": "hysteria2",

  "defaults": {
    "credentialGeneration": "random",
    "portGeneration": "automatic"
  }
}
```

这里只是内部数据结构示例。

具体字段必须根据当前 3X-UI API 和数据模型实现。

不要凭空创建与 3X-UI 不兼容的数据格式。

---

# 12. Template Engine

创建节点流程：

```
用户选择协议
        │
        ▼
Protocol Template
        │
        ▼
Generate Runtime Parameters
        │
        ▼
Pre-validate（端口冲突、证书）
        │
        ▼
Build 3X-UI Inbound
        │
        ▼
Attach to Subscription User（ClientRecord）
        │
        ▼
InboundService.AddInbound
        │
        ▼
Apply Runtime（Xray 或 TUIC，经上游 runtime.Runtime）
        │
        ▼
Health Check
        │
        ▼
Generate Share Links
```

注意：运行时不只有 Xray。

```
VLESS / Hysteria2  →  Xray-core（gRPC 热加载）
TUIC               →  上游进程内 TUIC 服务（internal/tuic）
```

Health Check 必须按协议实现（HealthChecker 接口）：

```
Xray 入站：通过 gRPC 确认入站已加载
TUIC：读取 tuic manager 运行状态与 lastStartErr
```

所有状态变更必须经过上游 runtime.Runtime，禁止直接调用 internal/xray/api.go。

---

# 13. 自动参数生成

系统至少需要自动生成：

```
Node ID

Node Name

Port

Client ID

UUID

Password / Credential

Subscription ID

Created Time
```

具体根据协议决定。

---

# 14. Node Manager

新增：

```
Nodes
```

页面。

不要要求用户必须通过传统 Inbounds 页面理解节点。

Node 是 EasyBox 面向用户的高级抽象。

---

# 15. Node 数据模型

建议增加：

```
EasyBox
```

逻辑模型。

例如：

```
id

name

protocol

templateId

inboundId

clientId

server

port

status

createdAt

updatedAt

health

subscriptionEnabled
```

其中：

```
inboundId
```

关联真实 3X-UI Inbound。

```
clientId
```

关联上游 ClientRecord（订阅用户），见 §23.1。

```
serverId
```

直接使用上游 Inbound.NodeID（为空 = 本机），不另建字段。

不要复制完整 Inbound 数据。

表名：

```
easybox_endpoints
```

---

# 16. Nodes 页面

例如：

```
Nodes

US-Oracle-01

Protocol
VLESS

Status
● Running

Clients
1

Traffic
18.2 GB

Actions

[ Copy ]
[ QR ]
[ Details ]
[ Regenerate ]
[ Disable ]
[ Delete ]
```

---

# 17. Node 状态

至少：

```
CREATING

RUNNING

ERROR

DISABLED

DELETING
```

避免 UI 只有：

```
true / false
```

DISABLED：

```
对应上游 Inbound.enable = false
自动从订阅输出中排除
```

---

# 18. 节点创建必须事务化

禁止：

```
创建 Inbound
↓
中途失败
↓
留下半个配置
```

上游现状（InboundService.AddInbound）：

```
校验
↓
DB 事务提交
↓
提交后下发运行时
↓
下发失败：只记日志并标记需要重启，由 30 秒一次的 cron 重启 Xray
```

上游没有预校验，也没有回滚。

EasyBox 不修改上游 AddInbound，在 EasyBox 服务层做「编排 + 补偿」：

```
Generate
↓
Pre-validate（端口冲突、证书存在）
↓
状态 CREATING
↓
InboundService.AddInbound
↓
Apply Runtime
↓
Health Check
↓
成功：状态 RUNNING
```

失败：

```
InboundService.DelInbound（补偿删除）
↓
状态 ERROR
↓
记录失败步骤与原因（见 §83）
```

说明：

```
保证失败后不残留配置、不影响已有线路
不保证 DB 与运行态的原子一致
```

---

# 19. Regenerate

每一个 EasyBox 都提供：

```
Regenerate
```

用途包括：

```
凭据轮换

端口调整

节点配置损坏

节点故障恢复

重新初始化节点
```

---

# 20. Regenerate 原则

禁止直接覆盖当前可用节点。

必须采用：

```
Existing Node
       │
       ▼
Generate New Configuration
       │
       ▼
Validate
       │
       ▼
Start
       │
       ▼
Health Check
       │
   ┌───┴───┐
 success   fail
   │        │
   ▼        ▼
activate   rollback
```

Regenerate 分两种模式：

```
轮换凭据（默认）
新线路验证成功后，旧线路立即禁用并从订阅移除
用于凭据泄露

保留旧线路
新旧线路同时存在，由用户决定何时删除旧线路
用于端口调整、故障恢复
```

实现：

```
新建 inbound
↓
挂到同一个订阅用户（同一 ClientRecord / SubID）
↓
验证成功后按模式处理旧 inbound
```

因此订阅 URL 不变（见 §37）。

服务器迁移需要多服务器能力，移至 P2。

---

# 21. Regenerate UX

点击：

```
Regenerate
```

弹窗：

```
重新生成节点

将创建新的节点配置。

现有节点在新节点验证成功前不会删除。

用途：
(●) 轮换凭据：验证成功后旧线路立即停用
( ) 保留旧线路：由我决定何时删除

订阅地址不会变，客户端刷新订阅即可。

[取消]

[生成]
```

成功以后：

```
New node ready.

[Copy]

[QR Code]

[Refresh Subscription]

[Delete Old Node Later]
```

---

# 22. Share Link

节点创建成功以后：

必须生成：

```
Share Link

QR Code
```

例如协议对应的标准 URI。

用户可以：

```
Copy

QR

Download
```

---

# 23. Universal Subscription

这是整个项目第二核心功能。

目标：

> 用户只有一个永久订阅 URL。

例如：

```
https://sub.example.com/s/USER_TOKEN
```

用户不需要理解：

```
raw
json
clash
mihomo
singbox
base64
```

---

# 23.1 多线路合并订阅

一个订阅 URL 同时包含多条线路。

用户导入一次订阅，客户端里同时出现：

```
US-Oracle-VLESS
US-Oracle-HY2
US-Oracle-TUIC
```

并可在客户端中自由切换。

实现（复用上游数据模型，已核对源码）：

```
订阅用户 = 上游 ClientRecord（email 唯一，带 SubID）

ClientRecord ←→ Inbound
多对多，经上游 client_inbounds 表

GET {subPath}/{SubID}
上游 getSubs(subId) 一次返回该用户挂载的全部入站
```

同一个 ClientRecord 的凭据字段分别用于：

```
VLESS      UUID
Hysteria2  Auth
TUIC       UUID + Password
```

规则：

```
Quick Create 默认把新线路挂到「默认订阅用户」

禁用的线路自动从订阅中排除

订阅中每条线路的显示名 = 线路名称
（设置上游 remark 模板，避免默认的 {{INBOUND}}-{{EMAIL}} 格式暴露 email）
实现：easybox.Init() 在启动时（订阅服务读取设置之前）执行一次，
仅当 remarkTemplate 仍是上游默认值时改为 {{INBOUND}}；
用 HistoryOfSeeders 记录 EasyBoxSubscriptionDefaults，管理员之后的修改不会被覆盖

线路在订阅中的顺序 = 线路页面中的排序
```

订阅页面提供：

```
勾选哪些线路包含在该订阅中
```

范围：

```
MVP
一个默认订阅用户，包含全部已启用线路，可勾选排除

P1
多个订阅用户（如家人各一个），每个用户选择自己的线路组合
每个用户独立的流量统计与订阅地址
```

能力过滤（§34）仍然生效：

```
同一个订阅中
客户端不支持的协议会被过滤
其余线路正常返回
```

---

# 24. Universal Subscription UX

后台：

```
My Subscription

https://sub.example.com/s/abc123xyz

[ Copy ]

[ QR Code ]

[ Test ]
```

自动刷新：

```
上游通过 Profile-Update-Interval 响应头通知客户端定期刷新
默认 subUpdates = 12（小时）
```

订阅页面需明确提示：

```
新增、删除、重新生成线路后
客户端最长 12 小时内自动更新，也可手动刷新订阅
```

---

# 25. Universal Subscription 架构

必须实现：

```
                    Universal URL

                         │
                         ▼

                 Client Detector

                         │
                         ▼

               Capability Resolver

                         │
                         ▼

                  Node Filtering

                         │
                         ▼

                    Renderer

                         │
        ┌────────────────┼───────────────┐
        │                │               │
        ▼                ▼               ▼
     Raw/Base64      Mihomo YAML     sing-box JSON
```

实现方式：

```
不另起订阅网关
扩展上游 internal/sub 订阅服务
```

上游已有（同一 {subPath}/{subId}）：

```
浏览器（Accept: text/html）  →  HTML 订阅页
UA 匹配 clash|mihomo          →  Clash / Mihomo YAML
UA 匹配 JSON 正则（默认为空）  →  Xray JSON
其他                          →  raw base64
附带 Subscription-Userinfo / Profile-Update-Interval 等响应头
记录 UA 日志
```

EasyBox 新增（放在 internal/sub 的新文件中）：

```
sing-box 分支（识别 sing-box UA → sing-box JSON）
按客户端能力过滤（§34）
```

对上游 controller.subs() 的改动限制为插入一个分支。

上游 subClashEnable 默认 false，EasyBox 首次初始化时开启。

---

# 26. Canonical Node Model

订阅转换禁止：

```
3X-UI格式
→ Clash

3X-UI格式
→ sing-box

3X-UI格式
→ Shadowrocket
```

每个 Renderer 各自读取数据库结构。

必须先建立：

```
Canonical Node Model
```

即内部统一节点表达。

范围（v0.2）：

```
上游 raw / Clash(Mihomo) / JSON 输出均直接读取 Inbound JSON
（internal/sub/service.go 等）

MVP：
Canonical Node Model 只用于 sing-box Renderer 与能力过滤
raw 与 Mihomo 直接复用上游输出

未来如需接管 raw / Mihomo 输出，再迁移到 Canonical Node Model
```

---

# 27. Canonical Node Model

概念：

```
CanonicalNode
```

可能包含：

```
id

name

protocol

server

port

credentials

transport

tls

udp

metadata
```

然后：

```
CanonicalNode
       │
       ├── Raw Renderer
       ├── Mihomo Renderer
       ├── sing-box Renderer
       └── Future Renderer
```

---

# 28. Renderer

创建：

```
SubscriptionRenderer
```

接口。

概念：

```
render(nodes, clientInfo)
```

实现：

```
RawRenderer

MihomoRenderer

SingBoxRenderer
```

后续可增加：

```
SurgeRenderer

LoonRenderer

QuantumultXRenderer
```

而不影响核心节点逻辑。

---

# 29. Universal URL

普通用户只看到一个完整 URL：

```
{subURI}{subPath}{SubID}
```

例如：

```
https://203.0.113.10:2096/p7x2kq9wmv4ta1hd/vq83nd02lx7c5e1k
```

说明：

```
subPath：沿用上游初始化时生成的 16 位随机路径（安全默认）
SubID：订阅用户 ClientRecord.SubID，即订阅 token
```

不要求字面上为 /s/。本文其他位置出现的 /s/xxx 仅为示意。

服务器根据客户端请求自动确定格式。

---

# 30. Client Detection

检测来源：

```
User-Agent

Accept

Headers

Optional query parameter
```

优先级：

```
explicit target

>

known User-Agent

>

default renderer
```

---

# 31. 显式格式

虽然用户正常情况下不需要使用，但必须保留：

```
/s/{token}?target=raw

/s/{token}?target=mihomo

/s/{token}?target=singbox

/s/{token}?target=shadowrocket

/s/{token}?target=v2rayn

/s/{token}?target=passwall
```

v0.2 实现：

```
MVP 直接使用上游已有的格式路径：
Clash / Mihomo：subClashPath、/mihomo/、/clash-legacy/
Xray JSON：subJsonPath
raw：subPath

sing-box：新增独立路径（与上游路径风格一致）

?target= 仅在确有需要时做到上游路径的轻量映射
```

用途：

```
测试

Debug

客户端 User-Agent 不标准

兼容性问题
```

---

# 32. 目标客户端

MVP 必须重点测试：

```
v2rayN

Shadowrocket

PassWall

sing-box

Mihomo / Clash compatible clients
```

---

# 33. 客户端能力矩阵

必须维护：

```
Client Capability Matrix
```

不要把所有节点无脑返回给所有客户端。

数据模型示例：

```
Client

v2rayN
Shadowrocket
PassWall
sing-box
Mihomo
```

每个客户端定义：

```
supportedProtocols

supportedTransports

supportedSecurityModes

preferredRenderer
```

---

# 34. Capability Filtering

请求订阅：

```
Client
↓
Detect
↓
Read Capability
↓
Filter Unsupported Nodes
↓
Render
↓
Return
```

例如：

客户端不支持：

```
Protocol X
```

则：

```
不要把 Protocol X 节点返回
```

而不是：

```
返回
↓
客户端报错
```

---

# 35. 兼容性原则

Universal Subscription 的目标不是：

```
所有客户端得到完全相同内容
```

而是：

> 同一个订阅 URL，在不同客户端中都能直接导入，并获得该客户端真正支持的节点。

---

# 36. 当前 3X-UI Subscription

优先复用 3X-UI 已有：

```
Raw subscription

Xray JSON

Clash/Mihomo subscription

Subscription ID

Traffic metadata

Expiry metadata
```

EasyBox Universal Subscription：

```
可以包装现有 Subscription Service
```

不要无必要全部重写。

Clash / Mihomo 输出：

```
代理组与分流规则沿用上游默认
EasyBox 最多增加「自动选择 / 手动选择」代理组
不自行维护分流规则集（同时符合 §89.1 边界）
```

---

# 37. Subscription URL 生命周期

订阅 URL：

```
必须长期稳定
```

节点发生：

```
添加

删除

修改

重新生成
```

以后：

```
Subscription URL
```

不能改变。

客户端：

```
Refresh Subscription
```

即可获取最新配置。

实现：

```
订阅 URL 绑定订阅用户（ClientRecord.SubID），不绑定 inbound

添加线路     = 新 inbound 挂到该用户
删除线路     = 解除挂载并删除 inbound
重新生成     = 新 inbound 挂到同一用户，再处理旧 inbound（§20）
```

---

# 38. Subscription Token

URL 使用：

```
random token
```

例如：

```
/s/L7qP6xM29abc
```

禁止：

```
/s/user@example.com

/s/1

/s/admin
```

必须避免暴露：

```
username
email
database ID
```

实现：token = 上游 ClientRecord.SubID（随机生成）。

---

# 39. Subscription Token 管理

提供：

```
Regenerate Subscription Token
```

操作。

用于：

```
订阅地址泄露
```

旧 Token：

```
立即失效
```

新 Token：

```
立即生效
```

实现：通过上游 ClientService 修改该订阅用户的 SubID。

旧地址随即返回 404（上游行为）。

---

# 40. Subscription Cache

上游只有单次请求内的缓存，没有跨请求缓存。

单机、线路少的场景下实时生成成本很低。

MVP：

```
不做跨请求缓存
```

因此不存在：

```
后台节点已经改变
↓
客户端刷新仍然拿旧配置
```

如未来需要缓存，节点更新时必须 invalidate。

---

# 41. Subscription HTTP Response

必须正确设置：

```
Content-Type

Content-Disposition if needed

Cache-Control

ETag optional

Subscription metadata
```

并尽可能兼容现有客户端订阅行为。

实现：沿用上游 ApplyCommonHeaders（Subscription-Userinfo、Profile-Update-Interval、标题等），sing-box 输出同样调用。

---

# 42. Subscription Error

不能返回：

```
200 OK

{}
```

这种难以排查的结果。

上游行为（writeSubError）：

```
404（无 body）  订阅不存在 / token 无效
500（无 body）  内部错误
```

上游注释说明：VPN 客户端只看状态码。

v0.2：

```
订阅接口保持上游行为，不新增 400 / 422
未知客户端返回 raw（§79），不返回错误
```

诊断信息：

```
写入服务端日志（含 UA、识别结果、过滤掉的线路、错误原因）
在管理后台 Preview（§43）中展示
```

这样既便于排查，也不改动上游订阅 controller 的错误处理。

---

# 43. Client Preview

后台提供：

```
Preview As
```

功能。

例如：

```
Preview subscription as:

v2rayN

Shadowrocket

PassWall

sing-box

Mihomo
```

方便管理员测试。

---

# 44. Client Import 页面

节点详情：

```
Import
```

页面提供：

```
Universal Subscription

Share Link

QR Code

Client Compatibility
```

例如：

```
Compatible

✓ v2rayN
✓ Shadowrocket
✓ PassWall
✓ sing-box
✓ Mihomo
```

实现：

```
后台线路详情：新增 Import 区块
最终用户：扩展上游订阅页 subpage.html（浏览器打开订阅地址即显示），
增加「兼容客户端」区块，不新建页面
```

兼容标记以 CLIENT_COMPATIBILITY.md 实测结果为准。

---

# 45. QR Code

至少支持：

```
Node Share Link QR

Subscription QR
```

二者必须明确区分。

例如：

```
Node QR

Subscription QR
```

避免用户扫码错误。

---

# 46. Dashboard

首页不要照搬专业运维面板思维。

核心信息：

```
Server

Online

CPU

RAM

Traffic

Nodes

3 Running

Clients

1

Subscription

Healthy
```

快捷按钮：

```
Create Node

Copy Subscription
```

---

# 47. 原始 3X-UI 功能

原有：

```
Inbounds

Routing

Xray Settings

System Settings
```

可以保留。

但定位为：

```
Advanced
```

普通用户默认主要使用：

```
Dashboard

Quick Node

Nodes

Subscriptions
```

---

# 48. 前端导航

建议：

```
Dashboard（仪表盘）

Quick Create（快速创建）

Endpoints（线路）

Subscriptions（订阅）

Advanced（上游原有页面）
    ├── Inbounds
    ├── Clients
    ├── Groups
    ├── Hosts
    ├── Servers（上游 Nodes：远程服务器）
    ├── Routing
    └── Xray Settings

System
    ├── Server
    ├── Logs
    ├── Backup
    └── Settings
```

实现（按上游 frontend/CLAUDE.md「Adding a panel route」）：

```
frontend/src/pages/<page>/<Page>.tsx
frontend/src/routes.tsx 注册（lazy import）
frontend/src/layouts/AppSidebar.tsx 加侧栏入口
```

对上游文件的改动控制在几行。

---

# 49. 安全要求

管理后台必须：

```
Authentication required
```

禁止匿名管理。

支持：

```
strong password

random panel path

HTTPS

session expiration
```

上游均已具备（另有 2FA、LDAP、登录失败事件），直接复用，不新增开发。

---

# 50. API Token

EasyBox 与面板在同一个 Go 进程内：

```
Frontend
↓
/panel/api/easybox/*（复用上游 session、CSRF、API Token 鉴权中间件）
↓
internal/easybox
↓
上游 Service 层（InboundService / ClientService ...）直接调用
```

不经过 HTTP 调用上游 API，因此不需要内部管理员 Token。

禁止把任何 Token 发送给浏览器。

---

# 51. Secrets

禁止出现在：

```
Git

logs

frontend bundle

README

error response
```

中的内容：

```
private keys

admin password

API token

subscription secret

client credential
```

---

# 52. 日志

记录：

```
Node create

Node delete

Node regenerate

Subscription token regenerate

Template change

Renderer error

Xray reload failure
```

---

# 53. Audit Log

建议数据：

```
timestamp

action

object

objectId

result

error
```

不要记录完整敏感凭据。

---

# 54. 数据库原则

现阶段：

```
优先继续使用 3X-UI 默认数据库
```

EasyBox 新增数据尽可能：

```
独立 table
```

而不是修改大量原有表。

目的：

```
降低上游升级冲突
```

---

# 55. 与上游 3X-UI 的关系

必须把项目视为：

```
3X-UI Fork
```

而不是完全独立项目。

开发过程中尽量减少：

```
大规模修改原始核心文件
```

优先：

```
新增 module

新增 service

新增 route

新增 table

新增 UI
```

---

# 55.1 必须遵守上游开发约束

上游仓库 CLAUDE.md 与 docs/architecture.md 的规则对 EasyBox 同样生效。

否则上游测试会直接失败：

```
新增 /panel/api/* 路由
必须登记到 frontend/src/pages/api-docs/endpoints.ts 并执行 make gen
（TestRouteRegistryContract 双向校验）

新增 Go 结构体若出现在 API 中
必须加入 tools/openapigen 的 StructAllow

新增 i18n key
必须同时加入 internal/web/translation/ 下全部 13 个语言文件
并在同一提交中被引用（i18n-dead-keys 测试双向校验）
EasyBox 做法：只认真编写 en-US 与 zh-CN，其余 11 个语言先填英文原文，后续再翻译

所有入站 / 客户端状态变更
必须经过 runtime.Runtime

数据库变更
必须在 internal/database/db.go 中写迁移

注释每块最多 2 行

TDD，禁止假测试

提交前 make verify 通过
```

EasyBox 代码放置：

```
后端：internal/easybox/（只调用上游 Service 层）
订阅扩展：internal/sub/ 下新增文件
前端：frontend/src/pages/ 下新增页面目录
```

---

# 56. Upstream Sync

代码结构要方便：

```
git remote upstream
```

持续拉取：

```
MHSanaei/3x-ui
```

未来：

```
upstream update
↓
merge/rebase
↓
resolve limited conflicts
```

上游迭代很快（2026-09 至 2026-10 约 180 个提交）。

节奏：

```
每个上游 release 合并一次
合并后跑 make verify 与 EasyBox 一致性测试
在 UPSTREAM_BASE.md 更新基线
```

可执行流程（步骤、冲突点、合并后必查、VPS 实测）：easybox-docs/UPSTREAM_SYNC.md。

冲突热点：

```
frontend/src/routes.tsx
frontend/src/layouts/AppSidebar.tsx
internal/sub/controller.go
internal/database/db.go（迁移列表）
internal/web/translation/*.json
```

---

# 57. 分支策略

建议：

```
main
```

当前稳定版本。

```
develop
```

开发版本。

功能分支：

```
feature/quick-node

feature/protocol-template

feature/universal-subscription

feature/singbox-renderer

feature/client-capabilities
```

仓库：

```
origin    https://github.com/zt0101/easybox.git（私有，自用）
upstream  https://github.com/MHSanaei/3x-ui.git
```

fork 后需处理的 GitHub Actions：

```
保留：ci.yml、release.yml

禁用（fork 中无对应 secrets 或不适用）：
claude-pr-review.yml
claude-issue-analyst.yml
docs-deploy.yml
mutation.yml

修改：docker.yml 中写死的 ghcr.io/mhsanaei/3x-ui 改为本仓库镜像地址（或先禁用）
```

实现方式（v0.2.2）：

```
不删除、不修改 workflow 文件（避免与上游冲突）
推送前：仓库 Settings → Actions → General → Disable actions（全部关闭）
MVP 发布前：重新启用 Actions，并在 Actions 页面逐个禁用不需要的 workflow
```

禁用清单：claude-pr-review、claude-issue-analyst、docs-ci、docs-deploy、mutation、cleanup_caches、docker。

推送凭据：

```
使用仓库 Deploy key（ed25519，允许写入），只能访问本仓库、只能 git 操作
私钥在开发容器内生成，保存在 Docker 卷（/root/.cache/ssh/），不离开容器
不使用个人 token
```

推送规则：

```
不要推送上游的 v*.*.* 标签（会触发 docker.yml / release.yml，上百次运行）
EasyBox 版本标签：上游基线版本 + 后缀，如 v3.10.2-eb.1
（install.sh 用 sort -V 校验最低版本，release.yml 按 v*.*.* 触发，easybox-v* 形式会被安装脚本拒绝）
install.sh / update.sh / x-ui.sh 与面板内置更新（panel.go）均已改为从 zt0101/easybox 获取
（否则面板「更新」会装回原版 3x-ui，覆盖 EasyBox）
```

---

# 58. 第一次 Fork

AI Coding 工具首先：

```
Fork / Clone 3X-UI
```

然后记录当前：

```
commit SHA

release version
```

写入：

```
UPSTREAM_BASE.md
```

例如：

```
Upstream:

MHSanaei/3x-ui

Base commit:

xxxxxxxx
```

---

# 59. License

3X-UI 当前采用：

```
GPL-3.0
```

因此二次开发必须保留并遵守 GPL-3.0 对衍生作品的相关要求。

开发开始前必须：

```
保留 LICENSE

保留原项目 copyright / attribution

记录新增代码
```

如果未来需要公开分发或商业化，应单独检查 GPL-3.0 的合规要求。

当前状态：

```
私有仓库，仅自用
```

GPL-3.0 下私有自用无需公开源码。

一旦把安装包分发给他人，必须同时提供对应源码。

命名：

```
「EasyBox」已被多个产品使用（网络设备、存储产品等）
自用无影响
公开发布前需检查商标与同名开源项目
```

---

# 60. AI Coding 开发第一原则

在写代码以前：

必须先研究当前 3X-UI 源码。

至少定位：

```
Inbound Model

Inbound Service

Client Model

Subscription Service

Share Link Generator

API Routes

Frontend Routing

Database Layer

Xray Config Generator
```

然后创建：

```
ARCHITECTURE_RESEARCH.md
```

记录：

```
哪些能力可以复用

哪些需要扩展

哪些不能直接复用

主要代码路径

数据库结构

调用关系
```

---

# 61. 禁止直接开始乱改

错误方式：

```
读 README
↓
开始写 EasyBox
```

正确方式：

```
Clone
↓
Build
↓
Run Tests
↓
Run Current Panel
↓
Study Architecture
↓
Document
↓
Design
↓
Code
```

---

# 62. 开发环境必须先跑通原项目

第一阶段必须确认：

```
3X-UI upstream
```

原始版本：

```
Build success

Start success

Login success

Create inbound success

Create client success

Generate subscription success
```

然后：

```
git tag
```

建立：

```
baseline-upstream
```

---

# 62.1 开发环境（Docker）

开发不在本机安装任何工具链。

```
3x-ui/devenv/
  Dockerfile    Go 1.27 + Node 26 + gcc + golangci-lint + Playwright Chromium + Xray
  compose.yml   源码挂载；node_modules / Go 缓存 / 数据库 / 日志放在 Docker 命名卷
  panel.sh      容器内构建前端 + 后端并运行面板
  README.md     使用说明
```

端口（宿主机 → 容器）：

```
12053 → 2053   面板
12096 → 2096   订阅
20000-20009    测试入站（TCP + UDP）
```

Windows 经常保留 2000-2099 端口（Hyper-V / WinNAT），因此宿主机端口做了重映射。

本机（Windows）实际采用：源码放在 Docker 卷中。

```
原因：Windows Defender 把上游 install.sh 判定为
Trojan:Win32/Egairtigado!rfn 并隔离（2026-10-20）
已从 git 原文审查：只访问上游 GitHub、acme.sh、公网 IP 查询服务、PostgreSQL 源，判断为误报

源码：Docker 卷 easybox-dev_src（分支 develop，remotes origin / upstream）
启动器：E:\ai_dev\ssr\easybox-dev\（compose.yml、rebuild-image.sh、README.md）
编辑：VS Code Dev Containers → Attach to Running Container → /src
```

不要在 Windows 文件系统上保留另一份工作副本，避免两份代码分叉。

---

# 62.2 测试服务器与真机

本地 Docker 只能验证「面板能创建、订阅能输出」。

无法验证：

```
真实网络连通（尤其 UDP）
证书签发与续期
云厂商安全组
```

因此：

```
MVP-0 完成时准备一台测试 VPS（Ubuntu，1C1G）
```

§77 兼容性实测需要提前准备：

```
v2rayN       Windows
Shadowrocket iOS 设备（App 付费）
PassWall     OpenWrt 路由器或 OpenWrt 虚拟机
sing-box     Android / iOS / 桌面任一
Mihomo       Clash Verge Rev 等任一客户端
```

---

# 63. 开发顺序

严格按照：

```
Phase 0
Upstream Research（build、运行、内存基线、ARCHITECTURE_RESEARCH.md）

Phase 1
Protocol Template + Go 模板引擎（三个协议）

Phase 2
Quick Create Backend（编排 + 补偿 + HealthChecker）

Phase 3
默认订阅用户 + 多线路合并订阅（复用上游 raw / Mihomo，先跑通端到端闭环）

Phase 4
Quick Create UI

Phase 5
Endpoint Manager（线路列表、禁用、删除）

Phase 6
Regenerate（两种模式）

Phase 7
Client Detection（在上游 UA 分流基础上补充）

Phase 8
Canonical Node Model + Client Capability Matrix + 能力过滤

Phase 9
sing-box Renderer

Phase 10
Subscription Preview + 订阅页兼容区块

Phase 11
真实客户端兼容性测试（CLIENT_COMPATIBILITY.md）

Phase 12
Deployment（fork release + install.sh）
```

测试不单独成阶段：每个 Phase 按上游 TDD 要求同步编写。

---

# 63.1 分批交付

P0 范围较大，分三批交付，每批都可独立使用：

```
MVP-0（当前目标）
VLESS + REALITY
快速创建 → 线路列表（禁用 / 删除）→ 多线路合并订阅
订阅只用上游 raw / Mihomo 输出
不需要证书、不需要新订阅格式
对应 Phase 0 – 5（仅 VLESS）

MVP-1
Hysteria2、TUIC
证书引导（§9.1）与订阅 HTTPS
Regenerate（两种模式）

MVP-2
sing-box Renderer
Canonical Node Model + 能力过滤
Subscription Preview、订阅页兼容区块
真实客户端兼容性测试
```

MVP-0 用来最早跑通完整闭环，并尽早用真实客户端测试。

---

# 64. MVP P0

以下必须完成：

```
Quick Node

VLESS template

Hysteria2 template

TUIC template

Node Manager

Share Link

QR Code

Universal Subscription URL

多线路合并订阅（一个订阅包含全部已启用线路）

v2rayN compatibility

Shadowrocket compatibility

PassWall compatibility

sing-box compatibility

Mihomo compatibility

Regenerate

Delete

Subscription Token Rotation
```

---

# 65. P1

后续：

```
More protocols

Multi-server node creation

Batch node creation

Template editor

Node cloning

Import/export templates

Backup UI

Client profiles

多个订阅用户（每人独立订阅地址、线路组合、流量统计）

More subscription renderers
```

---

# 66. P2

未来：

```
Server fleet management

Central control plane

Multiple VPS

Node migration

Node grouping

Regional nodes

API automation
```

---

# 67. Multi-Server 预留

MVP 只有一台服务器也必须避免：

```
hardcode localhost assumptions
```

EasyBox：

```
node.serverId
```

字段提前预留。

v0.2：直接使用上游 Inbound.NodeID（为空 = 本机），线路创建时经 runtime.Runtime 自动分发到本机或远程服务器。

未来可以：

```
EasyBox Panel

├── Oracle-US
├── AWS-JP
├── GCP-SG
└── VPS-HK
```

---

# 68. API 设计

EasyBox API 推荐独立命名空间：

```
/panel/api/easybox/
```

必须挂在上游 /panel/api 下，才能获得登录、CSRF、API Token 鉴权中间件。

每个新路由都要登记到 endpoints.ts（§55.1）。

已实现（MVP-0，沿用上游路由风格：GET 读取，POST + 动词路径写入）：

```
GET   /panel/api/easybox/endpoints/list
POST  /panel/api/easybox/endpoints/create        {"name","address"}
POST  /panel/api/easybox/endpoints/del/:id
POST  /panel/api/easybox/endpoints/setEnable/:id {"enable"}
POST  /panel/api/easybox/endpoints/setInSubscription/:id {"include"}（上游 excludeFromSub）
POST  /panel/api/easybox/endpoints/rename/:id    {"name"}（同时改入站 remark，优选节点跟着变）
GET   /panel/api/easybox/region                  自动命名用的地区代码
POST  /panel/api/easybox/region                  {"code"}（空值 = 重新识别）
GET   /panel/api/easybox/subscription/info       订阅地址 / Clash 地址 / 刷新间隔 / 线路
```

创建失败时：success=false，obj 为线路对象（status=ERROR、failedStep、lastError），

前端按 failedStep 做本地化提示；后端只复用已有 i18n key，不新增。

后续（MVP-1）：

```
POST  /panel/api/easybox/endpoints/regenerate/:id
```

OpenAPI：EndpointView / CreateEndpointRequest 已加入 tools/openapigen 白名单。

上游的「第四步」（把 openapi.json 复制到 docs/ 站点并重新生成 MDX）有意跳过：

EasyBox 不发布上游的 docs/ 站点，重新生成只会增加合并冲突。

---

# 69. Template API

```
GET
/panel/api/easybox/templates

GET
/panel/api/easybox/templates/:id
```

未来：

```
POST

PUT

DELETE
```

---

# 70. Subscription API

例如：

```
GET
{subPath}/:subId（上游订阅服务，订阅端口）
```

内部接口：

```
GET
/panel/api/easybox/subscription/preview

POST
/panel/api/easybox/subscription/token/regenerate
```

---

# 71. Client Detection Service

单独模块：

```
ClientDetector
```

输入：

```
request headers
```

输出：

```
ClientType
```

例如：

```
V2RAYN

SHADOWROCKET

PASSWALL

SINGBOX

MIHOMO

UNKNOWN
```

---

# 72. Capability Service

单独模块：

```
ClientCapabilityService
```

例如：

```
supports(client, node)
```

返回：

```
true / false
```

---

# 73. Subscription Service

建议流程：

```
SubscriptionController

↓

SubscriptionService

↓

NodeRepository

↓

CanonicalMapper

↓

ClientDetector

↓

CapabilityFilter

↓

RendererFactory

↓

Renderer

↓

HTTP Response
```

---

# 74. Renderer Factory

例如：

```
RendererFactory
```

决定：

```
RAW

MIHOMO

SINGBOX
```

不要在 Controller：

```
if shadowrocket
else if clash
else if...
```

堆大量逻辑。

---

# 75. Testing

必须至少包含：

```
Unit Tests

Integration Tests

Subscription Snapshot Tests

API Tests
```

---

# 76. Renderer Snapshot Tests

特别重要。

准备固定：

```
CanonicalNode Fixtures
```

然后测试：

```
Raw output

Mihomo output

sing-box output
```

sing-box 版本策略：

```
只支持 sing-box ≥ 1.12（1.10 – 1.12 之间配置格式有破坏性变化）
从 User-Agent 读取版本号
版本过低或无法识别：退回 raw 订阅
支持的版本范围写入 CLIENT_COMPATIBILITY.md
```

快照的期望值必须手工核对后固定，禁止为了让测试通过而重新生成（上游规则）。

避免以后修改代码导致：

```
订阅格式悄悄坏掉
```

---

# 77. Client Compatibility Tests

建立：

```
CLIENT_COMPATIBILITY.md
```

矩阵：

```
                  v2rayN   Shadowrocket   PassWall   sing-box   Mihomo

VLESS               ✓           ✓            ✓          ✓          ✓

Hysteria2           test        test         test       test       test

TUIC                 test        test         test       test       test
```

必须：

```
基于实际测试填写
```

不要凭印象标记。

---

# 78. 每次客户端版本变化

兼容性矩阵必须记录：

```
Client

Version

Platform

Test Date

Result
```

例如：

```
Shadowrocket

Version:
x.x.x

iOS

Result:
PASS
```

---

# 79. Unknown Client

无法识别客户端时：

默认：

```
Raw Subscription
```

或者项目测试后确定最兼容格式。

与上游行为一致：未命中 UA 规则即返回 raw base64。

注意：上游 Xray JSON 的 UA 自动识别默认关闭（正则为空），EasyBox 不依赖它。

但必须：

```
记录 User-Agent
```

以方便以后增加规则。

---

# 80. UI 原则

目标：

```
Simple by default

Powerful when needed
```

不要：

```
把所有专业选项都放第一页
```

---

# 81. 用户核心操作

整个产品最重要的三个按钮：

```
Create Node

Copy Subscription

Regenerate
```

用户正常使用时不应该频繁进入：

```
Inbound Editor
```

---

# 82. Empty State

第一次打开：

```
No nodes yet.

Create your first node.

[ Create Node ]
```

不要给用户一大片空表格。

---

# 83. Error UX

如果创建失败：

不要只显示：

```
Error 500
```

应该：

```
Node creation failed.

Step:
Xray validation

Reason:
...

Existing configuration was not changed.
```

---

# 84. Rollback

任何影响 Xray 主配置的操作必须：

```
backup
↓
change
↓
validate
↓
reload
↓
health check
```

失败：

```
restore
```

适用范围（v0.2）：

```
单条线路的增删改：按 §18「编排 + 补偿」处理，不做整份配置的 backup / restore

backup / restore 只用于修改 Xray 全局模板（Advanced 中的操作）

EasyBox MVP 不修改 Xray 全局模板
```

---

# 85. Backup

EasyBox 配置必须纳入：

```
3X-UI backup
```

实现：EasyBox 的表（easybox_*）放在上游同一个数据库中，通过 db.go 迁移创建。

上游 getDb / importDB（以及 Telegram 备份）会自动包含这些表，SQLite 与 PostgreSQL 均适用，不需要单独的备份流程。

备份文件属于敏感文件：

```
数据库中含 REALITY 私钥与全部客户端凭据
开启 Telegram 备份时，UI 必须提示：备份会把这些凭据发送到 Telegram
```

备份包含：

```
database

EasyBox metadata

templates

settings
```

敏感信息：

必须加密或严格控制文件权限。

---

# 86. Deployment

目标服务器初期：

```
Ubuntu

1 vCPU

1 GB RAM
```

因此：

禁止引入：

```
Redis

Kafka

Elasticsearch

Kubernetes

大型微服务体系
```

除非确有必要。

MVP 优先：

```
现有 3X-UI 进程

现有数据库

少量新增模块
```

部署方式（沿用上游）：

```
打包：在 fork 仓库用上游 release.yml（GitHub Actions）打 tag 出包
      x-ui-linux-amd64.tar.gz / x-ui-linux-arm64.tar.gz
      （SQLite 驱动需要 CGO，不建议在 Windows 本机交叉编译）

安装：install.sh / update.sh / x-ui.sh 中的 MHSanaei/3x-ui 替换为 fork 仓库地址
      bash <(curl -Ls https://raw.githubusercontent.com/<fork>/main/install.sh)

服务器上：
程序      /usr/local/x-ui/x-ui（面板 + 订阅 + EasyBox + TUIC 同一进程）
Xray      /usr/local/x-ui/bin/
数据库    /etc/x-ui/x-ui.db
服务      systemd x-ui

端口：面板（安装时随机）、订阅 2096、80（证书）、各线路端口（见 §9.2）

升级：x-ui update（指向 fork），启动时自动执行数据库迁移
```

首次进入面板时 EasyBox 自动检查：

```
面板证书是否可用（§9.1）
开启 Clash / Mihomo 订阅（上游默认关闭）
创建默认订阅用户（§23.1）
```

---

# 87. 性能要求

1C1G 环境：

后台空闲：

```
不能因为 EasyBox 模块显著增加长期内存占用
```

基准：Phase 0 先实测上游空闲内存（上游有 19 个 cron 任务），作为比较基线。

Subscription：

必须：

```
轻量

可缓存

无昂贵实时计算
```

---

# 88. 不做的事情

MVP 不做：

```
全新代理核心

取代 Xray

大型 SaaS

支付系统

用户商城

代理销售系统

复杂权限系统

移动 App

Kubernetes

复杂微服务
```

---

# 89. 网络功能边界

本项目实现：

```
协议配置管理

节点生命周期管理

客户端配置生成

订阅格式适配

配置轮换

节点故障恢复
```

不实现：

```
针对特定网络审查系统的自动规避策略

自动检测封锁并轮换配置以逃避检测

针对 DPI / 主动探测的专门规避参数生成器
```

协议模板应使用其项目支持的标准、安全配置，不把审查规避策略硬编码进产品逻辑。

## 89.1 REALITY 的使用边界

允许：

```
使用上游 3X-UI 已有的标准 REALITY 功能
（VLESS / Trojan，network = tcp / http / grpc / xhttp）

按上游默认行为生成参数：
x25519 密钥对（getNewX25519Cert）
shortIds
target / serverNames（取自上游 scanRealityTargets 扫描结果，
候选列表为上游 realityScanCandidates 设置项）
```

注意：上游"组装一个可用的 REALITY inbound"的逻辑在前端表单中
（frontend/src/pages/inbounds/form/useSecurityActions.ts），
后端只提供密钥生成与目标扫描接口。EasyBox 后端需自行编排这些调用。

EasyBox 不额外开发：

```
自研伪装目标站选择 / 评分策略
按地区或网络环境定制伪装参数
检测封锁后自动轮换伪装目标或配置
```

即：EasyBox 只调用上游能力，不在上游之上叠加规避策略逻辑。

VLESS 模板默认：

```
VLESS + REALITY（无需域名和证书）
```

可选：

```
VLESS + TLS（需要域名和证书，见证书策略）
```

Hysteria2 / TUIC 基于 QUIC，不适用 REALITY，仍需 TLS 证书。

---

# 90. README

最终必须生成：

```
README.md
```

至少包含：

```
项目目标

架构

如何开发

如何 Build

如何部署

如何升级

如何测试

如何从 upstream 合并

项目目录
```

---

# 91. ARCHITECTURE.md

生成：

```
ARCHITECTURE.md
```

包含：

```
3X-UI

EasyBox

Xray

Database

Subscription

Renderer

Client Detection
```

关系图。

---

# 92. DEVELOPMENT.md

包含：

```
Local setup

Build

Test

Debug

DB migration

Adding protocol templates

Adding renderer

Adding client support
```

---

# 93. HOW_TO_ADD_CLIENT.md

新增客户端时应该只需要：

```
1. 添加 User-Agent detection

2. 添加 capability

3. 指定 renderer

4. 添加 compatibility tests
```

不得修改核心 Node 逻辑。

---

# 94. HOW_TO_ADD_PROTOCOL.md

新增协议：

```
1. Add template

2. Add canonical mapping

3. Add renderer support

4. Add client capability

5. Add tests
```

---

# 95. 项目最终用户体验

用户第一次部署：

```
Login
↓
Quick Node
↓
选择 Hysteria2 / VLESS / TUIC
↓
Create
↓
Running
```

然后：

```
Copy Universal Subscription
```

得到：

```
https://sub.example.com/s/xxxxxxxx
```

---

# 96. 客户端使用体验

用户把同一个：

```
https://sub.example.com/s/xxxxxxxx
```

分别添加到：

```
v2rayN

Shadowrocket

PassWall

sing-box

Mihomo
```

服务器：

```
自动识别客户端
↓
过滤不支持节点
↓
返回对应格式
```

用户不需要：

```
找订阅转换网站

复制节点逐个转换

维护不同订阅地址
```

注意：上游订阅有 HWID 设备校验（enforceHwid）。

EasyBox 创建的订阅用户默认不设 HWID 限制，否则同一订阅地址在多个客户端 / 设备上可能被拒绝。

---

# 97. 后续节点变化

后台：

```
增加节点
删除节点
更新节点
重新生成节点
```

用户：

```
不更换订阅 URL
```

只需要：

```
Refresh Subscription
```

---

# 98. 项目核心价值

本项目不是：

```
又一个 Xray 面板
```

而是：

> 把专业代理节点配置和多客户端订阅格式封装成普通用户可以理解的一键式产品。

核心抽象：

```
Protocol Template
+
EasyBox
+
Canonical Node Model
+
Universal Subscription
+
Client Capability Matrix
```

这五个组件是整个项目最重要的架构。

---

# 99. AI Coding 第一次任务

收到本文档以后：

**不要立刻修改代码。**

第一阶段任务：

```
1. Clone 当前官方 3X-UI repository

2. 记录 upstream commit

3. 成功 Build

4. 成功运行

5. 阅读项目结构

6. 定位：
   - inbound
   - client
   - subscription
   - share link
   - API
   - database
   - Xray config
   - frontend routes

7. 输出：

ARCHITECTURE_RESEARCH.md
```

---

# 100. ARCHITECTURE_RESEARCH.md 必须回答

```
1. 当前 3X-UI 技术栈是什么？

2. 前端在哪里？

3. 后端在哪里？

4. Inbound 如何创建？

5. Client 如何创建？

6. Xray Config 如何生成？

7. Share Link 如何生成？

8. Subscription 如何生成？

9. User-Agent detection 当前如何实现？

10. Clash/Mihomo formatter 在哪里？

11. Database schema 在哪里？

12. 哪些模块可以直接复用？

13. EasyBox 最适合放在哪里？

14. Canonical Node Model 是否已有类似抽象？

15. sing-box renderer 应放在哪里？

16. 数据库需要增加哪些表？

17. Upstream 更新冲突可能集中在哪里？
```

---

# 101. 第二阶段任务

Architecture Research 完成以后：

生成：

```
IMPLEMENTATION_PLAN.md
```

必须包括：

```
Files to add

Files to modify

Database migrations

API changes

Frontend changes

Tests

Milestones

Rollback strategy
```

然后才开始开发。

---

# 102. MVP 验收标准

项目只有满足下面全部条件才能称为 MVP 完成。

### Node Creation

可以：

```
VLESS
Hysteria2
TUIC
```

一键创建。

### Node Management

可以：

```
查看
复制
二维码
重新生成
禁用
删除
```

### Universal Subscription

同一个 URL：

```
可以被不同客户端请求
```

并返回对应格式。

同一个 URL：

```
一次导入即包含全部已启用线路（VLESS / Hysteria2 / TUIC）
```

客户端不支持的协议被过滤，其余线路正常显示。

### Client Support

至少完成实际测试：

```
v2rayN

Shadowrocket

PassWall

sing-box

Mihomo
```

### Subscription Update

节点修改以后：

```
URL 不变化
```

客户端刷新：

```
获得最新配置
```

### Reliability

创建失败：

```
不残留配置
不影响已有线路
UI 显示失败步骤与原因
```

### Upgradeability

EasyBox 代码：

```
尽量不侵入 3X-UI 核心
```

可以继续同步上游。

---

# 103. 项目成功标准

最终普通用户只需要理解三个概念：

```
协议

节点

订阅
```

而不需要理解：

```
Inbound JSON

StreamSettings

Client JSON

Xray config

Clash YAML

sing-box JSON

Subscription converter
```

如果最终用户仍然需要大量手动配置这些内容：

> 产品设计即视为失败。

---

# 104. 最终架构总结

```
                         User
                           │
                           ▼

                  EasyBox Web UI
                           │
         ┌─────────────────┼─────────────────┐
         │                 │                 │
         ▼                 ▼                 ▼

   Quick Node           Nodes          Subscription
         │                 │                 │
         ▼                 ▼                 ▼

 Protocol Template    Node Manager    Universal Gateway
         │                 │                 │
         └──────────┬──────┘                 │
                    ▼                        │

              3X-UI Services                 │
                    │                        │
            ┌───────┴────────┐               │
            │                │               │
            ▼                ▼               │
         Database        Xray Core            │
                                             │
                                             ▼
                                   Canonical Node Model
                                             │
                              ┌──────────────┼──────────────┐
                              │              │              │
                              ▼              ▼              ▼
                             Raw           Mihomo        sing-box
                              │              │              │
                              ▼              ▼              ▼

                          Client Applications
```

核心目标：

> 选择协议 → 一键生成 → 一个订阅 URL → 多客户端直接导入。

---

# 附录 A. v0.2 变更记录（2026-10-20）

依据：上游 MHSanaei/3x-ui main `a8d0700`（v3.10.2）源码与 docs/architecture.md 核对。

```
项目名        EasyNode → EasyBox（包 internal/easybox，接口 /panel/api/easybox/，表 easybox_*）
术语          新增「术语约定」：节点 = 线路（Endpoint），避免与上游 model.Node（远程服务器）冲突
§4           修正可复用能力描述（证书、多节点、订阅、默认参数生成、安全）
§6 / §48     导航改为 仪表盘 / 快速创建 / 线路 / 订阅 / Advanced / System
§9           补充三个协议的上游支持情况
§9.1         新增证书策略（REALITY / 面板 IP 证书 / 自签 + 指纹锁定）
§9.2         新增端口与防火墙
§10          允许 Go 后端重写三个协议的默认参数生成（§4 例外），要求一致性测试
§12          运行时含 Xray 与 TUIC，按协议实现 HealthChecker，必须经 runtime.Runtime
§15          clientId 关联 ClientRecord，serverId 复用 Inbound.NodeID，表 easybox_endpoints
§17          DISABLED = Inbound.enable=false，自动从订阅排除
§18          「事务化」改为「编排 + 补偿」，不修改上游 AddInbound
§19 / §20 / §21  Regenerate 分「轮换凭据 / 保留旧线路」两种模式；服务器迁移移至 P2
§23.1        新增多线路合并订阅（一个订阅用户挂多条线路）
§25          不另起网关，扩展上游 internal/sub；首次初始化开启 Clash 订阅
§26          Canonical Node Model 仅用于 sing-box 与能力过滤
§29          订阅 URL = subURI + 随机 subPath + SubID，不要求 /s/
§31          格式选择优先使用上游路径
§37 / §38 / §39  订阅 token = ClientRecord.SubID，轮换 = 修改 SubID
§40          MVP 不做跨请求缓存
§41          沿用上游 ApplyCommonHeaders
§42          订阅接口保持上游 404 / 500 行为，诊断信息进日志与 Preview
§44          扩展上游 subpage.html
§49 / §50    复用上游安全能力；进程内直接调用 Service，不需要内部 Token
§55.1        新增「必须遵守上游开发约束」
§56          上游合并节奏与冲突热点
§63          开发顺序调整，测试随各 Phase 同步编写
§64 / §65    P0 增加多线路合并订阅；P1 增加多个订阅用户
§67          serverId 复用 Inbound.NodeID
§68 – §70    API 前缀改为 /panel/api/easybox/，nodes → endpoints
§79          与上游未知 UA 行为对齐
§84          backup / restore 仅用于 Xray 全局模板
§85          备份随上游数据库自动包含
§86          补充部署方式（fork release + install.sh）与首次初始化
§87          内存基线在 Phase 0 实测
§89.1        新增 REALITY 使用边界（方案 1）
§96          HWID 限制默认关闭
§102         Reliability 与 Universal Subscription 验收标准更新
§91 – §94 / §100  修复 Notion 导出的错误链接
```

---

# 附录 B. v0.2.1 变更记录（2026-10-20）

```
文件名        SSR.md → EasyBox.md
§9.1         订阅地址必须走 HTTPS（复用面板证书配置 subCertFile / subKeyFile）
§24          自动刷新：Profile-Update-Interval，默认 12 小时，订阅页提示
§36          Clash / Mihomo 代理组与规则沿用上游，最多加「自动选择 / 手动选择」
§55.1        i18n：只认真写 en-US / zh-CN，其余语言先填英文
§57          仓库 origin = github.com/zt0101/easybox（私有）；fork 后需处理的 Actions
§59          私有自用的 GPL 说明；名称核查
§62.1        新增 Docker 开发环境（devenv/）
§62.2        新增测试 VPS 与真机准备
§63.1        新增分批交付：MVP-0 / MVP-1 / MVP-2，当前目标 MVP-0
§76          sing-box 只支持 ≥ 1.12，按 UA 版本退回 raw；快照不得自动重生成
§85          备份文件敏感性与 Telegram 备份提示
```

---

# 附录 C. 实现状态（2026-10-21）

代码：github.com/zt0101/easybox，分支 feature/quick-create（基于 upstream a8d0700 / v3.10.2）。

```
MVP-0  完成  快速创建（VLESS+REALITY）、线路列表、合并订阅、前端三页
MVP-1  完成  Hysteria2（自签+指纹 / 面板证书）、TUIC（需面板证书，不可用时给出原因与引导）
             重新生成（替换 / 保留旧线路）、订阅轮换（SubID + 全部凭据）
             订阅 HTTPS（复用面板证书）、侧栏「高级」分组、EasyBox 品牌、概览卡片
MVP-2  完成  sing-box 完整配置（≥1.12，低版本退回 raw，?target=singbox）
             Canonical Node Model（由上游链接解析）、能力过滤、模拟客户端预览
             CLIENT_COMPATIBILITY.md（只记录实测）
文档    完成  easybox-docs/：README / ARCHITECTURE / DEVELOPMENT / HOW_TO_ADD_* / SPEC 副本
第一批新协议  完成（eb.6）  VLESS + TLS + Vision、Trojan + TLS（需面板证书）、Shadowsocks 2022（无需证书，备用）
             三种真实客户端内核（Xray / sing-box / Mihomo）回环实测通过，见 CLIENT_COMPATIBILITY.md
第二批新协议  进行中  VLESS + XHTTP + REALITY（eb.8，无需域名和证书；sing-box 不支持 XHTTP，订阅里去掉）
             VLESS + WebSocket + CDN（自己的域名 + Cloudflare API 令牌，一键建橙色云解析和源站证书；国内实测可用）
             VLESS + XHTTP + CDN：复用 CDN 线路的 Cloudflare 流程和优选地址，sing-box 不支持 XHTTP 自动过滤
             VLESS + Tunnel（备用，无域名）：随发布包附带 cloudflared，Cloudflare 临时隧道作为独立 systemd 服务运行，每台服务器一条，地址变化自动更新订阅
             订阅走 CDN：订阅页开关，加 sub-xxxxxx.域名 橙色云解析，IP 被封后仍能刷新订阅；IP 订阅照常可用；严格模式不开启
             CDN 优选地址：最多 10 个 Cloudflare IP/域名，各带备注，订阅多出「线路名-备注」节点（国内实测原节点超时、优选 300–500ms，速度明显提升）
             WebSocket 早期数据：放弃（Shadowrocket 2.2.92 实测加了 ?ed= 的节点连不上）
             XHTTP + CDN 模式固定 packet-up（eb.15，Shadowrocket 在 auto 下经 Cloudflare 无数据）
             trycloudflare 临时隧道实测国内连不上，放弃；Hysteria2 端口跳跃暂缓
发布    完成  install.sh / update.sh / x-ui.sh / 面板内更新改为从 zt0101/easybox 获取（否则会装回原版）
```

实测（开发容器回环，真实客户端内核经线路访问网页）：

```
                    Xray(v2rayN 内核)   sing-box 1.14.3        Mihomo 1.19.32
VLESS + REALITY     PASS                FAIL → 已过滤 ¹        PASS
Hysteria2 自签      PASS                PASS                   PASS
Hysteria2 面板证书  PASS                PASS                   PASS          ²
TUIC 面板证书       —（Xray 无 TUIC 客户端）PASS                 PASS          ²
¹ sing-box 无法验证 X25519MLKEM768 的 REALITY 服务端（全部指纹均失败），按 §34 对 sing-box 过滤
² 2026-10-21 在 VPS（v3.10.2-eb.2，Let's Encrypt IP 证书）实测，出口 IP 为 VPS
```

真机（用户反馈，2026-10-21，VPS）：v2rayN、手机 Clash、PassWall、手机 sing-box（通用地址）均可导入并连接；Stash（iOS）的 VLESS 可连接。
Shadowrocket：TUIC、Hysteria2 可连接；VLESS + REALITY + Vision 不通。
  原因：开着 Vision 时，Shadowrocket 不使用所选指纹，固定发只带 x25519 的 ClientHello，被 Xray 26.9+ REALITY 拒绝。
  证据：VPS 抓包证实；同一入站加一个不带 Vision 的用户，再选 Chrome140 指纹，就能连通。
  处理：不改 EasyBox（线路只保留一个标准 Vision 用户），等 Shadowrocket 修复。详见 easybox-docs/CLIENT_COMPATIBILITY.md「Known issue: Shadowrocket + Vision」。

```
```

与规划不同之处：

```
§9.2  默认线路端口范围定为 20000–20099
§40   不做跨请求订阅缓存
§42   订阅接口保持上游 404 / 500，诊断信息放在日志与预览
§44   公开订阅页（subpage）未增加兼容客户端区块（未做）
§57   版本标签用 v3.10.2-eb.N（easybox-v* 会被 install.sh 拒绝）；eb.4 起 release.yml 的 promote-latest job 把最高的 eb 版本标为 latest Release（上游的 Release 一律是预发布）
§68   路由沿用上游风格：/endpoints/list、/create、/del/:id、/setEnable/:id、/regenerate/:id、/setInSubscription/:id、/link/:id
§31   Clash / Mihomo 地址遇到 sing-box App（≥1.12）也返回 sing-box 配置，导错地址也能用
§22   Hysteria 链接在指纹为空时不输出 fp=（上游会输出空的 fp=）
§57   eb.6 起 x-ui -v、面板版本号显示 3.10.2-eb.N（发布构建写入 eb.N）；检查更新比较 eb 号，修复了一直提示有新版本的问题
§37   订阅用户的共用密码改为 16 字节密钥的 base64（Shadowsocks 2022 要求）；老面板第一次建 SS 线路时升级，界面先确认，设备需刷新订阅
§12   TUIC 线路创建后立即重启 Xray（上游要等最多 30 秒），否则「运行中」的线路暂时无法转发
§9    CDN 线路：SSL/TLS 模式只在「关闭」「灵活」时改成「完全」，不强制「完全（严格）」——这个设置对整个域名生效，强改会影响域名下的其他网站；源站证书本身是受信任的，管理员可以自己改成严格
```

需要人工处理的事项见交付说明。
