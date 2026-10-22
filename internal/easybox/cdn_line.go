package easybox

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// CDN lines (VLESS + WebSocket + TLS behind Cloudflare): the client reaches a
// proxied hostname on the admin's own domain, so the line survives a blocked
// server IP. trycloudflare.com was tried first and is unreachable from China.

// cdnPorts are the HTTPS ports Cloudflare proxies; it connects to the origin on
// the port the client used, so the inbound must listen on one of these.
var cdnPorts = []int{443, 8443, 2053, 2083, 2087, 2096}

// cdnLabelLength is the random part of a line's hostname (cdn-<label>.<domain>).
const cdnLabelLength = 6

const cdnPathLength = 10

// CloudflareView is the API view of the Cloudflare settings; never the token.
type CloudflareView struct {
	Configured bool   `json:"configured" example:"true"`
	Domain     string `json:"domain" example:"example.com"`
	TokenHint  string `json:"tokenHint" example:"…a1b2"`
}

type CloudflareRequest struct {
	Token  string `json:"token" form:"token"`
	Domain string `json:"domain" form:"domain" example:"example.com"`
}

func loadCloudflare() (*model.EasyboxCloudflare, bool, error) {
	row := &model.EasyboxCloudflare{}
	err := database.GetDB().First(row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return row, err == nil, err
}

func cloudflareView(row *model.EasyboxCloudflare) CloudflareView {
	if row == nil {
		return CloudflareView{}
	}
	hint := ""
	if n := len(row.APIToken); n >= 8 {
		hint = "…" + row.APIToken[n-4:]
	}
	return CloudflareView{
		Configured: true, Domain: row.Domain, TokenHint: hint,
	}
}

func (s *Service) GetCloudflare() (CloudflareView, error) {
	row, _, err := loadCloudflare()
	return cloudflareView(row), err
}

// normalizeDomain accepts what admins paste: a URL, a trailing dot, any case.
func normalizeDomain(d string) string {
	d = strings.ToLower(strings.TrimSpace(d))
	d = strings.TrimPrefix(strings.TrimPrefix(d, "https://"), "http://")
	d, _, _ = strings.Cut(d, "/")
	return strings.TrimSuffix(d, ".")
}

// SetCloudflare checks that the token can see the domain before storing both.
func (s *Service) SetCloudflare(req CloudflareRequest) (CloudflareView, error) {
	token, domain := strings.TrimSpace(req.Token), normalizeDomain(req.Domain)
	if token == "" || domain == "" || !strings.Contains(domain, ".") {
		return CloudflareView{}, errors.New("enter both the Cloudflare API token and your domain, e.g. example.com")
	}
	zoneID, err := newCFClient(token).zoneID(domain)
	if err != nil {
		auditObject("cloudflare.set", "cloudflare", 0, err)
		return CloudflareView{}, err
	}
	row, ok, err := loadCloudflare()
	if err != nil {
		return CloudflareView{}, err
	}
	if !ok {
		row = &model.EasyboxCloudflare{}
	}
	row.Domain, row.ZoneID, row.APIToken = domain, zoneID, token
	err = database.GetDB().Save(row).Error
	auditObject("cloudflare.set", "cloudflare", row.Id, err)
	return cloudflareView(row), err
}

// ClearCloudflare forgets the token. Existing CDN lines keep working; deleting
// one afterwards leaves its DNS record for the admin to remove.
func (s *Service) ClearCloudflare() error {
	err := database.GetDB().Where("1 = 1").Delete(&model.EasyboxCloudflare{}).Error
	auditObject("cloudflare.clear", "cloudflare", 0, err)
	return err
}

func cloudflareAvailable() string {
	if _, ok, err := loadCloudflare(); err != nil || !ok {
		return ErrCloudflareNotConfigured.Error()
	}
	return ""
}

// cdnResources is what a CDN line owns at Cloudflare.
type cdnResources struct {
	zoneID, host, recordID, certID string
}

// prepareCDN creates the proxied DNS record and origin certificate and builds
// the inbound. On error nothing is left at Cloudflare.
func (s *Service) prepareCDN(
	build func(name string, id Identity, host, certPEM, keyPEM string) (*model.Inbound, error),
	name, address string, id Identity,
) (*model.Inbound, *cdnResources, error) {
	cfg, ok, err := loadCloudflare()
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, ErrCloudflareNotConfigured
	}
	ip, err := s.lookupPublicIPv4(address)
	if err != nil {
		return nil, nil, err
	}
	c := newCFClient(cfg.APIToken)
	if err := ensureOriginTLS(c, cfg.ZoneID); err != nil {
		return nil, nil, err
	}
	res := &cdnResources{zoneID: cfg.ZoneID, host: "cdn-" + random.NumLower(cdnLabelLength) + "." + cfg.Domain}
	if res.recordID, err = c.createProxiedRecord(cfg.ZoneID, res.host, ip); err != nil {
		return nil, nil, err
	}
	fail := func(err error) (*model.Inbound, *cdnResources, error) {
		if rerr := releaseCDN(c, res); rerr != nil {
			err = fmt.Errorf("%w (Cloudflare cleanup also failed: %w)", err, rerr)
		}
		return nil, nil, err
	}
	keyPEM, csrPEM, err := originKeyAndCSR(res.host)
	if err != nil {
		return fail(err)
	}
	var certPEM string
	if res.certID, certPEM, err = c.createOriginCert(csrPEM, res.host); err != nil {
		return fail(err)
	}
	ib, err := build(name, id, res.host, certPEM, keyPEM)
	if err != nil {
		return fail(err)
	}
	return ib, res, nil
}

