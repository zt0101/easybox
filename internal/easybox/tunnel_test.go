package easybox

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// fakeTunnel stands in for systemd + cloudflared.
type fakeTunnel struct {
	mu        sync.Mutex
	running   bool
	port      int
	host      string // what /quicktunnel reports while running
	starts    int
	stops     int
	neverUp   bool
	notReady  bool // hostname handed out, no connection to Cloudflare yet
	noBinary  bool
	noSystemd bool
}

func useFakeTunnel(t *testing.T) *fakeTunnel {
	t.Helper()
	f := &fakeTunnel{host: "first-quick-tunnel.trycloudflare.com"}
	old, oldWait := tunnelEnv, tunnelWait
	tunnelWait = 2 * time.Second
	tunnelEnv = tunnelEnvironment{
		binary: func() string {
			if f.noBinary {
				return ""
			}
			return "/opt/x-ui/bin/cloudflared"
		},
		systemd: func() bool { return !f.noSystemd },
		start: func(_ string, port int) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.running, f.port = true, port
			f.starts++
			return nil
		},
		stop: func() error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.running = false
			f.stops++
			return nil
		},
		active: func() bool {
			f.mu.Lock()
			defer f.mu.Unlock()
			return f.running
		},
		hostname: func() (string, int, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if !f.running || f.neverUp {
				return "", 0, errors.New("connection refused")
			}
			if f.notReady {
				return f.host, 0, nil
			}
			return f.host, 1, nil
		},
	}
	t.Cleanup(func() { tunnelEnv, tunnelWait = old, oldWait })
	return f
}

func templateView(t *testing.T, svc *Service, id string) TemplateView {
	t.Helper()
	for _, tpl := range svc.ListTemplates() {
		if tpl.Id == id {
			return tpl
		}
	}
	t.Fatalf("no template %s", id)
	return TemplateView{}
}

func TestTunnelTemplate_Availability(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	f := useFakeTunnel(t)
	svc := cdnTestService(nil)
	f.noBinary = true
	if tpl := templateView(t, svc, TemplateVlessTunnel); tpl.Available || !strings.Contains(tpl.Reason, "cloudflared") {
		t.Fatalf("without cloudflared: %+v", tpl)
	}
	f.noBinary, f.noSystemd = false, true
	if tpl := templateView(t, svc, TemplateVlessTunnel); tpl.Available || !strings.Contains(tpl.Reason, "systemd") {
		t.Fatalf("without systemd: %+v", tpl)
	}
	f.noSystemd = false
	if tpl := templateView(t, svc, TemplateVlessTunnel); !tpl.Available {
		t.Fatalf("with cloudflared and systemd: %+v", tpl)
	}
	createLine(t, svc, TemplateVlessTunnel, "backup")
	if tpl := templateView(t, svc, TemplateVlessTunnel); tpl.Available || !strings.Contains(tpl.Reason, "only one") {
		t.Fatalf("second tunnel line offered: %+v", tpl)
	}
	if _, err := svc.CreateEndpoint(CreateEndpointRequest{Template: TemplateVlessTunnel}); err == nil {
		t.Fatal("a second tunnel line was created")
	}
}

func TestTunnelLine_LocalInboundAndNodesThroughCloudflare(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	f := useFakeTunnel(t)
	svc := cdnTestService(nil)
	ep := createLine(t, svc, TemplateVlessTunnel, "backup")
	if ep.Status != StatusRunning || ep.Address != f.host || f.port != ep.Port || f.starts != 1 {
		t.Fatalf("line %+v, tunnel %+v; want RUNNING at the tunnel hostname, tunnel on the line port", ep, f)
	}
	var in model.Inbound
	database.GetDB().First(&in, ep.InboundId)
	var stream map[string]any
	_ = json.Unmarshal([]byte(in.StreamSettings), &stream)
	if in.Listen != "127.0.0.1" || stream["network"] != "ws" || stream["security"] != "none" {
		t.Fatalf("inbound listen %q stream %v; want plain WebSocket on 127.0.0.1", in.Listen, stream)
	}

	if u := parseLineURL(t, lineLink(t, svc, ep)); u.Hostname() != f.host || u.Query().Get("security") != "tls" {
		t.Fatalf("Copy link = %s; want the tunnel hostname over TLS, not the local inbound", u)
	}
	setPreferred(t, svc, "104.17.138.161=手机")
	nodes := subNodes(t)
	for name, addr := range map[string]string{"backup": f.host, "backup-手机": "104.17.138.161"} {
		u := nodes[name]
		if u == nil {
			t.Fatalf("no node %s in %v", name, nodes)
		}
		q := u.Query()
		if u.Hostname() != addr || u.Port() != "443" || q.Get("security") != "tls" || q.Get("type") != "ws" ||
			q.Get("sni") != f.host || q.Get("host") != f.host || q.Get("fp") != "chrome" {
			t.Fatalf("node %s = %s; want %s:443 over TLS with SNI and Host %s", name, u, addr, f.host)
		}
	}
	if len(nodes) != 2 {
		t.Fatalf("nodes = %v; the unreachable 127.0.0.1 link must not be offered", nodes)
	}

	body, skipped := renderSingbox(t)
	if len(skipped) != 0 {
		t.Fatalf("skipped = %v", skipped)
	}
	var cfg singboxConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		t.Fatal(err)
	}
	out := outboundByTag(cfg, "backup-手机")
	tls, _ := out["tls"].(map[string]any)
	tr, _ := out["transport"].(map[string]any)
	if out["server"] != "104.17.138.161" || out["server_port"] != float64(443) || tls["server_name"] != f.host || tr["type"] != "ws" {
		t.Fatalf("sing-box outbound = %v", out)
	}
	checkWithSingbox(t, body)
}

