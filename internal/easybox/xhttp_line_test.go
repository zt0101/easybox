package easybox

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

var xhttpPath = regexp.MustCompile(`^/[0-9a-z]{8}$`)

func TestVlessXhttpLine_RealityWithoutCertificate(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	svc := newTestService(nil, nil)
	ep := createLine(t, svc, TemplateVlessXhttp, "HK-XHTTP")
	if ep.Protocol != "vless" || ep.Transport != "tcp" || ep.Status != StatusRunning {
		t.Fatalf("line = %+v, want a RUNNING vless line over tcp", ep)
	}
	u := parseLineURL(t, lineLink(t, svc, ep))
	id, _ := EnsureDefaultSubscription()
	q := u.Query()
	if u.Scheme != "vless" || u.User.Username() != id.UUID {
		t.Fatalf("link %q, want vless for the subscription user's uuid", u)
	}
	want := map[string]string{"type": "xhttp", "security": "reality", "mode": "auto", "pbk": testReality.PublicKey, "fp": "chrome"}
	for k, v := range want {
		if q.Get(k) != v {
			t.Fatalf("link %s=%q, want %q (link %q)", k, q.Get(k), v, u)
		}
	}
	if !xhttpPath.MatchString(q.Get("path")) {
		t.Fatalf("link path = %q, want a random /<8 chars>", q.Get("path"))
	}
	// Upstream's link builder drops flow off TCP, so check what Xray serves.
	var in model.Inbound
	if err := database.GetDB().First(&in).Error; err != nil {
		t.Fatal(err)
	}
	var settings vlessSettings
	if err := json.Unmarshal([]byte(in.Settings), &settings); err != nil {
		t.Fatal(err)
	}
	if len(settings.Clients) != 1 || settings.Clients[0].Flow != "" {
		t.Fatalf("clients = %+v, want one client without flow (Vision is TCP-only)", settings.Clients)
	}
}

func TestVlessXhttpLine_PathDiffersPerLine(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	svc := newTestService(nil, nil)
	createLine(t, svc, TemplateVlessXhttp, "x1")
	createLine(t, svc, TemplateVlessXhttp, "x2")
	var inbounds []model.Inbound
	if err := database.GetDB().Find(&inbounds).Error; err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, in := range inbounds {
		var stream xhttpStreamSettings
		if err := json.Unmarshal([]byte(in.StreamSettings), &stream); err != nil {
			t.Fatal(err)
		}
		paths[stream.XHTTPSettings.Path] = true
	}
	if len(inbounds) != 2 || len(paths) != 2 {
		t.Fatalf("paths = %v over %d lines, want two different ones", paths, len(inbounds))
	}
}

func TestSingboxConfig_SkipsXhttpLine(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	usePanelCert(t, "node.example.com")
	svc := newTestService(nil, nil)
	createLine(t, svc, TemplateVlessXhttp, "line-xhttp")
	createLine(t, svc, TemplateTrojan, "line-trojan")
	body, skipped := renderSingbox(t)
	if len(skipped) != 1 || !strings.HasPrefix(skipped[0], "line-xhttp:") {
		t.Fatalf("skipped = %v, want only the XHTTP line", skipped)
	}
	var cfg singboxConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		t.Fatal(err)
	}
	if outboundByTag(cfg, "line-trojan") == nil {
		t.Fatal("trojan line missing from the sing-box config")
	}
	checkWithSingbox(t, body)
}
