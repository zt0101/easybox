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
// on first use with a random email, SubID (the URL token) and credentials.
func EnsureDefaultSubscription() (Identity, error) {
	id, _, err := ensureDefaultSubscriptionRow()
	return id, err
}

func ensureDefaultSubscriptionRow() (Identity, int, error) {
	var row model.EasyboxSubscription
	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		err := tx.Where("is_default = ?", true).First(&row).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			row = model.EasyboxSubscription{
				Email:     "easybox-" + random.NumLower(8),
				SubID:     random.NumLower(16),
				UUID:      uuid.NewString(),
				Auth:      random.NumLower(16),
				Password:  random.NumLower(16),
				IsDefault: true,
			}
			return tx.Create(&row).Error
		case err != nil:
			return err
		case row.Auth == "" || row.Password == "":
			// Rows from before Hysteria2/TUIC support carry no Auth / Password yet.
			if row.Auth == "" {
				row.Auth = random.NumLower(16)
			}
			if row.Password == "" {
				row.Password = random.NumLower(16)
			}
			return tx.Save(&row).Error
		}
		return nil
	})
	if err != nil {
		return Identity{}, 0, err
	}
	return identityFromRow(row), row.Id, nil
}

func identityFromRow(row model.EasyboxSubscription) Identity {
	return Identity{Email: row.Email, SubID: row.SubID, UUID: row.UUID, Auth: row.Auth, Password: row.Password}
}
