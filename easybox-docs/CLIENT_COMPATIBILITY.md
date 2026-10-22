# EasyBox client compatibility

Filled from measurements only (EasyBox.md §77–§78). A cell says **PASS** only when a real
client core imported the subscription EasyBox serves and fetched
`https://www.gstatic.com/generate_204` through the line. Re-run with
`docker compose exec dev bash devenv/smoke-easybox.sh`.

## Environment of the current results

| Item | Value |
|---|---|
| Date | 2026-10-21 |
| Server | EasyBox dev container, Xray-core 26.10.10, upstream 3x-ui a8d0700 |
| Network | loopback inside the dev container; panel-certificate rows: real VPS (GCP asia-east2, v3.10.2-eb.2, Let's Encrypt IP certificate), client cores in the dev container over the internet |
| Kind of test | client **core** consuming the subscription format its app uses |

## Matrix

| Line | v2rayN (Xray core 26.10.10, raw links) | Shadowrocket | PassWall | sing-box 1.14.3 (sing-box JSON) | Mihomo 1.19.32 (Clash YAML) |
|---|---|---|---|---|---|
| VLESS + REALITY + Vision | **PASS** | **FAIL** ⁵ | untested | **FAIL → filtered** ¹ | **PASS** |
| Hysteria2, self-signed + pinned, salamander | **PASS** | untested | untested | **PASS** ² | **PASS** |
| Hysteria2, panel certificate | **PASS** ³ | untested | untested | **PASS** ³ | **PASS** ³ |
| TUIC v5, panel certificate | n/a ⁴ | untested | untested | **PASS** ³ ⁶ | **PASS** ³ ⁶ |
| VLESS + TLS + Vision, panel certificate | **PASS** ⁶ | untested | untested | **PASS** ⁶ | **PASS** ⁶ |
| Trojan + TLS, panel certificate | **PASS** ⁶ | untested | untested | **PASS** ⁶ | **PASS** ⁶ |
| Shadowsocks 2022 (`2022-blake3-aes-128-gcm`) | **PASS** ⁶ | untested | untested | **PASS** ⁶ | **PASS** ⁶ |

1. sing-box 1.14.3 fails `reality verification failed` against Xray 26.10.10 REALITY servers that
   advertise `support-x25519mlkem768=true` (every uTLS fingerprint tried: chrome, firefox, safari,
   edge, ios, randomized; also a minimal one-outbound config). Mihomo, which honours
   `support-x25519mlkem768`, and Xray's own client both pass. EasyBox therefore leaves such lines out
   of sing-box output (`errSingboxRealityMLKEM`, capability filtering §34) instead of shipping a line
   that cannot connect. Re-test when sing-box adds X25519MLKEM768 REALITY support.
2. sing-box gets the self-signed certificate embedded in `tls.certificate` (it trusts a certificate,
   not a pinned hash).
3. Measured on the VPS (2026-10-21) with the Let's Encrypt IP certificate `install.sh` issued; the
   exit IP seen through the line was the VPS address.
4. Xray-core has no TUIC client; v2rayN switches such lines to its sing-box core.
5. Real app on the VPS, not a client core; see [Known issue: Shadowrocket + Vision](#known-issue-shadowrocket--vision).
6. Loopback in the dev container (`devenv/smoke-tcp-lines.sh`, 2026-10-21): a throwaway panel whose
   certificate comes from a local CA standing in for Let's Encrypt. TUIC here carries the new
   base64 shared password. VPS measurement follows the release.

Xray-core 26.10.10 logs on start that Trojan and Shadowsocks are deprecated ("might be removed")
in favour of VLESS. Both still work; prefer VLESS lines for anything long-lived.

## Real apps (user reports, 2026-10-21, against the VPS)

Imported the subscription and connected; app versions and the line used were not recorded.

| App | Result |
|---|---|
| v2rayN (Windows) | imports and connects |
| Clash app (phone) | imports and connects |
| PassWall (OpenWrt) | imports and connects |
| sing-box app (phone) | imports and connects with the universal URL. The Clash URL returned YAML (`decode config: invalid character 'p'`); since eb.3 the Clash URL also answers sing-box apps with sing-box JSON |
| Shadowrocket (iOS) | TUIC and Hysteria2 connect. VLESS + REALITY + Vision does not; see the known issue below |
| Stash (iOS) | VLESS + REALITY + Vision connects (packet capture on the VPS confirmed the ClientHello carries X25519MLKEM768 and the connections carry traffic) |

## Known issue: Shadowrocket + Vision

Found 2026-10-21 against the VPS (v3.10.2-eb.3, Xray-core 26.10.10); Shadowrocket 2.2.88 (3308)
and the build after it (exact version not recorded). Diagnosed with packet captures on the VPS.
**Decision: no EasyBox change; wait for a Shadowrocket fix.**

- **Symptom.** The VLESS + REALITY line imports but does not connect. TUIC and Hysteria2 work.
- **Cause.** With flow `xtls-rprx-vision`, Shadowrocket ignores the fingerprint chosen in the node
  (even `Chrome140` / `Safari26_3`, which are labelled "supports X25519MLKEM768"). It sends a fixed
  517-byte ClientHello with only an x25519 key share, 21 cipher suites and no ALPN. Xray 26.9+
  REALITY requires an X25519MLKEM768 key share, so it fails authentication, forwards the connection
  to the real target, and Shadowrocket aborts on the real certificate.
- **Evidence.**
  - With Vision on: about 99% of connections used that fixed ClientHello, whatever the fingerprint
    or UDP-relay setting.
  - With the same node's flow set to none: every connection switched to a Chrome ClientHello
    carrying X25519MLKEM768 and passed REALITY authentication. The server then rejected the VLESS
    request, as expected for a flow mismatch.
  - A temporary user without Vision on the same inbound, with fingerprint `Chrome140` in
    Shadowrocket: connects, with long-lived connections carrying 100–800 KB each. The user was
    deleted afterwards.
- **Not the cause.** Shadowrocket drops `spx` (spiderX) on import. spiderX is Xray-only, is used
  only after REALITY verification fails, and plays no part in authentication.
- **Workarounds.**
  - Use Stash on iOS for VLESS.
  - Or use TUIC / Hysteria2 in Shadowrocket.
  - A user without Vision would also work, but EasyBox does not provision one: lines keep a single
    standard Vision user.
- **Re-test** when Shadowrocket ships an update: default flow, fingerprint `Chrome140`, then check
  that the ClientHello on the server carries X25519MLKEM768 (for example with tcpdump on the line
  port).

## Not yet measured (needs people or hardware)

| Client | What is needed |
|---|---|
| Clash Verge Rev | any desktop; the Mihomo core result above covers the format |

## Record format for new results (§78)

```
Client:   Shadowrocket
Version:  x.y.z
Platform: iOS 18.1
Date:     YYYY-MM-DD
Line:     VLESS + REALITY
Result:   PASS / FAIL (symptom)
```
