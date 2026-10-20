package easybox

import (
	"errors"
	"net"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

var ErrUnknownPreviewClient = errors.New("unknown client for preview")

// previewFormats maps each target client (EasyBox.md §32) to the format the
// universal URL serves it: their user agents route exactly this way.
var previewFormats = map[string]string{
	"v2rayn":       "raw",
	"shadowrocket": "raw",
	"passwall":     "raw",
	"mihomo":       "clash",
	"singbox":      "singbox",
}

// PreviewView is what one client would receive from the subscription (§43).
type PreviewView struct {
	Client  string   `json:"client" example:"singbox"`
	Format  string   `json:"format" example:"singbox"`
	Body    string   `json:"body" example:"{\"outbounds\": []}"`
	Skipped []string `json:"skipped"`
}

// PreviewSubscription renders the subscription with the same code the sub
// server uses (raw links are shown decoded; clients receive them base64).
func (s *Service) PreviewSubscription(client, host string) (*PreviewView, error) {
	format, ok := previewFormats[strings.ToLower(client)]
	if !ok {
		return nil, ErrUnknownPreviewClient
	}
	id, err := EnsureDefaultSubscription()
	if err != nil {
		return nil, err
	}
	settings := &service.SettingService{}
	tmpl, err := settings.GetRemarkTemplate()
	if err != nil {
		return nil, err
	}
	if h, _, splitErr := net.SplitHostPort(host); splitErr == nil {
		host = h
	}
	view := &PreviewView{Client: client, Format: format, Skipped: []string{}}
	subSvc := sub.NewSubService(tmpl)
	switch format {
	case "raw":
		links, _, _, _, err := subSvc.GetSubs(id.SubID, host)
		if err != nil {
			return nil, err
		}
		view.Body = strings.Join(links, "\n")
	case "clash":
		routing, err := settings.GetSubClashEnableRouting()
		if err != nil {
			return nil, err
		}
		rules, err := settings.GetSubClashRules()
		if err != nil {
			return nil, err
		}
		if view.Body, _, err = sub.NewSubClashService(routing, rules, subSvc).GetClash(id.SubID, host); err != nil {
			return nil, err
		}
	case "singbox":
		body, skipped, _, err := subSvc.GetSingboxConfig(id.SubID, host)
		if err != nil {
			return nil, err
		}
		view.Body = string(body)
		view.Skipped = append(view.Skipped, skipped...)
	}
	return view, nil
}
