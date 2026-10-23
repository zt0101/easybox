package easybox

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Line templates (EasyBox.md §10): each builds one upstream inbound shape.
const (
	TemplateVlessReality  = "vless-reality"
	TemplateVlessXhttp    = "vless-xhttp"
	TemplateVlessTLS      = "vless-tls"
	TemplateTrojan        = "trojan"
	TemplateHysteria2     = "hysteria2"
	TemplateTuic          = "tuic"
	TemplateShadowsocks   = "shadowsocks"
	TemplateVlessCDN      = "vless-cdn"
	TemplateVlessXhttpCDN = "vless-cdn-xhttp"
	TemplateVlessTunnel   = "vless-tunnel"
)

// Quick Create groups templates by how well they hold up in China; the
// registry below is ordered the same way.
const (
	tierRecommended = "recommended" // first choice for daily use
	tierBackup      = "backup"      // when the recommended lines do not work
	tierRisky       = "risky"       // easy to identify; abroad or emergencies only
)

const (
	transportTCP    = "tcp"
	transportUDP    = "udp"
	transportTCPUDP = "tcp+udp"
)

type lineTemplate struct {
	id        string
	name      string
	label     string
	transport string
	// nameStyle is the default name after the region: HK-<nameStyle>.
	nameStyle string
	tier      string
	build     func(s *Service, name string, id Identity) (*model.Inbound, error)
	// available reports why a template cannot be used right now ("" = usable).
	available func() string
	// relayed lines reach Xray through a relay inbound that only exists after an
	// Xray restart; upstream defers that restart to its 30-second job.
	relayed bool
	// cdnInbound marks a CDN line: prepareCDN makes the Cloudflare record and
	// certificate, then cdnInbound (not build) shapes the inbound. ports, when
	// set, replaces the EasyBox port range.
	cdnInbound func(name string, id Identity, host, certPEM, keyPEM string) (*model.Inbound, error)
	ports      []int
	// preferredPorts are tried before the EasyBox port range while free on this
	// server: on 443 a REALITY line looks like an ordinary HTTPS site. The first
	// such line takes it. Hysteria2 stays in the range: clouds often block UDP
	// 443 by default and the local health check cannot see that.
	preferredPorts []int
	// tunnel lines get a Cloudflare quick tunnel once their inbound is healthy.
	tunnel bool
}

var httpsPort = []int{443}

var lineTemplates = []lineTemplate{
	{
		id: TemplateVlessReality, name: "VLESS + REALITY", label: "vless",
		transport: transportTCP, nameStyle: "VLESS-Reality", tier: tierRecommended,
		preferredPorts: httpsPort,
		build: func(s *Service, name string, id Identity) (*model.Inbound, error) {
			reality, err := s.reality()
			if err != nil {
				return nil, err
			}
			return BuildVlessRealityInbound(name, 0, id, reality)
		},
		available: func() string { return "" },
	},
	{
		id: TemplateVlessXhttp, name: "VLESS + XHTTP", label: "vless",
		transport: transportTCP, nameStyle: "VLESS-XHTTP-Reality", tier: tierRecommended,
		preferredPorts: httpsPort,
		build: func(s *Service, name string, id Identity) (*model.Inbound, error) {
			reality, err := s.reality()
			if err != nil {
				return nil, err
			}
			return BuildVlessXhttpInbound(name, id, reality)
		},
		available: func() string { return "" },
	},
	{
		id: TemplateHysteria2, name: "Hysteria2", label: "hysteria2",
		transport: transportUDP, nameStyle: "HY2", tier: tierRecommended,
		build: func(_ *Service, name string, id Identity) (*model.Inbound, error) {
			cert, err := hysteriaCertificate()
			if err != nil {
				return nil, err
			}
			return BuildHysteria2Inbound(name, id, cert)
		},
		available: func() string { return "" },
	},
	{
		id: TemplateVlessXhttpCDN, name: "VLESS + XHTTP + CDN", label: "vless",
		transport: transportTCP, nameStyle: "VLESS-XHTTP-CDN", tier: tierRecommended,
		cdnInbound: BuildVlessXhttpCDNInbound, ports: cdnPorts,
		available: cloudflareAvailable,
	},
	{
		id: TemplateVlessCDN, name: "VLESS + WebSocket + CDN", label: "vless",
		transport: transportTCP, nameStyle: "VLESS-WS-CDN", tier: tierBackup,
		cdnInbound: BuildVlessCDNInbound, ports: cdnPorts,
		available: cloudflareAvailable,
	},
	{
		id: TemplateVlessTunnel, name: "VLESS + Tunnel", label: "vless",
		transport: transportTCP, nameStyle: "VLESS-WS-Tunnel", tier: tierBackup,
		build: func(_ *Service, name string, id Identity) (*model.Inbound, error) {
			return BuildVlessTunnelInbound(name, id)
		},
		available: tunnelAvailable,
		tunnel:    true,
	},
	{
		id: TemplateTuic, name: "TUIC v5", label: "tuic",
		transport: transportUDP, nameStyle: "TUIC", tier: tierBackup,
		build:     withPanelCertificate(BuildTuicInbound),
		available: needsPanelCertificate,
		relayed:   true,
	},
	{
		id: TemplateVlessTLS, name: "VLESS + TLS", label: "vless",
		transport: transportTCP, nameStyle: "VLESS-TLS", tier: tierRisky,
		build:     withPanelCertificate(BuildVlessTLSInbound),
		available: needsPanelCertificate,
	},
	{
		id: TemplateTrojan, name: "Trojan", label: "trojan",
		transport: transportTCP, nameStyle: "Trojan-TLS", tier: tierRisky,
		build:     withPanelCertificate(BuildTrojanInbound),
		available: needsPanelCertificate,
	},
	{
		id: TemplateShadowsocks, name: "Shadowsocks 2022", label: "shadowsocks",
		transport: transportTCPUDP, nameStyle: "SS2022", tier: tierRisky,
		build: func(s *Service, name string, id Identity) (*model.Inbound, error) {
			id, err := s.ensureSS2022Password(id)
			if err != nil {
				return nil, err
			}
			return BuildShadowsocks2022Inbound(name, id)
		},
		available: func() string { return "" },
	},
}

