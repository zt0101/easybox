package easybox

import (
	"errors"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// getSetting reads an EasyBox setting; ok is false when it was never set.
func getSetting(key string) (value string, ok bool, err error) {
	row := model.EasyboxSetting{}
	err = database.GetDB().Where("key = ?", key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	return row.Value, err == nil, err
}

func setSetting(key, value string) error {
	return database.GetDB().Save(&model.EasyboxSetting{Key: key, Value: value}).Error
}
