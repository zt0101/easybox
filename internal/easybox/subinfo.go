package easybox

import (
	"fmt"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// SubscriptionView is the "My Subscription" page (EasyBox.md §24): one URL for
// every client, plus the lines it carries.
type SubscriptionView struct {
	SubID          string          `json:"subId" example:"3dllwqwd8kpj3i5e"`
	SubURL         string          `json:"subUrl" example:"https://203.0.113.10:2096/p2z29sqh4lk5cmdg/3dllwqwd8kpj3i5e"`
	ClashURL       string          `json:"clashUrl" example:"https://203.0.113.10:2096/3zxrphzqdxumbq7i/3dllwqwd8kpj3i5e"`
	UpdateInterval string          `json:"updateInterval" example:"12"`
	Lines          []*EndpointView `json:"lines"`
}

// GetSubscription builds the URLs the same way the panel's own settings do: an
// admin-set subURI / subClashURI wins, else the sub server base for host.
func (s *Service) GetSubscription(host string) (*SubscriptionView, error) {
	id, err := EnsureDefaultSubscription()
	if err != nil {
		return nil, err
	}
	settings := &service.SettingService{}
	view := &SubscriptionView{SubID: id.SubID}
	if view.UpdateInterval, err = settings.GetSubUpdates(); err != nil {
		return nil, err
	}

	subURI, err := settings.GetSubURI()
	if err != nil {
		return nil, err
	}
	subPath, err := settings.GetSubPath()
	if err != nil {
		return nil, err
	}
	view.SubURL = subscriptionURL(settings, host, subURI, subPath, id.SubID)

	clashOn, err := settings.GetSubClashEnable()
	if err != nil {
		return nil, err
	}
	if clashOn {
		clashURI, err := settings.GetSubClashURI()
		if err != nil {
			return nil, err
		}
		clashPath, err := settings.GetSubClashPath()
		if err != nil {
			return nil, err
		}
		view.ClashURL = subscriptionURL(settings, host, clashURI, clashPath, id.SubID)
	}

	if view.Lines, err = s.ListEndpoints(); err != nil {
		return nil, err
	}
	return view, nil
}

func subscriptionURL(settings *service.SettingService, host, customURI, path, subID string) string {
	if customURI != "" {
		return customURI + subID
	}
	return settings.BuildSubURIBase(host) + path + subID
}

// SetEndpointInSubscription toggles upstream Inbound.ExcludeFromSub, which every
// subscription format (raw, Clash, JSON) already honours; the line keeps running.
func (s *Service) SetEndpointInSubscription(id int, include bool) error {
	row, err := loadEndpoint(id)
	if err != nil {
		return err
	}
	if row.InboundId == 0 {
		return fmt.Errorf("line %d has no inbound", id)
	}
	ib, err := s.inbounds.GetInbound(row.InboundId)
	if err != nil {
		return err
	}
	ib.ExcludeFromSub = !include
	_, needRestart, err := s.inbounds.UpdateInbound(ib)
	if needRestart {
		s.xray.SetToNeedRestart()
	}
	audit("endpoint.subscription", id, err)
	return err
}
