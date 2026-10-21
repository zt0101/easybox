# 新增协议（线路模板）

以新增一个模板为例，所有改动都在 EasyBox 自己的文件里，不需要改核心流程。

1. **模板**：在 `internal/easybox/templates.go` 的 `lineTemplates` 加一项：
   `id`、显示名、`label`（订阅/界面显示的协议名）、`transport`（`tcp` 或 `udp`，决定健康检查和防火墙提示）、
   `namePrefix`（默认线路名前缀）、`build`、`available`（不可用时返回原因，例如缺少证书）。
2. **入站生成**：`build` 按上游前端表单的默认值生成 `model.Inbound`
   （参考 `frontend/src/lib/xray/inbound-defaults.ts` 与 `pages/inbounds/form/InboundFormModal.tsx`）；
   客户端凭据用订阅用户 `Identity` 的字段，保证所有协议共用一个 ClientRecord。需要新凭据字段时，
   加到 `model.EasyboxSubscription` 和 `Identity`，并在 `ensureDefaultSubscriptionRow` 回填、在 `RotateSubscription` 轮换。
3. **Canonical 映射**：在 `internal/sub/easybox_singbox.go` 的 `canonicalFromLink` 解析新协议链接。
4. **渲染器**：在 `singboxOutbound` 输出 sing-box 出站；raw 和 Mihomo 沿用上游订阅服务。
5. **客户端能力**：无法表达或实测失败的情况返回带原因的错误（参考 `errSingboxRealityMLKEM`），线路会被过滤并在预览中说明。
6. **测试**：
   - `internal/easybox/*_test.go`：经上游 `AddInbound` 创建 → 订阅服务输出 → 上游链接解析器解析回来；
   - 多协议共用一个订阅用户（`TestMixedProtocols_ShareOneSubscriptionUser`）；
   - sing-box 配置（`SINGBOX_BINARY` 下用真实 `sing-box check`）；
   - `devenv/smoke-easybox.sh` 加一段真实客户端经线路访问网页；
   - 更新 `CLIENT_COMPATIBILITY.md`（只填实测结果）。
7. **界面与文案**：`easybox.templates.<id>` 写入 13 个翻译文件（en-US、zh-CN 认真写）。