// ensureOriginTLS makes Cloudflare reach the origin over TLS. "off" and
// "flexible" (plain HTTP to the origin) cannot reach a TLS inbound, so they are
// raised to "full"; "full" and "strict" both accept the Origin CA certificate
// and stay as the admin set them, since the mode applies to the whole domain.
func ensureOriginTLS(c *cfClient, zoneID string) error {
	mode, err := c.sslMode(zoneID)
	if err != nil {
		return err
	}
	if mode == "full" || mode == "strict" {
		return nil
	}
	err = c.setSSLMode(zoneID, "full")
	auditObject("cloudflare.ssl_mode_full", "cloudflare", 0, err)
	if err == nil {
		logger.Info("easybox: raised Cloudflare SSL/TLS mode from", mode, "to full for CDN lines")
	}
	return err
}

// releaseCDN removes what a CDN line owns; both deletions are attempted.
func releaseCDN(c *cfClient, res *cdnResources) error {
	var errs []error
	if res.recordID != "" {
		if err := c.deleteRecord(res.zoneID, res.recordID); err != nil {
			errs = append(errs, fmt.Errorf("DNS record %s: %w", res.host, err))
		}
	}
	if res.certID != "" {
		if err := c.revokeOriginCert(res.certID); err != nil {
			errs = append(errs, fmt.Errorf("origin certificate for %s: %w", res.host, err))
		}
	}
	return errors.Join(errs...)
}

func originKeyAndCSR(host string) (keyPEM, csrPEM string, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: host}, DNSNames: []string{host},
	}, key)
	if err != nil {
		return "", "", err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})),
		string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})), nil
}

// BuildVlessCDNInbound is VLESS + WebSocket + TLS with the line's Origin CA
// certificate (fixture inbound-full/vless-ws-tls.json). WebSocket is what every
// client can carry through Cloudflare; Vision cannot pass a CDN.
func BuildVlessCDNInbound(name string, id Identity, host, certPEM, keyPEM string) (*model.Inbound, error) {
	settings, err := json.Marshal(vlessSettings{
		Clients:    []vlessClient{{ID: id.UUID, Email: id.Email, Enable: true, SubID: id.SubID}},
		Decryption: "none",
		Encryption: "none",
		Fallbacks:  []any{},
	})
	if err != nil {
		return nil, err
	}
	stream, err := json.Marshal(map[string]any{
		"network": "ws",
		"wsSettings": map[string]any{
			"acceptProxyProtocol": false, "path": "/" + random.NumLower(cdnPathLength), "host": host,
			"headers": map[string]any{}, "heartbeatPeriod": 0,
		},
		"security": "tls",
		"tlsSettings": map[string]any{
			"serverName": host, "minVersion": "1.2", "maxVersion": "1.3", "cipherSuites": "",
			"rejectUnknownSni": false, "disableSystemRoot": false, "enableSessionResumption": false,
			"certificates": []any{map[string]any{
				"certificate": pemLines(certPEM), "key": pemLines(keyPEM),
				"oneTimeLoading": false, "usage": "encipherment", "buildChain": false,
			}},
			"alpn": []string{"http/1.1"}, "echServerKeys": "",
			"settings": map[string]any{"fingerprint": "chrome", "echConfigList": ""},
		},
	})
	if err != nil {
		return nil, err
	}
	return &model.Inbound{
		Remark: name, Enable: true, Protocol: model.VLESS,
		Settings: string(settings), StreamSettings: string(stream), Sniffing: tcpSniffing,
	}, nil
}

// pickPortFrom takes the first listed port no inbound uses and nothing else on
// the server holds (443 is often a web server's).
func pickPortFrom(ib *model.Inbound, ports []int, reserved map[int]bool, bindable func(int) bool) (int, error) {
	inbounds := &service.InboundService{}
	for _, port := range ports {
		if reserved[port] || !bindable(port) {
			continue
		}
		ib.Port = port
		conflict, err := inbounds.CheckPortConflict(ib)
		if err != nil {
			ib.Port = 0
			return 0, err
		}
		if conflict == "" {
			return port, nil
		}
	}
	ib.Port = 0
	return 0, fmt.Errorf("none of the Cloudflare HTTPS ports %v is free on this server", ports)
}

func tcpPortBindable(port int) bool {
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

var publicIPv4Services = []string{"https://api4.ipify.org", "https://ipv4.icanhazip.com", "https://4.ident.me"}

// detectPublicIPv4 is where the CDN record points: the line address when it is
// an IPv4 address, otherwise what the server looks like from outside.
func detectPublicIPv4(address string) (string, error) {
	if ip := net.ParseIP(strings.TrimSpace(address)); ip != nil && ip.To4() != nil {
		return ip.String(), nil
	}
	client := &http.Client{Timeout: 5 * time.Second}
	for _, u := range publicIPv4Services {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
		resp.Body.Close()
		if ip := net.ParseIP(strings.TrimSpace(string(body))); resp.StatusCode == http.StatusOK && ip != nil && ip.To4() != nil {
			return ip.String(), nil
		}
	}
	return "", errors.New("could not find this server's public IPv4 address: enter it as the server address")
}
