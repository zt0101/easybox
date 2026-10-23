#!/usr/bin/env bash
# Real-network check of the certificate, Shadowsocks and XHTTP lines on an installed EasyBox VPS,
# run from the dev container:
#   VPS=203.0.113.10 SSH_KEY=/root/.cache/ssh/easybox_vps bash devenv/smoke-vps.sh
# Reads the panel credentials over SSH (install.sh's /etc/x-ui/install-result.env) into
# variables and never prints them. Reuses one line per template (any name) or creates test-<template> and leaves
# them in place for device tests. The first Shadowsocks line on an older panel upgrades
# the shared password, so devices must refresh the subscription afterwards.
# See easybox-docs/UPSTREAM_SYNC.md.
set -uo pipefail
VPS=${VPS:?set VPS to the server IP}
SSH_KEY=${SSH_KEY:-/root/.cache/ssh/easybox_vps}
SSH_USER=${SSH_USER:-easybox}
SSH="ssh -i $SSH_KEY -o UserKnownHostsFile=/root/.cache/ssh/known_hosts -o StrictHostKeyChecking=accept-new $SSH_USER@$VPS"
eval "$($SSH 'sudo cat /etc/x-ui/install-result.env' | grep -E '^XUI_(USERNAME|PASSWORD|PANEL_PORT|WEB_BASE_PATH)=')"
PANEL="https://$VPS:$XUI_PANEL_PORT/${XUI_WEB_BASE_PATH%/}"
PANEL=${PANEL%/}
SINGBOX=/root/.cache/clients/sing-box
MIHOMO=/root/.cache/clients/mihomo
XRAY=$XUI_BIN_FOLDER/xray-linux-amd64
PROBE=https://www.gstatic.com/generate_204
JAR=$(mktemp); WORK=$(mktemp -d); pid=""
trap 'kill $pid 2>/dev/null; rm -rf "$JAR" "$WORK"' EXIT
ok()   { printf '  [OK]   %s\n' "$1"; }
bad()  { printf '  [FAIL] %s\n' "$1"; FAILED=1; }
FAILED=0
csrf() { curl -fsS -b "$JAR" -c "$JAR" "$PANEL/csrf-token" | jq -r .obj; }
api()  { local t; t=$(csrf); curl -fsS -b "$JAR" -c "$JAR" -X "$1" -H "X-CSRF-Token: $t" \
           -H 'Content-Type: application/json' ${3:+--data "$3"} "$PANEL$2"; }

# fetch through local SOCKS 10808; prints http code and exit IP
probe() {
  local code ip
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 15 --socks5-hostname 127.0.0.1:10808 "$PROBE")
  ip=$(curl -s --max-time 15 --socks5-hostname 127.0.0.1:10808 https://api4.ipify.org)
  echo "$code $ip"
}

t=$(csrf)
curl -fsS -b "$JAR" -c "$JAR" -H "X-CSRF-Token: $t" --data-urlencode "username=$XUI_USERNAME" \
  --data-urlencode "password=$XUI_PASSWORD" "$PANEL/login" | jq -e '.success' >/dev/null \
  && ok "login over HTTPS (trusted cert)" || { bad login; exit 1; }

api GET /panel/api/easybox/templates 2>/dev/null | jq -c '[.obj[]? | {id, available: (.unavailable // .reason // "")}]' || true

TEMPLATES="tuic hysteria2 vless-tls trojan shadowsocks vless-xhttp"
XRAY_TEMPLATES="hysteria2 vless-tls trojan shadowsocks vless-xhttp"
# CDN lines need Cloudflare connected in the panel (API token + domain).
if api GET /panel/api/easybox/templates | jq -e '.obj[] | select(.id=="vless-cdn") | .available' >/dev/null; then
  TEMPLATES="$TEMPLATES vless-cdn vless-cdn-xhttp"; XRAY_TEMPLATES="$XRAY_TEMPLATES vless-cdn vless-cdn-xhttp"
else
  echo "  [SKIP] vless-cdn, vless-cdn-xhttp: Cloudflare is not connected in the panel"
