package easybox

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"syscall"
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

type StepError struct {
	Step string
	Err  error
}

func (e *StepError) Error() string { return e.Step + ": " + e.Err.Error() }
func (e *StepError) Unwrap() error { return e.Err }

type CreateEndpointRequest struct {
	Template string `json:"template" form:"template" example:"vless-reality"`
	Name     string `json:"name" form:"name" example:"US-Oracle-01"`
	Address  string `json:"address" form:"address" example:"node.example.com"`
}

// EndpointView is the API view of a line; port, enable state and traffic are read
// live from the upstream Inbound.
type EndpointView struct {
	Id             int    `json:"id" example:"1"`
	Name           string `json:"name" example:"US-Oracle-01"`
	Protocol       string `json:"protocol" example:"vless"`
	Transport      string `json:"transport" example:"tcp"`
	Template       string `json:"template" example:"vless-reality"`
	Status         string `json:"status" example:"RUNNING"`
	InboundId      int    `json:"inboundId" example:"7"`
	Port           int    `json:"port" example:"20042"`
	Address        string `json:"address" example:"node.example.com"`
	InSubscription bool   `json:"inSubscription" example:"true"`
	Up             int64  `json:"up" example:"1048576"`
	Down           int64  `json:"down" example:"5242880"`
	FailedStep     string `json:"failedStep" example:""`
	LastError      string `json:"lastError" example:""`
	CreatedAt      int64  `json:"createdAt" example:"1792500000000"`
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
	health   func(port int, transport string) error
}

func NewService() *Service {
	s := &Service{Ports: DefaultPortRange}
	server := &service.ServerService{}
	s.reality = func() (RealityParams, error) { return GenerateRealityParams(server) }
	s.restart = func() error { return s.xray.RestartXray(false) }
	s.health = func(port int, transport string) error {
		if transport == transportUDP {
			return probeLocalUDP(port, 10*time.Second)
		}
		return probeLocalPort(&s.xray, port, 10*time.Second)
	}
	return s
}

// CreateEndpoint runs Quick Create: prepare, AddInbound, apply, health check.
// Any failure after AddInbound deletes that inbound again (EasyBox.md §18).
func (s *Service) CreateEndpoint(req CreateEndpointRequest) (*EndpointView, error) {
	s.createMu.Lock()
	defer s.createMu.Unlock()
	return s.createLocked(req)
}

func (s *Service) createLocked(req CreateEndpointRequest) (*EndpointView, error) {
	db := database.GetDB()
	tpl, ok := templateByID(req.Template)
	if !ok {
		return nil, fmt.Errorf("unknown line template %q", req.Template)
	}
	row := &model.EasyboxEndpoint{Name: strings.TrimSpace(req.Name), Template: tpl.id, Status: StatusCreating}
	if err := db.Create(row).Error; err != nil {
		return nil, err
	}
	id, subscriptionID, err := ensureDefaultSubscriptionRow()
	var p provisioned
	if err != nil {
		err = &StepError{Step: StepPrepare, Err: err}
	} else {
		row.SubscriptionId = subscriptionID
		p, err = s.provision(tpl, row.Name, req.Address, id)
	}
	if err != nil {
		var stepErr *StepError
		errors.As(err, &stepErr)
		row.Status, row.FailedStep, row.LastError = StatusError, stepErr.Step, stepErr.Err.Error()
		if dbErr := db.Save(row).Error; dbErr != nil {
			logger.Warning("easybox: save failed endpoint:", dbErr)
		}
		audit("endpoint.create", row.Id, err)
		return s.view(row), err
	}

	row.Name, row.InboundId, row.Status = p.name, p.inboundID, StatusRunning
	if err := db.Save(row).Error; err != nil {
		return nil, err
	}
	audit("endpoint.create", row.Id, nil)
	return s.view(row), nil
}

type provisioned struct {
	inboundID int
	port      int
	name      string
}

// provision brings up one new inbound from tpl for id. On failure it leaves no
// inbound behind and returns a *StepError naming the failed step.
func (s *Service) provision(tpl lineTemplate, name, address string, id Identity) (provisioned, error) {
	fail := func(step string, err error) (provisioned, error) {
		return provisioned{}, &StepError{Step: step, Err: err}
	}
	ib, err := tpl.build(s, name, id)
	if err != nil {
		return fail(StepPrepare, err)
	}
	if addr := strings.TrimSpace(address); addr != "" {
		ib.ShareAddrStrategy, ib.ShareAddr = "custom", addr
	}
	port, err := pickPort(ib, s.Ports, reservedPorts(), nil)
	if err != nil {
		return fail(StepPrepare, err)
	}
	if name == "" {
		name = tpl.namePrefix + "-" + strconv.Itoa(port)
		ib.Remark = name
	}

	created, needRestart, err := s.inbounds.AddInbound(ib)
	if err != nil {
		return fail(StepCreateInbound, err)
	}
	if needRestart {
		if err := s.restart(); err != nil {
			return fail(StepApplyRuntime, s.compensate(created.Id, err))
		}
	}
	if err := s.health(port, tpl.transport); err != nil {
		return fail(StepHealthCheck, s.compensate(created.Id, err))
	}
	return provisioned{inboundID: created.Id, port: port, name: name}, nil
}

// compensate removes the inbound a failed provision left behind and folds a
// cleanup failure into the reported error so it is never silent.
func (s *Service) compensate(inboundID int, cause error) error {
	needRestart, err := s.inbounds.DelInbound(inboundID)
	if needRestart {
		s.xray.SetToNeedRestart()
	}
	if err != nil {
		return fmt.Errorf("%w (cleanup of inbound %d also failed: %w)", cause, inboundID, err)
	}
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
	tpl, _ := templateByID(row.Template)
	ep := &EndpointView{
		Id: row.Id, Name: row.Name, Protocol: tpl.label, Transport: tpl.transport, Template: row.Template,
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
	ep.InSubscription = !ib.ExcludeFromSub
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

// probeLocalUDP is the health check for QUIC lines (Hysteria2, TUIC): a UDP
// port cannot be "connected", but binding it fails once the server holds it.
func probeLocalUDP(port int, timeout time.Duration) error {
	addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(port))
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := (&net.ListenConfig{}).ListenPacket(context.Background(), "udp", addr)
		if errors.Is(err, syscall.EADDRINUSE) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("probe UDP port %d: %w", port, err)
		}
		_ = conn.Close()
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("nothing is listening on UDP port %d after %s", port, timeout)
}

func audit(action string, objectID int, err error) {
	auditObject(action, "endpoint", objectID, err)
}

func auditObject(action, object string, objectID int, err error) {
	entry := model.EasyboxAuditLog{
		Timestamp: time.Now().UnixMilli(), Action: action, Object: object, ObjectId: objectID, Result: "ok",
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
