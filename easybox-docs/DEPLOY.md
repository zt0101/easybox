# 在新 VPS 上部署 EasyBox

本文以 Ubuntu / Debian 为例，`install.sh` 也支持 CentOS、Alma、Rocky、Fedora、Arch、Alpine、openSUSE。下文中的 `<版本>` 指一个 EasyBox Release tag，例如 `v3.10.2-eb.3`，可以在 [Releases](https://github.com/zt0101/easybox/releases) 查看。

## 先了解：版本号与「最新 Release」

`release.yml` 上传时会先把 Release 建成预发布（pre-release）。从 **v3.10.2-eb.4** 起，所有平台都上传成功后，`promote-latest` job 会把它改成正式版并标为 latest（前提是它是最高的 eb 版本）。之后这三种方式都能用，会自动装最新的 eb 版本：

- 不带版本号运行 `install.sh`；
- `x-ui update`；
- 面板里的「更新」按钮。

eb.1 到 eb.3 是改动之前发布的，都还是预发布；在 eb.4 发布之前，`releases/latest` 返回 404，上面三种方式会失败（报 "Failed to fetch x-ui version"）。

本文下面的命令都写明了版本号，任何时候都能用，也方便固定安装某个版本。如果某次构建没有自动标为 latest（比如有平台构建失败），可以在 GitHub → Releases → 编辑该版本里手动处理：取消勾选「Set as a pre-release」，勾选「Set as the latest release」。

## 1. 准备

- **root 权限**，或可以用 sudo 的用户。
- **固定公网 IP**：在云厂商控制台把 IP 设为静态。面板和 QUIC 线路的证书签给这个 IP，IP 一变证书就失效，订阅地址也会跟着失效。
- **放行端口**：云厂商的安全组或防火墙要放行；如果系统里开着 ufw 或 firewalld，也要放行。

| 端口 | 协议 | 用途 |
|---|---|---|
| 80 | TCP | 申请和续期 Let's Encrypt IP 证书（acme.sh），需要一直开着 |
| 面板端口（安装时指定） | TCP | 面板后台 |
| 2096 | TCP | 订阅地址 |
| 20000–20099 | TCP + UDP | EasyBox 线路：VLESS、Trojan 走 TCP，Hysteria2 和 TUIC 走 UDP，Shadowsocks 两者都用 |
| 443、8443、2053、2083、2087 | TCP | 只有建 CDN 线路时需要：Cloudflare 只会回源到这几个 HTTPS 端口，面板按顺序挑第一个空闲的（2096 是订阅地址） |

## 2. 安装

### 方式 A：交互安装（第一次用推荐这种）

```bash
sudo bash -c 'bash <(curl -Ls https://raw.githubusercontent.com/zt0101/easybox/<版本>/install.sh) <版本>'
```

按提示回答：

- **SSL**：选 2（IP 证书，默认就是这一项）。有域名的话可以选 1（域名证书）。
- **数据库**：选 1（SQLite）。
- **是否自定义面板端口**：建议选 y，填一个已经在防火墙里放行的端口（例如 22215）。不自定义的话会随机分配，装完还要再去放行。

### 方式 B：一条命令装完（不交互）

```bash
curl -fsSL -o install.sh https://raw.githubusercontent.com/zt0101/easybox/<版本>/install.sh
sudo XUI_NONINTERACTIVE=1 XUI_SSL_MODE=ip XUI_DB_TYPE=sqlite XUI_PANEL_PORT=22215 bash install.sh <版本>
```

可选的环境变量：

| 变量 | 作用 |
|---|---|
| `XUI_SSL_MODE` | `ip`（IP 证书）、`domain`（域名证书，需同时设置 `XUI_DOMAIN`）、`none`（不用 HTTPS，只适合放在反向代理或 SSH 隧道后面） |
| `XUI_PANEL_PORT` | 面板端口；不设置就随机分配 |
| `XUI_USERNAME`、`XUI_PASSWORD`、`XUI_WEB_BASE_PATH` | 自己指定登录信息；不设置就随机生成 |
| `XUI_ACME_EMAIL` | 申请证书时登记的邮箱 |

安装完成后服务会自动启动，开机也会自动启动（systemd `x-ui.service`，状态为 `enabled`，异常退出后 5 秒自动重启）。证书续期由 acme.sh 的 cron 任务负责，每天检查 4 次。

## 3. 查看登录信息

```bash
sudo cat /etc/x-ui/install-result.env
```

这个文件只有 root 能读，里面有：

- `XUI_ACCESS_URL`：面板地址，格式是 `https://<IP>:<端口>/<随机路径>/`
- `XUI_USERNAME`、`XUI_PASSWORD`：账号和密码

`sudo x-ui settings` 只显示端口、路径和访问地址，**不显示**账号密码。登录后建议先修改密码；注意改完后这个文件里记的还是旧的账号密码。忘记账号密码时，可以在 `sudo x-ui` 菜单里选 7（Reset Username & Password）重置。

## 4. 建线路和订阅

1. 登录面板，在**快速创建**里选模板：

   快速创建按国内的抗封锁能力给模板分三类（eb.16 起，卡片上有标签，选中后显示详细说明）：
   - **推荐**：VLESS + REALITY、VLESS + XHTTP、Hysteria2、VLESS + XHTTP + CDN
   - **备用**：VLESS + WebSocket + CDN、VLESS + Tunnel、TUIC v5
   - **易识别**：VLESS + TLS、Trojan、Shadowsocks 2022（国内容易被识别，可能连累服务器 IP；只建议境外或应急用）

   | 模板 | 证书 | 能用的客户端（实测见 [CLIENT_COMPATIBILITY.md](CLIENT_COMPATIBILITY.md)） |
   |---|---|---|
   | VLESS + REALITY | 不需要 | v2rayN、Clash/Mihomo、PassWall、Stash。sing-box 和 Shadowrocket 目前连不上：Xray 26.9+ 要求握手带 X25519MLKEM768，sing-box 不支持；Shadowrocket 开着 Vision 时不使用所选指纹（已知问题，等它修复，见 CLIENT_COMPATIBILITY.md） |
   | VLESS + XHTTP | 不需要（同样走 REALITY） | v2rayN、PassWall、Clash/Mihomo（实测见 CLIENT_COMPATIBILITY.md）。sing-box 没有 XHTTP，订阅里自动去掉。比 TCP 线路更难识别 |
   | Hysteria2 | 面板证书；没有面板证书时用自签证书加证书指纹 | 全部 |
   | TUIC v5 | 必须有面板证书 | 除 Xray 内核外的全部客户端（v2rayN 会自动切到 sing-box 内核） |
   | VLESS + TLS | 必须有面板证书 | 全部；不走 REALITY，所以 sing-box 和 Shadowrocket 也能用（2026-10-22 真机实测） |
   | Trojan | 必须有面板证书 | 全部（含 Shadowrocket，已真机实测）。Xray 已标为「不推荐，将来可能移除」 |
   | Shadowsocks 2022 | 不需要 | 全部（含 Shadowrocket，已真机实测）。备用线路：国内较易被识别；同样被 Xray 标为不推荐 |
   | VLESS + WebSocket + CDN | 不需要面板证书，要有自己的域名并接入 Cloudflare（见下） | 全部（VLESS + WebSocket + TLS）。经 Cloudflare 中转，服务器 IP 被封也能用；延迟比直连高 |
   | VLESS + Tunnel（备用） | 都不需要：经 Cloudflare 临时隧道（xxx.trycloudflare.com），没有域名也能有一条 IP 被封后仍可用的线路 | 全部（VLESS + WebSocket + TLS）。每台服务器一条；**国内要配合优选地址**（不填时延迟高、不稳定）。VPS 重启后隧道地址会变，面板自动更新订阅，客户端要刷新订阅——如果那时 IP 已被封、订阅刷不到，这条线路就失效了。有域名的话优先用 VLESS + WebSocket + CDN。需要 systemd 和发布包附带的 cloudflared（amd64、arm64、armv7、386） |
   | VLESS + XHTTP + CDN | 同上 | v2rayN、v2rayNG、PassWall、Clash/Mihomo、Shadowrocket（eb.15 起，XHTTP 模式固定为 packet-up）；sing-box 不支持 XHTTP，订阅里自动去掉。比 WebSocket 更难识别，可以和上一条互为备份（各占一个 Cloudflare 端口，443 之后是 8443） |

   老面板第一次建 Shadowsocks 2022 时，会把 TUIC / Trojan 共用的密码换成新格式（界面会先确认），之后设备要刷新一次订阅。

   **CDN 线路的准备**（只做一次）：
   1. 域名的 NS 改成 Cloudflare 的，在 Cloudflare 后台显示「有效」。
   2. Cloudflare 后台 → 我的个人资料 → API 令牌 → 创建令牌 →「创建自定义令牌」，添加四项权限：区域-区域-读取、区域-DNS-编辑、区域-SSL 和证书-编辑、区域-区域设置-编辑；区域资源选「特定区域」和这个域名。
   3. 快速创建页的「CDN 线路：连接 Cloudflare」里填域名和令牌，点「验证并保存」。

   之后每建一条 CDN 线路，面板自动在域名下加一条橙色云解析 `cdn-xxxxxx.你的域名`，申请 Cloudflare 源站证书（15 年有效，不用续期）；SSL/TLS 模式是「关闭」或「灵活」时改成「完全」，已经是「完全」或「完全（严格）」的不动（这个设置对整个域名生效）。删除线路时解析记录和证书一起删掉。

   **优选地址**（可选，强烈建议）：国内直连 Cloudflare 往往很慢，甚至连不上（2026-10-22 公司网络实测：原节点超时，优选 IP 300–500ms）。在自己常用的网络下（先关掉代理）运行 [CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest/releases)：`cfst.exe -dd -tl 400`，把延迟最低的几个 IP 填进「连接 Cloudflare」卡片的「优选地址」，每行一个 IP 加一个备注，最多 10 个。每条 CDN 线路在订阅里多出「线路名-备注」节点（备注留空为「线路名-优选1」），域名和证书不变，原节点保留；Clash 和 sing-box 的「自动选择」会挑最快的。不同网络（公司、家里、手机流量）测出的最快 IP 不一样，可以各填几个，用备注区分（如「公司移动」「家里电信」）。

   节点数量：优选节点都连同一个 CDN 入站，不增加服务器负担，只会让订阅列表变长。建议只建一条 CDN 线路，整个订阅控制在 20–25 个节点以内；1 GB 内存的小机器上线路（入站）不超过 10–12 条。真正限制小机器的是同时在用的设备数和流量费用。

   隧道线路（VLESS + Tunnel）不需要域名和 Cloudflare 账号：面板用随发布包附带的 cloudflared 开一个 Cloudflare 临时隧道，作为单独的 systemd 服务 `easybox-tunnel` 运行（以无权限的临时用户运行、不能写文件），所以面板重启、升级、证书续期都不会改变隧道地址；只有 VPS 重启或 cloudflared 崩溃时地址才会变，面板每分钟检查一次并更新订阅。

   **订阅走 CDN**（有域名时强烈建议）：在**订阅**页打开「通过 Cloudflare 提供订阅」，面板加一条橙色云解析 `sub-xxxxxx.你的域名`，订阅多一个经 Cloudflare 的地址；服务器 IP 被封后客户端仍能用它刷新订阅（隧道地址变化等也能及时拿到）。IP 订阅地址照常可用。客户端里换成这个地址，并打开「通过代理更新订阅」。条件：已连接 Cloudflare、订阅端口是 Cloudflare 支持的（默认 2096 就是）、SSL/TLS 模式不是「完全（严格）」。2026-10-22 实测：国内手机 v2rayNG 经这个地址（开着「通过代理更新订阅」）刷新成功，节点和 IP 订阅一致。

2. 在**订阅**页复制**通用订阅**地址，导入任何客户端都用这一个地址，服务端会按客户端返回对应的格式。Clash / Mihomo 地址一般用不到，sing-box App 导入它也能用。

## 5. 验证

- **用设备验证**：导入订阅后连接节点，打开 `https://api4.ipify.org`，显示的是这台 VPS 的 IP 就说明连上了。
- **在开发容器里验证**：把开发容器的 SSH 公钥加到这台 VPS 上，然后运行：

  ```bash
  VPS=<IP> SSH_USER=<用户> SSH_KEY=<私钥路径> bash devenv/smoke-vps.sh
  ```

  脚本会创建 `test-tuic` 和 `test-hysteria2` 两条线路，用 Xray、sing-box、Mihomo 三种客户端真实连接。这两条线路测完会留着，方便设备测试，不需要的话在面板里删掉。

## 6. 升级

```bash
curl -fsSL -o update.sh https://raw.githubusercontent.com/zt0101/easybox/<新版本>/update.sh
sudo XUI_UPDATE_TAG=<新版本> bash update.sh
```

升级会保留设置、线路、订阅和证书，并校验下载包的 SHA-256。新版本被标为最新 Release 之后（eb.4 起自动标，见开头），也可以直接用 `x-ui update` 或面板里的更新按钮。

## 常见问题

| 现象 | 原因与处理 |
|---|---|
| 安装时提示 "Failed to fetch x-ui version" | 没写版本号，而且当前没有被标为 latest 的 Release，见开头 |
| 证书申请失败 | 80 端口没放行，或者被其他程序占用。用 `sudo ss -ltnp \| grep ':80 '` 查看 |
| 能打开面板，但节点连不上 | 20000–20099 没放行，Hysteria2 和 TUIC 要放行 **UDP** |
| 某些网络下 Hysteria2 / TUIC 连不上 | 当前网络限制了 UDP，换一个网络（比如手机流量）再试 |
| 保存 Cloudflare 时提示 "rejected the API token" | 令牌没复制完整，或者已被删除；重新复制一次 |
| 建 CDN 线路时提示 "lacks a permission" | 令牌少了某项权限，按上面四项补齐（在 Cloudflare 里编辑令牌即可，令牌本身不变） |
| CDN 线路建好了但连不上 | 等一两分钟让解析生效；确认云厂商防火墙放行了线路的端口（443 等）；Cloudflare 的 SSL/TLS 模式不能是「灵活」 |
| 某台设备上 REALITY、XHTTP 线路连不上（报 EOF 或 TLS handshake timeout），别的设备正常 | 客户端内核太旧。例如 v2rayNG 1.9.45（Xray 25.3.31）连不上，更新到 2.3.11 后正常；应用商店常滞后，从 GitHub 下载最新版 |
| 隧道线路突然连不上 | VPS 重启过、隧道地址变了：客户端刷新订阅。面板「线路」页显示当前隧道地址；`systemctl status easybox-tunnel` 看隧道服务 |
| CDN 线路能连但延迟很高，或时通时断 | 国内到 Cloudflare 默认节点的线路差，填「优选地址」，见上 |
| 访问面板根路径返回 404 | 正常现象：面板地址带随机路径，要用 `XUI_ACCESS_URL` 里的完整地址 |
| VPS 重启后 IP 变了 | 证书和订阅都会失效。先把 IP 设为静态，再重新签发证书：`sudo x-ui` 菜单选 20（SSL Certificate Management），再选 6（Get SSL for IP Address）。开着「通过 Cloudflare 提供订阅」的话，到订阅页把它关掉再打开，解析记录才会指向新 IP |
