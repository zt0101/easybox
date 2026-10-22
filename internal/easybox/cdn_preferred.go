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

// preferredSettingKey stores the list in easybox_settings: it is not tied to a
// Cloudflare token, since lines without a domain use it too.
const preferredSettingKey = "cdnPreferredAddresses"

// PreferredView is the API view of the preferred addresses.
type PreferredView struct {
	Addresses []PreferredAddress `json:"addresses"`
}

func loadPreferred() ([]PreferredAddress, error) {
	v, _, err := getSetting(preferredSettingKey)
	return decodePreferred(v), err
}

func (s *Service) GetPreferred() (PreferredView, error) {
	prefs, err := loadPreferred()
	return PreferredView{Addresses: prefs}, err
}

// SetPreferred stores the preferred addresses and applies them to every CDN
// line's subscription nodes.
func (s *Service) SetPreferred(req CloudflarePreferredRequest) (PreferredView, error) {
	prefs, err := normalizePreferred(req.Addresses)
	if err != nil {
		return PreferredView{}, err
	}
	raw, err := json.Marshal(prefs)
	if err != nil {
		return PreferredView{}, err
	}
	if err := setSetting(preferredSettingKey, string(raw)); err != nil {
		return PreferredView{}, err
	}
	err = syncCDNHosts()
	auditObject("preferred.set", "preferred", 0, err)
	return PreferredView{Addresses: prefs}, err
}

// cdnTemplates are the lines whose own link already carries TLS to Cloudflare.
var cdnTemplates = []string{TemplateVlessCDN, TemplateVlessXhttpCDN}

// tunnelHostGroupPrefix marks the Host groups of the tunnel line.
const tunnelHostGroupPrefix = "easybox-tunnel-"

func preferredRemark(i int, p PreferredAddress) string {
	if p.Remark != "" {
		return "-" + p.Remark
	}
	return "-" + preferredDefaultRemark + strconv.Itoa(i+1)
}

// syncCDNHosts rebuilds EasyBox's Host groups.
//   - CDN lines: with preferred addresses, one host for the line's own hostname
//     (kept first) plus one per address; without any, their plain link.
//   - The tunnel line: always hosts, since its inbound has no TLS while clients
//     reach https://<hostname>:443 — one for the hostname, one per address.
func syncCDNHosts() error {
	db := database.GetDB()
	if err := db.Where("group_id LIKE ? OR group_id LIKE ?", cdnHostGroupPrefix+"%", tunnelHostGroupPrefix+"%").
		Delete(&model.Host{}).Error; err != nil {
		return err
	}
	prefs, err := loadPreferred()
	if err != nil {
		return err
	}
	var groups []entity.HostGroup
	tag := []string{"easybox"}

	var cdnIDs []int
	if err := db.Model(&model.EasyboxEndpoint{}).
		Where("template IN ? AND inbound_id <> 0", cdnTemplates).Order("id").Pluck("inbound_id", &cdnIDs).Error; err != nil {
		return err
	}
	if len(prefs) > 0 && len(cdnIDs) > 0 {
		groups = append(groups, entity.HostGroup{
			GroupId: cdnHostGroupPrefix + "0", InboundIds: cdnIDs, Remark: "", Security: "same", Tags: tag,
		})
		for i, p := range prefs {
			groups = append(groups, entity.HostGroup{
				GroupId: cdnHostGroupPrefix + strconv.Itoa(i+1), InboundIds: cdnIDs, Hosts: []string{p.Address},
				Remark: preferredRemark(i, p), Security: "same", SortOrder: i + 1, Tags: tag,
			})
		}
	}

	tunnel, ok, err := tunnelLine()
	if err != nil {
		return err
	}
	if ok && tunnel.InboundId != 0 && tunnel.CdnHost != "" {
		host := tunnel.CdnHost
		viaCloudflare := func(g entity.HostGroup) entity.HostGroup {
			g.InboundIds, g.Port, g.Security = []int{tunnel.InboundId}, 443, "tls"
			g.Sni, g.HostHeader, g.Fingerprint, g.Alpn = host, host, "chrome", []string{"http/1.1"}
			g.Tags = tag
			return g
		}
		groups = append(groups, viaCloudflare(entity.HostGroup{
			GroupId: tunnelHostGroupPrefix + "0", Hosts: []string{host}, Remark: "",
		}))
		for i, p := range prefs {
			groups = append(groups, viaCloudflare(entity.HostGroup{
				GroupId: tunnelHostGroupPrefix + strconv.Itoa(i+1), Hosts: []string{p.Address},
				Remark: preferredRemark(i, p), SortOrder: i + 1,
			}))
		}
	}

	hosts := &service.HostService{}
	for i := range groups {
		if _, err := hosts.AddHostGroup(&groups[i]); err != nil {
			return err
		}
	}
	return nil
}
