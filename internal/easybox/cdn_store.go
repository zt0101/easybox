package easybox

import (
	"fmt"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// store records a new CDN line's Cloudflare resources on its row; a nil
// receiver (any other line) clears them.
func (r *cdnResources) store(row *model.EasyboxEndpoint) {
	if r == nil {
		row.CdnZoneID, row.CdnHost, row.CdnRecordID, row.CdnCertID = "", "", "", ""
		return
	}
	row.CdnZoneID, row.CdnHost, row.CdnRecordID, row.CdnCertID = r.zoneID, r.host, r.recordID, r.certID
}

// releaseStoredCDN removes a line's Cloudflare resources with the token
// configured now.
func releaseStoredCDN(res *cdnResources) error {
	cfg, ok, err := loadCloudflare()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("Cloudflare token removed: delete the DNS record %s in the Cloudflare dashboard", res.host)
	}
	return releaseCDN(newCFClient(cfg.APIToken), res)
}

func releaseLineCDN(row *model.EasyboxEndpoint) error {
	if row.CdnRecordID == "" && row.CdnCertID == "" {
		return nil
	}
	return releaseStoredCDN(&cdnResources{
		zoneID: row.CdnZoneID, host: row.CdnHost, recordID: row.CdnRecordID, certID: row.CdnCertID,
	})
}

func (s *Service) bindable() func(int) bool {
	if s.portBindable != nil {
		return s.portBindable
	}
	return tcpPortBindable
}

func (s *Service) lookupPublicIPv4(address string) (string, error) {
	if s.publicIPv4 != nil {
		return s.publicIPv4(address)
	}
	return detectPublicIPv4(address)
}
