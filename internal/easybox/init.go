package easybox

import (
	"errors"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

const (
	initSeeder = "EasyBoxSubscriptionDefaults"
	// lineNameRemarkTemplate makes every subscription link show only the line name.
	lineNameRemarkTemplate = "{{INBOUND}}"
)

// Init applies EasyBox first-run defaults. It must run before the sub server
// starts (it reads settings once); a HistoryOfSeeders row keeps it one-time.
func Init() error {
	if _, err := EnsureDefaultSubscription(); err != nil {
		return err
	}
	db := database.GetDB()
	err := db.Where("seeder_name = ?", initSeeder).First(&model.HistoryOfSeeders{}).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	// No surrounding transaction: SettingService writes on its own handle and
	// SQLite would deadlock. Re-running after a crash yields the same settings.
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
	return db.Create(&model.HistoryOfSeeders{SeederName: initSeeder}).Error
}
