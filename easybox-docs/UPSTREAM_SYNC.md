# 跟进上游 3X-UI 更新

EasyBox 是 MHSanaei/3x-ui 的 fork。当前基线记录在仓库根目录 [UPSTREAM_BASE.md](../UPSTREAM_BASE.md)，规划见 SPEC §56。

改动规模（基线 a8d0700 / v3.10.2）：动过的上游文件 41 个（约 1.9%），删除或改写的上游代码 83 行（约 0.02%），其余都是新增文件。所以上游更新大多可以直接合并，冲突集中在少数几个文件。

## 节奏

- **常规**：上游每发一个正式 release（v3.10.3、v3.11.0 ……），合并一次。
- **紧急**：上游修了安全问题，或者内置 Xray 的改动影响客户端连接时，尽快合并，不等下一个版本。
- **不合并 main 上的中间提交**：只跟 release tag，避免带进上游还没稳定的改动。

查看上游是否有新版本：

```
git fetch upstream --tags
git ls-remote upstream refs/heads/main          # 和 UPSTREAM_BASE.md 里的 Base commit 比较
git tag -l 'v*' --sort=-v:refname | grep -v eb | head -3
```

## 步骤

```
1. git fetch upstream --tags
2. git checkout develop && git checkout -b sync/<上游版本>       # 例：sync/v3.10.3
3. git merge <上游版本>                                         # 用 merge，不用 rebase：eb tag 已经发布，不能改写历史
4. 解决冲突（见下文「冲突点」）
5. make gen                                                   # 重新生成，不要手工合并自动生成的文件
6. 检查（见下文「合并后必查」）
7. make verify
   bash devenv/smoke-baseline.sh                              # 上游原有功能
   bash devenv/smoke-easybox.sh                               # EasyBox 功能
8. 合回 develop，更新 UPSTREAM_BASE.md 表格（Base commit、Commit date、Latest release at base、Recorded）
9. develop 以 --no-ff 合入 main，打 tag v<上游版本>-eb.1，推送 main 和这个 tag
   （不要推送上游自己的 tag，例如 v3.10.3；main 上必须删掉 .github/dependabot.yml）
10. 等 Release 构建完成，并确认它已被标为 latest（Actions 里的 promote-latest job）→ VPS 上运行 sudo XUI_UPDATE_TAG=<tag> bash update.sh → 在 VPS 上实测各客户端
    → 更新 easybox-docs/CLIENT_COMPATIBILITY.md
```

版本号规则：`v<上游版本>-eb.N`。同一个上游版本上的 EasyBox 补丁递增 N（eb.1、eb.2 ……），上游版本一变，N 从 1 重新开始。不要用 `easybox-v*`，install.sh 会拒绝这种格式（SPEC 附录 C §57）。

## 冲突点

| 文件 | EasyBox 改了什么 | 冲突时怎么处理 |
|---|---|---|
| `frontend/src/layouts/AppSidebar.tsx`、`frontend/src/routes.tsx` | EasyBox 菜单、「高级」分组 | 保留上游的新菜单，再加回 EasyBox 入口 |
| `internal/sub/controller.go`、`internal/sub/service.go` | sing-box 分支（通用地址和 Clash 地址都有）、Hysteria 链接不输出空 `fp=` | 改动只有几行，按上游的新结构重新插进去；`easybox_clash_singbox_test.go` 会验证 |
| `internal/database/db.go`、`internal/database/migrate_data.go` | 注册 EasyBox 的表 | 两边的表都保留 |
| `internal/web/translation/*.json` | `easybox.*` 文字，加在文件末尾 | 两边都保留 |
| `install.sh`、`update.sh`、`x-ui.sh`、`DockerEntrypoint.sh` | 下载地址改成 zt0101/easybox；`x-ui setting -show` 等布尔参数不带 `true` | **最需要小心**：上游经常改这几个脚本，见「合并后必查」第 1 条 |
| `internal/web/service/panel/panel.go` | 面板内更新改从 zt0101/easybox 下载 | 同上 |
| `internal/sub/host_sub.go`、`internal/web/service/host.go`（只读依赖） | CDN 优选地址用上游 Host 组实现（组 ID `easybox-cdn-*`，名称靠 remark 模板里的 `{{HOST}}`）。上游如果改了 Host 的字段、`buildHostRows` 或 `{{HOST}}` 的含义，要重跑 `cdn_preferred_test.go` | 不改上游文件，只核对行为 |
| `main.go` | `easybox.Init()` 之后调用 `easybox.StartTunnelWatch()` | 两边都保留 |
| `internal/sub/controller.go`（initRouter） | 订阅路由组加 `noStore` 中间件（订阅会经过 Cloudflare，不能被缓存） | 保留；上游新增订阅路由组时也加上 |
| `internal/sub/service.go`（PrepareForRequest）、`internal/sub/easybox_host.go` | `SubRequestHostOverride`：经订阅 CDN 域名的请求，默认节点地址换回服务器 IP | 保留；上游改了默认节点地址的取法要重跑 `subcdn_test.go` |
| `.github/workflows/release.yml`（cloudflared） | Linux 构建里固定版本 + SHA256 下载 cloudflared 到 `bin/cloudflared`（amd64/arm64/armv7/386） | 保留；升级 cloudflared 时同时改版本号和四个 SHA256 |
| `install.sh`、`update.sh`（cloudflared） | 解压后 `chmod +x bin/cloudflared`（文件存在时） | 保留 |
| `tools/openapigen/main.go` | `internal/easybox` 的类型白名单（含 CloudflareView / CloudflareRequest） | 两边都保留 |
| `frontend/public/openapi.json`、`frontend/src/generated/*` | EasyBox 接口的文档和类型 | 不要手工合并，跑 `make gen` |
| `frontend/src/pages/api-docs/endpoints.ts` | EasyBox 接口出现在 API 文档页 | 两边都保留 |
| `go.mod`、`.oss-scanner/Dockerfile` | Go 版本 | 取较高的版本；两处要一致 |
| `.github/workflows/release.yml` | 文件末尾新增 `promote-latest` job：eb 版本构建完成后标为 latest；Linux 和 Windows 的 `go build` 前把 tag 里的 `eb.N` 写入 `config.easyboxRelease` | 保留这个 job 和两处 `EasyBox: stamp eb.N` 代码块；如果上游改了 build / build-windows 的 job 名，要同步修改 `needs` |
| `internal/config/config.go`、`internal/util/version/version.go`、`internal/web/service/panel/panel.go`、`x-ui.sh` | 版本号带 `-eb.N`：`GetPanelVersion` 调 `GetReleaseVersion`；`parse` 先去掉 `-eb.N`；检查更新用 `version.CompareRelease`；`installed_script_url` 的正则允许 `-eb.N` | 每处只有一两行，带 `EasyBox:` 注释；`internal/config/easybox_test.go`、`internal/util/version/easybox_test.go`、`internal/web/service/panel/easybox_version_test.go` 会验证 |
| `internal/web/web.go`、`internal/sub/sub.go` | 停止时忽略监听端口「已关闭」的错误（`httpServer.Shutdown` 已经关过一次）；web.go 停止 Xray 时，如果 Xray 已经退出（systemd 会同时给它发信号）不算错误 | 上游如果自己修了，用上游的写法 |

