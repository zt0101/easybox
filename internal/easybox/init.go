package easybox

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

const (
	initSeeder       = "EasyBoxSubscriptionDefaults"
	subTLSSeeder     = "EasyBoxSubTLSFromPanel"
	hostRemarkSeeder = "EasyBoxRemarkHostToken"
	preferredSeeder  = "EasyBoxPreferredStandalone"
	// cdnXhttpModeSeeder moves eb.12-eb.14 CDN XHTTP lines from mode auto to
	// packet-up (cdn_xhttp.go).
	cdnXhttpModeSeeder = "EasyBoxCdnXhttpPacketUp"
	// lineNameRemarkTemplate makes every subscription link show only the line
	// name, plus a Host's own suffix (a CDN preferred address: "<line>-优选1").
	lineNameRemarkTemplate = "{{INBOUND}}{{HOST}}"
	// oldLineNameRemarkTemplate is what EasyBox set before Host suffixes.
	oldLineNameRemarkTemplate = "{{INBOUND}}"
)

// Init applies EasyBox first-run defaults. It must run before the sub server
// starts (it reads settings once); HistoryOfSeeders rows keep each one-time.
func Init() error {
	sub.SubRequestHostOverride = subCDNRequestHost
	if _, err := EnsureDefaultSubscription(); err != nil {
		return err
	}
	if err := applySubscriptionDefaults(); err != nil {
		return err
	}
	if err := applyHostRemarkToken(); err != nil {
		return err
	}
	if err := movePreferredToSettings(); err != nil {
		return err
	}
	if err := moveCDNXhttpToPacketUp(); err != nil {
		return err
	}
	return applySubTLSFromPanel()
}

// applyHostRemarkToken moves panels still on EasyBox's old remark template to
// the one that names preferred-address nodes; a template the admin changed
// stays as it is.
func applyHostRemarkToken() error {
	if done, err := seeded(hostRemarkSeeder); err != nil || done {
		return err
	}
	settings := &service.SettingService{}
	tmpl, err := settings.GetRemarkTemplate()
	if err != nil {
		return err
	}
	if tmpl == oldLineNameRemarkTemplate {
		if err := settings.SetRemarkTemplate(lineNameRemarkTemplate); err != nil {
			return err
		}
	}
	return markSeeded(hostRemarkSeeder)
}

// No surrounding transactions here: SettingService writes on its own handle and
// SQLite would deadlock. Re-running after a crash yields the same settings.
func applySubscriptionDefaults() error {
	if done, err := seeded(initSeeder); err != nil || done {
		return err
	}
	settings := &service.SettingService{}
	tmpl, err := settings.GetRemarkTemplate()
	if err != nil {
		return err
	}
	if tmpl == service.DefaultRemarkTemplate {
		if err := settings.SetRemarkTemplate(lineNameRemarkTemplate); err != nil {
			return err
		}
	}
	if err := settings.SetSubClashEnable(true); err != nil {
		return err
	}
	return markSeeded(initSeeder)
}

// applySubTLSFromPanel serves the subscription over HTTPS with the panel's
// certificate (EasyBox.md §9.1). It waits for one to exist, then runs once.
func applySubTLSFromPanel() error {
	if done, err := seeded(subTLSSeeder); err != nil || done {
		return err
	}
	settings := &service.SettingService{}
	certFile, err := settings.GetCertFile()
	if err != nil {
		return err
	}
	keyFile, err := settings.GetKeyFile()
	if err != nil {
		return err
	}
	if certFile == "" || keyFile == "" {
		return nil
	}
	subCert, err := settings.GetSubCertFile()
	if err != nil {
		return err
	}
	subKey, err := settings.GetSubKeyFile()
	if err != nil {
		return err
	}
	if subCert == "" && subKey == "" {
		if err := settings.SetSubCertFile(certFile); err != nil {
			return err
		}
		if err := settings.SetSubKeyFile(keyFile); err != nil {
			return err
		}
	}
	return markSeeded(subTLSSeeder)
}

func seeded(name string) (bool, error) {
	err := database.GetDB().Where("seeder_name = ?", name).First(&model.HistoryOfSeeders{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}

func markSeeded(name string) error {
	return database.GetDB().Create(&model.HistoryOfSeeders{SeederName: name}).Error
}

// movePreferredToSettings carries eb.10-eb.12 preferred addresses out of the
// Cloudflare row, so they survive disconnecting Cloudflare and serve lines
// without a domain.
func movePreferredToSettings() error {
	if done, err := seeded(preferredSeeder); err != nil || done {
		return err
	}
	row, ok, err := loadCloudflare()
	if err != nil {
		return err
	}
	if ok && strings.TrimSpace(row.PreferredAddresses) != "" {
		if _, set, err := getSetting(preferredSettingKey); err != nil {
			return err
		} else if !set {
			raw, err := json.Marshal(decodePreferred(row.PreferredAddresses))
			if err != nil {
				return err
			}
			if err := setSetting(preferredSettingKey, string(raw)); err != nil {
				return err
			}
		}
		row.PreferredAddresses = ""
		if err := database.GetDB().Save(row).Error; err != nil {
			return err
		}
	}
	return markSeeded(preferredSeeder)
}

// moveCDNXhttpToPacketUp rewrites the stored inbound of each CDN XHTTP line;
// Init runs before Xray starts, so the new mode is what Xray loads.
func moveCDNXhttpToPacketUp() error {
	if done, err := seeded(cdnXhttpModeSeeder); err != nil || done {
		return err
	}
	db := database.GetDB()
	var ids []int
	if err := db.Model(&model.EasyboxEndpoint{}).
		Where("template = ? AND inbound_id <> 0", TemplateVlessXhttpCDN).Pluck("inbound_id", &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		var in model.Inbound
		if err := db.First(&in, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
		var stream map[string]any
		if err := json.Unmarshal([]byte(in.StreamSettings), &stream); err != nil {
			return fmt.Errorf("inbound %d stream settings: %w", id, err)
		}
		xh, _ := stream["xhttpSettings"].(map[string]any)
		if xh == nil || xh["mode"] == xhttpCDNMode {
			continue
		}
		xh["mode"] = xhttpCDNMode
		raw, err := json.Marshal(stream)
		if err != nil {
			return err
		}
		if err := db.Model(&model.Inbound{}).Where("id = ?", id).Update("stream_settings", string(raw)).Error; err != nil {
			return err
		}
	}
	return markSeeded(cdnXhttpModeSeeder)
}
