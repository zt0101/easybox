package easybox

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// RegenerateEndpoint gives a line a fresh port and REALITY keys (EasyBox.md §19-§21).
// Replace mode swaps the line only once the new inbound passed its health check;
// keepOld adds a separate "<name>-new" line and leaves the old one running.
func (s *Service) RegenerateEndpoint(id int, keepOld bool) (*EndpointView, error) {
	s.createMu.Lock()
	defer s.createMu.Unlock()
	row, err := loadEndpoint(id)
	if err != nil {
		return nil, err
	}
	if row.InboundId == 0 {
		return nil, fmt.Errorf("line %d has no inbound to regenerate", id)
	}
	old, err := s.inbounds.GetInbound(row.InboundId)
	if err != nil {
		return nil, err
	}
	address := ""
	if old.ShareAddrStrategy == "custom" {
		address = old.ShareAddr
	}
	if keepOld {
		return s.createLocked(CreateEndpointRequest{Template: row.Template, Name: row.Name + "-new", Address: address})
	}

	tpl, ok := templateByID(row.Template)
	if !ok {
		return nil, fmt.Errorf("line %d has unknown template %q", id, row.Template)
	}
	owner, err := identityOf(row.SubscriptionId)
	if err != nil {
		return nil, err
	}
	p, err := s.provision(tpl, row.Name, address, owner)
	if err != nil {
		audit("endpoint.regenerate", id, err)
		return s.view(row), err
	}
	oldInboundID := row.InboundId
	row.InboundId, row.Status, row.FailedStep, row.LastError = p.inboundID, StatusRunning, "", ""
	if err := database.GetDB().Save(row).Error; err != nil {
		return nil, err
	}
	needRestart, delErr := s.inbounds.DelInbound(oldInboundID)
	if needRestart {
		s.xray.SetToNeedRestart()
	}
	if delErr != nil {
		delErr = fmt.Errorf("new inbound is live but removing old inbound %d failed: %w", oldInboundID, delErr)
	}
	audit("endpoint.regenerate", id, delErr)
	return s.view(row), delErr
}

// RotateSubscription replaces the default subscription user's SubID (the URL)
// and every credential on every line at once: the response to a leaked URL (§39).
func (s *Service) RotateSubscription() (Identity, error) {
	db := database.GetDB()
	if _, err := EnsureDefaultSubscription(); err != nil {
		return Identity{}, err
	}
	var row model.EasyboxSubscription
	if err := db.Where("is_default = ?", true).First(&row).Error; err != nil {
		return Identity{}, err
	}
	next := Identity{
		Email: row.Email, SubID: random.NumLower(16), UUID: uuid.NewString(),
		Auth: random.NumLower(16), Password: newSharedPassword(),
	}

	clients := &service.ClientService{}
	rec, err := clients.GetRecordByEmail(nil, row.Email)
	switch {
	case err == nil:
		updated := rec.ToClient()
		updated.ID, updated.SubID = next.UUID, next.SubID
		updated.Auth, updated.Password = next.Auth, next.Password
		needRestart, err := clients.UpdateByEmail(&s.inbounds, row.Email, *updated, rec.LimitHwid)
		if needRestart {
			s.xray.SetToNeedRestart()
		}
		if err != nil {
			auditObject("subscription.rotate", "subscription", row.Id, err)
			return Identity{}, err
		}
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return Identity{}, err
	}
	row.SubID, row.UUID, row.Auth, row.Password = next.SubID, next.UUID, next.Auth, next.Password
	err = db.Save(&row).Error
	auditObject("subscription.rotate", "subscription", row.Id, err)
	return next, err
}

func identityOf(subscriptionID int) (Identity, error) {
	var row model.EasyboxSubscription
	if err := database.GetDB().First(&row, subscriptionID).Error; err != nil {
		return Identity{}, fmt.Errorf("subscription user %d: %w", subscriptionID, err)
	}
	return identityFromRow(row), nil
}