## 合并后必查

1. **没有装回原版的地址**。下面这条命令必须没有输出：

   ```
   git grep -n "MHSanaei/3x-ui" -- install.sh update.sh x-ui.sh DockerEntrypoint.sh DockerInit.sh internal/web/service/panel/
   ```

   作为参照，a8d0700 基线上 `zt0101/easybox` 出现的次数：install.sh 11 次、update.sh 9 次、x-ui.sh 6 次、panel.go 3 次。

2. **脚本的参数写法**。下面这条命令必须没有输出（上游新增的调用可能又带上 `true`）：

   ```
   git grep -nE "x-ui setting -(show|getCert|getListen) true"
   ```

3. **内置 Xray 版本**。对用户影响最大的就是这一项：Xray 26.9.x 起 REALITY 要求握手带 X25519MLKEM768，导致 sing-box 和 Shadowrocket 连不上 VLESS。

   ```
   git diff <旧基线> <上游版本> -- .github/workflows/release.yml | grep Xray-core/releases/download
   ```

   版本有变化的话，在 VPS 上用 Xray、sing-box、Mihomo 把所有线路模板都实测一遍（`devenv/smoke-vps.sh`），有真机条件也测一下，结果写进 CLIENT_COMPATIBILITY.md 再发版。

   EasyBox 目前比上游新：eb.6 起用 Xray v26.10.21（release.yml 两处、DockerInit.sh 一处）。上游还停在 v26.10.10，而 Xray 已撤回这个版本（内置的根证书有问题），下载 404，用旧地址构建会失败（tag v3.10.2-eb.5 就是这样失败的，没有发布）。合并上游时，Xray 下载地址保留两者中较新的版本。

4. **上游改了 EasyBox 依赖的接口**。EasyBox 调用了上游的入站服务、订阅服务、REALITY 扫描（`ScanRealityTargets`）、证书设置（`GetCertFile` / `GetKeyFile`）。编译或 `internal/easybox` 测试失败，就说明这里有变化，要跟着上游改，不要去改上游。

5. **版本号**。合并后 `internal/config/version` 改成上游的新版本，第一个 tag 必须是 `v<这个版本>-eb.1`：release.yml 会核对 tag 和这个文件，不一致就构建失败。

6. **govulncheck**。CI 用 go.mod 里的 Go 版本跑 govulncheck，标准库有新漏洞时会失败（eb.1 就是这样）。需要时把 Go 版本升到修复版。

## VPS 实测

在开发容器里运行 `VPS=<服务器 IP> bash devenv/smoke-vps.sh`（SSH 密钥和用户见脚本开头的变量）。脚本会登录面板，创建 `test-tuic` 和 `test-hysteria2`，再用 Xray、sing-box、Mihomo 三种客户端连接，确认返回 204、出口 IP 是 VPS。另外还会检查 Clash 地址对 sing-box 返回 JSON、链接里没有空 `fp=`、`x-ui settings` 不再报警告。
