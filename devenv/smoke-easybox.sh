#!/usr/bin/env bash
# EasyBox end-to-end check against a running panel + real Xray: Quick Create → merged
# subscription (raw + Mihomo) → real proxied request → exclude/include → regenerate →
# subscription rotation → disable → delete. Run inside the dev container:
#   bash devenv/smoke-easybox.sh
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
code_of() { curl -s -o /dev/null -w '%{http_code}' "$1"; }
cleanup() {
  [ -n "$client_pid" ] && kill "$client_pid" 2>/dev/null || true
  [ -n "$line_id" ] && api POST "/panel/api/easybox/endpoints/del/$line_id" >/dev/null 2>&1 || true
  rm -rf "$JAR" "$WORK"
}
trap cleanup EXIT

sub_link_for_port() { # sub_url port → vless link of that line
  { curl -fsS "$1" 2>/dev/null || true; } | base64 -d 2>/dev/null | grep "^vless://.*:$2?" | head -1 || true
}

# Starts a second Xray as a REALITY client built from a vless:// link and fetches
# PROBE_URL through it: proves the line really proxies traffic, not just listens.
proxy_through() { # link port
  local link=$1 port=$2 uuid query
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
  local code
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 15 --socks5-hostname 127.0.0.1:10808 "$PROBE_URL" || true)
  kill "$client_pid" 2>/dev/null || true; wait "$client_pid" 2>/dev/null || true; client_pid=""
  [ "$code" = 204 ] || { cat "$WORK/client.log"; return 1; }
}

# Same check for any protocol: upstream's link parser builds the client config,
# the way a link-importing client does (devenv/linkclient).
proxy_through_link() { # link
  go run ./devenv/linkclient "$1" > "$WORK/client.json" || return 1
  "$XRAY" run -c "$WORK/client.json" > "$WORK/client.log" 2>&1 &
  client_pid=$!
  sleep 2
  local code
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 15 --socks5-hostname 127.0.0.1:10808 "$PROBE_URL" || true)
  kill "$client_pid" 2>/dev/null || true; wait "$client_pid" 2>/dev/null || true; client_pid=""
  [ "$code" = 204 ] || { cat "$WORK/client.log"; return 1; }
}

echo "EasyBox smoke test"

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

link=$(sub_link_for_port "$sub_url" "$port")
[ -n "$link" ] || fail "subscription has no link for port $port"
[ "${link##*#}" = "smoke-line" ] || fail "link name is '${link##*#}', want the line name smoke-line"
ok "merged subscription contains the line, named after it"

[ -n "$clash_url" ] || fail "Clash/Mihomo subscription not enabled"
curl -fsS -A "mihomo/1.19" "$clash_url" | grep -q "name: smoke-line" || fail "Clash/Mihomo YAML lacks the line"
ok "Clash/Mihomo subscription serves the line"

proxy_through "$link" "$port" || fail "request through the line failed"
ok "real request through the line → 204"

api POST "/panel/api/easybox/endpoints/setInSubscription/$line_id" '{"include":false}' | jq -e '.success == true' >/dev/null || fail "exclude"
[ -z "$(sub_link_for_port "$sub_url" "$port")" ] || fail "excluded line still in the subscription"
api POST "/panel/api/easybox/endpoints/setInSubscription/$line_id" '{"include":true}' | jq -e '.success == true' >/dev/null || fail "include"
[ -n "$(sub_link_for_port "$sub_url" "$port")" ] || fail "re-included line missing from the subscription"
ok "exclude / include toggles the line in the subscription"

res=$(api POST "/panel/api/easybox/endpoints/regenerate/$line_id" '{"keepOld":false}')
jq -e '.success == true and .obj.status == "RUNNING"' <<<"$res" >/dev/null || fail "regenerate: $res"
new_port=$(jq -r .obj.port <<<"$res")
[ "$new_port" != "$port" ] || fail "regenerate kept port $port"
[ -z "$(sub_link_for_port "$sub_url" "$port")" ] || fail "old port $port still in the subscription"
link=$(sub_link_for_port "$sub_url" "$new_port")
proxy_through "$link" "$new_port" || fail "request through the regenerated line failed"
ok "regenerate (replace) → port $port → $new_port, old gone, new line proxies"
port=$new_port

info=$(api POST /panel/api/easybox/subscription/rotate)
new_sub_url=$(jq -r .obj.subUrl <<<"$info")
[ -n "$new_sub_url" ] && [ "$new_sub_url" != "$sub_url" ] || fail "rotate: $info"
[ "$(code_of "$sub_url")" = 404 ] || fail "old subscription URL still answers after rotation"
old_uuid=${link#vless://}; old_uuid=${old_uuid%%@*}
link=$(sub_link_for_port "$new_sub_url" "$port")
new_uuid=${link#vless://}; new_uuid=${new_uuid%%@*}
[ -n "$new_uuid" ] && [ "$new_uuid" != "$old_uuid" ] || fail "rotation kept UUID $old_uuid"
proxy_through "$link" "$port" || fail "request with the rotated credentials failed"
ok "rotation → old URL 404, new URL + new UUID proxy"
sub_url=$new_sub_url

api POST "/panel/api/easybox/endpoints/setEnable/$line_id" '{"enable":false}' | jq -e '.success == true' >/dev/null || fail "disable"
[ -z "$(sub_link_for_port "$sub_url" "$port")" ] || fail "disabled line still in the subscription"
ok "disabled line leaves the subscription"

api POST "/panel/api/easybox/endpoints/del/$line_id" | jq -e '.success == true' >/dev/null || fail "delete"
line_id=""
api GET /panel/api/easybox/endpoints/list | jq -e '[.obj[] | select(.name == "smoke-line")] | length == 0' >/dev/null || fail "list after delete"
ok "delete removes the line"

res=$(api POST /panel/api/easybox/endpoints/create '{"template":"hysteria2","name":"smoke-hy2","address":"127.0.0.1"}')
jq -e '.success == true and .obj.status == "RUNNING" and .obj.transport == "udp"' <<<"$res" >/dev/null || fail "Hysteria2 Quick Create: $res"
line_id=$(jq -r .obj.id <<<"$res"); port=$(jq -r .obj.port <<<"$res")
link=$(api GET "/panel/api/easybox/endpoints/link/$line_id" | jq -r .obj)
grep -q "pinSHA256=" <<<"$link" || fail "Hysteria2 link without certificate pin: $link"
[ -n "$(curl -fsS "$sub_url" | base64 -d | grep "^hysteria2://.*:$port?")" ] || fail "Hysteria2 line missing from the subscription"
proxy_through_link "$link" || fail "request through the Hysteria2 line failed"
ok "Hysteria2 (self-signed, pinned, salamander) → RUNNING, in subscription, proxies"
api POST "/panel/api/easybox/endpoints/del/$line_id" | jq -e '.success == true' >/dev/null || fail "delete Hysteria2 line"
line_id=""

echo "PASS"
