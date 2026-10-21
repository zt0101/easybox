# 新增客户端支持

不需要修改线路或订阅用户的逻辑（EasyBox.md §93）。

1. **识别**：客户端如何被识别、拿到什么格式：
   - 使用 Clash/Mihomo YAML：上游设置「Clash User-Agent 正则」（默认 `(?i)(clash|mihomo)`）已覆盖大多数；
   - 使用 sing-box 完整配置：在 `internal/sub/easybox_singbox.go` 的 `SingboxClient` 识别其 User-Agent，
     并在 `TestSingboxClient` 加一行真实 UA；
   - 其余客户端默认拿到 raw base64（与上游一致）。
2. **能力**：客户端用不了的线路在对应渲染器里返回带原因的错误并被过滤（例如 `errSingboxRealityMLKEM`）；
   不要把用不了的线路发给客户端。
3. **预览**：在 `internal/easybox/preview.go` 的 `previewFormats` 和前端 `pages/easybox/PreviewCard.tsx` 的 `CLIENTS`
   各加一项，管理员就能在「模拟客户端预览」里看到它会收到什么。
4. **兼容性测试**：能在容器里运行客户端内核的，在 `devenv/smoke-easybox.sh` 加一段真实流量测试；
   只能在设备上测试的，按 `CLIENT_COMPATIBILITY.md` 的记录格式填写版本、平台、日期和结果。
