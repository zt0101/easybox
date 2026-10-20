# EasyBox dev environment (Docker)

Everything (Go 1.27, Node 26, gcc, golangci-lint, Playwright Chromium, Xray) lives in the
`easybox-dev` image. The host only needs Docker. Source is bind-mounted; `node_modules`,
Go module cache, build cache, DB and logs live in named volumes.

## Start

```
cd devenv
docker compose up -d --build
docker compose exec dev bash
```

## Run the panel

```
docker compose exec dev bash devenv/panel.sh          # build frontend + backend, run
docker compose exec -e SKIP_FE=1 dev bash devenv/panel.sh   # backend-only rebuild
```

Panel: http://localhost:12053 (container port 2053) — a fresh DB seeds `admin` / `admin` (dev only).
Show settings with `docker compose exec dev /tmp/x-ui setting -show true`.
Subscription server: http://localhost:12096 (container 2096). Test inbounds: ports 20000–20009 (TCP and UDP).
Host ports are remapped because Windows often reserves 2000–2099 (`netsh int ipv4 show excludedportrange protocol=tcp`).

## Gates

```
docker compose exec dev make test-go
docker compose exec dev make verify
```

## Reset

```
docker compose down          # keep volumes (DB, caches)
docker compose down -v       # wipe DB and caches
```

## Windows hosts

Windows Defender may quarantine upstream shell scripts (seen: `install.sh` flagged as
`Trojan:Win32/Egairtigado!rfn`, a false positive after review). On Windows keep the source in a
Docker volume instead of a bind mount: clone the repo into a volume and mount it at `/src`
(same layout as `compose.yml`, with `src:/src` in place of `..:/src`).
