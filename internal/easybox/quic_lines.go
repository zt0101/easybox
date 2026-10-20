package easybox

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
)

// BuildHysteria2Inbound follows upstream's form defaults for a new Hysteria
// inbound: version 2, network "hysteria", ALPN h3 and salamander obfs.
func BuildHysteria2Inbound(name string, id Identity, cert lineCertificate) (*model.Inbound, error) {
	settings, err := json.Marshal(map[string]any{
		"version": 2,
		"clients": []map[string]any{{
			"auth": id.Auth, "email": id.Email, "limitIp": 0, "totalGB": 0, "expiryTime": 0,
			"enable": true, "tgId": 0, "subId": id.SubID, "comment": "", "reset": 0,
		}},
	})
	if err != nil {
		return nil, err
	}
	stream, err := json.Marshal(map[string]any{
		"network":          "hysteria",
		"security":         "tls",
		"hysteriaSettings": map[string]any{"version": 2, "auth": "", "udpIdleTimeout": 60},
		"tlsSettings":      xrayTLSSettings(cert),
		"finalmask": map[string]any{
			"tcp": []any{},
			"udp": []any{map[string]any{
				"type":     "salamander",
				"settings": map[string]any{"password": random.NumLower(16)},
			}},
		},
	})
	if err != nil {
		return nil, err
	}
	return &model.Inbound{
		Remark: name, Enable: true, Protocol: model.Hysteria,
		Settings: string(settings), StreamSettings: string(stream), Sniffing: vlessRealitySniffing,
	}, nil
}

// BuildTuicInbound uses upstream createDefaultTuicInboundSettings values and the
// panel's trusted certificate files (TUIC links never allow insecure TLS).
func BuildTuicInbound(name string, id Identity, cert lineCertificate) (*model.Inbound, error) {
	settings, err := json.Marshal(map[string]any{
		"server": map[string]any{
			"certificate": cert.certFile, "private_key": cert.keyFile,
			"congestion_control": "bbr", "alpn": []string{"h3", "spdy/3.1"},
			"udp_relay_mode": "native", "zero_rtt_handshake": true, "log_level": "info",
			"max_idle_time": 15, "authentication_timeout": 3, "max_udp_relay_packet_size": 1500,
			"sni": cert.serverName,
		},
		"clients": []map[string]any{{
			"uuid": id.UUID, "id": id.UUID, "password": id.Password, "email": id.Email,
			"limitIp": 0, "totalGB": 0, "expiryTime": 0, "enable": true, "tgId": 0,
			"subId": id.SubID, "comment": "", "reset": 0,
		}},
	})
	if err != nil {
		return nil, err
	}
	return &model.Inbound{
		Remark: name, Enable: true, Protocol: model.TUIC, Settings: string(settings),
	}, nil
}
