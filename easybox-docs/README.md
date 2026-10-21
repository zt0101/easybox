# EasyBox

EasyBox 是基于 [3X-UI](https://github.com/MHSanaei/3x-ui)（MHSanaei/3x-ui，GPL-3.0）的二次开发：
把复杂的协议配置封装成「选择协议 → 一键生成线路 → 一个订阅地址 → 多客户端直接导入」。

需求与规格见 [SPEC.md](SPEC.md)（与 `E:\ai_dev\ssr\EasyBox.md` 同步）。

## 功能

| 功能 | 说明 |
|---|---|
| 快速创建 | VLESS + REALITY（无需域名和证书）、Hysteria2（自签证书 + 指纹锁定，或面板证书）、TUIC v5（需要面板的受信任证书）；自动生成端口、密钥、凭据，失败自动清理并说明卡在哪一步 |
| 线路管理 | 列表、复制链接、二维码、启用 / 禁用、重新生成（替换 / 保留旧线路）、删除 |
| 通用订阅 | 一个地址包含全部线路；按客户端自动返回 raw（v2rayN / Shadowrocket / PassWall）、Clash/Mihomo YAML、sing-box 完整配置；客户端无法使用的线路自动过滤 |
| 订阅管理 | 订阅地址 / 二维码、勾选包含哪些线路、模拟客户端预览、泄露后重置地址（同时更换全部凭据） |
| 自动默认 | 订阅链接显示线路名、开启 Clash 订阅、有面板证书时订阅走 HTTPS（均只执行一次，不覆盖管理员的修改） |

原 3X-UI 的全部功能保留在侧栏「高级」中。客户端兼容性实测结果见 [CLIENT_COMPATIBILITY.md](CLIENT_COMPATIBILITY.md)。

## 安装（服务器）

需要 GitHub 上已有 EasyBox 的 Release（见下方「发布」）。

```bash
bash <(curl -Ls https://raw.githubusercontent.com/zt0101/easybox/main/install.sh)
```

安装脚本会随机生成面板端口、路径和账号，并可通过 acme.sh 为服务器 IP 签发 Let's Encrypt 证书（需开放 80 端口）。

需要放行的端口（系统防火墙 + 云厂商安全组）：

| 端口 | 协议 | 用途 |
|---|---|---|
| 80 | TCP | 证书签发与续期 |
| 面板端口 | TCP | 管理面板 |
| 2096 | TCP | 订阅地址 |
| 20000–20099 | TCP + UDP | EasyBox 线路（VLESS 用 TCP，Hysteria2 / TUIC 用 UDP） |

## 升级

在服务器执行 `x-ui update`，或面板内的更新按钮。两者都从 `zt0101/easybox` 获取，不会装回原版 3X-UI。

## 发布

版本号 = 上游基线版本 + 后缀，例如 `v3.10.2-eb.1`（`install.sh` 用 `sort -V` 校验版本，`release.yml` 按 `v*.*.*` 触发）。

```bash
git tag v3.10.2-eb.1 && git push origin v3.10.2-eb.1   # 需要仓库启用 GitHub Actions
```

不要推送上游的 `v*.*.*` 标签。

## 开发

见 [DEVELOPMENT.md](DEVELOPMENT.md)。架构见 [ARCHITECTURE.md](ARCHITECTURE.md)。
新增客户端 / 协议见 [HOW_TO_ADD_CLIENT.md](HOW_TO_ADD_CLIENT.md)、[HOW_TO_ADD_PROTOCOL.md](HOW_TO_ADD_PROTOCOL.md)。

## 从上游合并

```bash
git fetch upstream --tags
git checkout develop && git merge <上游 release 标签>
# 冲突热点：frontend/src/routes.tsx、src/layouts/AppSidebar.tsx、src/pages/api-docs/endpoints.ts、
# internal/sub/controller.go、internal/database/db.go 与 migrate_data.go、internal/web/translation/*.json、
# install.sh / update.sh / x-ui.sh / panel/panel.go（保留指向 zt0101/easybox 的地址）、.github/dependabot.yml（保留删除）
docker compose exec dev make verify
docker compose exec dev bash devenv/smoke-easybox.sh
```

完成后更新 `UPSTREAM_BASE.md`。完整流程、合并后必查项和 VPS 实测见 [UPSTREAM_SYNC.md](UPSTREAM_SYNC.md)。

## 许可

GPL-3.0（继承自 3X-UI）。分发安装包时必须同时提供对应源码；保留原项目版权与署名。
