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
| VLESS + REALITY + Vision | **PASS** | untested | untested | **FAIL → filtered** ¹ | **PASS** |
| Hysteria2, self-signed + pinned, salamander | **PASS** | untested | untested | **PASS** ² | **PASS** |
| Hysteria2, panel certificate | **PASS** ³ | untested | untested | **PASS** ³ | **PASS** ³ |
| TUIC v5, panel certificate | n/a ⁴ | untested | untested | **PASS** ³ | **PASS** ³ |

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

## Real apps (user reports, 2026-10-21, against the VPS)

Imported the subscription and connected; app versions and the line used were not recorded.

| App | Result |
|---|---|
| v2rayN (Windows) | imports and connects |
| Clash app (phone) | imports and connects |
| PassWall (OpenWrt) | imports and connects |
| sing-box app (phone) | imports and connects with the universal URL. The Clash URL returned YAML (`decode config: invalid character 'p'`); since eb.3 the Clash URL also answers sing-box apps with sing-box JSON |

## Not yet measured (needs people or hardware)

| Client | What is needed |
|---|---|
| Shadowrocket (iOS) | an iPhone with Shadowrocket |
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
