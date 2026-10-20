// Package easybox is the EasyBox product layer on top of 3x-ui: one-click lines
// and a merged subscription, built only from upstream services (EasyBox.md).
package easybox

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Identity is the subscription user a line is attached to; every line embeds
// the same Email+SubID so upstream keeps one ClientRecord per subscription.
type Identity struct {
	Email string
	SubID string
	UUID  string
	// Auth (Hysteria2) and Password (TUIC) sit beside UUID so one subscription
	// user spans every protocol.
	Auth     string
	Password string
}

type RealityParams struct {
	PrivateKey  string
	PublicKey   string
	Target      string
	ServerNames []string
	ShortIDs    []string
}

const vlessVisionFlow = "xtls-rprx-vision"

type vlessClient struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	Flow       string `json:"flow"`
	LimitIP    int    `json:"limitIp"`
	TotalGB    int64  `json:"totalGB"`
	ExpiryTime int64  `json:"expiryTime"`
	Enable     bool   `json:"enable"`
	TgID       int64  `json:"tgId"`
	SubID      string `json:"subId"`
	Comment    string `json:"comment"`
	Reset      int    `json:"reset"`
}

type vlessSettings struct {
	Clients    []vlessClient `json:"clients"`
	Decryption string        `json:"decryption"`
	Encryption string        `json:"encryption"`
	Fallbacks  []any         `json:"fallbacks"`
}

type realityClientSettings struct {
	PublicKey     string `json:"publicKey"`
	Fingerprint   string `json:"fingerprint"`
	ServerName    string `json:"serverName"`
	SpiderX       string `json:"spiderX"`
	Mldsa65Verify string `json:"mldsa65Verify"`
}

type realitySettings struct {
	Show         bool                  `json:"show"`
	Xver         int                   `json:"xver"`
	Target       string                `json:"target"`
	ServerNames  []string              `json:"serverNames"`
	PrivateKey   string                `json:"privateKey"`
	MinClientVer string                `json:"minClientVer"`
	MaxClientVer string                `json:"maxClientVer"`
	MaxTimediff  int                   `json:"maxTimediff"`
	ShortIDs     []string              `json:"shortIds"`
	Mldsa65Seed  string                `json:"mldsa65Seed"`
	Settings     realityClientSettings `json:"settings"`
}

type tcpStreamSettings struct {
	Network         string          `json:"network"`
	TCPSettings     json.RawMessage `json:"tcpSettings"`
	Security        string          `json:"security"`
	RealitySettings realitySettings `json:"realitySettings"`
}

const vlessRealitySniffing = `{"enabled":true,"destOverride":["http","tls","quic"],"metadataOnly":false,"routeOnly":false}`

// BuildVlessRealityInbound returns a VLESS + REALITY (TCP, Vision) inbound in the
// shape of upstream frontend/src/test/golden/fixtures/inbound-full/vless-tcp-reality.json.
func BuildVlessRealityInbound(name string, port int, id Identity, r RealityParams) (*model.Inbound, error) {
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
	stream, err := json.Marshal(tcpStreamSettings{
		Network:     "tcp",
		TCPSettings: json.RawMessage(`{"header":{"type":"none"}}`),
		Security:    "reality",
		RealitySettings: realitySettings{
			Target:      r.Target,
			ServerNames: r.ServerNames,
			PrivateKey:  r.PrivateKey,
			ShortIDs:    r.ShortIDs,
			Settings: realityClientSettings{
				PublicKey:   r.PublicKey,
				Fingerprint: "chrome",
				SpiderX:     "/",
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return &model.Inbound{
		Remark:         name,
		Enable:         true,
		Port:           port,
		Protocol:       model.VLESS,
		Settings:       string(settings),
		StreamSettings: string(stream),
		Sniffing:       vlessRealitySniffing,
	}, nil
}