fi
# One tunnel line per server: test the existing one, or create it when possible.
tunnel_name=$(api GET /panel/api/easybox/endpoints/list | jq -r '[.obj[]? | select(.template=="vless-tunnel")][0].name // empty')
if [ -n "$tunnel_name" ] || api GET /panel/api/easybox/templates | jq -e '.obj[] | select(.id=="vless-tunnel") | .available' >/dev/null; then
  TEMPLATES="$TEMPLATES vless-tunnel"; XRAY_TEMPLATES="$XRAY_TEMPLATES vless-tunnel"
else
  echo "  [SKIP] vless-tunnel: not available on this server"
fi
api GET /panel/api/easybox/templates | jq -e '.obj[] | select(.id=="shadowsocks") | .passwordUpgrade' >/dev/null \
  && echo "  [NOTE] creating test-shadowsocks upgrades the shared password: refresh the subscription on devices"
# Lines are found by template, whatever they are called (names follow the
# HK-VLESS-Reality rule or the admin's choice); test-<template> only when the
# server has none of that kind yet.
declare -A ID LINK NAME
for tpl in $TEMPLATES; do
  existing=$(api GET /panel/api/easybox/endpoints/list | jq -c --arg t "$tpl" '[.obj[]? | select(.template==$t and .inboundId>0)][0] // empty')
  if [ -n "$existing" ]; then
    ID[$tpl]=$(jq -r .id <<<"$existing"); NAME[$tpl]=$(jq -r .name <<<"$existing")
  else
    res=$(api POST /panel/api/easybox/endpoints/create "{\"template\":\"$tpl\",\"name\":\"test-$tpl\",\"address\":\"$VPS\"}")
    jq -e '.success and .obj.status=="RUNNING"' <<<"$res" >/dev/null || { bad "create $tpl: $res"; continue; }
    ID[$tpl]=$(jq -r .obj.id <<<"$res"); NAME[$tpl]=$(jq -r .obj.name <<<"$res")
  fi
done
# Links only after every create: a Shadowsocks create can change the shared password.
for tpl in $TEMPLATES; do
  [ -n "${ID[$tpl]:-}" ] || continue
  LINK[$tpl]=$(api GET "/panel/api/easybox/endpoints/link/${ID[$tpl]}" | jq -r .obj)
  ok "$tpl line ${NAME[$tpl]} RUNNING (id ${ID[$tpl]}): $(sed -E 's#://[^@]+@#://***@#; s/(password|auth|obfs-password)=[^&]+/\1=***/g' <<<"${LINK[$tpl]}")"
  grep -q pinSHA256 <<<"${LINK[$tpl]}" && bad "$tpl link pins a cert (should use the trusted panel cert)"
done

info=$(api GET /panel/api/easybox/subscription/info)
sub=$(jq -r .obj.subUrl <<<"$info"); clash=$(jq -r .obj.clashUrl <<<"$info")
echo "  subUrl host: $(sed -E 's#(https?://[^/]+)/.*#\1#' <<<"$sub")"
raw=$(curl -fsS "$sub" | base64 -d)
for tpl in $TEMPLATES; do
  # REALITY lines pick one of several server names and short IDs per render: compare without them.
  nosni() { sed -E 's/(sni|sid)=[^&#]*//g'; }
  nosni <<<"$raw" | grep -xF "$(nosni <<<"${LINK[$tpl]:-none}")" >/dev/null && ok "raw subscription has ${NAME[$tpl]:-$tpl}" || bad "raw subscription lacks ${NAME[$tpl]:-$tpl}"
done

# 1) xray client via upstream link parser (Xray has no TUIC client)
cd "$(dirname "$0")/.."
for tpl in $XRAY_TEMPLATES; do
  go run ./devenv/linkclient "${LINK[$tpl]}" > "$WORK/x.json" || { bad "linkclient $tpl"; continue; }
  "$XRAY" run -c "$WORK/x.json" >"$WORK/x.log" 2>&1 & pid=$!; sleep 3
  r=$(probe); kill $pid; wait $pid 2>/dev/null; pid=""
  [[ $r == "204 $VPS" ]] && ok "Xray client → $tpl → $r" || { bad "Xray $tpl: $r"; tail -5 "$WORK/x.log"; }
done

