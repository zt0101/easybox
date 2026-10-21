#!/usr/bin/env bash
# Upstream baseline check (EasyBox.md §62): login → REALITY keys → VLESS+REALITY inbound → client → subscription.
# Run inside the dev container against a running panel: bash devenv/smoke-baseline.sh
set -euo pipefail

PANEL=${PANEL:-http://127.0.0.1:2053}
SUB=${SUB:-http://127.0.0.1:2096}
USER_NAME=${USER_NAME:-admin}
USER_PASS=${USER_PASS:-admin}
PORT=${PORT:-20001}
JAR=$(mktemp)
inbound_id=""
cleanup() {
  if [ -n "$inbound_id" ]; then api POST "/panel/api/inbounds/del/$inbound_id" >/dev/null 2>&1 || true; fi
  rm -f "$JAR"
}
trap cleanup EXIT

ok()   { printf '  [OK]   %s\n' "$1"; }
fail() { printf '  [FAIL] %s\n' "$1"; exit 1; }

csrf() { curl -fsS -b "$JAR" -c "$JAR" "$PANEL/csrf-token" | jq -r .obj; }

api() { # method path [json]
  local token; token=$(csrf)
  curl -fsS -b "$JAR" -c "$JAR" -X "$1" -H "X-CSRF-Token: $token" \
    -H 'Content-Type: application/json' ${3:+--data "$3"} "$PANEL$2"
}

echo "EasyBox upstream baseline smoke test"

token=$(csrf)
curl -fsS -b "$JAR" -c "$JAR" -H "X-CSRF-Token: $token" \
  --data-urlencode "username=$USER_NAME" --data-urlencode "password=$USER_PASS" \
  "$PANEL/login" | jq -e '.success == true' >/dev/null || fail "login"
ok "login"

# Fresh installs get a random 16-char subPath (setting.go), so never assume /sub/.
sub_path=$(api POST /panel/api/setting/all | jq -r .obj.subPath)
[ -n "$sub_path" ] && [ "$sub_path" != null ] || fail "read subPath"
ok "subPath = $sub_path"

keys=$(api GET /panel/api/server/getNewX25519Cert)
priv=$(jq -r .obj.privateKey <<<"$keys"); pub=$(jq -r .obj.publicKey <<<"$keys")
[ -n "$priv" ] && [ "$priv" != null ] || fail "x25519 keypair: $keys"
ok "x25519 keypair"

uuid=$(cat /proc/sys/kernel/random/uuid)
subid="smoke$(head -c4 /dev/urandom | od -An -tx1 | tr -d ' \n')"
email="smoke-$subid"
settings=$(jq -cn --arg id "$uuid" --arg email "$email" --arg sub "$subid" '{
  clients:[{id:$id,email:$email,flow:"xtls-rprx-vision",limitIp:0,totalGB:0,expiryTime:0,enable:true,tgId:0,subId:$sub,comment:"",reset:0}],
  decryption:"none",encryption:"none",fallbacks:[]}')
stream=$(jq -cn --arg priv "$priv" --arg pub "$pub" '{
  network:"tcp",tcpSettings:{header:{type:"none"}},security:"reality",
  realitySettings:{show:false,xver:0,target:"www.microsoft.com:443",serverNames:["www.microsoft.com"],
    privateKey:$priv,minClientVer:"",maxClientVer:"",maxTimediff:0,shortIds:["a1b2c3d4"],mldsa65Seed:"",
    settings:{publicKey:$pub,fingerprint:"chrome",serverName:"",spiderX:"/",mldsa65Verify:""}}}')
sniff='{"enabled":true,"destOverride":["http","tls","quic"],"metadataOnly":false,"routeOnly":false}'
body=$(jq -cn --arg s "$settings" --arg st "$stream" --arg sn "$sniff" --argjson port "$PORT" '{
  up:0,down:0,total:0,remark:"smoke-vless-reality",enable:true,expiryTime:0,listen:"",port:$port,
  protocol:"vless",settings:$s,streamSettings:$st,sniffing:$sn}')

res=$(api POST /panel/api/inbounds/add "$body")
jq -e '.success == true' <<<"$res" >/dev/null || fail "create inbound: $res"
inbound_id=$(jq -r .obj.id <<<"$res")
ok "create VLESS+REALITY inbound (id=$inbound_id, port=$PORT) with client $email"

sleep 2
raw=$(curl -fsS "$SUB${sub_path}$subid") || fail "subscription fetch"
link=$(base64 -d <<<"$raw" 2>/dev/null | head -1)
grep -q "^vless://$uuid@" <<<"$link" || fail "subscription content: $link"
grep -q "security=reality" <<<"$link" || fail "subscription missing reality: $link"
ok "subscription returns vless:// link"

api POST "/panel/api/inbounds/del/$inbound_id" | jq -e '.success == true' >/dev/null || fail "delete inbound"
inbound_id=""
ok "delete inbound"

code=$(curl -s -o /dev/null -w '%{http_code}' "$SUB${sub_path}$subid")
[ "$code" = 404 ] || fail "subscription after delete returned $code (want 404)"
ok "subscription returns 404 after delete"

echo "PASS"
