package easybox

import (
	"errors"
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
