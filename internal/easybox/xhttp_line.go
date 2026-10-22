package easybox

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
)

// xhttpPathLength is the random path segment of an XHTTP line; a fixed path
// would be one more constant every EasyBox server shares.
const xhttpPathLength = 8

// xhttpSettings carries upstream's XHTTP form defaults
// (frontend/src/schemas/protocols/stream/xhttp.ts) for the fields Xray reads on
// the server; the rest stay at Xray's own defaults.
type xhttpSettings struct {
	Path                 string            `json:"path"`
	Host                 string            `json:"host"`
	Mode                 string            `json:"mode"`
	XPaddingBytes        string            `json:"xPaddingBytes"`
	NoSSEHeader          bool              `json:"noSSEHeader"`
	ScMaxBufferedPosts   int               `json:"scMaxBufferedPosts"`
	ScStreamUpServerSecs string            `json:"scStreamUpServerSecs"`
	Headers              map[string]string `json:"headers"`
}

type xhttpStreamSettings struct {
	Network         string          `json:"network"`
	XHTTPSettings   xhttpSettings   `json:"xhttpSettings"`
	Security        string          `json:"security"`
	RealitySettings realitySettings `json:"realitySettings"`
}

// BuildVlessXhttpInbound is a VLESS + XHTTP + REALITY line: like the REALITY
// line it needs no certificate or domain. Vision is TCP-only, so no flow.
func BuildVlessXhttpInbound(name string, id Identity, r RealityParams) (*model.Inbound, error) {
	settings, err := json.Marshal(vlessSettings{
		Clients: []vlessClient{{
			ID:     id.UUID,
			Email:  id.Email,
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
	stream, err := json.Marshal(xhttpStreamSettings{
		Network: "xhttp",
		XHTTPSettings: xhttpSettings{
			Path:                 "/" + random.NumLower(xhttpPathLength),
			Mode:                 "auto",
			XPaddingBytes:        "100-1000",
			ScMaxBufferedPosts:   30,
			ScStreamUpServerSecs: "20-80",
			Headers:              map[string]string{},
		},
		Security: "reality",
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
		Protocol:       model.VLESS,
		Settings:       string(settings),
		StreamSettings: string(stream),
		Sniffing:       vlessRealitySniffing,
	}, nil
}
