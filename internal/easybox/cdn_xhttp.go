package easybox

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
)

// BuildVlessXhttpCDNInbound is VLESS + XHTTP + TLS behind Cloudflare: the CDN
// shape Xray recommends for XHTTP, harder to fingerprint than WebSocket. Mode
// "auto" makes a TLS client use packet-up, which Cloudflare passes; sing-box
// has no XHTTP, so its config leaves these lines out like the REALITY XHTTP one.
func BuildVlessXhttpCDNInbound(name string, id Identity, host, certPEM, keyPEM string) (*model.Inbound, error) {
	settings, err := json.Marshal(vlessSettings{
		Clients:    []vlessClient{{ID: id.UUID, Email: id.Email, Enable: true, SubID: id.SubID}},
		Decryption: "none",
		Encryption: "none",
		Fallbacks:  []any{},
	})
	if err != nil {
		return nil, err
	}
	stream, err := json.Marshal(map[string]any{
		"network": "xhttp",
		"xhttpSettings": xhttpSettings{
			Path:                 "/" + random.NumLower(xhttpPathLength),
			Host:                 host,
			Mode:                 "auto",
			XPaddingBytes:        "100-1000",
			ScMaxBufferedPosts:   30,
			ScStreamUpServerSecs: "20-80",
			Headers:              map[string]string{},
		},
		"security": "tls",
		"tlsSettings": map[string]any{
			"serverName": host, "minVersion": "1.2", "maxVersion": "1.3", "cipherSuites": "",
			"rejectUnknownSni": false, "disableSystemRoot": false, "enableSessionResumption": false,
			"certificates": []any{map[string]any{
				"certificate": pemLines(certPEM), "key": pemLines(keyPEM),
				"oneTimeLoading": false, "usage": "encipherment", "buildChain": false,
			}},
			"alpn": []string{"h2", "http/1.1"}, "echServerKeys": "",
			"settings": map[string]any{"fingerprint": "chrome", "echConfigList": ""},
		},
	})
	if err != nil {
		return nil, err
	}
	return &model.Inbound{
		Remark: name, Enable: true, Protocol: model.VLESS,
		Settings: string(settings), StreamSettings: string(stream), Sniffing: tcpSniffing,
	}, nil
}
