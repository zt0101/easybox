package easybox

import (
	"encoding/base64"
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
)

// TCP lines that need no REALITY: VLESS and Trojan over TLS with the panel's
// trusted certificate, and Shadowsocks 2022, which needs no certificate.

const tcpSniffing = `{"enabled":true,"destOverride":["http","tls","quic"],"metadataOnly":false,"routeOnly":false}`

// ss2022Method is the cipher of every EasyBox Shadowsocks line. Its 16-byte key
// size is what the shared subscription password is generated for.
const ss2022Method = "2022-blake3-aes-128-gcm"

const ss2022KeyBytes = 16

// newSharedPassword returns a subscription password that every protocol on a
// line accepts as is: TUIC and Trojan take any string, and Shadowsocks 2022
// needs exactly a base64 key of ss2022KeyBytes bytes.
func newSharedPassword() string {
	return random.Base64Bytes(ss2022KeyBytes)
}

// isSS2022Password reports whether p can serve as a Shadowsocks 2022 user key.
func isSS2022Password(p string) bool {
	key, err := base64.StdEncoding.DecodeString(p)
	return err == nil && len(key) == ss2022KeyBytes
}

// tcpTLSStream mirrors upstream's default TCP + TLS stream (fixture
// inbound-full/vmess-tcp-tls.json) with the panel certificate files filled in.
func tcpTLSStream(cert lineCertificate) (string, error) {
	stream, err := json.Marshal(map[string]any{
		"network":     "tcp",
		"tcpSettings": map[string]any{"acceptProxyProtocol": false, "header": map[string]any{"type": "none"}},
		"security":    "tls",
		"tlsSettings": map[string]any{
			"serverName": cert.serverName, "minVersion": "1.2", "maxVersion": "1.3", "cipherSuites": "",
			"rejectUnknownSni": false, "disableSystemRoot": false, "enableSessionResumption": false,
			"certificates": []any{map[string]any{
				"certificateFile": cert.certFile, "keyFile": cert.keyFile,
				"oneTimeLoading": false, "usage": "encipherment", "buildChain": false,
			}},
			"alpn": []string{"h2", "http/1.1"}, "echServerKeys": "",
			"settings": map[string]any{"fingerprint": "chrome", "echConfigList": ""},
		},
	})
	return string(stream), err
}

// BuildVlessTLSInbound is a VLESS + TCP + TLS line with Vision. Unlike REALITY it
// does not depend on the client offering X25519MLKEM768 (CLIENT_COMPATIBILITY.md).
func BuildVlessTLSInbound(name string, id Identity, cert lineCertificate) (*model.Inbound, error) {
	settings, err := json.Marshal(vlessSettings{
		Clients: []vlessClient{{
			ID:     id.UUID,
			Email:  id.Email,
			Flow:   vlessVisionFlow,
			Enable: true,
			SubID:  id.SubID,
		}},
		Decryption: "none",
		Encryption: "none",
		Fallbacks:  []any{},
	})
	if err != nil {
		return nil, err
	}
	stream, err := tcpTLSStream(cert)
	if err != nil {
		return nil, err
	}
	return &model.Inbound{
		Remark: name, Enable: true, Protocol: model.VLESS,
		Settings: string(settings), StreamSettings: stream, Sniffing: tcpSniffing,
	}, nil
}

// BuildTrojanInbound is a Trojan + TCP + TLS line (fixture trojan-ws-tls.json,
// with upstream's TCP transport).
func BuildTrojanInbound(name string, id Identity, cert lineCertificate) (*model.Inbound, error) {
	settings, err := json.Marshal(map[string]any{
		"clients": []map[string]any{{
			"password": id.Password, "email": id.Email, "limitIp": 0, "totalGB": 0, "expiryTime": 0,
			"enable": true, "tgId": 0, "subId": id.SubID, "comment": "", "reset": 0,
		}},
		"fallbacks": []any{},
	})
	if err != nil {
		return nil, err
	}
	stream, err := tcpTLSStream(cert)
	if err != nil {
		return nil, err
	}
	return &model.Inbound{
		Remark: name, Enable: true, Protocol: model.Trojan,
		Settings: string(settings), StreamSettings: stream, Sniffing: tcpSniffing,
	}, nil
}

// BuildShadowsocks2022Inbound is a multi-user Shadowsocks 2022 line over TCP and
// UDP (fixture shadowsocks-tcp-2022.json). id.Password must be a 16-byte key.
func BuildShadowsocks2022Inbound(name string, id Identity) (*model.Inbound, error) {
	settings, err := json.Marshal(map[string]any{
		"method":   ss2022Method,
		"password": random.Base64Bytes(ss2022KeyBytes),
		"network":  "tcp,udp",
		"clients": []map[string]any{{
			"method": "", "password": id.Password, "email": id.Email, "limitIp": 0, "totalGB": 0,
			"expiryTime": 0, "enable": true, "tgId": 0, "subId": id.SubID, "comment": "", "reset": 0,
		}},
		"ivCheck": false,
	})
	if err != nil {
		return nil, err
	}
	stream, err := json.Marshal(map[string]any{
		"network":     "tcp",
		"tcpSettings": map[string]any{"acceptProxyProtocol": false, "header": map[string]any{"type": "none"}},
		"security":    "none",
	})
	if err != nil {
		return nil, err
	}
	return &model.Inbound{
		Remark: name, Enable: true, Protocol: model.Shadowsocks,
		Settings: string(settings), StreamSettings: string(stream), Sniffing: tcpSniffing,
	}, nil
}
