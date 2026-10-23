package easybox

import (
	"errors"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/util/link"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// Only process/network boundaries are faked: xray key generation + target scan,
// Xray restart and the local port probe. DB and upstream services are real.
func newTestService(restartErr, healthErr error) *Service {
	return &Service{
		Ports:   PortRange{Min: 20000, Max: 20099},
		reality: func() (RealityParams, error) { return testReality, nil },
		restart: func() error { return restartErr },
		health:  func(int, string) error { return healthErr },
		// No network in tests: the server is in Hong Kong.
		regionLookup: func() string { return "HK" },
		// 443 is held elsewhere unless a test says otherwise, so lines land in
		// the range whether or not the test process may bind it.
		preferredFree: func(int, string) bool { return false },
	}
}

func countInbounds(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := database.GetDB().Model(&model.Inbound{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func auditResults(t *testing.T, action string) []string {
	t.Helper()
	var rows []model.EasyboxAuditLog
	if err := database.GetDB().Where("action = ?", action).Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Result)
	}
	return out
}

func subLinks(t *testing.T) []string {
	t.Helper()
	id, err := EnsureDefaultSubscription()
	if err != nil {
		t.Fatal(err)
	}
	links, _, _, _, err := sub.NewSubService("").GetSubs(id.SubID, "203.0.113.10")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	return links
}

func TestCreateEndpoint_RunningLineAppearsInSubscription(t *testing.T) {
	initTestDB(t)
	ep, err := newTestService(nil, nil).CreateEndpoint(CreateEndpointRequest{Name: "US-Oracle-01", Address: "node.example.com"})
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}
	if ep.Status != StatusRunning || ep.InboundId == 0 {
		t.Fatalf("endpoint = %+v, want RUNNING with an inbound", ep)
	}
	if ep.Port < 20000 || ep.Port > 20099 {
		t.Fatalf("port %d outside the EasyBox range", ep.Port)
	}
	links := subLinks(t)
	if len(links) != 1 {
		t.Fatalf("subscription links = %v, want 1", links)
	}
	res, err := link.ParseLink(links[0])
	if err != nil {
		t.Fatal(err)
	}
	if addr := res.Outbound["settings"].(map[string]any)["address"]; addr != "node.example.com" {
		t.Fatalf("link address = %v, want the requested server address", addr)
	}
	if got := auditResults(t, "endpoint.create"); len(got) != 1 || got[0] != "ok" {
		t.Fatalf("audit endpoint.create = %v, want [ok]", got)
	}
}

// EasyBox.md §18: a failed create leaves no inbound. apply_runtime needs a live
// Xray hot-apply failure, so the health-check path drives compensate() here.
func TestCreateEndpoint_FailuresCompensateAndNameTheStep(t *testing.T) {
	cases := []struct {
		name       string
		restartErr error
		healthErr  error
		wantStep   string
	}{
		{"health check fails", nil, errors.New("port 20042 not accepting connections"), StepHealthCheck},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			initTestDB(t)
			ep, err := newTestService(tc.restartErr, tc.healthErr).CreateEndpoint(CreateEndpointRequest{Name: "broken"})
			var stepErr *StepError
			if !errors.As(err, &stepErr) || stepErr.Step != tc.wantStep {
				t.Fatalf("error = %v, want StepError at %s", err, tc.wantStep)
			}
			if ep == nil || ep.Status != StatusError || ep.FailedStep != tc.wantStep || ep.LastError == "" {
				t.Fatalf("endpoint = %+v, want ERROR at %s with a reason", ep, tc.wantStep)
			}
			if n := countInbounds(t); n != 0 {
				t.Fatalf("inbounds after failed create = %d, want 0 (compensated)", n)
			}
			if links := subLinks(t); len(links) != 0 {
				t.Fatalf("subscription still lists a failed line: %v", links)
			}
			if got := auditResults(t, "endpoint.create"); len(got) != 1 || got[0] != "error" {
				t.Fatalf("audit endpoint.create = %v, want [error]", got)
			}
		})
	}
}

func TestCreateEndpoint_NoFreePortFailsBeforeAnyInbound(t *testing.T) {
	initTestDB(t)
	svc := newTestService(nil, nil)
	svc.Ports = PortRange{Min: 20000, Max: 20000}
	if _, err := svc.CreateEndpoint(CreateEndpointRequest{Name: "first"}); err != nil {
		t.Fatal(err)
	}
	ep, err := svc.CreateEndpoint(CreateEndpointRequest{Name: "second"})
	if !errors.Is(err, ErrNoFreePort) {
		t.Fatalf("error = %v, want ErrNoFreePort", err)
	}
	if ep.FailedStep != StepPrepare {
		t.Fatalf("failed step = %q, want %s", ep.FailedStep, StepPrepare)
	}
	if n := countInbounds(t); n != 1 {
		t.Fatalf("inbounds = %d, want 1", n)
	}
}

func TestSetEndpointEnable_DisabledLineLeavesSubscription(t *testing.T) {
	initTestDB(t)
	svc := newTestService(nil, nil)
	ep, err := svc.CreateEndpoint(CreateEndpointRequest{Name: "line"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetEndpointEnable(ep.Id, false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if links := subLinks(t); len(links) != 0 {
		t.Fatalf("disabled line still in subscription: %v", links)
	}
	list, err := svc.ListEndpoints()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Status != StatusDisabled {
		t.Fatalf("list = %+v, want one DISABLED line", list)
	}
	if err := svc.SetEndpointEnable(ep.Id, true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if links := subLinks(t); len(links) != 1 {
		t.Fatalf("re-enabled line missing from subscription: %v", links)
	}
}

func TestDeleteEndpoint_RemovesInboundAndRow(t *testing.T) {
	initTestDB(t)
	svc := newTestService(nil, nil)
	ep, err := svc.CreateEndpoint(CreateEndpointRequest{Name: "line"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteEndpoint(ep.Id); err != nil {
		t.Fatalf("DeleteEndpoint: %v", err)
	}
	if n := countInbounds(t); n != 0 {
		t.Fatalf("inbounds after delete = %d, want 0", n)
	}
	list, err := svc.ListEndpoints()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("list after delete = %+v, want empty", list)
	}
	if got := auditResults(t, "endpoint.delete"); len(got) != 1 || got[0] != "ok" {
		t.Fatalf("audit endpoint.delete = %v, want [ok]", got)
	}
}

// A line whose inbound was deleted in the Advanced pages must show as broken,
// not as RUNNING, and must still be deletable.
func TestListEndpoints_InboundRemovedOutsideEasyBox(t *testing.T) {
	initTestDB(t)
	svc := newTestService(nil, nil)
	ep, err := svc.CreateEndpoint(CreateEndpointRequest{Name: "line"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (&service.InboundService{}).DelInbound(ep.InboundId); err != nil {
		t.Fatal(err)
	}
	list, err := svc.ListEndpoints()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Status != StatusError || !strings.Contains(list[0].LastError, "inbound") {
		t.Fatalf("list = %+v, want one ERROR line mentioning the missing inbound", list)
	}
	if err := svc.DeleteEndpoint(ep.Id); err != nil {
		t.Fatalf("DeleteEndpoint of orphaned line: %v", err)
	}
}
