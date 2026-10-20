package easybox

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
)

// EnsureDefaultSubscription returns the default subscription user, creating it
// on first use with a random email, SubID (the URL token) and UUID.
func EnsureDefaultSubscription() (Identity, error) {
	id, _, err := ensureDefaultSubscriptionRow()
	return id, err
}

func ensureDefaultSubscriptionRow() (Identity, int, error) {
	var row model.EasyboxSubscription
	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		err := tx.Where("is_default = ?", true).First(&row).Error
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		row = model.EasyboxSubscription{
			Email:     "easybox-" + random.NumLower(8),
			SubID:     random.NumLower(16),
			UUID:      uuid.NewString(),
			IsDefault: true,
		}
		return tx.Create(&row).Error
	})
	if err != nil {
		return Identity{}, 0, err
	}
	return Identity{Email: row.Email, SubID: row.SubID, UUID: row.UUID}, row.Id, nil
}
