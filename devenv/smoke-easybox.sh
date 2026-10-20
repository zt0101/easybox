#!/usr/bin/env bash
# EasyBox MVP-0 end-to-end check against a running panel + real Xray: Quick Create →
# merged subscription (raw + Mihomo) → real proxied request → exclude/include → disable → delete.
# Run inside the dev container: bash devenv/smoke-easybox.sh
set -euo pipefail

PANEL=${PANEL:-http://127.0.0.1:2053}
USER_NAME=${USER_NAME:-admin}
USER_PASS=${USER_PASS:-admin}
PROBE_URL=${PROBE_URL:-https://www.gstatic.com/generate_204}
XRAY=${XRAY:-$XUI_BIN_FOLDER/xray-linux-amd64}
JAR=$(mktemp)
WORK=$(mktemp -d)
line_id=""
client_pid=""

ok()   { printf '  [OK]   %s\n' "$1"; }
fail() { printf '  [FAIL] %s\n' "$1"; exit 1; }
csrf() { curl -fsS -b "$JAR" -c "$JAR" "$PANEL/csrf-token" | jq -r .obj; }
api()  { # method path [json]
  local token; token=$(csrf)
  curl -fsS -b "$JAR" -c "$JAR" -X "$1" -H "X-CSRF-Token: $token" \
    -H 'Content-Type: application/json' ${3:+--data "$3"} "$PANEL$2"
}
sub_code() { curl -s -o /dev/null -w '%{http_code}' "$sub_url"; }
cleanup() {
  [ -n "$client_pid" ] && kill "$client_pid" 2>/dev/null || true
  [ -n "$line_id" ] && api POST "/panel/api/easybox/endpoints/del/$line_id" >/dev/null 2>&1 || true
  rm -rf "$JAR" "$WORK"
}
trap cleanup EXIT

echo "EasyBox MVP-0 smoke test"

token=$(csrf)
curl -fsS -b "$JAR" -c "$JAR" -H "X-CSRF-Token: $token" \
  --data-urlencode "username=$USER_NAME" --data-urlencode "password=$USER_PASS" \
  "$PANEL/login" | jq -e '.success == true' >/dev/null || fail "login"
ok "login"

res=$(api POST /panel/api/easybox/endpoints/create '{"name":"smoke-line","address":"127.0.0.1"}')
jq -e '.success == true and .obj.status == "RUNNING"' <<<"$res" >/dev/null || fail "Quick Create: $res"
line_id=$(jq -r .obj.id <<<"$res"); port=$(jq -r .obj.port <<<"$res")
ok "Quick Create → RUNNING (line $line_id, port $port)"

info=$(api GET /panel/api/easybox/subscription/info)
sub_url=$(jq -r .obj.subUrl <<<"$info"); clash_url=$(jq -r .obj.clashUrl <<<"$info")
[ -n "$sub_url" ] && [ "$sub_url" != null ] || fail "subscription info: $info"
ok "subscription info: $sub_url"

link=$(curl -fsS "$sub_url" | base64 -d | grep "^vless://.*:$port?" | head -1)
[ -n "$link" ] || fail "subscription has no link for port $port"
[ "${link##*#}" = "smoke-line" ] || fail "link name is '${link##*#}', want the line name smoke-line"
ok "merged subscription contains the line, named after it"

[ -n "$clash_url" ] || fail "Clash/Mihomo subscription not enabled"
curl -fsS -A "mihomo/1.19" "$clash_url" | grep -q "name: smoke-line" || fail "Clash/Mihomo YAML lacks the line"
ok "Clash/Mihomo subscription serves the line"

# Build an Xray client from the link: proves REALITY + Vision really proxy traffic.
uuid=${link#vless://}; uuid=${uuid%%@*}
query=${link#*\?}; query=${query%%#*}
param() { tr '&' '\n' <<<"$query" | sed -n "s/^$1=//p" | head -1 | sed 's/%2F/\//g'; }
jq -n --arg id "$uuid" --argjson port "$port" --arg pbk "$(param pbk)" --arg sid "$(param sid)" \
      --arg sni "$(param sni)" --arg fp "$(param fp)" --arg flow "$(param flow)" '{
  log:{loglevel:"warning"},
  inbounds:[{listen:"127.0.0.1",port:10808,protocol:"socks",settings:{udp:false}}],
  outbounds:[{protocol:"vless",settings:{vnext:[{address:"127.0.0.1",port:$port,
    users:[{id:$id,encryption:"none",flow:$flow}]}]},
    streamSettings:{network:"tcp",security:"reality",
      realitySettings:{serverName:$sni,publicKey:$pbk,shortId:$sid,fingerprint:$fp}}}]}' > "$WORK/client.json"
"$XRAY" run -c "$WORK/client.json" > "$WORK/client.log" 2>&1 &
client_pid=$!
sleep 2
code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 15 --socks5-hostname 127.0.0.1:10808 "$PROBE_URL" || true)
[ "$code" = 204 ] || { cat "$WORK/client.log"; fail "request through the line returned HTTP $code"; }
ok "real request through the line: $PROBE_URL → 204"
kill "$client_pid"; client_pid=""

api POST "/panel/api/easybox/endpoints/setInSubscription/$line_id" '{"include":false}' | jq -e '.success == true' >/dev/null || fail "exclude"
[ "$(sub_code)" = 404 ] || fail "subscription with only an excluded line returned $(sub_code) (want 404)"
api POST "/panel/api/easybox/endpoints/setInSubscription/$line_id" '{"include":true}' | jq -e '.success == true' >/dev/null || fail "include"
[ "$(sub_code)" = 200 ] || fail "subscription after re-include returned $(sub_code)"
ok "exclude / include toggles the line in the subscription"

api POST "/panel/api/easybox/endpoints/setEnable/$line_id" '{"enable":false}' | jq -e '.success == true' >/dev/null || fail "disable"
[ "$(sub_code)" = 404 ] || fail "subscription with only a disabled line returned $(sub_code) (want 404)"
ok "disabled line leaves the subscription"

api POST "/panel/api/easybox/endpoints/del/$line_id" | jq -e '.success == true' >/dev/null || fail "delete"
line_id=""
api GET /panel/api/easybox/endpoints/list | jq -e '.obj | length == 0' >/dev/null || fail "list after delete"
ok "delete removes the line"

echo "PASS"
