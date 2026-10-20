package easybox

import (
	"errors"
	"regexp"
	"slices"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// EasyBox.md §89.1: take upstream's ranked scan as-is and use the first result
// upstream itself calls usable; never re-rank.
func TestChooseRealityTarget_FirstUsableInUpstreamOrder(t *testing.T) {
	results := []*service.RealityScanResult{
		{Target: "down.example:443", Host: "down.example", Feasible: false, Reason: "timeout"},
		{Target: "lan.example:443", Host: "lan.example", Feasible: true, PrivateTarget: true},
		{Target: "www.microsoft.com:443", Host: "www.microsoft.com", Feasible: true, ServerNames: []string{"www.microsoft.com", "microsoft.com"}},
		{Target: "www.amd.com:443", Host: "www.amd.com", Feasible: true, ServerNames: []string{"www.amd.com"}},
	}
	target, names, err := chooseRealityTarget(results)
	if err != nil {
		t.Fatalf("chooseRealityTarget: %v", err)
	}
	if target != "www.microsoft.com:443" {
		t.Fatalf("target = %q, want www.microsoft.com:443", target)
	}
	if !slices.Equal(names, []string{"www.microsoft.com", "microsoft.com"}) {
		t.Fatalf("serverNames = %v", names)
	}
}

func TestChooseRealityTarget_FallsBackToHostWithoutSANs(t *testing.T) {
	results := []*service.RealityScanResult{{Target: "www.amd.com:443", Host: "www.amd.com", Feasible: true}}
	_, names, err := chooseRealityTarget(results)
	if err != nil {
		t.Fatalf("chooseRealityTarget: %v", err)
	}
	if !slices.Equal(names, []string{"www.amd.com"}) {
		t.Fatalf("serverNames = %v, want [www.amd.com]", names)
	}
}

func TestChooseRealityTarget_NoUsableTarget(t *testing.T) {
	results := []*service.RealityScanResult{{Target: "down.example:443", Feasible: false, Reason: "timeout"}}
	_, _, err := chooseRealityTarget(results)
	if !errors.Is(err, ErrNoRealityTarget) {
		t.Fatalf("error = %v, want ErrNoRealityTarget", err)
	}
}

// Mirrors upstream RandomUtil.randomShortIds: Xray needs even-length hex <= 16.
func TestNewShortIDs_MatchUpstreamShape(t *testing.T) {
	ids, err := newShortIDs()
	if err != nil {
		t.Fatal(err)
	}
	hex := regexp.MustCompile(`^[0-9a-f]+$`)
	var lengths []int
	for _, id := range ids {
		if !hex.MatchString(id) {
			t.Fatalf("short id %q is not lowercase hex", id)
		}
		lengths = append(lengths, len(id))
	}
	slices.Sort(lengths)
	if want := []int{2, 4, 6, 8, 10, 12, 14, 16}; !slices.Equal(lengths, want) {
		t.Fatalf("short id lengths = %v, want %v", lengths, want)
	}
}
