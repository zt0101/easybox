package easybox

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/util/link"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// usePanelCert installs a certificate for host as the panel's own TLS
// certificate (what install.sh's IP or domain certificate becomes).
func usePanelCert(t *testing.T, host string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(7),
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{host}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "panel.crt"), filepath.Join(dir, "panel.key")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"webCertFile": certFile, "webKeyFile": keyFile} {
		if err := database.GetDB().Create(&model.Setting{Key: k, Value: v}).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func createLine(t *testing.T, svc *Service, template, name string) *EndpointView {
	t.Helper()
	ep, err := svc.CreateEndpoint(CreateEndpointRequest{Template: template, Name: name})
	if err != nil {
		t.Fatalf("CreateEndpoint(%s): %v", template, err)
	}
	return ep
}

func lineLink(t *testing.T, svc *Service, ep *EndpointView) string {
	t.Helper()
	l, err := svc.GetEndpointLink(ep.Id, "203.0.113.10")
	if err != nil {
		t.Fatalf("GetEndpointLink: %v", err)
	}
	return l
}

// EasyBox.md §9.1: without a trusted certificate a Hysteria2 line gets a
// self-signed one that clients pin; insecure is never used.
func TestHysteria2Line_SelfSignedCertificateIsPinned(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	svc := newTestService(nil, nil)
	ep := createLine(t, svc, TemplateHysteria2, "HK-HY2")
	if ep.Protocol != "hysteria2" || ep.Transport != "udp" || ep.Status != StatusRunning {
		t.Fatalf("line = %+v, want a RUNNING hysteria2 line over udp", ep)
	}
	raw := lineLink(t, svc, ep)
	res, err := link.ParseLink(raw)
	if err != nil {
		t.Fatalf("ParseLink(%q): %v", raw, err)
	}
	if res.Outbound["protocol"] != "hysteria" || res.Outbound["tag"] != "HK-HY2" {
		t.Fatalf("outbound = %v, want hysteria named HK-HY2", res.Outbound)
	}
	stream := res.Outbound["streamSettings"].(map[string]any)
	id, _ := EnsureDefaultSubscription()
	if auth := stream["hysteriaSettings"].(map[string]any)["auth"]; auth != id.Auth || id.Auth == "" {
		t.Fatalf("auth = %v, want the subscription user's auth %q", auth, id.Auth)
	}
	tls := stream["tlsSettings"].(map[string]any)
	if pin, _ := tls["pinnedPeerCertSha256"].(string); len(pin) != 64 {
		t.Fatalf("pinSHA256 = %q, want a hex SHA-256 pin (link %q)", pin, raw)
	}
	if strings.Contains(raw, "insecure=1") {
		t.Fatalf("link %q allows insecure TLS", raw)
	}
	if !strings.Contains(raw, "obfs=salamander") {
		t.Fatalf("link %q lacks upstream's default salamander obfs", raw)
	}
}

func TestHysteria2Line_PrefersPanelCertificate(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	usePanelCert(t, "203.0.113.10")
	svc := newTestService(nil, nil)
	raw := lineLink(t, svc, createLine(t, svc, TemplateHysteria2, "HK-HY2"))
	res, err := link.ParseLink(raw)
	if err != nil {
		t.Fatal(err)
	}
	tls := res.Outbound["streamSettings"].(map[string]any)["tlsSettings"].(map[string]any)
	if tls["serverName"] != "203.0.113.10" {
		t.Fatalf("sni = %v, want the panel certificate's name", tls["serverName"])
	}
	if pin, _ := tls["pinnedPeerCertSha256"].(string); pin != "" {
		t.Fatalf("pinSHA256 = %q, want none for a trusted certificate", pin)
	}
}

// TUIC links always say allow_insecure=0 and carry no pin, so a self-signed
// certificate would fail every client: TUIC needs the panel's trusted one.
func TestTuicLine_RequiresTrustedCertificate(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	ep, err := newTestService(nil, nil).CreateEndpoint(CreateEndpointRequest{Template: TemplateTuic})
	if !errors.Is(err, ErrNoTrustedCertificate) || ep.FailedStep != StepPrepare {
		t.Fatalf("err = %v, line = %+v; want ErrNoTrustedCertificate at prepare", err, ep)
	}
	if n := countInbounds(t); n != 0 {
		t.Fatalf("inbounds = %d, want 0", n)
	}
}

func TestTuicLine_UsesPanelCertificate(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	usePanelCert(t, "node.example.com")
	svc := newTestService(nil, nil)
	ep := createLine(t, svc, TemplateTuic, "HK-TUIC")
	if ep.Protocol != "tuic" || ep.Transport != "udp" {
		t.Fatalf("line = %+v, want tuic over udp", ep)
	}
	u, err := url.Parse(lineLink(t, svc, ep))
	if err != nil {
		t.Fatal(err)
	}
	id, _ := EnsureDefaultSubscription()
	pass, _ := u.User.Password()
	if u.Scheme != "tuic" || u.User.Username() != id.UUID || pass != id.Password || id.Password == "" {
		t.Fatalf("tuic link %q, want the subscription user's uuid:password", u)
	}
	if q := u.Query(); q.Get("sni") != "node.example.com" || q.Get("allow_insecure") != "0" {
		t.Fatalf("tuic link query = %v, want sni from the panel certificate and allow_insecure=0", q)
	}
}

// EasyBox.md §23.1: lines of different protocols still share one subscription user.
func TestMixedProtocols_ShareOneSubscriptionUser(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	usePanelCert(t, "node.example.com")
	svc := newTestService(nil, nil)
	for _, tpl := range []string{TemplateVlessReality, TemplateHysteria2, TemplateTuic} {
		createLine(t, svc, tpl, "line-"+tpl)
	}
	id, _ := EnsureDefaultSubscription()
	records, err := (&service.ClientService{}).GetRecordsBySubID(id.SubID)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("subscription users = %d, want 1", len(records))
	}
	links, _, _, _, err := sub.NewSubService("").GetSubs(id.SubID, "203.0.113.10")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 3 {
		t.Fatalf("links = %v, want vless + hysteria2 + tuic", links)
	}
}

func TestListTemplates_TuicNeedsPanelCertificate(t *testing.T) {
	initTestDB(t)
	svc := newTestService(nil, nil)
	avail := func() map[string]bool {
		out := map[string]bool{}
		for _, tpl := range svc.ListTemplates() {
			out[tpl.Id] = tpl.Available
		}
		return out
	}
	if got := avail(); !got[TemplateVlessReality] || !got[TemplateHysteria2] || got[TemplateTuic] {
		t.Fatalf("availability without panel cert = %v, want vless+hysteria2 only", got)
	}
	usePanelCert(t, "node.example.com")
	if got := avail(); !got[TemplateTuic] {
		t.Fatalf("availability with panel cert = %v, want tuic available", got)
	}
}

// The UDP health check: a server holding the port makes binding it fail.
func TestProbeLocalUDP_DetectsListener(t *testing.T) {
	conn, err := net.ListenPacket("udp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	if err := probeLocalUDP(port, time.Second); err != nil {
		t.Fatalf("probe with a listener: %v", err)
	}
	conn.Close()
	if err := probeLocalUDP(port, 700*time.Millisecond); err == nil {
		t.Fatal("probe without a listener reported healthy")
	}
}
