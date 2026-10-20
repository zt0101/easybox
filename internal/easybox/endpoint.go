package easybox

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

const (
	StatusCreating = "CREATING"
	StatusRunning  = "RUNNING"
	StatusError    = "ERROR"
	StatusDisabled = "DISABLED"
	StatusDeleting = "DELETING"
)

// Steps of Quick Create (EasyBox.md §12); the UI localizes failures by step.
const (
	StepPrepare       = "prepare"
	StepCreateInbound = "create_inbound"
	StepApplyRuntime  = "apply_runtime"
	StepHealthCheck   = "health_check"
)

const templateVlessReality = "vless-reality"

type StepError struct {
	Step string
	Err  error
}

func (e *StepError) Error() string { return e.Step + ": " + e.Err.Error() }
func (e *StepError) Unwrap() error { return e.Err }

type CreateEndpointRequest struct {
	Name    string `json:"name" form:"name" example:"US-Oracle-01"`
	Address string `json:"address" form:"address" example:"node.example.com"`
}

// EndpointView is the API view of a line; port, enable state and traffic are read
// live from the upstream Inbound.
type EndpointView struct {
	Id         int    `json:"id" example:"1"`
	Name       string `json:"name" example:"US-Oracle-01"`
	Protocol   string `json:"protocol" example:"vless"`
	Template   string `json:"template" example:"vless-reality"`
	Status     string `json:"status" example:"RUNNING"`
	InboundId  int    `json:"inboundId" example:"7"`
	Port       int    `json:"port" example:"20042"`
	Address    string `json:"address" example:"node.example.com"`
	Up         int64  `json:"up" example:"1048576"`
	Down       int64  `json:"down" example:"5242880"`
	FailedStep string `json:"failedStep" example:""`
	LastError  string `json:"lastError" example:""`
	CreatedAt  int64  `json:"createdAt" example:"1792500000000"`
}

type Service struct {
	Ports PortRange
	// createMu: the port is picked before AddInbound's transaction, so two
	// concurrent creates could otherwise race for the same port.
	createMu sync.Mutex
	inbounds service.InboundService
	xray     service.XrayService
	reality  func() (RealityParams, error)
	restart  func() error
	health   func(port int) error
}

func NewService() *Service {
	s := &Service{Ports: DefaultPortRange}
	server := &service.ServerService{}
	s.reality = func() (RealityParams, error) { return GenerateRealityParams(server) }
	s.restart = func() error { return s.xray.RestartXray(false) }
	s.health = func(port int) error { return probeLocalPort(&s.xray, port, 10*time.Second) }
	return s
}

// CreateEndpoint runs Quick Create: prepare, AddInbound, apply, health check.
// Any failure after AddInbound deletes that inbound again (EasyBox.md §18).
func (s *Service) CreateEndpoint(req CreateEndpointRequest) (*EndpointView, error) {
	s.createMu.Lock()
	defer s.createMu.Unlock()
	db := database.GetDB()
	row := &model.EasyboxEndpoint{Name: strings.TrimSpace(req.Name), Template: templateVlessReality, Status: StatusCreating}
	if err := db.Create(row).Error; err != nil {
		return nil, err
	}
	fail := func(step string, err error) (*EndpointView, error) {
		row.Status, row.FailedStep, row.LastError = StatusError, step, err.Error()
		if dbErr := db.Save(row).Error; dbErr != nil {
			logger.Warning("easybox: save failed endpoint:", dbErr)
		}
		audit("endpoint.create", row.Id, err)
		return s.view(row), &StepError{Step: step, Err: err}
	}

	id, subscriptionID, err := ensureDefaultSubscriptionRow()
	if err != nil {
		return fail(StepPrepare, err)
	}
	row.SubscriptionId = subscriptionID
	reality, err := s.reality()
	if err != nil {
		return fail(StepPrepare, err)
	}
	ib, err := BuildVlessRealityInbound(row.Name, 0, id, reality)
	if err != nil {
		return fail(StepPrepare, err)
	}
	if addr := strings.TrimSpace(req.Address); addr != "" {
		ib.ShareAddrStrategy, ib.ShareAddr = "custom", addr
	}
	port, err := pickPort(ib, s.Ports, reservedPorts(), nil)
	if err != nil {
		return fail(StepPrepare, err)
	}
	if row.Name == "" {
		row.Name = "VLESS-" + strconv.Itoa(port)
		ib.Remark = row.Name
	}

	created, needRestart, err := s.inbounds.AddInbound(ib)
	if err != nil {
		return fail(StepCreateInbound, err)
	}
	row.InboundId = created.Id
	if needRestart {
		if err := s.restart(); err != nil {
			return fail(StepApplyRuntime, s.compensate(row, err))
		}
	}
	if err := s.health(port); err != nil {
		return fail(StepHealthCheck, s.compensate(row, err))
	}

	row.Status = StatusRunning
	if err := db.Save(row).Error; err != nil {
		return nil, err
	}
	audit("endpoint.create", row.Id, nil)
	return s.view(row), nil
}

