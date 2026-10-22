package easybox

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// The naming rule agreed on 2026-10-22: <region>-<protocol>-<transport>[-CDN|-Tunnel].
func TestLineTemplates_NameStyles(t *testing.T) {
	want := map[string]string{
		TemplateVlessReality:  "VLESS-Reality",
		TemplateVlessXhttp:    "VLESS-XHTTP-Reality",
		TemplateVlessTLS:      "VLESS-TLS",
		TemplateTrojan:        "Trojan-TLS",
		TemplateHysteria2:     "HY2",
		TemplateTuic:          "TUIC",
		TemplateShadowsocks:   "SS2022",
		TemplateVlessCDN:      "VLESS-WS-CDN",
		TemplateVlessXhttpCDN: "VLESS-XHTTP-CDN",
		TemplateVlessTunnel:   "VLESS-WS-Tunnel",
	}
	if len(lineTemplates) != len(want) {
		t.Fatalf("%d templates, %d name styles to check", len(lineTemplates), len(want))
	}
	for _, tpl := range lineTemplates {
		if tpl.nameStyle != want[tpl.id] {
			t.Fatalf("%s: name style %q, want %q", tpl.id, tpl.nameStyle, want[tpl.id])
		}
	}
}

func TestDefaultName_RegionStyleAndNumbering(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	svc := newTestService(nil, nil)
	first := createLine(t, svc, TemplateVlessReality, "")
	second := createLine(t, svc, TemplateVlessReality, "")
	hy2 := createLine(t, svc, TemplateHysteria2, "")
	if first.Name != "HK-VLESS-Reality" || second.Name != "HK-VLESS-Reality-2" || hy2.Name != "HK-HY2" {
		t.Fatalf("names %q %q %q", first.Name, second.Name, hy2.Name)
	}
	var in model.Inbound
	database.GetDB().First(&in, first.InboundId)
	if in.Remark != first.Name {
		t.Fatalf("inbound remark %q, want the line name", in.Remark)
	}
	if err := svc.DeleteEndpoint(first.Id); err != nil {
		t.Fatal(err)
	}
	if again := createLine(t, svc, TemplateVlessReality, ""); again.Name != "HK-VLESS-Reality" {
		t.Fatalf("after delete: %q, want the freed name back", again.Name)
	}
	if typed := createLine(t, svc, TemplateVlessReality, "my line"); typed.Name != "my line" {
		t.Fatalf("typed name %q", typed.Name)
	}
}

func TestRegion_DetectedOnceAndEditable(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	svc := newTestService(nil, nil)
	calls := 0
	svc.regionLookup = func() string { calls++; return "hk\n" }
	if v := svc.GetRegion(); v.Code != "HK" {
		t.Fatalf("detected %q", v.Code)
	}
	svc.GetRegion()
	createLine(t, svc, TemplateShadowsocks, "")
	if calls != 1 {
		t.Fatalf("lookups = %d, want the code detected once and stored", calls)
	}
	if _, err := svc.SetRegion(RegionRequest{Code: "Tokyo!"}); err == nil {
		t.Fatal("invalid code accepted")
	}
	if v, err := svc.SetRegion(RegionRequest{Code: " jp "}); err != nil || v.Code != "JP" {
		t.Fatalf("SetRegion: %+v %v", v, err)
	}
	if ep := createLine(t, svc, TemplateShadowsocks, ""); ep.Name != "JP-SS2022" {
		t.Fatalf("after manual region: %q", ep.Name)
	}
	svc.regionLookup = func() string { return "" } // detection fails
	if v, _ := svc.SetRegion(RegionRequest{Code: ""}); v.Code != "" {
		t.Fatalf("reset with a failing lookup: %q", v.Code)
	}
	if ep := createLine(t, svc, TemplateHysteria2, ""); ep.Name != "HY2" {
		t.Fatalf("unknown region: %q, want the style alone", ep.Name)
	}
}

func TestRenameEndpoint_LineNodeAndPreferredNodesFollow(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	newFakeCloudflare(t)
	svc := cdnTestService(nil)
	configureCloudflare(t, svc)
	cdn := createLine(t, svc, TemplateVlessCDN, "")
	direct := createLine(t, svc, TemplateVlessReality, "")
	if cdn.Name != "HK-VLESS-WS-CDN" {
		t.Fatalf("default CDN name %q", cdn.Name)
	}
	setPreferred(t, svc, "104.17.138.161=公司优选1")

	for _, bad := range []string{"", "   ", "a#b", "{{EMAIL}}", strings.Repeat("长", maxLineName+1), "hk-vless-reality"} {
		if err := svc.RenameEndpoint(cdn.Id, bad); err == nil {
			t.Fatalf("name %q accepted", bad)
		}
	}
	if err := svc.RenameEndpoint(cdn.Id, " 香港-CDN "); err != nil {
		t.Fatal(err)
	}
	nodes := subNodes(t)
	if nodes["香港-CDN"] == nil || nodes["香港-CDN-公司优选1"] == nil || nodes["HK-VLESS-WS-CDN"] != nil || nodes[direct.Name] == nil {
		t.Fatalf("nodes after rename: %v", nodes)
	}
	next, err := svc.RegenerateEndpoint(cdn.Id, false)
	if err != nil || next.Name != "香港-CDN" {
		t.Fatalf("regenerated line %+v, %v; want the new name kept", next, err)
	}
}
