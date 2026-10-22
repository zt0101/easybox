package easybox

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// Subscription through Cloudflare: a proxied sub-<label>.<domain> record in
// front of the sub server, so devices can still refresh the subscription once
// the server IP is blocked (and pick up a new tunnel hostname). The IP address
// keeps working; Cloudflare reaches the sub server on its own port with the
// certificate it already has, which SSL/TLS mode "full" accepts.

const (
	subCDNHostKey   = "subCdnHost"
	subCDNRecordKey = "subCdnRecordId"
	subCDNZoneKey   = "subCdnZoneId"
	// subCDNOriginKey is the server IP the record points at; requests through the
	// hostname get it back as their default node address.
	subCDNOriginKey = "subCdnOrigin"
)

var (
	errSubCDNStrict = errors.New(`Cloudflare SSL/TLS mode is "Full (strict)": the subscription server's certificate is for its IP, so Cloudflare would reject it; switch the domain to "Full" to serve the subscription through Cloudflare`)
	errSubCDNNoTLS  = errors.New("the subscription server has no TLS certificate: Cloudflare can only reach it over HTTPS")
)

// subCDNUnavailable says why the subscription cannot go through Cloudflare
// ("" = it can).
func subCDNUnavailable() string {
	if _, ok, err := loadCloudflare(); err != nil || !ok {
		return ErrCloudflareNotConfigured.Error()
	}
	settings := &service.SettingService{}
	port, err := settings.GetSubPort()
	if err != nil {
		return err.Error()
	}
	if !slices.Contains(cdnPorts, port) {
		return fmt.Sprintf("the subscription port %d is not one Cloudflare proxies (%v)", port, cdnPorts)
	}
	if cert, err := settings.GetSubCertFile(); err != nil || strings.TrimSpace(cert) == "" {
		return errSubCDNNoTLS.Error()
	}
	return ""
}

// SetSubscriptionCDN turns the Cloudflare subscription address on or off.
func (s *Service) SetSubscriptionCDN(enable bool) error {
	host, _, err := getSetting(subCDNHostKey)
	if err != nil {
		return err
	}
	if enable == (host != "") {
		return nil
	}
	if !enable {
		err := s.disableSubCDN(host)
		auditObject("subscription.cdn_off", "subscription", 0, err)
		return err
	}
	err = s.enableSubCDN()
	auditObject("subscription.cdn_on", "subscription", 0, err)
	return err
}

func (s *Service) enableSubCDN() error {
	if reason := subCDNUnavailable(); reason != "" {
		return errors.New(reason)
	}
	cfg, _, err := loadCloudflare()
	if err != nil {
		return err
	}
	c := newCFClient(cfg.APIToken)
	mode, err := c.sslMode(cfg.ZoneID)
	if err != nil {
		return err
	}
	if mode == "strict" {
		return errSubCDNStrict
	}
	if err := ensureOriginTLS(c, cfg.ZoneID); err != nil {
		return err
	}
	ip, err := s.lookupPublicIPv4("")
	if err != nil {
		return err
	}
	host := "sub-" + random.NumLower(cdnLabelLength) + "." + cfg.Domain
	recordID, err := c.createProxiedRecord(cfg.ZoneID, host, ip)
	if err != nil {
		return err
	}
	for k, v := range map[string]string{subCDNRecordKey: recordID, subCDNZoneKey: cfg.ZoneID, subCDNOriginKey: ip, subCDNHostKey: host} {
		if err := setSetting(k, v); err != nil {
			return err
		}
	}
	return nil
}

// disableSubCDN forgets the address even when the record cannot be removed, and
// names the record so the admin can delete it by hand.
func (s *Service) disableSubCDN(host string) error {
	recordID, _, _ := getSetting(subCDNRecordKey)
	zoneID, _, _ := getSetting(subCDNZoneKey)
	var cleanup error
	if recordID != "" {
		cleanup = releaseStoredCDN(&cdnResources{zoneID: zoneID, host: host, recordID: recordID})
	}
	for _, k := range []string{subCDNHostKey, subCDNRecordKey, subCDNZoneKey, subCDNOriginKey} {
		if err := setSetting(k, ""); err != nil {
			return err
		}
	}
	if cleanup != nil {
		return fmt.Errorf("Cloudflare subscription address turned off, but its DNS record was not removed: %w", cleanup)
	}
	return nil
}

// subCDNBase is https://<host>[:port] for the Cloudflare subscription address,
// or "" when it is off.
func subCDNBase(settings *service.SettingService) (string, error) {
	host, _, err := getSetting(subCDNHostKey)
	if err != nil || host == "" {
		return "", err
	}
	port, err := settings.GetSubPort()
	if err != nil {
		return "", err
	}
	if port == 443 {
		return "https://" + host, nil
	}
	return "https://" + host + ":" + strconv.Itoa(port), nil
}

// subCDNRequestHost maps a request that came in on the Cloudflare subscription
// hostname to the server IP, for the default node address (sub.SubRequestHostOverride).
func subCDNRequestHost(host string) string {
	cdnHost, _, err := getSetting(subCDNHostKey)
	if err != nil || cdnHost == "" || !strings.EqualFold(host, cdnHost) {
		return host
	}
	if origin, _, err := getSetting(subCDNOriginKey); err == nil && origin != "" {
		return origin
	}
	return host
}
