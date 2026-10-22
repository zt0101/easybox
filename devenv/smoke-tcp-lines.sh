#!/usr/bin/env bash
# TLS and Shadowsocks lines end to end, on a throwaway panel that has a trusted
# certificate (a local CA standing in for Let's Encrypt): VLESS + TLS, Trojan,
# Shadowsocks 2022 and TUIC, which shares the password, through real Xray,
# sing-box and Mihomo clients. Run inside the dev container:
#   bash devenv/smoke-tcp-lines.sh
# It stops the dev panel while it runs (the lines need ports 20000-20099 and
# Xray's API port); start it again afterwards with: bash devenv/panel.sh
set -euo pipefail
cd /src

PROBE_URL=${PROBE_URL:-https://www.gstatic.com/generate_204}
XRAY=$XUI_BIN_FOLDER/xray-linux-amd64
SINGBOX=${SINGBOX:-/root/.cache/clients/sing-box}
MIHOMO=${MIHOMO:-/root/.cache/clients/mihomo}
WORK=$(mktemp -d)
BIN=$WORK/x-ui
PANEL=https://127.0.0.1:22991
JAR=$WORK/jar
panel_pid=""
client_pid=""

ok()   { printf '  [OK]   %s\n' "$1"; }
fail() { printf '  [FAIL] %s\n' "$1"; exit 1; }
cleanup() {
  [ -n "$client_pid" ] && kill "$client_pid" 2>/dev/null || true
  [ -n "$panel_pid" ] && kill "$panel_pid" 2>/dev/null && wait "$panel_pid" 2>/dev/null || true
  [ -n "${KEEP:-}" ] && echo "kept $WORK" || rm -rf "$WORK"
}
trap cleanup EXIT

echo "EasyBox TLS / Shadowsocks lines smoke test"

# Local CA + certificate for 127.0.0.1; every client trusts the CA via SSL_CERT_FILE.
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes -days 2 \
  -subj "/CN=EasyBox Smoke CA" -keyout "$WORK/ca.key" -out "$WORK/ca.pem" 2>/dev/null
openssl req -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes \
  -subj "/CN=127.0.0.1" -keyout "$WORK/srv.key" -out "$WORK/srv.csr" 2>/dev/null
printf 'subjectAltName=IP:127.0.0.1\nextendedKeyUsage=serverAuth\n' > "$WORK/ext"
openssl x509 -req -in "$WORK/srv.csr" -CA "$WORK/ca.pem" -CAkey "$WORK/ca.key" -CAcreateserial \
  -days 2 -extfile "$WORK/ext" -out "$WORK/srv.pem" 2>/dev/null
export SSL_CERT_FILE=$WORK/ca.pem
ok "local CA and a certificate for 127.0.0.1"

go build -o "$BIN" .
mkdir "$WORK/bin"
for f in "$XUI_BIN_FOLDER"/*; do
  [ "$(basename "$f")" = config.json ] || ln -s "$f" "$WORK/bin/"
done
export XUI_DB_FOLDER=$WORK XUI_LOG_FOLDER=$WORK XUI_BIN_FOLDER=$WORK/bin
"$BIN" setting -port 22991 -username admin -password admin >/dev/null
"$BIN" cert -webCert "$WORK/srv.pem" -webCertKey "$WORK/srv.key" >/dev/null
python3 -c 'import sqlite3,sys; c=sqlite3.connect(sys.argv[1]); c.execute("insert into settings(key,value) values(?,?)",("subPort","22992")); c.commit()' "$WORK/x-ui.db"

pkill -x x-ui 2>/dev/null && sleep 2 || true
"$BIN" > "$WORK/panel.log" 2>&1 &
panel_pid=$!
for _ in $(seq 1 40); do
  curl -fsS -o /dev/null "$PANEL/csrf-token" 2>/dev/null && break
  sleep 0.5
done
ok "throwaway panel with the certificate on $PANEL (dev panel stopped)"

csrf() { curl -fsS -b "$JAR" -c "$JAR" "$PANEL/csrf-token" | jq -r .obj; }
api()  { # method path [json]
  local token; token=$(csrf)
  curl -fsS -b "$JAR" -c "$JAR" -X "$1" -H "X-CSRF-Token: $token" \
    -H 'Content-Type: application/json' ${3:+--data "$3"} "$PANEL$2"
}
token=$(csrf)
curl -fsS -b "$JAR" -c "$JAR" -H "X-CSRF-Token: $token" \
  --data-urlencode username=admin --data-urlencode password=admin "$PANEL/login" \
  | jq -e '.success == true' >/dev/null || fail "login"

api GET /panel/api/easybox/templates \
  | jq -e '[.obj[] | select(.id == "vless-tls" or .id == "trojan" or .id == "shadowsocks" or .id == "tuic") | .available] | length == 4 and all' >/dev/null \
  || fail "templates not all available with a panel certificate"
declare -A port_of
for tpl in vless-tls trojan shadowsocks tuic; do
  res=$(api POST /panel/api/easybox/endpoints/create "{\"template\":\"$tpl\",\"name\":\"smoke-$tpl\",\"address\":\"127.0.0.1\"}")
  jq -e '.success == true and .obj.status == "RUNNING"' <<<"$res" >/dev/null || fail "Quick Create $tpl: $res"
  port_of[$tpl]=$(jq -r .obj.port <<<"$res")
done
ok "Quick Create → RUNNING: vless-tls ${port_of[vless-tls]}, trojan ${port_of[trojan]}, shadowsocks ${port_of[shadowsocks]}, tuic ${port_of[tuic]}"

info=$(api GET /panel/api/easybox/subscription/info)
sub_url=$(jq -r .obj.subUrl <<<"$info"); clash_url=$(jq -r .obj.clashUrl <<<"$info")
curl -fsS "$sub_url" | base64 -d > "$WORK/raw.txt"
link_for() { grep -E "^$1://.*:$2[?#]" "$WORK/raw.txt" | head -1; }
for pair in "vless vless-tls" "trojan trojan" "ss shadowsocks" "tuic tuic"; do
  set -- $pair
  [ -n "$(link_for "$1" "${port_of[$2]}")" ] || fail "raw subscription lacks the $2 line"
done
ok "raw subscription carries all four lines"

# v2rayN / PassWall style: upstream's link parser builds an Xray client config.
# Xray ignores SSL_CERT_FILE, so the local CA goes in as a "verify" certificate.
xray_through() { # link
  go run ./devenv/linkclient "$1" > "$WORK/client.raw.json" || return 1
  jq --rawfile ca "$WORK/ca.pem" '(.outbounds[0].streamSettings | select(.security == "tls") | .tlsSettings.certificates)
      = [{usage: "verify", certificate: ($ca | rtrimstr("\n") | split("\n"))}]' \
    "$WORK/client.raw.json" > "$WORK/client.json"
  "$XRAY" run -c "$WORK/client.json" > "$WORK/client.log" 2>&1 &
  client_pid=$!
  sleep 2
  local code
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 15 --socks5-hostname 127.0.0.1:10808 "$PROBE_URL" || true)
  kill "$client_pid" 2>/dev/null || true; wait "$client_pid" 2>/dev/null || true; client_pid=""
  [ "$code" = 204 ] || { cat "$WORK/client.log"; return 1; }
}
for pair in "vless vless-tls" "trojan trojan" "ss shadowsocks"; do
  set -- $pair
  xray_through "$(link_for "$1" "${port_of[$2]}")" || fail "Xray client through $2 failed"
done
ok "real Xray client (raw links) → 204 via VLESS + TLS, Trojan, Shadowsocks 2022"

curl -fsS -A "SFA/1.14.3 (8; sing-box 1.14.3; language en_US)" -o "$WORK/sb.json" "$sub_url" || fail "sing-box subscription fetch"
"$SINGBOX" check -c "$WORK/sb.json" || fail "sing-box rejects the served config"
singbox_through() { # tag
  jq --arg tag "$1" '.inbounds = [{type:"mixed",tag:"mixed-in",listen:"127.0.0.1",listen_port:10808}]
    | (.outbounds[] | select(.tag == "proxy")) .default = $tag
    | del(.route.auto_detect_interface)' "$WORK/sb.json" > "$WORK/sb-run.json"
  "$SINGBOX" run -c "$WORK/sb-run.json" > "$WORK/sb.log" 2>&1 &
  client_pid=$!
  sleep 2
  local code
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 15 --socks5-hostname 127.0.0.1:10808 "$PROBE_URL" || true)
  kill "$client_pid" 2>/dev/null || true; wait "$client_pid" 2>/dev/null || true; client_pid=""
  [ "$code" = 204 ] || { tail -5 "$WORK/sb.log"; return 1; }
}
for tpl in vless-tls trojan shadowsocks tuic; do
  singbox_through "smoke-$tpl" || fail "sing-box client through smoke-$tpl failed"
done
ok "real sing-box client (sing-box JSON) → 204 via all four lines"

curl -fsS -A "mihomo/1.19.32" -o "$WORK/sub.yaml" "$clash_url" || fail "Mihomo subscription fetch"
mihomo_through() { # proxy-name
  mkdir -p "$WORK/mh"
  { printf 'mixed-port: 10809\nexternal-controller: 127.0.0.1:19090\nallow-lan: false\n'
    grep -vE '^(mixed-port|port|socks-port|external-controller|allow-lan):' "$WORK/sub.yaml"; } > "$WORK/mh/config.yaml"
  "$MIHOMO" -d "$WORK/mh" > "$WORK/mh.log" 2>&1 &
  client_pid=$!
  sleep 3
  local res
  res=$(curl -s --max-time 15 "http://127.0.0.1:19090/proxies/$1/delay?url=$(jq -rn --arg u "$PROBE_URL" '$u|@uri')&timeout=8000" || true)
  kill "$client_pid" 2>/dev/null || true; wait "$client_pid" 2>/dev/null || true; client_pid=""
  jq -e '.delay > 0' >/dev/null 2>&1 <<<"$res" || { echo "$res"; tail -5 "$WORK/mh.log"; return 1; }
}
for tpl in vless-tls trojan shadowsocks tuic; do
  mihomo_through "smoke-$tpl" || fail "Mihomo client through smoke-$tpl failed"
done
ok "real Mihomo client (Clash/Mihomo YAML) reaches the web via all four lines"

echo "PASS (start the dev panel again: bash devenv/panel.sh)"
