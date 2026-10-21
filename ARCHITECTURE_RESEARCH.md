# EasyBox — Architecture Research (Phase 0)

Upstream base: MHSanaei/3x-ui `a8d0700` (v3.10.2), see `UPSTREAM_BASE.md`.
Answers the questions in EasyBox.md §100. Upstream's own map is `docs/architecture.md`; this
file only records what EasyBox needs and what was verified.

## Baseline verification (2026-10-20, devenv container)

| Check | Result |
|---|---|
| Frontend build (`npm ci && npm run build`) | OK |
| Backend build (`go build`) | OK (2m15s cold) |
| Panel start, Xray 26.10.10 start | OK |
| Login (admin/admin on fresh DB) | OK |
| X25519 keypair via `/panel/api/server/getNewX25519Cert` | OK |
| Create VLESS+REALITY inbound with embedded client | OK |
| Subscription returns `vless://…security=reality` | OK |
| Delete inbound → subscription 404 | OK |
| Port conflict on duplicate port rejected | OK (observed) |
| `make test-go` | OK — 50 packages pass, 0 failures (PG / e2e / scale suites skip by design) |
| Idle RSS | x-ui ≈ 117 MB, xray ≈ 40 MB (≈ 158 MB total) |

Reproduce: `docker compose exec dev bash devenv/smoke-baseline.sh` (see `devenv/README.md`).

## §100 answers

1. **Stack** — Go 1.27, Gin, GORM (SQLite default, PostgreSQL optional); Xray-core as a supervised
   child process with gRPC API; React 19 + Ant Design 6 + Vite 8 + TypeScript, embedded via `embed.FS`.
2. **Frontend** — `frontend/` (routes in `src/routes.tsx`, sidebar in `src/layouts/AppSidebar.tsx`,
   xray domain logic in `src/lib/xray/`), built into `internal/web/dist/`.
3. **Backend** — `internal/`; panel server `internal/web/` (controller → service → GORM / runtime),
   subscription server `internal/sub/`.
4. **Inbound creation** — `InboundService.AddInbound` (`internal/web/service/inbound.go:1231`):
   validation → DB tx (port conflict, save, client stats, `SyncInbound`) → post-commit
   `runtime.AddInbound`. Hot-apply failure only sets need-restart (30 s cron). No pre-validation,
   no rollback. Controller calls `xrayService.SetToNeedRestart()` when asked.
5. **Client creation** — `ClientService` (`client_crud.go`): `Create`, `Attach(id, inboundIds)`,
   `AttachByEmail`, `Detach`, `Update`. Clients persist as `ClientRecord` (`clients` table, unique
   email, `SubID`); many-to-many with inbounds via `ClientInbound` (`client_inbounds`).
   Duplicate check (`client_inbound_apply.go:300`) **accepts the same email when SubID matches**,
   so one identity may be embedded in several inbounds' settings.
6. **Xray config** — `XrayService.GetXrayConfig` builds from DB; `tryHotApply` + `xray/hot_diff.go`
   push deltas over gRPC; fallback full restart (`xray/process.go`).
7. **Share links** — panel: `service/client_link.go` + `util/link/outbound.go`; frontend:
   `src/lib/xray/inbound-link.ts`; docs site: `docs/lib/xray/` (three implementations).
8. **Subscription** — `internal/sub/controller.go` (`subs()`), `service.go` (`getSubs(subId)` →
   all inbounds attached to that SubID), `json_service.go`, `clash_service.go`.
9. **UA detection** — `controller.subs()`: HTML for browsers, Clash regex `(?i)(clash|mihomo)`,
   JSON regex (default empty = off), Happ / Incy detection, else raw base64. UA logged at debug.
10. **Clash/Mihomo** — `internal/sub/clash_service.go` (+ `clash_yaml.go`), feature flag
    `subClashEnable` (default **false**), paths `subClashPath`, `/mihomo/`, `/clash-legacy/`.
11. **DB schema** — `internal/database/model/model.go` (+ siblings); AutoMigrate + migrations in
    `internal/database/db.go`, data migrations in `migrate_data.go`.
12. **Reusable as-is** — InboundService, ClientService (Create/Attach/Detach), runtime.Runtime,
    `GetNewX25519Cert`, `ScanRealityTargets`, port conflict check, sub server (raw/Clash/HTML page,
    userinfo headers), auth/CSRF/API tokens, backup, install/release scripts.
13. **Where EasyBox goes** — backend `internal/easybox/` (calls services only; never
    `internal/xray/api.go`); controller `internal/web/controller/easybox.go` under `/panel/api/easybox`;
    frontend `frontend/src/pages/easybox-*`; sub extensions as new files in `internal/sub/`.
14. **Canonical model** — none upstream; all renderers read Inbound JSON directly. EasyBox adds one
    only for sing-box + capability filtering (MVP-2).
15. **sing-box renderer** — `internal/sub/singbox_service.go` (new), branch in `controller.subs()`.
16. **New tables** — `easybox_endpoints` (MVP-0); `easybox_audit_logs` (MVP-0, §53);
    templates stay built-in Go code in MVP-0 (no table until the template editor, P1).
17. **Conflict hotspots** — `frontend/src/routes.tsx`, `src/layouts/AppSidebar.tsx`,
    `src/pages/api-docs/endpoints.ts`, `internal/sub/controller.go`, `internal/database/db.go`,
    `internal/web/translation/*.json`, `.github/workflows/*`.

## MVP-0 call plan (VLESS + REALITY)

```
Quick Create (internal/easybox)
  ensure default subscription user (ClientRecord, random SubID, no HWID limit)
  pick port: free in 20000–60000, not used (port_conflict), not panel/sub/80
  REALITY: GetNewX25519Cert, target from ScanRealityTargets(realityScanCandidates)
  build model.Inbound (shape = frontend/src/test/golden/fixtures/inbound-full/vless-tcp-reality.json)
    settings.clients = [default user: same email + SubID + UUID, flow xtls-rprx-vision]
  InboundService.AddInbound → SetToNeedRestart if requested
  health check: inbound tag present in running Xray (gRPC) within N seconds
  failure → InboundService.DelInbound, endpoint status ERROR, audit log
  insert easybox_endpoints row (inboundId, clientId, status RUNNING)
```

To verify in Phase 1 with a test: two inbounds embedding the same email+SubID keep one
`ClientRecord`, and `getSubs(SubID)` returns both links.

## Things found during Phase 0

- Fresh install randomizes `subPath` / `subJsonPath` / `subClashPath` (16 chars) — never assume `/sub/`.
- Windows host reserves TCP 2000–2099 (Hyper-V/WinNAT): devenv maps 12053/12096.
- `git` checkout is LF via `.gitattributes` (`* text=auto eol=lf`) even with `core.autocrlf=true`.
