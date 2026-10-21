#!/usr/bin/env bash
# Real TUIC + Hysteria2 check against an installed EasyBox VPS, run from the dev container:
#   VPS=203.0.113.10 SSH_KEY=/root/.cache/ssh/easybox_vps bash devenv/smoke-vps.sh
# Reads the panel credentials over SSH (install.sh's /etc/x-ui/install-result.env) into
# variables and never prints them. Creates (or reuses) lines test-tuic and test-hysteria2
# and leaves them in place for device tests. See easybox-docs/UPSTREAM_SYNC.md.
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

declare -A ID LINK
for tpl in tuic hysteria2; do
  name="test-$tpl"
  existing=$(api GET /panel/api/easybox/endpoints/list | jq -r --arg n "$name" '.obj[]? | select(.name==$n) | .id')
  if [ -n "$existing" ]; then ID[$tpl]=$existing; else
    res=$(api POST /panel/api/easybox/endpoints/create "{\"template\":\"$tpl\",\"name\":\"$name\",\"address\":\"$VPS\"}")
    jq -e '.success and .obj.status=="RUNNING"' <<<"$res" >/dev/null || { bad "create $tpl: $res"; continue; }
    ID[$tpl]=$(jq -r .obj.id <<<"$res")
  fi
  LINK[$tpl]=$(api GET "/panel/api/easybox/endpoints/link/${ID[$tpl]}" | jq -r .obj)
  ok "$tpl line RUNNING (id ${ID[$tpl]}): $(sed -E 's#://[^@]+@#://***@#; s/(password|auth|obfs-password)=[^&]+/\1=***/g' <<<"${LINK[$tpl]}")"
  grep -q pinSHA256 <<<"${LINK[$tpl]}" && bad "$tpl link pins a cert (should use the trusted panel cert)"
done

info=$(api GET /panel/api/easybox/subscription/info)
sub=$(jq -r .obj.subUrl <<<"$info"); clash=$(jq -r .obj.clashUrl <<<"$info")
echo "  subUrl host: $(sed -E 's#(https?://[^/]+)/.*#\1#' <<<"$sub")"
raw=$(curl -fsS "$sub" | base64 -d)
for p in tuic hysteria2; do grep -q "^$p://" <<<"$raw" && ok "raw subscription has $p" || bad "raw subscription lacks $p"; done

# 1) xray client via upstream link parser (Hysteria2 only; Xray has no TUIC client)
cd "$(dirname "$0")/.."
go run ./devenv/linkclient "${LINK[hysteria2]}" > "$WORK/x.json" && { "$XRAY" run -c "$WORK/x.json" >"$WORK/x.log" 2>&1 & pid=$!; sleep 3
  r=$(probe); kill $pid; wait $pid 2>/dev/null; pid=""
  [[ $r == "204 $VPS" ]] && ok "Xray client → Hysteria2 → $r" || { bad "Xray Hysteria2: $r"; tail -5 "$WORK/x.log"; }; }

# 2) sing-box client using the served sing-box config
curl -fsS -A "SFA/1.14.3 (8; sing-box 1.14.3; language en_US)" -o "$WORK/sb.json" "$sub"
"$SINGBOX" check -c "$WORK/sb.json" && ok "sing-box config check" || bad "sing-box check"
for tag in test-tuic test-hysteria2; do
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
for tag in test-tuic test-hysteria2; do
  d=$(curl -s --max-time 15 "http://127.0.0.1:19090/proxies/$tag/delay?url=$(jq -rn --arg u "$PROBE" '$u|@uri')&timeout=8000")
  jq -e '.delay>0' <<<"$d" >/dev/null && ok "Mihomo → $tag delay $(jq .delay <<<"$d")ms" || { bad "Mihomo $tag: $d"; tail -5 "$WORK/mh.log"; }
done
kill $pid; wait $pid 2>/dev/null; pid=""

# 4) eb.3: Clash URL answers sing-box apps with JSON, Clash clients with YAML
for ua in "SFA/1.14.3 (8; sing-box 1.14.3; language en_US)" "SFI/1.14.3 (Build 1; sing-box 1.14.3; language zh_CN)"; do
  curl -fsS -A "$ua" -o "$WORK/cl.json" "$clash"
  "$SINGBOX" check -c "$WORK/cl.json" 2>/dev/null && ok "Clash URL + ${ua%% *} → valid sing-box config" || bad "Clash URL + ${ua%% *}: $(head -c 40 "$WORK/cl.json")"
done
jq --arg tag test-tuic '.inbounds=[{type:"mixed",tag:"mixed-in",listen:"127.0.0.1",listen_port:10808}]
  | (.outbounds[]|select(.tag=="proxy")).default=$tag | del(.route.auto_detect_interface)' "$WORK/cl.json" > "$WORK/clr.json"
"$SINGBOX" run -c "$WORK/clr.json" >"$WORK/sb.log" 2>&1 & pid=$!; sleep 3
r=$(probe); kill $pid; wait $pid 2>/dev/null; pid=""
[[ $r == "204 $VPS" ]] && ok "sing-box via Clash URL → test-tuic → $r" || bad "sing-box via Clash URL: $r"
head -c 9 "$WORK/sub.yaml" | grep -q "proxies:" && ok "Clash URL + mihomo UA still YAML" || bad "mihomo UA no longer YAML"

# 5) eb.3: no empty fp= in Hysteria2 links, x-ui settings without warning
grep -q '[?&]fp=&\|[?&]fp=$\|[?&]fp=#' <<<"${LINK[hysteria2]}" && bad "Hysteria2 link still has empty fp=" || ok "Hysteria2 link has no empty fp="
grep -q '[?&]fp=\(&\|#\|$\)' <<<"$raw" && bad "raw subscription has empty fp=" || ok "raw subscription has no empty fp="
w=$($SSH "sudo x-ui settings 2>&1 | grep -c 'ignored'")
[ "$w" = 0 ] && ok "x-ui settings prints no 'ignored' warning" || bad "x-ui settings still warns"

# Server side: UDP listeners + no errors
$SSH "sudo /usr/local/x-ui/x-ui -v; sudo ss -lunp | grep -E 'xray|sing-box|x-ui' ; sudo journalctl -u x-ui --since '-10 min' --no-pager | grep -iE 'error|panic' | tail -5"
[ $FAILED = 0 ] && echo PASS || echo FAILED
