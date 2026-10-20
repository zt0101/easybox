package easybox

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestGetSubscription_URLsFollowSubServerSettings(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	svc := newTestService(nil, nil)
	for _, name := range []string{"line-a", "line-b"} {
		if _, err := svc.CreateEndpoint(CreateEndpointRequest{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	settings := &service.SettingService{}
	subPath, _ := settings.GetSubPath()
	clashPath, _ := settings.GetSubClashPath()
	id, _ := EnsureDefaultSubscription()

	view, err := svc.GetSubscription("panel.example.com:2053")
	if err != nil {
		t.Fatalf("GetSubscription: %v", err)
	}
	if want := "http://panel.example.com:2096" + subPath + id.SubID; view.SubURL != want {
		t.Fatalf("subUrl = %q, want %q", view.SubURL, want)
	}
	if want := "http://panel.example.com:2096" + clashPath + id.SubID; view.ClashURL != want {
		t.Fatalf("clashUrl = %q, want %q", view.ClashURL, want)
	}
	if len(view.Lines) != 2 || !view.Lines[0].InSubscription || !view.Lines[1].InSubscription {
		t.Fatalf("lines = %+v, want 2 lines in the subscription", view.Lines)
	}
}

// Behind a reverse proxy the admin sets subURI; the copied URL must use it.
func TestGetSubscription_AdminSubURIWins(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	if err := database.GetDB().Create(&model.Setting{Key: "subURI", Value: "https://sub.example.com/s/"}).Error; err != nil {
		t.Fatal(err)
	}
	id, _ := EnsureDefaultSubscription()
	view, err := newTestService(nil, nil).GetSubscription("panel.example.com:2053")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://sub.example.com/s/" + id.SubID; view.SubURL != want {
		t.Fatalf("subUrl = %q, want %q", view.SubURL, want)
	}
}

func TestGetSubscription_NoClashURLWhenDisabled(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	if err := (&service.SettingService{}).SetSubClashEnable(false); err != nil {
		t.Fatal(err)
	}
	view, err := newTestService(nil, nil).GetSubscription("panel.example.com:2053")
	if err != nil {
		t.Fatal(err)
	}
	if view.ClashURL != "" {
		t.Fatalf("clashUrl = %q with Clash disabled, want empty", view.ClashURL)
	}
}

// EasyBox.md §23.1: the subscription page chooses which lines it carries.
func TestSetEndpointInSubscription_ExcludeAndInclude(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	svc := newTestService(nil, nil)
	keep, err := svc.CreateEndpoint(CreateEndpointRequest{Name: "keep"})
	if err != nil {
		t.Fatal(err)
	}
	drop, err := svc.CreateEndpoint(CreateEndpointRequest{Name: "drop"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetEndpointInSubscription(drop.Id, false); err != nil {
		t.Fatalf("exclude: %v", err)
	}
	if names := subLinkNames(t); len(names) != 1 || names[0] != "keep" {
		t.Fatalf("link names after exclude = %q, want [keep]", names)
	}
	list, err := svc.ListEndpoints()
	if err != nil {
		t.Fatal(err)
	}
	if list[1].InSubscription || list[1].Status != StatusRunning {
		t.Fatalf("excluded line = %+v, want RUNNING but not in subscription", list[1])
	}
	if err := svc.SetEndpointInSubscription(drop.Id, true); err != nil {
		t.Fatalf("include: %v", err)
	}
	if names := subLinkNames(t); len(names) != 2 {
		t.Fatalf("link names after include = %q, want 2", names)
	}
	_ = keep
}
