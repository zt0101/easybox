package easybox

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/util/link"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func mustInit(t *testing.T) {
	t.Helper()
	if err := Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
}

// subLinkNames renders the subscription with the remark template the sub server
// would load from settings, and returns each link's display name.
func subLinkNames(t *testing.T) []string {
	t.Helper()
	tmpl, err := (&service.SettingService{}).GetRemarkTemplate()
	if err != nil {
		t.Fatal(err)
	}
	id, err := EnsureDefaultSubscription()
	if err != nil {
		t.Fatal(err)
	}
	links, _, _, _, err := sub.NewSubService(tmpl).GetSubs(id.SubID, "203.0.113.10")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	names := make([]string, 0, len(links))
	for _, l := range links {
		res, err := link.ParseLink(l)
		if err != nil {
			t.Fatalf("ParseLink(%q): %v", l, err)
		}
		names = append(names, res.Outbound["tag"].(string))
	}
	return names
}

// EasyBox.md §23.1: clients show the line name, never the subscription
// user's email that upstream's default template appends.
func TestInit_LinkNameIsLineName(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	if _, err := newTestService(nil, nil).CreateEndpoint(CreateEndpointRequest{Name: "US-Oracle-01"}); err != nil {
		t.Fatal(err)
	}
	names := subLinkNames(t)
	if len(names) != 1 || names[0] != "US-Oracle-01" {
		t.Fatalf("link names = %q, want [US-Oracle-01]", names)
	}
}

func TestInit_EnablesClashSubscription(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	on, err := (&service.SettingService{}).GetSubClashEnable()
	if err != nil {
		t.Fatal(err)
	}
	if !on {
		t.Fatal("subClashEnable = false after Init, want true (Mihomo clients need it)")
	}
}

func TestInit_KeepsAdminRemarkTemplate(t *testing.T) {
	initTestDB(t)
	settings := &service.SettingService{}
	if err := settings.SetRemarkTemplate("🌐{{INBOUND}}"); err != nil {
		t.Fatal(err)
	}
	mustInit(t)
	got, err := settings.GetRemarkTemplate()
	if err != nil {
		t.Fatal(err)
	}
	if got != "🌐{{INBOUND}}" {
		t.Fatalf("remarkTemplate = %q, want the admin's own template kept", got)
	}
}

// Defaults apply once: an admin who later restores upstream's template or turns
// Clash off must not be overridden on the next start.
func TestInit_AppliesDefaultsOnlyOnce(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	settings := &service.SettingService{}
	if err := settings.SetRemarkTemplate(service.DefaultRemarkTemplate); err != nil {
		t.Fatal(err)
	}
	if err := settings.SetSubClashEnable(false); err != nil {
		t.Fatal(err)
	}
	mustInit(t)

	if got, _ := settings.GetRemarkTemplate(); got != service.DefaultRemarkTemplate {
		t.Fatalf("remarkTemplate = %q after second Init, want the admin's choice kept", got)
	}
	if on, _ := settings.GetSubClashEnable(); on {
		t.Fatal("subClashEnable re-enabled by second Init")
	}
	var seeded int64
	database.GetDB().Model(&model.HistoryOfSeeders{}).Where("seeder_name = ?", initSeeder).Count(&seeded)
	if seeded != 1 {
		t.Fatalf("seeder rows = %d, want 1", seeded)
	}
}
