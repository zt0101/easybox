package easybox

import (
	"errors"
	"fmt"
	"net"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// GetEndpointLink renders one line's share link through the subscription
// service itself, so Copy / QR always match the subscription (EasyBox.md §22).
func (s *Service) GetEndpointLink(id int, host string) (string, error) {
	row, err := loadEndpoint(id)
	if err != nil {
		return "", err
	}
	if row.InboundId == 0 {
		return "", fmt.Errorf("line %d has no inbound", id)
	}
	ib, err := s.inbounds.GetInbound(row.InboundId)
	if err != nil {
		return "", err
	}
	var owner model.EasyboxSubscription
	if err := database.GetDB().First(&owner, row.SubscriptionId).Error; err != nil {
		return "", fmt.Errorf("subscription user of line %d: %w", id, err)
	}
	tmpl, err := (&service.SettingService{}).GetRemarkTemplate()
	if err != nil {
		return "", err
	}
	if h, _, splitErr := net.SplitHostPort(host); splitErr == nil {
		host = h
	}
	l := sub.NewSubService(tmpl).ForRequest(host).GetLink(ib, owner.Email)
	if l == "" {
		return "", errors.New("upstream produced no link for this line")
	}
	return l, nil
}