# Node names start with the line tier ("🟢推荐|HK-TUIC"); these find the node
# for a line name in a sing-box config / Clash YAML (the plain name if absent).
sb_tag() { jq -r --arg n "$2" '[.outbounds[].tag | select(. == $n or endswith("|" + $n))][0] // $n' "$1"; }
yaml_name() { sed -n 's/^  name: //p' "$1" | awk -v n="$2" '$0 == n || substr($0, length($0) - length(n)) == "|" n {print; f=1; exit} END {if (!f) print n}'; }

# 2) sing-box client using the served sing-box config
curl -fsS -A "SFA/1.14.3 (8; sing-box 1.14.3; language en_US)" -o "$WORK/sb.json" "$sub"
"$SINGBOX" check -c "$WORK/sb.json" && ok "sing-box config check" || bad "sing-box check"
for tpl in $TEMPLATES; do
  tag=$(sb_tag "$WORK/sb.json" "${NAME[$tpl]:-test-$tpl}")
  if [[ $tpl == *xhttp* ]]; then # sing-box has no XHTTP transport
    ! jq -e --arg tag "$tag" '.outbounds[]|select(.tag==$tag)' "$WORK/sb.json" >/dev/null \
      && ok "sing-box config leaves out $tag" || bad "sing-box config carries $tag"
    continue
  fi
  jq --arg tag "$tag" '.inbounds=[{type:"mixed",tag:"mixed-in",listen:"127.0.0.1",listen_port:10808}]
    | (.outbounds[]|select(.tag=="proxy")).default=$tag | del(.route.auto_detect_interface)' "$WORK/sb.json" > "$WORK/sbr.json"
  "$SINGBOX" run -c "$WORK/sbr.json" >"$WORK/sb.log" 2>&1 & pid=$!; sleep 3
  r=$(probe); kill $pid; wait $pid 2>/dev/null; pid=""
  [[ $r == "204 $VPS" ]] && ok "sing-box client → $tag → $r" || { bad "sing-box $tag: $r"; tail -5 "$WORK/sb.log"; }
done

# 3) Mihomo client using the served Clash YAML
curl -fsS -A "mihomo/1.19.32" -o "$WORK/sub.yaml" "$clash"
mkdir -p "$WORK/mh"
{ printf 'mixed-port: 10808\nexternal-controller: 127.0.0.1:19090\nallow-lan: false\n'
  grep -vE '^(mixed-port|port|socks-port|external-controller|allow-lan):' "$WORK/sub.yaml"; } > "$WORK/mh/config.yaml"
"$MIHOMO" -d "$WORK/mh" >"$WORK/mh.log" 2>&1 & pid=$!; sleep 4
for tpl in $TEMPLATES; do
  tag=$(yaml_name "$WORK/sub.yaml" "${NAME[$tpl]:-test-$tpl}")
  d=$(curl -s --max-time 15 "http://127.0.0.1:19090/proxies/$(jq -rn --arg t "$tag" '$t|@uri')/delay?url=$(jq -rn --arg u "$PROBE" '$u|@uri')&timeout=8000")
  jq -e '.delay>0' <<<"$d" >/dev/null && ok "Mihomo → $tag delay $(jq .delay <<<"$d")ms" || { bad "Mihomo $tag: $d"; tail -5 "$WORK/mh.log"; }
done
kill $pid; wait $pid 2>/dev/null; pid=""

