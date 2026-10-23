package easybox

import (
	"errors"
	"fmt"
	"testing"
)

func fixedOffset(off int) func(int) int { return func(int) int { return off } }

// The picker must defer to upstream's conflict check, or Quick Create would
// hand AddInbound a port it then rejects.
func TestPickPort_SkipsPortTakenByExistingInbound(t *testing.T) {
	initTestDB(t)
	addBuiltInbound(t, "existing", 20001)

	ib, err := BuildVlessRealityInbound("new", 0, testIdentity, testReality)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pickPort(ib, PortRange{Min: 20000, Max: 20099}, nil, fixedOffset(1))
	if err != nil {
		t.Fatalf("pickPort: %v", err)
	}
	if got != 20002 {
		t.Fatalf("pickPort = %d, want 20002 (20001 is taken)", got)
	}
}

func TestPickPort_SkipsReservedPanelAndSubPorts(t *testing.T) {
	initTestDB(t)
	ib, err := BuildVlessRealityInbound("new", 0, testIdentity, testReality)
	if err != nil {
		t.Fatal(err)
	}
	reserved := map[int]bool{20098: true, 20099: true}
	got, err := pickPort(ib, PortRange{Min: 20000, Max: 20099}, reserved, fixedOffset(98))
	if err != nil {
		t.Fatalf("pickPort: %v", err)
	}
	if got != 20000 {
		t.Fatalf("pickPort = %d, want 20000 (wraps past reserved 20098/20099)", got)
	}
}

func TestPickPort_ReportsExhaustedRange(t *testing.T) {
	initTestDB(t)
	addBuiltInbound(t, "a", 20000)
	addBuiltInbound(t, "b", 20001)
	ib, err := BuildVlessRealityInbound("new", 0, testIdentity, testReality)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pickPort(ib, PortRange{Min: 20000, Max: 20001}, nil, fixedOffset(0))
	if !errors.Is(err, ErrNoFreePort) {
		t.Fatalf("pickPort error = %v, want ErrNoFreePort", err)
	}
}

// The first REALITY line takes TCP 443; the next REALITY line, with TCP 443
// taken, goes to the range. Hysteria2 never asks for 443.
func TestPreferredPort_FirstRealityTakes443(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	svc := newTestService(nil, nil)
	svc.preferredFree = func(int, string) bool { return true }
	if ep := createLine(t, svc, TemplateVlessReality, "reality"); ep.Port != 443 {
		t.Fatalf("REALITY port = %d, want 443", ep.Port)
	}
	if ep := createLine(t, svc, TemplateVlessXhttp, "xhttp"); ep.Port < 20000 || ep.Port > 20099 {
		t.Fatalf("second REALITY port = %d, want the range once 443 is taken", ep.Port)
	}
	if ep := createLine(t, svc, TemplateHysteria2, "hy2"); ep.Port < 20000 || ep.Port > 20099 {
		t.Fatalf("Hysteria2 port = %d, want the range (UDP 443 is often blocked)", ep.Port)
	}
}

// 443 held by another program (a web server) leaves the line in the range.
func TestPreferredPort_HeldElsewhereUsesRange(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	svc := newTestService(nil, nil)
	var asked []string
	svc.preferredFree = func(port int, transport string) bool {
		asked = append(asked, fmt.Sprintf("%s/%d", transport, port))
		return false
	}
	if ep := createLine(t, svc, TemplateVlessReality, "reality"); ep.Port < 20000 || ep.Port > 20099 {
		t.Fatalf("port = %d, want the range", ep.Port)
	}
	if len(asked) != 1 || asked[0] != "tcp/443" {
		t.Fatalf("checked %v, want tcp/443", asked)
	}
}

// Regenerating the line on 443 brings the new inbound back to 443 once the old
// one is gone; if 443 then fails its health check, the line keeps its range port.
func TestRegenerate_KeepsPreferredPort(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	svc := newTestService(nil, nil)
	svc.preferredFree = func(int, string) bool { return true }
	ep := createLine(t, svc, TemplateVlessReality, "reality")
	if ep.Port != 443 {
		t.Fatalf("port = %d, want 443", ep.Port)
	}
	got, err := svc.RegenerateEndpoint(ep.Id, false)
	if err != nil || got.Port != 443 {
		t.Fatalf("regenerated port = %d (%v), want back on 443", got.Port, err)
	}

	svc.health = func(port int, _ string) error {
		if port == 443 {
			return errors.New("443 not accepting")
		}
		return nil
	}
	got, err = svc.RegenerateEndpoint(ep.Id, false)
	if err != nil || got.Port < 20000 || got.Port > 20099 || got.Status != StatusRunning {
		t.Fatalf("line = %+v (%v), want RUNNING on its range port when 443 fails", got, err)
	}
}