func TestTunnelWatch_FollowsHostnameAndRestartsAfterReboot(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	f := useFakeTunnel(t)
	svc := cdnTestService(nil)
	ep := createLine(t, svc, TemplateVlessTunnel, "backup")

	f.host = "second-quick-tunnel.trycloudflare.com" // cloudflared crashed and came back
	if err := svc.tunnelWatchOnce(); err != nil {
		t.Fatal(err)
	}
	if nodes := subNodes(t); nodes["backup"] == nil || nodes["backup"].Hostname() != f.host {
		t.Fatalf("nodes = %v, want the new hostname %s", nodes, f.host)
	}
	if got := auditResults(t, "tunnel.hostname"); len(got) != 1 {
		t.Fatalf("audit = %v, want one hostname change", got)
	}

	f.running = false // the VPS rebooted: the transient unit is gone
	if err := svc.tunnelWatchOnce(); err != nil {
		t.Fatal(err)
	}
	if !f.running || f.port != ep.Port || f.starts != 2 {
		t.Fatalf("tunnel %+v; want it started again on port %d", f, ep.Port)
	}
}

func TestTunnelLine_DeleteStopsTunnelAndWatchStopsOrphans(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	f := useFakeTunnel(t)
	svc := cdnTestService(nil)
	ep := createLine(t, svc, TemplateVlessTunnel, "backup")
	if err := svc.DeleteEndpoint(ep.Id); err != nil {
		t.Fatal(err)
	}
	if f.running {
		t.Fatal("tunnel still running after the line was deleted")
	}
	var n int64
	database.GetDB().Model(&model.Host{}).Where("group_id LIKE ?", tunnelHostGroupPrefix+"%").Count(&n)
	if n != 0 {
		t.Fatalf("%d tunnel hosts left", n)
	}
	f.running = true // a unit nobody owns any more
	if err := svc.tunnelWatchOnce(); err != nil {
		t.Fatal(err)
	}
	if f.running {
		t.Fatal("watch left an orphaned tunnel running")
	}
}

func TestTunnelLine_TunnelThatNeverComesUpLeavesNothing(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	f := useFakeTunnel(t)
	f.neverUp = true
	svc := cdnTestService(nil)
	ep, err := svc.CreateEndpoint(CreateEndpointRequest{Template: TemplateVlessTunnel})
	if err == nil || ep.FailedStep != StepHealthCheck || !strings.Contains(err.Error(), "quick tunnel") {
		t.Fatalf("err = %v, line = %+v; want a health-check failure naming the tunnel", err, ep)
	}
	if n := countInbounds(t); n != 0 || f.running {
		t.Fatalf("inbounds = %d, tunnel running = %v; want nothing left", n, f.running)
	}
}

func TestTunnelAndCDNLines_KeepTheirOwnHosts(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	newFakeCloudflare(t)
	f := useFakeTunnel(t)
	svc := cdnTestService(nil)
	configureCloudflare(t, svc)
	cdn := createLine(t, svc, TemplateVlessCDN, "cdn")
	createLine(t, svc, TemplateVlessTunnel, "backup")
	setPreferred(t, svc, "104.17.138.161=公司")
	nodes := subNodes(t)
	if nodes["cdn-公司"].Query().Get("sni") != cdn.Address || nodes["backup-公司"].Query().Get("sni") != f.host {
		t.Fatalf("each line's preferred node must keep its own SNI: %v", nodes)
	}
	setPreferred(t, svc)
	nodes = subNodes(t)
	if nodes["backup"] == nil || nodes["backup"].Query().Get("security") != "tls" || nodes["cdn-公司"] != nil {
		t.Fatalf("without preferred addresses the tunnel node must stay on TLS: %v", nodes)
	}
}

func TestTunnelLine_RegenerateMovesTunnelToNewPort(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	f := useFakeTunnel(t)
	svc := cdnTestService(nil)
	ep := createLine(t, svc, TemplateVlessTunnel, "backup")
	f.host = "after-regenerate.trycloudflare.com"
	next, err := svc.RegenerateEndpoint(ep.Id, false)
	if err != nil {
		t.Fatalf("RegenerateEndpoint: %v", err)
	}
	if next.Port == ep.Port || f.port != next.Port || !f.running || next.Address != f.host {
		t.Fatalf("line %+v, tunnel %+v; want the tunnel on the new port at the new hostname", next, f)
	}
	if nodes := subNodes(t); len(nodes) != 1 || nodes["backup"].Hostname() != f.host {
		t.Fatalf("nodes = %v", nodes)
	}
}

func TestTunnelLine_WaitsForAReadyConnection(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	f := useFakeTunnel(t)
	f.notReady = true
	svc := cdnTestService(nil)
	if _, err := svc.CreateEndpoint(CreateEndpointRequest{Template: TemplateVlessTunnel}); err == nil || !strings.Contains(err.Error(), "ready") {
		t.Fatalf("err = %v; a hostname without a ready connection is not a working tunnel", err)
	}
}
