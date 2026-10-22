package easybox

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestXhttpCDNLine_OwnRecordCertAndNextPort(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	cf := newFakeCloudflare(t)
	svc := cdnTestService(nil)
	configureCloudflare(t, svc)
	ws := createLine(t, svc, TemplateVlessCDN, "hk-cdn")
	xh := createLine(t, svc, TemplateVlessXhttpCDN, "hk-cdn-xhttp")
	if ws.Port != 443 || xh.Port != 8443 || xh.Status != StatusRunning {
		t.Fatalf("ports %d / %d, status %s; want 443 for WebSocket, 8443 for XHTTP", ws.Port, xh.Port, xh.Status)
	}
	if !cdnHost.MatchString(xh.Address) || xh.Address == ws.Address {
		t.Fatalf("XHTTP line address %q, want its own cdn-<label> host (WebSocket: %q)", xh.Address, ws.Address)
	}
	recs, certs := cf.snapshot()
	if len(recs) != 2 || len(certs) != 2 {
		t.Fatalf("records %v certs %v, want one of each per line", recs, certs)
	}

	u := parseLineURL(t, lineLink(t, svc, xh))
	q := u.Query()
	for k, v := range map[string]string{"type": "xhttp", "security": "tls", "sni": xh.Address, "host": xh.Address, "mode": "auto", "fp": "chrome"} {
		if q.Get(k) != v {
			t.Fatalf("link %s=%q, want %q (link %q)", k, q.Get(k), v, u)
		}
	}
	if u.Hostname() != xh.Address || u.Port() != "8443" || !xhttpPath.MatchString(q.Get("path")) || q.Get("pbk") != "" {
		t.Fatalf("link %q, want the CDN host on 8443, a random path and no REALITY", u)
	}

	var in model.Inbound
	database.GetDB().First(&in, xh.InboundId)
	var settings vlessSettings
	_ = json.Unmarshal([]byte(in.Settings), &settings)
	if len(settings.Clients) != 1 || settings.Clients[0].Flow != "" {
		t.Fatalf("clients = %+v, want one client without flow", settings.Clients)
	}
}

func TestXhttpCDNLine_PreferredNodesAndSingboxFilter(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	newFakeCloudflare(t)
	svc := cdnTestService(nil)
	configureCloudflare(t, svc)
	ws := createLine(t, svc, TemplateVlessCDN, "ws")
	xh := createLine(t, svc, TemplateVlessXhttpCDN, "xh")
	setPreferred(t, svc, "104.17.138.161=公司")

	nodes := subNodes(t)
	pref := nodes["xh-公司"]
	if pref == nil || nodes["ws-公司"] == nil || pref.Hostname() != "104.17.138.161" ||
		pref.Query().Get("sni") != xh.Address || pref.Query().Get("host") != xh.Address || pref.Query().Get("type") != "xhttp" {
		t.Fatalf("nodes = %v, want xh-公司 at the preferred IP with the XHTTP line's SNI and Host", nodes)
	}

	body, skipped := renderSingbox(t)
	for _, name := range []string{"xh", "xh-公司"} {
		found := false
		for _, s := range skipped {
			found = found || strings.HasPrefix(s, name+":")
		}
		if !found {
			t.Fatalf("skipped = %v, want %s left out (sing-box has no XHTTP)", skipped, name)
		}
	}
	var cfg singboxConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		t.Fatal(err)
	}
	if outboundByTag(cfg, "ws") == nil || outboundByTag(cfg, "ws-公司") == nil || outboundByTag(cfg, "xh") != nil {
		t.Fatalf("sing-box outbounds must keep the WebSocket line and its preferred node only (ws %s)", ws.Address)
	}
	checkWithSingbox(t, body)
}

func TestXhttpCDNTemplate_NeedsCloudflare(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	newFakeCloudflare(t)
	svc := cdnTestService(nil)
	ep, err := svc.CreateEndpoint(CreateEndpointRequest{Template: TemplateVlessXhttpCDN})
	if err == nil || ep.FailedStep != StepPrepare || !strings.Contains(err.Error(), "Cloudflare") {
		t.Fatalf("err = %v, line = %+v; want a prepare failure asking for Cloudflare", err, ep)
	}
}
