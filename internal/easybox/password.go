package easybox

import (
	"errors"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// Subscription users created before Shadowsocks support carry a 16-character
// password, which Shadowsocks 2022 rejects. The first Shadowsocks line upgrades
// it to a key; TUIC and Trojan lines share the password, so they change too and
// devices must refresh the subscription once (the UI asks before creating).

// ensureSS2022Password returns id with a Shadowsocks 2022 key as password,
// upgrading the subscription user and every line that carries it if needed.
func (s *Service) ensureSS2022Password(id Identity) (Identity, error) {
	if isSS2022Password(id.Password) {
		return id, nil
	}
	next := newSharedPassword()
	clients := &service.ClientService{}
	rec, err := clients.GetRecordByEmail(nil, id.Email)
	switch {
	case err == nil:
		updated := rec.ToClient()
		updated.Password = next
		needRestart, err := clients.UpdateByEmail(&s.inbounds, id.Email, *updated, rec.LimitHwid)
		if needRestart {
			s.xray.SetToNeedRestart()
		}
		if err != nil {
			auditObject("subscription.password_upgrade", "subscription", 0, err)
			return id, err
		}
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return id, err
	}
	err = database.GetDB().Model(&model.EasyboxSubscription{}).
		Where("email = ?", id.Email).Update("password", next).Error
	auditObject("subscription.password_upgrade", "subscription", 0, err)
	if err != nil {
		return id, err
	}
	id.Password = next
	return id, nil
}

// passwordUpgradePending reports whether creating a Shadowsocks line would first
// change the password of existing TUIC or Trojan lines.
func passwordUpgradePending() bool {
	var row model.EasyboxSubscription
	if err := database.GetDB().Where("is_default = ?", true).First(&row).Error; err != nil {
		return false
	}
	if isSS2022Password(row.Password) {
		return false
	}
	var n int64
	err := database.GetDB().Model(&model.EasyboxEndpoint{}).
		Where("subscription_id = ? AND template IN ?", row.Id, []string{TemplateTuic, TemplateTrojan}).
		Count(&n).Error
	return err == nil && n > 0
}
