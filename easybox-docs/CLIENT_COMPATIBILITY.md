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
| Network | loopback inside the dev container (not a real VPS / real network yet) |
| Kind of test | client **core** consuming the subscription format its app uses |

## Matrix

| Line | v2rayN (Xray core 26.10.10, raw links) | Shadowrocket | PassWall | sing-box 1.14.3 (sing-box JSON) | Mihomo 1.19.32 (Clash YAML) |
|---|---|---|---|---|---|
| VLESS + REALITY + Vision | **PASS** | untested | untested | **FAIL → filtered** ¹ | **PASS** |
| Hysteria2, self-signed + pinned, salamander | **PASS** | untested | untested | **PASS** ² | **PASS** |
| Hysteria2, panel certificate | untested ³ | untested | untested | untested ³ | untested ³ |
| TUIC v5, panel certificate | n/a ⁴ | untested | untested | untested ³ | untested ³ |

1. sing-box 1.14.3 fails `reality verification failed` against Xray 26.10.10 REALITY servers that
   advertise `support-x25519mlkem768=true` (every uTLS fingerprint tried: chrome, firefox, safari,
   edge, ios, randomized; also a minimal one-outbound config). Mihomo, which honours
   `support-x25519mlkem768`, and Xray's own client both pass. EasyBox therefore leaves such lines out
   of sing-box output (`errSingboxRealityMLKEM`, capability filtering §34) instead of shipping a line
   that cannot connect. Re-test when sing-box adds X25519MLKEM768 REALITY support.
2. sing-box gets the self-signed certificate embedded in `tls.certificate` (it trusts a certificate,
   not a pinned hash).
3. Needs a trusted certificate, which the dev container does not have; to be measured on the VPS with
   the Let's Encrypt IP certificate from `install.sh`.
4. Xray-core has no TUIC client; v2rayN switches such lines to its sing-box core.

## Not yet measured (needs people or hardware)

| Client | What is needed |
|---|---|
| v2rayN app (Windows) | import the universal URL in the GUI against the VPS |
| Shadowrocket (iOS) | an iPhone with Shadowrocket |
| PassWall (OpenWrt) | an OpenWrt router or VM |
| sing-box apps (SFA/SFI) | a phone; the core result above already covers the config format |
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
