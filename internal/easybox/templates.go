package easybox

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Line templates (EasyBox.md §10): each builds one upstream inbound shape.
const (
	TemplateVlessReality = "vless-reality"
	TemplateHysteria2    = "hysteria2"
	TemplateTuic         = "tuic"
)

const (
	transportTCP = "tcp"
	transportUDP = "udp"
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
		build: func(_ *Service, name string, id Identity) (*model.Inbound, error) {
			cert, ok, err := panelCertificate()
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, ErrNoTrustedCertificate
			}
			return BuildTuicInbound(name, id, cert)
		},
		available: func() string {
			if _, ok, err := panelCertificate(); err != nil || !ok {
				return ErrNoTrustedCertificate.Error()
			}
			return ""
		},
	},
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
}

func (s *Service) ListTemplates() []TemplateView {
	out := make([]TemplateView, 0, len(lineTemplates))
	for _, t := range lineTemplates {
		reason := t.available()
		out = append(out, TemplateView{
			Id: t.id, Name: t.name, Protocol: t.label, Transport: t.transport,
			Available: reason == "", Reason: reason,
		})
	}
	return out
}
