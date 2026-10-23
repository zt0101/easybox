package easybox

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func feasible(host string, ms int) *service.RealityScanResult {
	return &service.RealityScanResult{Target: host + ":443", Host: host, Feasible: true, LatencyMs: ms, ServerNames: []string{host}}
}

// poolOf returns every target chooseRealityTarget can pick for these results.
func poolOf(t *testing.T, results []*service.RealityScanResult, used map[string]bool) []string {
	t.Helper()
	saved := pickIndex
	t.Cleanup(func() { pickIndex = saved })
	var size int
	pickIndex = func(n int) int { size = n; return 0 }
	if _, _, err := chooseRealityTarget(results, used); err != nil {
		t.Fatalf("chooseRealityTarget: %v", err)
	}
	var out []string
	for i := 0; i < size; i++ {
		pickIndex = func(int) int { return i }
		target, _, _ := chooseRealityTarget(results, used)
		out = append(out, target)
	}
	return out
}

// The pick is random among the fastest usable targets: unusable, private and
// far ones never join the pool.
func TestChooseRealityTarget_PoolIsFastestUsable(t *testing.T) {
	results := []*service.RealityScanResult{
		feasible("a.example", 8),
		feasible("b.example", 20),
		{Target: "lan.example:443", Host: "lan.example", Feasible: true, PrivateTarget: true, LatencyMs: 1},
		feasible("c.example", 36),
		feasible("far.example", 37), // > 2*8+20
		{Target: "down.example:443", Feasible: false, Reason: "timeout"},
	}
	got := poolOf(t, results, nil)
	if want := []string{"a.example:443", "b.example:443", "c.example:443"}; !slices.Equal(got, want) {
		t.Fatalf("pool = %v, want %v", got, want)
	}
}

func TestChooseRealityTarget_PoolCapped(t *testing.T) {
	var results []*service.RealityScanResult
	for i := range 12 {
		results = append(results, feasible(fmt.Sprintf("h%d.example", i), 5))
	}
	if got := poolOf(t, results, nil); len(got) != realityPoolMax {
		t.Fatalf("pool size = %d, want %d", len(got), realityPoolMax)
	}
}

// A target another inbound already uses is skipped while an unused one is in
// the pool, and reused only when every pooled target is taken.
func TestChooseRealityTarget_PrefersUnusedTarget(t *testing.T) {
	results := []*service.RealityScanResult{feasible("a.example", 3), feasible("b.example", 4), feasible("c.example", 5)}
	got := poolOf(t, results, map[string]bool{"a.example:443": true, "c.example:443": true})
	if !slices.Equal(got, []string{"b.example:443"}) {
		t.Fatalf("pool = %v, want only the unused target", got)
	}
	all := map[string]bool{"a.example:443": true, "b.example:443": true, "c.example:443": true}
	if got := poolOf(t, results, all); len(got) != 3 {
		t.Fatalf("pool with every target used = %v, want all three", got)
	}
}

// serverNames is only the probed host, never the certificate's other names.
func TestChooseRealityTarget_ServerNameIsProbedHost(t *testing.T) {
	results := []*service.RealityScanResult{{
		Target: "www.apple.com:443", Host: "www.apple.com", Feasible: true,
		ServerNames: []string{"www.apple.com.cn", "www.apple.com"},
	}}
	target, names, err := chooseRealityTarget(results, nil)
	if err != nil {
		t.Fatalf("chooseRealityTarget: %v", err)
	}
	if target != "www.apple.com:443" || !slices.Equal(names, []string{"www.apple.com"}) {
		t.Fatalf("target %q serverNames %v, want www.apple.com only", target, names)
	}
}

func TestChooseRealityTarget_NoUsableTarget(t *testing.T) {
	results := []*service.RealityScanResult{{Target: "down.example:443", Feasible: false, Reason: "timeout"}}
	_, _, err := chooseRealityTarget(results, nil)
	if !errors.Is(err, ErrNoRealityTarget) {
		t.Fatalf("error = %v, want ErrNoRealityTarget", err)
	}
}

func TestUsedRealityTargets_ReadsInbounds(t *testing.T) {
	initTestDB(t)
	for i, stream := range []string{
		`{"security":"reality","realitySettings":{"target":"www.ikea.com:443"}}`,
		`{"security":"reality","realitySettings":{"dest":"www.amd.com:443"}}`,
		`{"security":"tls","tlsSettings":{"serverName":"x"}}`,
	} {
		ib := &model.Inbound{Tag: fmt.Sprintf("t%d", i), Port: 30000 + i, Protocol: model.VLESS, StreamSettings: stream}
		if err := database.GetDB().Create(ib).Error; err != nil {
			t.Fatal(err)
		}
	}
	used := usedRealityTargets()
	if len(used) != 2 || !used["www.ikea.com:443"] || !used["www.amd.com:443"] {
		t.Fatalf("used = %v", used)
	}
}

// The shipped list: no Google-owned names, no duplicates, every entry host:443.
func TestDefaultRealityCandidates(t *testing.T) {
	hosts := strings.Split(service.DefaultRealityScanCandidatesCSV, ",")
	if len(hosts) != 50 {
		t.Fatalf("%d candidates, want 50", len(hosts))
	}
	seen := map[string]bool{}
	for _, h := range hosts {
		if !strings.HasSuffix(h, ":443") || seen[h] || strings.Contains(h, "google") {
			t.Fatalf("bad or duplicate candidate %q", h)
		}
		seen[h] = true
	}
}

// A stored copy of upstream's old list moves to the shipped one; an admin's
// own list stays.
func TestInit_ReplacesOldRealityCandidatesOnly(t *testing.T) {
	settings := &service.SettingService{}
	initTestDB(t)
	if err := settings.SetRealityScanCandidates(oldRealityScanCandidates); err != nil {
		t.Fatal(err)
	}
	mustInit(t)
	if got, _ := settings.GetRealityScanCandidates(); got != service.DefaultRealityScanCandidatesCSV {
		t.Fatalf("candidates = %q, want the shipped list", got)
	}

	initTestDB(t)
	if err := settings.SetRealityScanCandidates("www.example.com:443"); err != nil {
		t.Fatal(err)
	}
	mustInit(t)
	if got, _ := settings.GetRealityScanCandidates(); got != "www.example.com:443" {
		t.Fatalf("custom candidates changed to %q", got)
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
