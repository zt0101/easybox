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
)

const (
	transportTCP    = "tcp"
	transportUDP    = "udp"
	transportTCPUDP = "tcp+udp"
)

type lineTemplate struct {
	id         string
	name       string
	label      string
	transport  string
	namePrefix string
	build      func(s *Service, name string, id Identity) (*model.Inbound, error)
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
}

var lineTemplates = []lineTemplate{
	{
		id: TemplateVlessReality, name: "VLESS + REALITY", label: "vless",
		transport: transportTCP, namePrefix: "VLESS",
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
		transport: transportTCP, namePrefix: "XHTTP",
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
		id: TemplateVlessTLS, name: "VLESS + TLS", label: "vless",
		transport: transportTCP, namePrefix: "VLESS-TLS",
		build:     withPanelCertificate(BuildVlessTLSInbound),
		available: needsPanelCertificate,
	},
	{
		id: TemplateTrojan, name: "Trojan", label: "trojan",
		transport: transportTCP, namePrefix: "TROJAN",
		build:     withPanelCertificate(BuildTrojanInbound),
		available: needsPanelCertificate,
	},
	{
		id: TemplateHysteria2, name: "Hysteria2", label: "hysteria2",
		transport: transportUDP, namePrefix: "HY2",
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
		id: TemplateTuic, name: "TUIC v5", label: "tuic",
		transport: transportUDP, namePrefix: "TUIC",
		build:     withPanelCertificate(BuildTuicInbound),
		available: needsPanelCertificate,
		relayed:   true,
	},
	{
		id: TemplateShadowsocks, name: "Shadowsocks 2022", label: "shadowsocks",
		transport: transportTCPUDP, namePrefix: "SS",
		build: func(s *Service, name string, id Identity) (*model.Inbound, error) {
			id, err := s.ensureSS2022Password(id)
			if err != nil {
				return nil, err
			}
			return BuildShadowsocks2022Inbound(name, id)
		},
		available: func() string { return "" },
	},
	{
		id: TemplateVlessCDN, name: "VLESS + CDN", label: "vless",
		transport: transportTCP, namePrefix: "CDN",
		cdnInbound: BuildVlessCDNInbound, ports: cdnPorts,
		available: cloudflareAvailable,
	},
	{
		id: TemplateVlessXhttpCDN, name: "VLESS + XHTTP + CDN", label: "vless",
		transport: transportTCP, namePrefix: "CDN-XHTTP",
		cdnInbound: BuildVlessXhttpCDNInbound, ports: cdnPorts,
		available: cloudflareAvailable,
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
}

func (s *Service) ListTemplates() []TemplateView {
	out := make([]TemplateView, 0, len(lineTemplates))
	for _, t := range lineTemplates {
		reason := t.available()
		out = append(out, TemplateView{
			Id: t.id, Name: t.name, Protocol: t.label, Transport: t.transport,
			Available: reason == "", Reason: reason,
			PasswordUpgrade: t.id == TemplateShadowsocks && passwordUpgradePending(),
		})
	}
	return out
}
