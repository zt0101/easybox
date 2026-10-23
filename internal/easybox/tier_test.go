package easybox

import (
	"net/url"
	"slices"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// Node names start with the Quick Create tier tag and the subscription lists
// recommended lines first, whatever order the lines were created in.
func TestTier_NodeNamesTaggedAndOrdered(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	svc := newTestService(nil, nil)
	if err := svc.SetTierLabels(true); err != nil {
		t.Fatal(err)
	}
	createLine(t, svc, TemplateShadowsocks, "ss")
	usePanelCert(t, "node.example.com")
	createLine(t, svc, TemplateTuic, "tuic")
	createLine(t, svc, TemplateVlessReality, "reality")

	want := []string{"🟢推荐|reality", "🔵备用|tuic", "🟠易识别|ss"}
	if got := linkFragments(t); !slices.Equal(got, want) {
		t.Fatalf("names = %q, want %q", got, want)
	}
	view, err := svc.GetSubscription("203.0.113.10")
	if err != nil || !view.TierLabels {
		t.Fatalf("view.TierLabels = %v (%v), want on", view != nil && view.TierLabels, err)
	}

	// Off: plain names, same order. The line names never held the tag.
	if err := svc.SetTierLabels(false); err != nil {
		t.Fatal(err)
	}
	if got := linkFragments(t); !slices.Equal(got, []string{"reality", "tuic", "ss"}) {
		t.Fatalf("names with tags off = %q, want plain names in tier order", got)
	}
	var names []string
	database.GetDB().Model(&model.EasyboxEndpoint{}).Order("id").Pluck("name", &names)
	if !slices.Equal(names, []string{"ss", "tuic", "reality"}) {
		t.Fatalf("stored line names = %q, want them untouched", names)
	}
}

// linkFragments returns each subscription link's name (#fragment), in order;
// unlike subLinkNames it also reads tuic:// links.
func linkFragments(t *testing.T) []string {
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
		u, err := url.Parse(l)
		if err != nil {
			t.Fatalf("url.Parse(%q): %v", l, err)
		}
		names = append(names, u.Fragment)
	}
	return names
}

// A fresh install shows the tags; unset means on.
func TestTier_LabelsOnByDefault(t *testing.T) {
	initTestDB(t)
	if err := database.GetDB().Where("key = ?", tierLabelsKey).Delete(&model.EasyboxSetting{}).Error; err != nil {
		t.Fatal(err)
	}
	if on, err := TierLabelsEnabled(); err != nil || !on {
		t.Fatalf("TierLabelsEnabled() = %v, %v; want on when never set", on, err)
	}
}

// The eb.10-eb.17 template gains {{TIER}} once; an admin's own template stays.
func TestInit_AddsTierTokenToEasyBoxTemplateOnly(t *testing.T) {
	seedBefore := func(t *testing.T, tmpl string) *service.SettingService {
		t.Helper()
		initTestDB(t)
		settings := &service.SettingService{}
		if err := settings.SetRemarkTemplate(tmpl); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{initSeeder, hostRemarkSeeder} {
			if err := database.GetDB().Create(&model.HistoryOfSeeders{SeederName: name}).Error; err != nil {
				t.Fatal(err)
			}
		}
		mustInit(t)
		return settings
	}

	settings := seedBefore(t, hostRemarkTemplate)
	if got, _ := settings.GetRemarkTemplate(); got != lineNameRemarkTemplate {
		t.Fatalf("template = %q, want %q", got, lineNameRemarkTemplate)
	}

	settings = seedBefore(t, "🌐{{INBOUND}}{{HOST}}")
	if got, _ := settings.GetRemarkTemplate(); got != "🌐{{INBOUND}}{{HOST}}" {
		t.Fatalf("custom template changed to %q", got)
	}
}
