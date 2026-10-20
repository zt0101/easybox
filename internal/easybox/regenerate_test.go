package easybox

import (
	"errors"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/util/link"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func onlyLinkPort(t *testing.T, subID string) (int, string) {
	t.Helper()
	links, _, _, _, err := sub.NewSubService("").GetSubs(subID, "203.0.113.10")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("subscription links = %v, want exactly 1", links)
	}
	res, err := link.ParseLink(links[0])
	if err != nil {
		t.Fatal(err)
	}
	settings := res.Outbound["settings"].(map[string]any)
	return settings["port"].(int), settings["id"].(string)
}

// EasyBox.md §20: replace mode swaps the line onto a fresh inbound (new port and
// REALITY keys) and removes the old one, keeping the line itself and its URL.
func TestRegenerateEndpoint_ReplaceSwapsInbound(t *testing.T) {
	initTestDB(t)
	svc := newTestService(nil, nil)
	ep, err := svc.CreateEndpoint(CreateEndpointRequest{Name: "line", Address: "node.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.RegenerateEndpoint(ep.Id, false)
	if err != nil {
		t.Fatalf("RegenerateEndpoint: %v", err)
	}
	if got.Id != ep.Id || got.Name != "line" || got.Status != StatusRunning {
		t.Fatalf("regenerated line = %+v, want the same RUNNING line", got)
	}
	if got.InboundId == ep.InboundId || got.Port == ep.Port {
		t.Fatalf("inbound %d port %d unchanged, want a fresh inbound", got.InboundId, got.Port)
	}
	if got.Address != "node.example.com" {
		t.Fatalf("address = %q, want the line's server address kept", got.Address)
	}
	if _, err := (&service.InboundService{}).GetInbound(ep.InboundId); err == nil {
		t.Fatal("old inbound still exists after replace")
	}
	id, _ := EnsureDefaultSubscription()
	if port, _ := onlyLinkPort(t, id.SubID); port != got.Port {
		t.Fatalf("subscription port = %d, want the new %d", port, got.Port)
	}
}

// The old line must survive a regenerate whose new inbound fails its check.
func TestRegenerateEndpoint_FailureKeepsOldLine(t *testing.T) {
	initTestDB(t)
	svc := newTestService(nil, nil)
	ep, err := svc.CreateEndpoint(CreateEndpointRequest{Name: "line"})
	if err != nil {
		t.Fatal(err)
	}
	svc.health = func(int) error { return errors.New("port not accepting connections") }
	_, err = svc.RegenerateEndpoint(ep.Id, false)
	var stepErr *StepError
	if !errors.As(err, &stepErr) || stepErr.Step != StepHealthCheck {
		t.Fatalf("error = %v, want StepError at %s", err, StepHealthCheck)
	}
	if n := countInbounds(t); n != 1 {
		t.Fatalf("inbounds = %d, want only the old one", n)
	}
	id, _ := EnsureDefaultSubscription()
	if port, _ := onlyLinkPort(t, id.SubID); port != ep.Port {
		t.Fatalf("subscription port = %d, want the old %d", port, ep.Port)
	}
	list, _ := svc.ListEndpoints()
	if len(list) != 1 || list[0].Status != StatusRunning || list[0].InboundId != ep.InboundId {
		t.Fatalf("list = %+v, want the old line unchanged and RUNNING", list)
	}
}

func TestRegenerateEndpoint_KeepOldAddsSecondLine(t *testing.T) {
	initTestDB(t)
	svc := newTestService(nil, nil)
	ep, err := svc.CreateEndpoint(CreateEndpointRequest{Name: "line"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.RegenerateEndpoint(ep.Id, true)
	if err != nil {
		t.Fatalf("RegenerateEndpoint: %v", err)
	}
	if got.Id == ep.Id || got.Name != "line-new" {
		t.Fatalf("new line = %+v, want a separate line named line-new", got)
	}
	if n := countInbounds(t); n != 2 {
		t.Fatalf("inbounds = %d, want old and new", n)
	}
}

// EasyBox.md §39: after a leak, rotation retires the old URL and the old
// credentials together; every line keeps working under the new ones.
func TestRotateSubscription_RetiresOldURLAndUUID(t *testing.T) {
	initTestDB(t)
	svc := newTestService(nil, nil)
	if _, err := svc.CreateEndpoint(CreateEndpointRequest{Name: "line"}); err != nil {
		t.Fatal(err)
	}
	before, _ := EnsureDefaultSubscription()

	after, err := svc.RotateSubscription()
	if err != nil {
		t.Fatalf("RotateSubscription: %v", err)
	}
	if after.SubID == before.SubID || after.UUID == before.UUID || after.Email != before.Email {
		t.Fatalf("after = %+v, before = %+v: want new SubID and UUID, same user", after, before)
	}
	if again, _ := EnsureDefaultSubscription(); again != after {
		t.Fatalf("EnsureDefaultSubscription = %+v, want the rotated identity", again)
	}
	oldLinks, _, _, _, err := sub.NewSubService("").GetSubs(before.SubID, "203.0.113.10")
	if err != nil {
		t.Fatal(err)
	}
	if len(oldLinks) != 0 {
		t.Fatalf("old SubID still serves %v", oldLinks)
	}
	if _, uuid := onlyLinkPort(t, after.SubID); uuid != after.UUID {
		t.Fatalf("link UUID = %s, want the rotated %s", uuid, after.UUID)
	}
}

func TestRotateSubscription_WithoutLines(t *testing.T) {
	initTestDB(t)
	before, _ := EnsureDefaultSubscription()
	after, err := newTestService(nil, nil).RotateSubscription()
	if err != nil {
		t.Fatalf("RotateSubscription: %v", err)
	}
	if after.SubID == before.SubID {
		t.Fatal("SubID unchanged")
	}
}
