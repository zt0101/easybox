package easybox

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

var subCDNURL = regexp.MustCompile(`^https://sub-[0-9a-z]{6}\.example\.com:2096/`)

func setSettingRow(t *testing.T, key, value string) {
	t.Helper()
	db := database.GetDB()
	db.Where("key = ?", key).Delete(&model.Setting{})
	if err := db.Create(&model.Setting{Key: key, Value: value}).Error; err != nil {
		t.Fatal(err)
	}
}

// subCDNReady is a panel whose sub server serves TLS on the default 2096.
func subCDNReady(t *testing.T) (*Service, *fakeCloudflare) {
	t.Helper()
	initTestDB(t)
	mustInit(t)
	cf := newFakeCloudflare(t)
	setSettingRow(t, "subCertFile", "/root/cert/ip/fullchain.pem")
	setSettingRow(t, "subKeyFile", "/root/cert/ip/privkey.pem")
	svc := cdnTestService(nil)
	return svc, cf
}

func subView(t *testing.T, svc *Service) *SubscriptionView {
	t.Helper()
	v, err := svc.GetSubscription("203.0.113.10:2053")
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestSubCDN_UnavailableReasons(t *testing.T) {
	svc, _ := subCDNReady(t)
	if v := subView(t, svc); !strings.Contains(v.CdnUnavailable, "Cloudflare") {
		t.Fatalf("without Cloudflare: %q", v.CdnUnavailable)
	}
	configureCloudflare(t, svc)
	if v := subView(t, svc); v.CdnUnavailable != "" {
		t.Fatalf("ready panel: %q", v.CdnUnavailable)
	}
	setSettingRow(t, "subPort", "2097")
	if v := subView(t, svc); !strings.Contains(v.CdnUnavailable, "2097") {
		t.Fatalf("port 2097: %q", v.CdnUnavailable)
	}
	if err := svc.SetSubscriptionCDN(true); err == nil {
		t.Fatal("turned on with a port Cloudflare does not proxy")
	}
	setSettingRow(t, "subPort", "2096")
	setSettingRow(t, "subCertFile", "")
	if v := subView(t, svc); !strings.Contains(v.CdnUnavailable, "TLS") {
		t.Fatalf("plain HTTP sub server: %q", v.CdnUnavailable)
	}
}

func TestSubCDN_OnAddsProxiedRecordAndAddresses(t *testing.T) {
	svc, cf := subCDNReady(t)
	configureCloudflare(t, svc)
	before := subView(t, svc)
	if err := svc.SetSubscriptionCDN(true); err != nil {
		t.Fatalf("SetSubscriptionCDN: %v", err)
	}
	if err := svc.SetSubscriptionCDN(true); err != nil { // already on: nothing new
		t.Fatal(err)
	}
	recs, certs := cf.snapshot()
	if len(recs) != 1 || len(certs) != 0 {
		t.Fatalf("records %v certs %v; want one record and no certificate", recs, certs)
	}
	v := subView(t, svc)
	var rec fakeRecord
	for _, r := range recs {
		rec = r
	}
	if !rec.Proxied || rec.Content != testServerIP || !strings.HasPrefix(v.CdnSubURL, "https://"+rec.Name+":2096/") {
		t.Fatalf("record %+v, cdnSubUrl %q", rec, v.CdnSubURL)
	}
	if !subCDNURL.MatchString(v.CdnSubURL) || !strings.HasSuffix(v.CdnSubURL, "/"+v.SubID) ||
		!subCDNURL.MatchString(v.CdnClashURL) || !strings.HasSuffix(v.CdnClashURL, "/"+v.SubID) {
		t.Fatalf("cdn urls %q %q", v.CdnSubURL, v.CdnClashURL)
	}
	if v.SubURL != before.SubURL || v.ClashURL != before.ClashURL {
		t.Fatalf("IP addresses changed: %q → %q", before.SubURL, v.SubURL)
	}
	if len(cf.sslPatches) != 1 || cf.sslPatches[0] != "full" {
		t.Fatalf("SSL patches %v, want flexible raised to full", cf.sslPatches)
	}

	if _, err := svc.RotateSubscription(); err != nil {
		t.Fatal(err)
	}
	if r := subView(t, svc); !strings.HasSuffix(r.CdnSubURL, "/"+r.SubID) || r.SubID == v.SubID {
		t.Fatalf("after rotation cdnSubUrl %q, subId %q", r.CdnSubURL, r.SubID)
	}

	if err := svc.SetSubscriptionCDN(false); err != nil {
		t.Fatal(err)
	}
	if recs, _ := cf.snapshot(); len(recs) != 0 {
		t.Fatalf("records left after off: %v", recs)
	}
	if v := subView(t, svc); v.CdnSubURL != "" || v.CdnClashURL != "" || v.SubURL == "" {
		t.Fatalf("after off: %+v", v)
	}
}

func TestSubCDN_StrictModeRefused(t *testing.T) {
	svc, cf := subCDNReady(t)
	cf.sslMode = "strict"
	configureCloudflare(t, svc)
	err := svc.SetSubscriptionCDN(true)
	if err == nil || !strings.Contains(err.Error(), "strict") {
		t.Fatalf("err = %v, want strict mode refused", err)
	}
	if recs, _ := cf.snapshot(); len(recs) != 0 || len(cf.sslPatches) != 0 {
		t.Fatalf("records %v, patches %v; want nothing touched", recs, cf.sslPatches)
	}
	if v := subView(t, svc); v.CdnSubURL != "" {
		t.Fatalf("cdnSubUrl %q after refusal", v.CdnSubURL)
	}
}

func TestSubCDN_OffWithoutTokenNamesTheRecord(t *testing.T) {
	svc, cf := subCDNReady(t)
	configureCloudflare(t, svc)
	if err := svc.SetSubscriptionCDN(true); err != nil {
		t.Fatal(err)
	}
	host, _, _ := getSetting(subCDNHostKey)
	if err := svc.ClearCloudflare(); err != nil {
		t.Fatal(err)
	}
	if v := subView(t, svc); v.CdnSubURL == "" {
		t.Fatal("disconnecting Cloudflare must not drop a working subscription address")
	}
	err := svc.SetSubscriptionCDN(false)
	if err == nil || !strings.Contains(err.Error(), host) {
		t.Fatalf("err = %v, want it to name %s", err, host)
	}
	if v := subView(t, svc); v.CdnSubURL != "" {
		t.Fatal("address still shown after off")
	}
	if recs, _ := cf.snapshot(); len(recs) != 1 {
		t.Fatalf("records = %v; without a token nothing can be removed", recs)
	}
}

// A line without a fixed address takes the host the subscription was fetched
// from; through the Cloudflare hostname that must still be the server IP.
func TestSubCDN_DirectLinesKeepTheServerAddress(t *testing.T) {
	svc, _ := subCDNReady(t)
	configureCloudflare(t, svc)
	createLine(t, svc, TemplateVlessReality, "direct") // no address: the request host
	if err := svc.SetSubscriptionCDN(true); err != nil {
		t.Fatal(err)
	}
	cdnHost, _, _ := getSetting(subCDNHostKey)
	tmpl, _ := (&service.SettingService{}).GetRemarkTemplate()
	id, _ := EnsureDefaultSubscription()
	for _, reqHost := range []string{cdnHost, strings.ToUpper(cdnHost), testServerIP} {
		links, _, _, _, err := sub.NewSubService(tmpl).GetSubs(id.SubID, reqHost)
		if err != nil || len(links) != 1 {
			t.Fatalf("GetSubs(%s): %v %v", reqHost, links, err)
		}
		u, _ := url.Parse(links[0])
		if u.Hostname() != testServerIP {
			t.Fatalf("fetched via %s: node address %s, want the server IP %s", reqHost, u.Hostname(), testServerIP)
		}
	}
	if err := svc.SetSubscriptionCDN(false); err != nil {
		t.Fatal(err)
	}
	links, _, _, _, _ := sub.NewSubService(tmpl).GetSubs(id.SubID, "node.example.org")
	if u, _ := url.Parse(links[0]); u.Hostname() != "node.example.org" {
		t.Fatalf("after off the request host must pass through: %s", links[0])
	}
}