// compensate removes the inbound a failed create left behind and folds a
// cleanup failure into the reported error so it is never silent.
func (s *Service) compensate(row *model.EasyboxEndpoint, cause error) error {
	needRestart, err := s.inbounds.DelInbound(row.InboundId)
	if needRestart {
		s.xray.SetToNeedRestart()
	}
	if err != nil {
		return fmt.Errorf("%w (cleanup of inbound %d also failed: %w)", cause, row.InboundId, err)
	}
	row.InboundId = 0
	return cause
}

func (s *Service) ListEndpoints() ([]*EndpointView, error) {
	var rows []*model.EasyboxEndpoint
	if err := database.GetDB().Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*EndpointView, 0, len(rows))
	for _, row := range rows {
		out = append(out, s.view(row))
	}
	return out, nil
}

func (s *Service) SetEndpointEnable(id int, enable bool) error {
	row, err := loadEndpoint(id)
	if err != nil {
		return err
	}
	if row.InboundId == 0 {
		return fmt.Errorf("line %d has no inbound to enable", id)
	}
	needRestart, err := s.inbounds.SetInboundEnable(row.InboundId, enable)
	if needRestart {
		s.xray.SetToNeedRestart()
	}
	action := "endpoint.disable"
	if enable {
		action = "endpoint.enable"
	}
	audit(action, id, err)
	if err != nil {
		return err
	}
	row.Status = StatusDisabled
	if enable {
		row.Status = StatusRunning
	}
	return database.GetDB().Save(row).Error
}

func (s *Service) DeleteEndpoint(id int) error {
	db := database.GetDB()
	row, err := loadEndpoint(id)
	if err != nil {
		return err
	}
	row.Status = StatusDeleting
	if err := db.Save(row).Error; err != nil {
		return err
	}
	if row.InboundId != 0 {
		_, getErr := s.inbounds.GetInbound(row.InboundId)
		switch {
		case getErr == nil:
			needRestart, delErr := s.inbounds.DelInbound(row.InboundId)
			if needRestart {
				s.xray.SetToNeedRestart()
			}
			if delErr != nil {
				audit("endpoint.delete", id, delErr)
				return delErr
			}
		case !errors.Is(getErr, gorm.ErrRecordNotFound):
			return getErr
		}
	}
	err = db.Delete(row).Error
	audit("endpoint.delete", id, err)
	return err
}

func (s *Service) view(row *model.EasyboxEndpoint) *EndpointView {
	ep := &EndpointView{
		Id: row.Id, Name: row.Name, Protocol: string(model.VLESS), Template: row.Template,
		Status: row.Status, InboundId: row.InboundId, FailedStep: row.FailedStep,
		LastError: row.LastError, CreatedAt: row.CreatedAt,
	}
	if row.InboundId == 0 || row.Status == StatusCreating || row.Status == StatusDeleting {
		return ep
	}
	ib, err := s.inbounds.GetInbound(row.InboundId)
	if err != nil {
		ep.Status = StatusError
		ep.LastError = fmt.Sprintf("inbound %d is missing (removed outside EasyBox?): %v", row.InboundId, err)
		return ep
	}
	ep.Port, ep.Up, ep.Down = ib.Port, ib.Up, ib.Down
	if ib.ShareAddrStrategy == "custom" {
		ep.Address = ib.ShareAddr
	}
	if !ib.Enable {
		ep.Status = StatusDisabled
	}
	return ep
}

func reservedPorts() map[int]bool {
	settings := &service.SettingService{}
	reserved := map[int]bool{80: true}
	if p, err := settings.GetPort(); err == nil {
		reserved[p] = true
	}
	if p, err := settings.GetSubPort(); err == nil {
		reserved[p] = true
	}
	return reserved
}

func loadEndpoint(id int) (*model.EasyboxEndpoint, error) {
	row := &model.EasyboxEndpoint{}
	if err := database.GetDB().First(row, id).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// probeLocalPort is the MVP-0 health check for TCP lines: Xray must be up and
// accepting on the line port, which proves the inbound is really loaded.
func probeLocalPort(xray *service.XrayService, port int, timeout time.Duration) error {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if !xray.IsXrayRunning() {
			lastErr = errors.New("xray is not running")
		} else if conn, err := dialOnce(addr); err == nil {
			_ = conn.Close()
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("port %d not accepting connections after %s: %w", port, timeout, lastErr)
}

func audit(action string, objectID int, err error) {
	entry := model.EasyboxAuditLog{
		Timestamp: time.Now().UnixMilli(), Action: action, Object: "endpoint", ObjectId: objectID, Result: "ok",
	}
	if err != nil {
		entry.Result, entry.Error = "error", err.Error()
	}
	if dbErr := database.GetDB().Create(&entry).Error; dbErr != nil {
		logger.Warning("easybox: write audit log:", dbErr)
	}
}

func dialOnce(addr string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	var d net.Dialer
	return d.DialContext(ctx, "tcp", addr)
}
