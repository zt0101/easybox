package easybox

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// Preferred addresses: Cloudflare IPs (or domains resolving to them) that reach
// Cloudflare faster from the admin's own network than the line's hostname does.
// Measured on 2026-10-22 from a Chinese company network: the hostname timed out
// while CloudflareST-picked IPs answered in 300-500 ms. Each address becomes an
// extra subscription node with the same SNI and Host, through upstream's Host
// rows, so raw, Clash and sing-box output all carry it.

const (
	maxPreferredAddresses = 5
	// cdnHostGroupPrefix marks the Host groups EasyBox owns; they are rebuilt
	// from the settings, so edits made to them elsewhere do not last.
	cdnHostGroupPrefix = "easybox-cdn-"
	// preferredRemarkPrefix names a preferred node "<line>-优选1" through the
	// {{HOST}} part of the remark template.
	preferredRemarkPrefix = "-优选"
)

var hostnamePattern = regexp.MustCompile(`^(?i:[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)(\.(?i:[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?))+$`)

type CloudflarePreferredRequest struct {
	Addresses []string `json:"addresses" form:"addresses" example:"[\"104.17.138.161\"]"`
}

func normalizePreferred(in []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, raw := range in {
		a := strings.ToLower(strings.TrimSpace(raw))
		a = strings.TrimSuffix(strings.TrimPrefix(a, "["), "]")
		if a == "" || seen[a] {
			continue
		}
		if net.ParseIP(a) == nil && !hostnamePattern.MatchString(a) {
			return nil, fmt.Errorf("%q is not an IP address or domain name", raw)
		}
		seen[a] = true
		out = append(out, a)
	}
	if len(out) > maxPreferredAddresses {
		return nil, fmt.Errorf("at most %d preferred addresses", maxPreferredAddresses)
	}
	return out, nil
}

func splitPreferred(stored string) []string {
	if strings.TrimSpace(stored) == "" {
		return []string{}
	}
	return strings.Split(stored, "\n")
}

// SetCloudflarePreferred stores the preferred addresses and applies them to
// every CDN line's subscription nodes.
func (s *Service) SetCloudflarePreferred(req CloudflarePreferredRequest) (CloudflareView, error) {
	addrs, err := normalizePreferred(req.Addresses)
	if err != nil {
		return CloudflareView{}, err
	}
	row, ok, err := loadCloudflare()
	if err != nil {
		return CloudflareView{}, err
	}
	if !ok {
		return CloudflareView{}, ErrCloudflareNotConfigured
	}
	row.PreferredAddresses = strings.Join(addrs, "\n")
	if err := database.GetDB().Save(row).Error; err != nil {
		return CloudflareView{}, err
	}
	err = syncCDNHosts()
	auditObject("cloudflare.preferred", "cloudflare", row.Id, err)
	return cloudflareView(row), err
}

// syncCDNHosts rebuilds EasyBox's Host groups: with preferred addresses, every
// CDN line gets one host for its own hostname (kept first) plus one per
// preferred address; without any, the lines fall back to their plain link.
func syncCDNHosts() error {
	db := database.GetDB()
	if err := db.Where("group_id LIKE ?", cdnHostGroupPrefix+"%").Delete(&model.Host{}).Error; err != nil {
		return err
	}
	row, ok, err := loadCloudflare()
	if err != nil || !ok {
		return err
	}
	prefs := splitPreferred(row.PreferredAddresses)
	var inboundIDs []int
	if err := db.Model(&model.EasyboxEndpoint{}).
		Where("cdn_host <> '' AND inbound_id <> 0").Order("id").Pluck("inbound_id", &inboundIDs).Error; err != nil {
		return err
	}
	if len(prefs) == 0 || len(inboundIDs) == 0 {
		return nil
	}
	hosts := &service.HostService{}
	groups := []entity.HostGroup{{
		GroupId: cdnHostGroupPrefix + "0", InboundIds: inboundIDs, Remark: "", Security: "same",
		Tags: []string{"easybox"},
	}}
	for i, addr := range prefs {
		n := strconv.Itoa(i + 1)
		groups = append(groups, entity.HostGroup{
			GroupId: cdnHostGroupPrefix + n, InboundIds: inboundIDs, Hosts: []string{addr},
			Remark: preferredRemarkPrefix + n, Security: "same", SortOrder: i + 1, Tags: []string{"easybox"},
		})
	}
	for i := range groups {
		if _, err := hosts.AddHostGroup(&groups[i]); err != nil {
			return err
		}
	}
	return nil
}
