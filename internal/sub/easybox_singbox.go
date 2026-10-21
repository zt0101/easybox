package sub

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// sing-box output for EasyBox (EasyBox.md §25-§28, §76): the official sing-box
// apps import a complete config, so this renders one from the subscription.

const singboxMinMinor = 12 // sing-box 1.12 changed DNS and route formats incompatibly.

var (
	singboxAppUA     = regexp.MustCompile(`^(SFA|SFI|SFM|SFT)/`)
	singboxVersionUA = regexp.MustCompile(`(?i)sing-box[ /]v?(\d+)\.(\d+)`)
)

var (
	errSingboxUnsupported = errors.New("not expressible for sing-box")
	// Measured, not assumed (CLIENT_COMPATIBILITY.md): sing-box 1.14.3 fails
	// "reality verification" against Xray 26.10.10 REALITY offering X25519MLKEM768.
	errSingboxRealityMLKEM = errors.New("sing-box cannot verify REALITY servers that negotiate X25519MLKEM768")
)

// SingboxClient reports whether a User-Agent belongs to a sing-box app and
// whether its version is known to be older than 1.12.
func SingboxClient(userAgent string) (isSingbox, tooOld bool) {
	m := singboxVersionUA.FindStringSubmatch(userAgent)
	isSingbox = singboxAppUA.MatchString(userAgent) || m != nil
	if m != nil {
		major, _ := strconv.Atoi(m[1])
		minor, _ := strconv.Atoi(m[2])
		tooOld = major < 1 || (major == 1 && minor < singboxMinMinor)
	}
	return isSingbox, tooOld
}

// CanonicalNode is the client-neutral form of one subscription line
// (EasyBox.md §26-§27); renderers read only this, never inbound JSON.
type CanonicalNode struct {
	Name              string
	Protocol          string // vless | hysteria2 | tuic
	Server            string
	Port              int
	UUID              string
	Password          string
	Flow              string
	SNI               string
	ALPN              []string
	Fingerprint       string
	RealityPublicKey  string
	RealityShortID    string
	CertificatePEM    string // self-signed certificate a client must trust
	ObfsType          string
	ObfsPassword      string
	CongestionControl string
	UDPRelayMode      string
}

// canonicalFromLink reads a share link produced by this subscription service.
func canonicalFromLink(raw string) (CanonicalNode, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return CanonicalNode{}, err
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return CanonicalNode{}, fmt.Errorf("port %q: %w", u.Port(), err)
	}
	q := u.Query()
	n := CanonicalNode{Name: u.Fragment, Server: u.Hostname(), Port: port, SNI: q.Get("sni")}
	if alpn := q.Get("alpn"); alpn != "" {
		n.ALPN = strings.Split(alpn, ",")
	}
	switch u.Scheme {
	case "vless":
		if q.Get("security") != "reality" || (q.Get("type") != "" && q.Get("type") != "tcp") {
			return n, fmt.Errorf("vless %s/%s: %w", q.Get("type"), q.Get("security"), errSingboxUnsupported)
		}
		if q.Get("support-x25519mlkem768") == "true" {
			return n, errSingboxRealityMLKEM
		}
		n.Protocol, n.UUID, n.Flow = "vless", u.User.Username(), q.Get("flow")
		n.Fingerprint, n.RealityPublicKey, n.RealityShortID = q.Get("fp"), q.Get("pbk"), q.Get("sid")
	case "hysteria2", "hy2":
		n.Protocol, n.Password = "hysteria2", u.User.Username()
		if obfs := q.Get("obfs"); obfs != "" {
			if !strings.EqualFold(obfs, "salamander") {
				return n, fmt.Errorf("hysteria2 obfs %q: %w", obfs, errSingboxUnsupported)
			}
			n.ObfsType, n.ObfsPassword = "salamander", q.Get("obfs-password")
		}
	case "tuic":
		n.Protocol, n.UUID = "tuic", u.User.Username()
		n.Password, _ = u.User.Password()
		n.CongestionControl, n.UDPRelayMode = q.Get("congestion_control"), q.Get("udp_relay_mode")
	default:
		return n, fmt.Errorf("scheme %s: %w", u.Scheme, errSingboxUnsupported)
	}
	return n, nil
}

func singboxOutbound(n CanonicalNode) map[string]any {
	out := map[string]any{"type": n.Protocol, "tag": n.Name, "server": n.Server, "server_port": n.Port}
	tls := map[string]any{"enabled": true}
	if n.SNI != "" {
		tls["server_name"] = n.SNI
	}
	if len(n.ALPN) > 0 {
		tls["alpn"] = n.ALPN
	}
	switch n.Protocol {
	case "vless":
		out["uuid"] = n.UUID
		if n.Flow != "" {
			out["flow"] = n.Flow
		}
		fp := n.Fingerprint
		if fp == "" {
			fp = "chrome"
		}
		tls["utls"] = map[string]any{"enabled": true, "fingerprint": fp}
		tls["reality"] = map[string]any{"enabled": true, "public_key": n.RealityPublicKey, "short_id": n.RealityShortID}
	case "hysteria2":
		out["password"] = n.Password
		if n.ObfsType != "" {
			out["obfs"] = map[string]any{"type": n.ObfsType, "password": n.ObfsPassword}
		}
		if n.CertificatePEM != "" {
			tls["certificate"] = n.CertificatePEM
		}
	case "tuic":
		out["uuid"], out["password"] = n.UUID, n.Password
		if n.CongestionControl != "" {
			out["congestion_control"] = n.CongestionControl
		}
		if n.UDPRelayMode != "" {
			out["udp_relay_mode"] = n.UDPRelayMode
		}
	}
	out["tls"] = tls
	return out
}

