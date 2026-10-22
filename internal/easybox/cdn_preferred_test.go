package easybox

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// subNodes is every raw subscription node as name → URL, one per link line,
// named with the panel's remark template as the sub server does.
func subNodes(t *testing.T) map[string]*url.URL {
	t.Helper()
	tmpl, err := (&service.SettingService{}).GetRemarkTemplate()
	if err != nil {
		t.Fatal(err)
	}
	id, _ := EnsureDefaultSubscription()
	links, _, _, _, err := sub.NewSubService(tmpl).GetSubs(id.SubID, "203.0.113.10")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	out := map[string]*url.URL{}
	for _, entry := range links {
		for _, line := range strings.Split(entry, "\n") {
			u, err := url.Parse(line)
			if err != nil {
				t.Fatalf("link %q: %v", line, err)
			}
			out[u.Fragment] = u
		}
	}
	return out
}

func setPreferred(t *testing.T, svc *Service, addrs ...string) CloudflareView {
	t.Helper()
	view, err := svc.SetCloudflarePreferred(CloudflarePreferredRequest{Addresses: addrs})
	if err != nil {
		t.Fatalf("SetCloudflarePreferred: %v", err)
	}
	return view
}

func TestPreferredAddresses_Validated(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	newFakeCloudflare(t)
	svc := cdnTestService(nil)
	if _, err := svc.SetCloudflarePreferred(CloudflarePreferredRequest{Addresses: []string{"104.17.138.161"}}); err != ErrCloudflareNotConfigured {
		t.Fatalf("without Cloudflare: err = %v", err)
	}
	configureCloudflare(t, svc)
	for _, bad := range [][]string{{"not an ip"}, {"http://1.2.3.4"}, {"1.1.1.1", "1.1.1.2", "1.1.1.3", "1.1.1.4", "1.1.1.5", "1.1.1.6"}} {
		if _, err := svc.SetCloudflarePreferred(CloudflarePreferredRequest{Addresses: bad}); err == nil {
			t.Fatalf("%v accepted", bad)
		}
	}
	view := setPreferred(t, svc, " 104.17.138.161 ", "", "104.17.138.161", "CF.Example.ORG", "2606:4700::1")
	want := []string{"104.17.138.161", "cf.example.org", "2606:4700::1"}
	if strings.Join(view.PreferredAddresses, ",") != strings.Join(want, ",") {
		t.Fatalf("stored %v, want %v (trimmed, deduplicated, lower-cased)", view.PreferredAddresses, want)
	}
}

func TestPreferredAddresses_ExtraNodesKeepSNIAndHost(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	newFakeCloudflare(t)
	svc := cdnTestService(nil)
	configureCloudflare(t, svc)
	tmpl, _ := (&service.SettingService{}).GetRemarkTemplate()
	if tmpl != lineNameRemarkTemplate {
		t.Fatalf("remark template %q, want %q so preferred nodes get their suffix", tmpl, lineNameRemarkTemplate)
	}
	cdn := createLine(t, svc, TemplateVlessCDN, "hk-cdn")
	createLine(t, svc, TemplateHysteria2, "hk-hy2")
	setPreferred(t, svc, "104.17.138.161", "104.29.111.157")

	body, skipped := renderSingbox(t)
	if len(skipped) != 0 {
		t.Fatalf("skipped = %v", skipped)
	}
	var cfg singboxConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		t.Fatal(err)
	}
	for tag, server := range map[string]string{
		"hk-cdn": cdn.Address, "hk-cdn-优选1": "104.17.138.161", "hk-cdn-优选2": "104.29.111.157",
	} {
		out := outboundByTag(cfg, tag)
		tls, _ := out["tls"].(map[string]any)
		tr, _ := out["transport"].(map[string]any)
		headers, _ := tr["headers"].(map[string]any)
		if out == nil || out["server"] != server || tls["server_name"] != cdn.Address || headers["Host"] != cdn.Address {
			t.Fatalf("sing-box %s = %v, want server %s with SNI and Host %s", tag, out, server, cdn.Address)
		}
	}
	if outboundByTag(cfg, "hk-hy2") == nil || outboundByTag(cfg, "hk-hy2-优选1") != nil {
		t.Fatal("preferred addresses must touch CDN lines only")
	}
	checkWithSingbox(t, body)

	nodes := subNodes(t)
	pref := nodes["hk-cdn-优选1"]
	if pref == nil || pref.Hostname() != "104.17.138.161" || pref.Query().Get("sni") != cdn.Address || pref.Query().Get("host") != cdn.Address {
		t.Fatalf("raw nodes = %v, want hk-cdn-优选1 at 104.17.138.161 with the line's SNI and Host", nodes)
	}
}

func TestPreferredAddresses_FollowLinesAndClearing(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	newFakeCloudflare(t)
	svc := cdnTestService(nil)
	configureCloudflare(t, svc)
	setPreferred(t, svc, "104.17.138.161")
	a := createLine(t, svc, TemplateVlessCDN, "a")
	b := createLine(t, svc, TemplateVlessCDN, "b")
	nodes := subNodes(t)
	if nodes["a-优选1"] == nil || nodes["b-优选1"] == nil {
		t.Fatalf("lines created after the setting lack preferred nodes: %v", nodes)
	}
	if err := svc.DeleteEndpoint(a.Id); err != nil {
		t.Fatal(err)
	}
	next, err := svc.RegenerateEndpoint(b.Id, false)
	if err != nil {
		t.Fatal(err)
	}
	nodes = subNodes(t)
	if nodes["a-优选1"] != nil || nodes["b-优选1"] == nil || nodes["b"].Hostname() != next.Address {
		t.Fatalf("after delete + regenerate: %v", nodes)
	}
	setPreferred(t, svc)
	var n int64
	database.GetDB().Model(&model.Host{}).Where("group_id LIKE ?", cdnHostGroupPrefix+"%").Count(&n)
	if nodes = subNodes(t); n != 0 || nodes["b-优选1"] != nil || nodes["b"] == nil {
		t.Fatalf("empty list left %d hosts, nodes %v", n, nodes)
	}
}

func TestInit_MovesOldRemarkTemplateOnly(t *testing.T) {
	initTestDB(t)
	settings := &service.SettingService{}
	if err := settings.SetRemarkTemplate(oldLineNameRemarkTemplate); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Create(&model.HistoryOfSeeders{SeederName: initSeeder}).Error; err != nil {
		t.Fatal(err)
	}
	mustInit(t)
	if got, _ := settings.GetRemarkTemplate(); got != lineNameRemarkTemplate {
		t.Fatalf("template = %q, want the old EasyBox default moved to %q", got, lineNameRemarkTemplate)
	}

	initTestDB(t)
	if err := settings.SetRemarkTemplate("🌐{{INBOUND}}"); err != nil {
		t.Fatal(err)
	}
	mustInit(t)
	if got, _ := settings.GetRemarkTemplate(); got != "🌐{{INBOUND}}" {
		t.Fatalf("template = %q, want the admin's own template kept", got)
	}
}