# 3b) subscription through Cloudflare (only when turned on in the panel): same
# nodes as the IP address — direct lines must keep the server IP — and the
# sing-box / Clash formats load.
cdn_sub=$(jq -r '.obj.cdnSubUrl // ""' <<<"$info"); cdn_clash=$(jq -r '.obj.cdnClashUrl // ""' <<<"$info")
if [ -n "$cdn_sub" ]; then
  norm() { sed -E 's/(sni|sid)=[^&#]*//g' | sort; }
  cdn_raw=$(curl -fsS --max-time 30 "$cdn_sub" | base64 -d)
  [ -n "$cdn_raw" ] && [ "$(norm <<<"$cdn_raw")" = "$(norm <<<"$raw")" ] \
    && ok "subscription through Cloudflare = IP subscription ($(wc -l <<<"$cdn_raw") nodes)" \
    || bad "subscription through Cloudflare differs from the IP one"
  grep -q "@$VPS:" <<<"$cdn_raw" && ok "direct lines keep $VPS through Cloudflare" || bad "no node on $VPS via Cloudflare"
  curl -fsS -D "$WORK/cdn.h" -o /dev/null --max-time 30 "$cdn_sub" && grep -qi '^cache-control: no-cache, no-store' "$WORK/cdn.h" \
    && ok "subscription through Cloudflare is not cacheable" || bad "subscription through Cloudflare lacks no-store"
  curl -fsS --max-time 30 -A "SFA/1.14.3 (8; sing-box 1.14.3; language en_US)" -o "$WORK/cdn-sb.json" "$cdn_sub" \
    && "$SINGBOX" check -c "$WORK/cdn-sb.json" && ok "sing-box config through Cloudflare checks" || bad "sing-box config through Cloudflare"
  # Into a file first: grep -q closing the pipe early fails curl under pipefail.
  [ -z "$cdn_clash" ] || { curl -fsS --max-time 30 -A "mihomo/1.19.32" -o "$WORK/cdn-clash.yaml" "$cdn_clash" \
    && grep -q '^proxies:' "$WORK/cdn-clash.yaml" && ok "Clash YAML through Cloudflare" || bad "Clash YAML through Cloudflare"; }
else
  echo "  [SKIP] subscription through Cloudflare: off in the panel"
fi

# 4) eb.3: Clash URL answers sing-box apps with JSON, Clash clients with YAML
for ua in "SFA/1.14.3 (8; sing-box 1.14.3; language en_US)" "SFI/1.14.3 (Build 1; sing-box 1.14.3; language zh_CN)"; do
  curl -fsS -A "$ua" -o "$WORK/cl.json" "$clash"
  "$SINGBOX" check -c "$WORK/cl.json" 2>/dev/null && ok "Clash URL + ${ua%% *} → valid sing-box config" || bad "Clash URL + ${ua%% *}: $(head -c 40 "$WORK/cl.json")"
done
jq --arg tag "$(sb_tag "$WORK/cl.json" "${NAME[tuic]:-test-tuic}")" '.inbounds=[{type:"mixed",tag:"mixed-in",listen:"127.0.0.1",listen_port:10808}]
  | (.outbounds[]|select(.tag=="proxy")).default=$tag | del(.route.auto_detect_interface)' "$WORK/cl.json" > "$WORK/clr.json"
"$SINGBOX" run -c "$WORK/clr.json" >"$WORK/sb.log" 2>&1 & pid=$!; sleep 3
r=$(probe); kill $pid; wait $pid 2>/dev/null; pid=""
[[ $r == "204 $VPS" ]] && ok "sing-box via Clash URL → ${NAME[tuic]:-test-tuic} → $r" || bad "sing-box via Clash URL: $r"
head -c 9 "$WORK/sub.yaml" | grep -q "proxies:" && ok "Clash URL + mihomo UA still YAML" || bad "mihomo UA no longer YAML"

# 5) eb.3: no empty fp= in Hysteria2 links, x-ui settings without warning
grep -q '[?&]fp=&\|[?&]fp=$\|[?&]fp=#' <<<"${LINK[hysteria2]}" && bad "Hysteria2 link still has empty fp=" || ok "Hysteria2 link has no empty fp="
grep -q '[?&]fp=\(&\|#\|$\)' <<<"$raw" && bad "raw subscription has empty fp=" || ok "raw subscription has no empty fp="
w=$($SSH "sudo x-ui settings 2>&1 | grep -c 'ignored'")
[ "$w" = 0 ] && ok "x-ui settings prints no 'ignored' warning" || bad "x-ui settings still warns"

# Server side: UDP listeners + no errors
$SSH "sudo /usr/local/x-ui/x-ui -v; sudo ss -lunp | grep -E 'xray|sing-box|x-ui' ; sudo journalctl -u x-ui --since '-10 min' --no-pager | grep -iE 'error|panic' | tail -5"
[ $FAILED = 0 ] && echo PASS || echo FAILED
