# EasyBox 开发

## 本机环境（Docker，不在本机装工具链）

源码在 Docker 卷 `easybox-dev_src`，启动器在 `E:\ai_dev\ssr\easybox-dev\`（见其中 README）。
开发镜像（`devenv/Dockerfile`）包含 Go 1.27、Node 26、gcc、golangci-lint、Playwright Chromium、Xray。

```bash
cd /e/ai_dev/ssr/easybox-dev
docker compose up -d
docker compose exec dev bash                     # 进入 /src
docker compose exec dev bash devenv/panel.sh      # 构建前端 + 后端并运行面板（http://localhost:12053）
```

编辑代码：VS Code「Dev Containers: Attach to Running Container」→ `easybox-dev-dev-1` → `/src`。

## 构建与测试

```bash
make test-go           # Go 测试
make verify            # 上游完整门禁：代码生成、lint、格式、类型检查、全部测试、构建、Storybook
SINGBOX_BINARY=/root/.cache/clients/sing-box go test ./internal/easybox/   # 用真实 sing-box 校验配置
bash devenv/smoke-easybox.sh    # 端到端：真实 Xray / sing-box / Mihomo 客户端经线路访问网页
bash devenv/smoke-baseline.sh   # 上游基线（合并上游后先跑）
```

客户端二进制（sing-box、Mihomo）从官方 GitHub Release 下载到 `/root/.cache/clients/`，按 Release 公布的 SHA-256 校验。

## 必须遵守的上游规则（上游 CLAUDE.md）

- TDD：先写会失败的测试；测试走真实代码路径（真实 DB、上游 Service），只替换进程 / 网络边界。
- 新增 `/panel/api/*` 路由必须登记到 `frontend/src/pages/api-docs/endpoints.ts`，然后 `make gen`（路由契约测试双向校验）；
  返回的结构体加入 `tools/openapigen` 的 StructAllow。
- 新增 i18n key 必须写入 `internal/web/translation/` 全部 13 个文件，并且被代码引用（dead-key 测试）。
  EasyBox：en-US、zh-CN 认真写，其余填英文。
- 状态变更经过 `runtime.Runtime` / 上游 Service，不直接调用 `internal/xray/api.go`。
- 数据库变更：模型 + `db.go` + `migrate_data.go`。
- 注释每块最多 2 行；提交信息用 conventional commits。

## DB 迁移

新表：在 `internal/database/model/easybox.go` 定义，加入 `allModels()`（db.go）和 `migrationModels()`（migrate_data.go）。
新列：AutoMigrate 会自动添加；旧数据需要补值时在代码里回填（参考 `ensureDefaultSubscriptionRow` 对 Auth / Password 的回填）。

## 调试

- 面板日志：`/var/log/x-ui/panel.out`；sing-box 跳过的线路会记为 `easybox singbox: skipped line`。
- 订阅预览：面板「订阅」页的「模拟客户端预览」，或 `GET /panel/api/easybox/subscription/preview?client=singbox`。
- 显式格式：通用订阅地址加 `?target=singbox`；Clash/Mihomo 使用上游的 Clash 订阅地址。

## 新增模板 / 渲染器 / 客户端

见 [HOW_TO_ADD_PROTOCOL.md](HOW_TO_ADD_PROTOCOL.md) 与 [HOW_TO_ADD_CLIENT.md](HOW_TO_ADD_CLIENT.md)。
