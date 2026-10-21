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
10. 等 Release 构建完成 → VPS 上运行 sudo XUI_UPDATE_TAG=<tag> bash update.sh → 在 VPS 上实测各客户端
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
| `frontend/public/openapi.json`、`frontend/src/generated/*` | EasyBox 接口的文档和类型 | 不要手工合并，跑 `make gen` |
| `frontend/src/pages/api-docs/endpoints.ts` | EasyBox 接口出现在 API 文档页 | 两边都保留 |
| `go.mod`、`.oss-scanner/Dockerfile` | Go 版本 | 取较高的版本；两处要一致 |

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

   版本有变化的话，在 VPS 上用 Xray、sing-box、Mihomo 把三种线路（VLESS+REALITY、Hysteria2、TUIC）都实测一遍，有真机条件也测一下，结果写进 CLIENT_COMPATIBILITY.md 再发版。

4. **上游改了 EasyBox 依赖的接口**。EasyBox 调用了上游的入站服务、订阅服务、REALITY 扫描（`ScanRealityTargets`）、证书设置（`GetCertFile` / `GetKeyFile`）。编译或 `internal/easybox` 测试失败，就说明这里有变化，要跟着上游改，不要去改上游。

5. **govulncheck**。CI 用 go.mod 里的 Go 版本跑 govulncheck，标准库有新漏洞时会失败（eb.1 就是这样）。需要时把 Go 版本升到修复版。

## VPS 实测

在开发容器里运行 `VPS=<服务器 IP> bash devenv/smoke-vps.sh`（SSH 密钥和用户见脚本开头的变量）。脚本会登录面板，创建 `test-tuic` 和 `test-hysteria2`，再用 Xray、sing-box、Mihomo 三种客户端连接，确认返回 204、出口 IP 是 VPS。另外还会检查 Clash 地址对 sing-box 返回 JSON、链接里没有空 `fp=`、`x-ui settings` 不再报警告。
