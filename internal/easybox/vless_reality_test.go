package easybox

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/util/link"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

var testIdentity = Identity{
	Email: "easybox-default",
	SubID: "ebsub0001",
	UUID:  "22222222-3333-4444-9555-666666666666",
}

var testReality = RealityParams{
	PrivateKey:  "wM-2_oQRWXyLcXhV5q1ifTBcS3K8mYR3wQI3PqGFK1k",
	PublicKey:   "Tx5yj1bRcOPHkdvT2pIAQ2zh0gQ8m4OPdnzqXJxxV3o",
	Target:      "www.microsoft.com:443",
	ServerNames: []string{"www.microsoft.com"},
	ShortID:     "a1b2c3d4",
}

func initTestDB(t *testing.T) {
	t.Helper()
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
}

func addBuiltInbound(t *testing.T, name string, port int) {
	t.Helper()
	ib, err := BuildVlessRealityInbound(name, port, testIdentity, testReality)
	if err != nil {
		t.Fatalf("BuildVlessRealityInbound(%s): %v", name, err)
	}
	if _, _, err := (&service.InboundService{}).AddInbound(ib); err != nil {
		t.Fatalf("AddInbound(%s): %v", name, err)
	}
}

// Drift guard (EasyBox.md §10): the generated inbound must survive upstream
// AddInbound, the subscription renderer and upstream's own link parser.
func TestVlessRealityInbound_SubscriptionLinkParsesBack(t *testing.T) {
	initTestDB(t)
	addBuiltInbound(t, "US-Oracle-01", 20001)

	links, _, _, _, err := sub.NewSubService("").GetSubs(testIdentity.SubID, "203.0.113.10")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("GetSubs returned %d links, want 1: %v", len(links), links)
	}
	res, err := link.ParseLink(links[0])
	if err != nil {
		t.Fatalf("ParseLink(%q): %v", links[0], err)
	}

	if got := res.Outbound["protocol"]; got != "vless" {
		t.Fatalf("protocol = %v, want vless", got)
	}
	settings := res.Outbound["settings"].(map[string]any)
	wantSettings := map[string]any{
		"address": "203.0.113.10",
		"port":    20001,
		"id":      testIdentity.UUID,
		"flow":    "xtls-rprx-vision",
	}
	for k, want := range wantSettings {
		if settings[k] != want {
			t.Errorf("settings.%s = %v, want %v (link %q)", k, settings[k], want, links[0])
		}
	}
	stream := res.Outbound["streamSettings"].(map[string]any)
	if stream["security"] != "reality" {
		t.Fatalf("security = %v, want reality (link %q)", stream["security"], links[0])
	}
	reality := stream["realitySettings"].(map[string]any)
	wantReality := map[string]any{
		"publicKey":  testReality.PublicKey,
		"serverName": "www.microsoft.com",
		"shortId":    testReality.ShortID,
	}
	for k, want := range wantReality {
		if reality[k] != want {
			t.Errorf("realitySettings.%s = %v, want %v (link %q)", k, reality[k], want, links[0])
		}
	}
}

// EasyBox.md §23.1: several lines embedding one identity must share a single
// subscription user, so one URL imports every line.
func TestVlessRealityInbound_TwoLinesShareOneSubscription(t *testing.T) {
	initTestDB(t)
	addBuiltInbound(t, "US-Oracle-01", 20001)
	addBuiltInbound(t, "US-Oracle-02", 20002)

	records, err := (&service.ClientService{}).GetRecordsBySubID(testIdentity.SubID)
	if err != nil {
		t.Fatalf("GetRecordsBySubID: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("subscription users for %s = %d, want 1", testIdentity.SubID, len(records))
	}
	links, _, _, _, err := sub.NewSubService("").GetSubs(testIdentity.SubID, "203.0.113.10")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("GetSubs returned %d links, want 2: %v", len(links), links)
	}
}
