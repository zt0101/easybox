package easybox

import (
	"encoding/json"
	"fmt"
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

// pa builds preferred addresses from "address" or "address=remark" strings.
func pa(specs ...string) []PreferredAddress {
	out := []PreferredAddress{}
	for _, s := range specs {
		a, r, _ := strings.Cut(s, "=")
		out = append(out, PreferredAddress{Address: a, Remark: r})
	}
	return out
}

func setPreferred(t *testing.T, svc *Service, specs ...string) PreferredView {
	t.Helper()
	view, err := svc.SetPreferred(CloudflarePreferredRequest{Addresses: pa(specs...)})
	if err != nil {
		t.Fatalf("SetPreferred: %v", err)
	}
	return view
}

func TestPreferredAddresses_Validated(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	newFakeCloudflare(t)
	svc := cdnTestService(nil)
	// Lines without a domain use them too: no Cloudflare token needed.
	if _, err := svc.SetPreferred(CloudflarePreferredRequest{Addresses: pa("104.17.138.161")}); err != nil {
		t.Fatalf("without Cloudflare: %v", err)
	}
	eleven := make([]string, 0, 11)
	for i := 1; i <= 11; i++ {
		eleven = append(eleven, fmt.Sprintf("1.1.1.%d", i))
	}
	for name, bad := range map[string][]string{
		"not an address":    {"not an ip"},
		"URL":               {"http://1.2.3.4"},
		"eleven":            eleven,
		"same IP twice":     {"1.1.1.1=a", "1.1.1.1=b"},
		"same remark twice": {"1.1.1.1=公司", "1.1.1.2=公司"},
		"template braces":   {"1.1.1.1={{EMAIL}}"},
		"fragment mark":     {"1.1.1.1=a#b"},
		"remark only":       {"=公司"},
		"remark too long":   {"1.1.1.1=" + strings.Repeat("长", maxPreferredRemark+1)},
	} {
		if _, err := svc.SetPreferred(CloudflarePreferredRequest{Addresses: pa(bad...)}); err == nil {
			t.Fatalf("%s: %v accepted", name, bad)
		}
	}
	ten := make([]string, 0, 10)
	for i := 1; i <= 10; i++ {
		ten = append(ten, fmt.Sprintf("1.1.1.%d", i))
	}
	if _, err := svc.SetPreferred(CloudflarePreferredRequest{Addresses: pa(ten...)}); err != nil {
		t.Fatalf("ten addresses rejected: %v", err)
	}
	view := setPreferred(t, svc, " 104.17.138.161 = 公司移动 ", "", "CF.Example.ORG", "2606:4700::1=家里")
	want := pa("104.17.138.161=公司移动", "cf.example.org", "2606:4700::1=家里")
	if fmt.Sprint(view.Addresses) != fmt.Sprint(want) {
		t.Fatalf("stored %v, want %v (trimmed, blank rows dropped, lower-cased)", view.Addresses, want)
	}
}

func TestPreferredAddresses_NodesNamedByRemark(t *testing.T) {
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
	setPreferred(t, svc, "104.17.138.161=公司移动", "104.29.111.157")

	body, skipped := renderSingbox(t)
	if len(skipped) != 0 {
		t.Fatalf("skipped = %v", skipped)
	}
	var cfg singboxConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		t.Fatal(err)
	}
	for tag, server := range map[string]string{
		"hk-cdn": cdn.Address, "hk-cdn-公司移动": "104.17.138.161", "hk-cdn-优选2": "104.29.111.157",
	} {
		out := outboundByTag(cfg, tag)
		tls, _ := out["tls"].(map[string]any)
		tr, _ := out["transport"].(map[string]any)
		headers, _ := tr["headers"].(map[string]any)
		if out == nil || out["server"] != server || tls["server_name"] != cdn.Address || headers["Host"] != cdn.Address {
			t.Fatalf("sing-box %s = %v, want server %s with SNI and Host %s", tag, out, server, cdn.Address)
		}
	}
	if outboundByTag(cfg, "hk-hy2") == nil || outboundByTag(cfg, "hk-hy2-公司移动") != nil {
		t.Fatal("preferred addresses must touch CDN lines only")
	}
	checkWithSingbox(t, body)

	nodes := subNodes(t)
	pref := nodes["hk-cdn-公司移动"]
	if pref == nil || pref.Hostname() != "104.17.138.161" || pref.Query().Get("sni") != cdn.Address || pref.Query().Get("host") != cdn.Address {
		t.Fatalf("raw nodes = %v, want hk-cdn-公司移动 at 104.17.138.161 with the line's SNI and Host", nodes)
	}
}

// eb.10-eb.12 kept the list on the Cloudflare row; an upgrade moves it to
// easybox_settings once, from either stored format.
func TestPreferredAddresses_MovedOutOfCloudflareRow(t *testing.T) {
	for name, stored := range map[string]string{
		"eb.10 bare addresses": "104.17.138.161\n104.29.111.157",
		"eb.11 JSON":           `[{"address":"104.17.138.161","remark":"公司"},{"address":"104.29.111.157","remark":""}]`,
	} {
		t.Run(name, func(t *testing.T) {
			initTestDB(t)
			mustInit(t)
			newFakeCloudflare(t)
			svc := cdnTestService(nil)
			configureCloudflare(t, svc)
			createLine(t, svc, TemplateVlessCDN, "hk-cdn")
			db := database.GetDB()
			if err := db.Model(&model.EasyboxCloudflare{}).Where("1 = 1").Update("preferred_addresses", stored).Error; err != nil {
				t.Fatal(err)
			}
			db.Where("seeder_name = ?", preferredSeeder).Delete(&model.HistoryOfSeeders{})
			mustInit(t) // the upgrade
			if err := syncCDNHosts(); err != nil {
				t.Fatal(err)
			}
			view, _ := svc.GetPreferred()
			if len(view.Addresses) != 2 || view.Addresses[0].Address != "104.17.138.161" {
				t.Fatalf("migrated %v", view.Addresses)
			}
			row, _, _ := loadCloudflare()
			if row.PreferredAddresses != "" {
				t.Fatalf("Cloudflare row still holds %q", row.PreferredAddresses)
			}
			if err := svc.ClearCloudflare(); err != nil {
				t.Fatal(err)
			}
			if view, _ = svc.GetPreferred(); len(view.Addresses) != 2 {
				t.Fatalf("disconnecting Cloudflare dropped the preferred addresses: %v", view.Addresses)
			}
			if nodes := subNodes(t); len(nodes) < 3 {
				t.Fatalf("preferred nodes lost after migration: %v", nodes)
			}
		})
	}
}

func TestPreferredAddresses_FollowLinesAndClearing(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	newFakeCloudflare(t)
	svc := cdnTestService(nil)
	configureCloudflare(t, svc)
	setPreferred(t, svc, "104.17.138.161=公司")
	a := createLine(t, svc, TemplateVlessCDN, "a")
	b := createLine(t, svc, TemplateVlessCDN, "b")
	nodes := subNodes(t)
	if nodes["a-公司"] == nil || nodes["b-公司"] == nil {
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
	if nodes["a-公司"] != nil || nodes["b-公司"] == nil || nodes["b"].Hostname() != next.Address {
		t.Fatalf("after delete + regenerate: %v", nodes)
	}
	setPreferred(t, svc)
	var n int64
	database.GetDB().Model(&model.Host{}).Where("group_id LIKE ?", cdnHostGroupPrefix+"%").Count(&n)
	if nodes = subNodes(t); n != 0 || nodes["b-公司"] != nil || nodes["b"] == nil {
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