// withPanelCertificate adapts a builder for lines whose links never allow
// insecure TLS, so they need the panel's trusted certificate (EasyBox.md §9.1).
func withPanelCertificate(build func(string, Identity, lineCertificate) (*model.Inbound, error)) func(*Service, string, Identity) (*model.Inbound, error) {
	return func(_ *Service, name string, id Identity) (*model.Inbound, error) {
		cert, ok, err := panelCertificate()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrNoTrustedCertificate
		}
		return build(name, id, cert)
	}
}

func needsPanelCertificate() string {
	if _, ok, err := panelCertificate(); err != nil || !ok {
		return ErrNoTrustedCertificate.Error()
	}
	return ""
}

// templateByID resolves a template; "" means the default VLESS + REALITY.
func templateByID(id string) (lineTemplate, bool) {
	if id == "" {
		id = TemplateVlessReality
	}
	for _, t := range lineTemplates {
		if t.id == id {
			return t, true
		}
	}
	return lineTemplate{}, false
}

// TemplateView lists a template for Quick Create (EasyBox.md §69).
type TemplateView struct {
	Id        string `json:"id" example:"tuic"`
	Name      string `json:"name" example:"TUIC v5"`
	Protocol  string `json:"protocol" example:"tuic"`
	Transport string `json:"transport" example:"udp"`
	Available bool   `json:"available" example:"false"`
	Reason    string `json:"reason" example:"no trusted TLS certificate: set the panel certificate first"`
	// PasswordUpgrade: creating this line first changes the password of existing
	// TUIC / Trojan lines, so devices must refresh the subscription.
	PasswordUpgrade bool `json:"passwordUpgrade" example:"false"`
	// NameStyle is a new line's default name after the region (HK-<nameStyle>).
	NameStyle string `json:"nameStyle" example:"VLESS-Reality"`
	// Tier: recommended / backup / risky (easy to identify).
	Tier string `json:"tier" example:"recommended"`
}

func (s *Service) ListTemplates() []TemplateView {
	out := make([]TemplateView, 0, len(lineTemplates))
	for _, t := range lineTemplates {
		reason := t.available()
		out = append(out, TemplateView{
			Id: t.id, Name: t.name, Protocol: t.label, Transport: t.transport,
			Available: reason == "", Reason: reason,
			PasswordUpgrade: t.id == TemplateShadowsocks && passwordUpgradePending(),
			NameStyle:       t.nameStyle,
			Tier:            t.tier,
		})
	}
	return out
}
