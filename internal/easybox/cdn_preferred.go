package easybox

import (
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// Preferred addresses: Cloudflare IPs (or domains resolving to them) that reach
// Cloudflare faster from the admin's own network than the line's hostname does.
// Measured on 2026-10-22 from a Chinese company network: the hostname timed out
// while CloudflareST-picked IPs answered in 300-500 ms. Each address becomes an
// extra subscription node "<line>-<remark>" with the same SNI and Host, through
// upstream's Host rows, so raw, Clash and sing-box output all carry it.

const (
	maxPreferredAddresses = 10
	maxPreferredRemark    = 24 // characters
	// cdnHostGroupPrefix marks the Host groups EasyBox owns; they are rebuilt
	// from the settings, so edits made to them elsewhere do not last.
	cdnHostGroupPrefix = "easybox-cdn-"
	// preferredDefaultRemark names an address without a remark ("优选1"); the
	// node name comes from the {{HOST}} part of the remark template.
	preferredDefaultRemark = "优选"
)

var hostnamePattern = regexp.MustCompile(`^(?i:[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)(\.(?i:[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?))+$`)

// PreferredAddress is one preferred Cloudflare address and the remark that
// names its subscription nodes.
type PreferredAddress struct {
	Address string `json:"address" example:"104.17.138.161"`
	Remark  string `json:"remark" example:"公司移动"`
}

type CloudflarePreferredRequest struct {
	Addresses []PreferredAddress `json:"addresses" form:"addresses"`
}

func normalizePreferred(in []PreferredAddress) ([]PreferredAddress, error) {
	out := []PreferredAddress{}
	seenAddr, seenRemark := map[string]bool{}, map[string]bool{}
	for _, p := range in {
		a := strings.ToLower(strings.TrimSpace(p.Address))
		a = strings.TrimSuffix(strings.TrimPrefix(a, "["), "]")
		r := strings.TrimSpace(p.Remark)
		if a == "" {
			if r != "" {
				return nil, fmt.Errorf("remark %q has no address", r)
			}
			continue
		}
		if net.ParseIP(a) == nil && !hostnamePattern.MatchString(a) {
			return nil, fmt.Errorf("%q is not an IP address or domain name", p.Address)
		}
		if seenAddr[a] {
			return nil, fmt.Errorf("%s is listed twice", a)
		}
		if err := validRemark(r); err != nil {
			return nil, err
		}
		if key := strings.ToLower(r); r != "" {
			if seenRemark[key] {
				return nil, fmt.Errorf("remark %q is used twice: node names would clash", r)
			}
			seenRemark[key] = true
		}
		seenAddr[a] = true
		out = append(out, PreferredAddress{Address: a, Remark: r})
	}
	if len(out) > maxPreferredAddresses {
		return nil, fmt.Errorf("at most %d preferred addresses", maxPreferredAddresses)
	}
	return out, nil
}

// validRemark keeps a remark printable and out of the remark template's syntax.
func validRemark(r string) error {
	if utf8.RuneCountInString(r) > maxPreferredRemark {
		return fmt.Errorf("remark %q is longer than %d characters", r, maxPreferredRemark)
	}
	if strings.ContainsAny(r, "{}#") || strings.IndexFunc(r, unicode.IsControl) >= 0 {
		return fmt.Errorf("remark %q may not contain { } # or control characters", r)
	}
	return nil
}

// decodePreferred reads the stored list: a JSON array, or the newline-separated
// addresses eb.10 stored before remarks existed.
func decodePreferred(stored string) []PreferredAddress {
	stored = strings.TrimSpace(stored)
	out := []PreferredAddress{}
	if stored == "" {
		return out
	}
	if strings.HasPrefix(stored, "[") {
		if err := json.Unmarshal([]byte(stored), &out); err == nil {
			return out
		}
		return []PreferredAddress{}
	}
	for _, a := range strings.Split(stored, "\n") {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, PreferredAddress{Address: a})
		}
	}
	return out
}

// SetCloudflarePreferred stores the preferred addresses and applies them to
// every CDN line's subscription nodes.
func (s *Service) SetCloudflarePreferred(req CloudflarePreferredRequest) (CloudflareView, error) {
	prefs, err := normalizePreferred(req.Addresses)
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
	raw, err := json.Marshal(prefs)
	if err != nil {
		return CloudflareView{}, err
	}
	row.PreferredAddresses = string(raw)
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
	prefs := decodePreferred(row.PreferredAddresses)
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
	for i, p := range prefs {
		n := strconv.Itoa(i + 1)
		remark := p.Remark
		if remark == "" {
			remark = preferredDefaultRemark + n
		}
		groups = append(groups, entity.HostGroup{
			GroupId: cdnHostGroupPrefix + n, InboundIds: inboundIDs, Hosts: []string{p.Address},
			Remark: "-" + remark, Security: "same", SortOrder: i + 1, Tags: []string{"easybox"},
		})
	}
	for i := range groups {
		if _, err := hosts.AddHostGroup(&groups[i]); err != nil {
			return err
		}
	}
	return nil
}