// renderSingbox builds a complete sing-box >= 1.12 config: TUN, the lines behind
// a "proxy" selector with an "auto" url-test, DNS hijack and private IPs direct.
func renderSingbox(nodes []CanonicalNode) ([]byte, error) {
	seen := map[string]int{}
	tags := make([]string, 0, len(nodes))
	outbounds := make([]any, 0, len(nodes)+3)
	for _, n := range nodes {
		seen[n.Name]++
		if c := seen[n.Name]; c > 1 {
			n.Name = fmt.Sprintf("%s (%d)", n.Name, c)
		}
		tags = append(tags, n.Name)
		outbounds = append(outbounds, singboxOutbound(n))
	}
	head := []any{
		map[string]any{"type": "selector", "tag": "proxy", "outbounds": append([]string{"auto"}, tags...), "default": "auto"},
		map[string]any{"type": "urltest", "tag": "auto", "outbounds": tags},
	}
	outbounds = append(head, append(outbounds, map[string]any{"type": "direct", "tag": "direct"})...)
	cfg := map[string]any{
		"log": map[string]any{"level": "warn"},
		"dns": map[string]any{
			"servers": []any{
				map[string]any{"type": "https", "tag": "remote", "server": "1.1.1.1", "detour": "proxy"},
				map[string]any{"type": "local", "tag": "local"},
			},
			"final": "remote",
		},
		"inbounds": []any{map[string]any{
			"type": "tun", "tag": "tun-in", "address": []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"},
			"auto_route": true, "strict_route": true,
		}},
		"outbounds": outbounds,
		"route": map[string]any{
			"rules": []any{
				map[string]any{"action": "sniff"},
				map[string]any{"protocol": "dns", "action": "hijack-dns"},
				map[string]any{"ip_is_private": true, "outbound": "direct"},
			},
			"final":                   "proxy",
			"auto_detect_interface":   true,
			"default_domain_resolver": "local",
		},
	}
	return json.MarshalIndent(cfg, "", "  ")
}

// GetSingboxConfig renders subId's lines for sing-box. Lines sing-box cannot
// express are left out (capability filtering, EasyBox.md §34) and listed.
func (s *SubService) GetSingboxConfig(subId, host string) ([]byte, []string, xray.ClientTraffic, error) {
	req := s.ForRequest(host)
	links, _, _, traffic, err := req.getSubs(subId)
	if err != nil {
		return nil, nil, traffic, err
	}
	certs, err := s.selfSignedHysteriaCerts(subId)
	if err != nil {
		return nil, nil, traffic, err
	}
	var nodes []CanonicalNode
	var skipped []string
	for _, l := range links {
		n, err := canonicalFromLink(l)
		if err == nil && n.Protocol == "hysteria2" && strings.Contains(l, "pinSHA256=") {
			if n.CertificatePEM = certs[n.Port]; n.CertificatePEM == "" {
				err = fmt.Errorf("pinned certificate for port %d not found: %w", n.Port, errSingboxUnsupported)
			}
		}
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", n.Name, err))
			continue
		}
		nodes = append(nodes, n)
	}
	if len(nodes) == 0 {
		return nil, skipped, traffic, nil
	}
	body, err := renderSingbox(nodes)
	return body, skipped, traffic, err
}

// maybeServeSingbox answers sing-box apps (or ?target=singbox) with a complete
// config; a sing-box known to be older than 1.12 falls through to raw.
func (a *SUBController) maybeServeSingbox(c *gin.Context, userAgent string) bool {
	isSingbox, tooOld := SingboxClient(userAgent)
	if c.Query("target") != "singbox" && (!isSingbox || tooOld) {
		return false
	}
	subId := c.Param("subid")
	scheme, host, hostWithPort, _ := a.subService.ResolveRequest(c)
	body, skipped, traffic, err := a.subService.GetSingboxConfig(subId, host)
	for _, s := range skipped {
		logger.Info("easybox singbox: skipped line", s)
	}
	if err != nil || body == nil {
		writeSubError(c, err)
		return true
	}
	subReq := a.subService.ForRequest(host)
	metadata := a.metadataForSubRequest(func() *SubService { return subReq }, subId, builtinProfileURL(c, scheme, hostWithPort))
	a.ApplyCommonHeaders(c, subReq.subscriptionUserinfo(traffic), a.updateInterval, metadata.Title, metadata.SupportURL, metadata.ProfileURL, metadata.Announce, false, "", a.subHideSettings)
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
	return true
}

// selfSignedHysteriaCerts maps a Hysteria inbound's port to its inline
// certificate: sing-box trusts a certificate, not a pinned hash.
func (s *SubService) selfSignedHysteriaCerts(subId string) (map[int]string, error) {
	inbounds, err := s.getInboundsBySubId(subId)
	if err != nil {
		return nil, err
	}
	out := map[int]string{}
	for _, ib := range inbounds {
		if ib.Protocol != model.Hysteria {
			continue
		}
		var stream struct {
			TLSSettings struct {
				Certificates []struct {
					Certificate []string `json:"certificate"`
				} `json:"certificates"`
			} `json:"tlsSettings"`
		}
		if err := json.Unmarshal([]byte(ib.StreamSettings), &stream); err != nil {
			logger.Warning("easybox singbox: inbound", ib.Id, "stream settings:", err)
			continue
		}
		for _, c := range stream.TLSSettings.Certificates {
			if len(c.Certificate) > 0 {
				out[ib.Port] = strings.Join(c.Certificate, "\n") + "\n"
				break
			}
		}
	}
	return out, nil
}
