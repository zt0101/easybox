package easybox

import (
	"errors"
	"fmt"
	"net"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
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
	if h, _, splitErr := net.SplitHostPort(host); splitErr == nil {
		host = h
	}
	// Host rows first, like the subscription: the tunnel line's own inbound link
	// (127.0.0.1, no TLS) is never what a client should get. The first row is
	// the line's own address; preferred addresses follow it.
	links := sub.NewLinkProvider().LinksForClient(host, ib, owner.Email)
	if len(links) == 0 || links[0] == "" {
		return "", errors.New("upstream produced no link for this line")
	}
	return links[0], nil
}
