package easybox

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const (
	testCFToken  = "cf-test-token-0123456789abcdef"
	testCFDomain = "example.com"
	testCFZone   = "zone-1"
	testServerIP = "203.0.113.7"
)

type fakeRecord struct {
	Name, Content string
	Proxied       bool
}

// fakeCloudflare implements the Cloudflare v4 calls a CDN line makes and
// signs Origin CA certificates with its own test CA.
type fakeCloudflare struct {
	t          *testing.T
	mu         sync.Mutex
	zoneStatus string
	sslMode    string
	sslPatches []string
	records    map[string]fakeRecord
	certs      map[string]string // id → host
	revoked    []string
	failCert   bool
	nextID     int
	caKey      *ecdsa.PrivateKey
	caCert     *x509.Certificate
}

func newFakeCloudflare(t *testing.T) *fakeCloudflare {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Fake Origin CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(20, 0, 0),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	ca, _ := x509.ParseCertificate(der)
	f := &fakeCloudflare{
		t: t, zoneStatus: "active", sslMode: "flexible",
		records: map[string]fakeRecord{}, certs: map[string]string{}, caKey: key, caCert: ca,
	}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	old := cloudflareAPI
	cloudflareAPI = srv.URL
	t.Cleanup(func() { cloudflareAPI = old; srv.Close() })
	return f
}

func (f *fakeCloudflare) reply(w http.ResponseWriter, status int, result any, errs ...string) {
	out := map[string]any{"success": len(errs) == 0, "result": result, "errors": []any{}}
	if len(errs) > 0 {
		e := make([]any, 0, len(errs))
		for _, m := range errs {
			e = append(e, map[string]any{"code": 1000, "message": m})
		}
		out["errors"] = e
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(out)
}

func (f *fakeCloudflare) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer "+testCFToken {
		// What api.cloudflare.com answers to a malformed token.
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false, "result": nil,
			"errors": []any{map[string]any{"code": 6003, "message": "Invalid request headers"}},
		})
		return
	}
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	zonePrefix := "/zones/" + testCFZone
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/zones":
		zones := []any{}
		if r.URL.Query().Get("name") == testCFDomain {
			zones = append(zones, map[string]any{"id": testCFZone, "name": testCFDomain, "status": f.zoneStatus})
		}
		f.reply(w, http.StatusOK, zones)
	case r.URL.Path == zonePrefix+"/settings/ssl" && r.Method == http.MethodGet:
		f.reply(w, http.StatusOK, map[string]any{"id": "ssl", "value": f.sslMode})
	case r.URL.Path == zonePrefix+"/settings/ssl" && r.Method == http.MethodPatch:
		f.sslMode, _ = body["value"].(string)
		f.sslPatches = append(f.sslPatches, f.sslMode)
		f.reply(w, http.StatusOK, map[string]any{"id": "ssl", "value": f.sslMode})
	case r.URL.Path == zonePrefix+"/dns_records" && r.Method == http.MethodPost:
		f.nextID++
		id := "rec-" + string(rune('0'+f.nextID))
		proxied, _ := body["proxied"].(bool)
		f.records[id] = fakeRecord{Name: body["name"].(string), Content: body["content"].(string), Proxied: proxied}
		f.reply(w, http.StatusOK, map[string]any{"id": id})
	case strings.HasPrefix(r.URL.Path, zonePrefix+"/dns_records/") && r.Method == http.MethodDelete:
		id := strings.TrimPrefix(r.URL.Path, zonePrefix+"/dns_records/")
		if _, ok := f.records[id]; !ok {
			f.reply(w, http.StatusNotFound, nil, "Record not found")
			return
		}
		delete(f.records, id)
		f.reply(w, http.StatusOK, map[string]any{"id": id})
	case r.URL.Path == "/certificates" && r.Method == http.MethodPost:
		if f.failCert {
			f.reply(w, http.StatusForbidden, nil, "Unauthorized to access requested resource")
			return
		}
		f.reply(w, http.StatusOK, f.sign(body))
	case strings.HasPrefix(r.URL.Path, "/certificates/") && r.Method == http.MethodDelete:
		id := strings.TrimPrefix(r.URL.Path, "/certificates/")
		f.revoked = append(f.revoked, id)
		delete(f.certs, id)
		f.reply(w, http.StatusOK, map[string]any{"id": id})
	default:
		f.t.Errorf("unexpected Cloudflare call %s %s", r.Method, r.URL)
		f.reply(w, http.StatusNotFound, nil, "unexpected call")
	}
}

func (f *fakeCloudflare) sign(body map[string]any) map[string]any {
	block, _ := pem.Decode([]byte(body["csr"].(string)))
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil || csr.CheckSignature() != nil {
		f.t.Errorf("bad CSR: %v", err)
	}
	hosts, _ := body["hostnames"].([]any)
	host, _ := hosts[0].(string)
	if body["request_type"] != "origin-ecc" || body["requested_validity"] != float64(originCertValidityDays) {
		f.t.Errorf("certificate request = %v", body)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(int64(len(f.certs) + 2)), Subject: pkix.Name{CommonName: host},
		DNSNames: []string{host}, NotBefore: time.Now(), NotAfter: time.Now().AddDate(15, 0, 0),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, f.caCert, csr.PublicKey, f.caKey)
	if err != nil {
		f.t.Fatal(err)
	}
	f.nextID++
	id := "cert-" + string(rune('0'+f.nextID))
	f.certs[id] = host
	return map[string]any{"id": id, "certificate": string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))}
}

func (f *fakeCloudflare) snapshot() (map[string]fakeRecord, map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	recs, certs := map[string]fakeRecord{}, map[string]string{}
	for k, v := range f.records {
		recs[k] = v
	}
	for k, v := range f.certs {
		certs[k] = v
	}
	return recs, certs
}

func cdnTestService(healthErr error, busy ...int) *Service {
	svc := newTestService(nil, healthErr)
	svc.publicIPv4 = func(string) (string, error) { return testServerIP, nil }
	svc.portBindable = func(p int) bool {
		for _, b := range busy {
			if b == p {
				return false
			}
		}
		return true
	}
	return svc
}

func configureCloudflare(t *testing.T, svc *Service) {
	t.Helper()
	if _, err := svc.SetCloudflare(CloudflareRequest{Token: testCFToken, Domain: testCFDomain}); err != nil {
		t.Fatalf("SetCloudflare: %v", err)
	}
}

var cdnHost = regexp.MustCompile(`^cdn-[0-9a-z]{6}\.example\.com$`)

func TestSetCloudflare_ChecksTokenAndNeverReturnsIt(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	newFakeCloudflare(t)
	svc := cdnTestService(nil)

	if _, err := svc.SetCloudflare(CloudflareRequest{Token: "wrong-token-123456", Domain: testCFDomain}); err == nil || !strings.Contains(err.Error(), "rejected the API token") {
		t.Fatalf("bad token: err = %v, want Cloudflare's rejection worded for the admin", err)
	}
	if _, ok, _ := loadCloudflare(); ok {
		t.Fatal("a rejected token was stored")
	}
	view, err := svc.SetCloudflare(CloudflareRequest{Token: " " + testCFToken + " ", Domain: "https://Example.COM./"})
	if err != nil {
		t.Fatalf("SetCloudflare: %v", err)
	}
	if !view.Configured || view.Domain != testCFDomain || view.TokenHint != "…cdef" {
		t.Fatalf("view = %+v", view)
	}
	raw, _ := json.Marshal(view)
	got, _ := svc.GetCloudflare()
	raw2, _ := json.Marshal(got)
	var logs []model.EasyboxAuditLog
	database.GetDB().Find(&logs)
	logRaw, _ := json.Marshal(logs)
	for _, s := range []string{string(raw), string(raw2), string(logRaw)} {
		if strings.Contains(s, testCFToken) {
			t.Fatalf("token leaked: %s", s)
		}
	}
	row, _, _ := loadCloudflare()
	if row.APIToken != testCFToken || row.ZoneID != testCFZone {
		t.Fatalf("stored row = domain %q zone %q", row.Domain, row.ZoneID)
	}
}

func TestSetCloudflare_RejectsDomainNotYetActive(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	newFakeCloudflare(t).zoneStatus = "pending"
	_, err := cdnTestService(nil).SetCloudflare(CloudflareRequest{Token: testCFToken, Domain: testCFDomain})
	if err == nil || !strings.Contains(err.Error(), "not active") {
		t.Fatalf("err = %v, want a not-active error", err)
	}
}

func TestCDNTemplate_AvailableOnceCloudflareIsSet(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	newFakeCloudflare(t)
	svc := cdnTestService(nil)
	available := func() TemplateView {
		for _, tpl := range svc.ListTemplates() {
			if tpl.Id == TemplateVlessCDN {
				return tpl
			}
		}
		t.Fatal("no vless-cdn template")
		return TemplateView{}
	}
	if tpl := available(); tpl.Available || !strings.Contains(tpl.Reason, "Cloudflare") {
		t.Fatalf("template = %+v, want unavailable with a Cloudflare reason", tpl)
	}
	configureCloudflare(t, svc)
	if tpl := available(); !tpl.Available {
		t.Fatalf("template = %+v, want available after Cloudflare was set", tpl)
	}
}

func TestCDNLine_ProxiedRecordOriginCertAndWebSocketInbound(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	cf := newFakeCloudflare(t)
	svc := cdnTestService(nil)
	configureCloudflare(t, svc)
	ep := createLine(t, svc, TemplateVlessCDN, "HK-CDN")
	if ep.Port != 443 || ep.Status != StatusRunning || !cdnHost.MatchString(ep.Address) {
		t.Fatalf("line = %+v, want RUNNING on 443 at cdn-<label>.example.com", ep)
	}
	recs, certs := cf.snapshot()
	if len(recs) != 1 || len(certs) != 1 {
		t.Fatalf("records %v certs %v, want one of each", recs, certs)
	}
	for _, r := range recs {
		if r.Name != ep.Address || r.Content != testServerIP || !r.Proxied {
			t.Fatalf("record = %+v, want proxied %s → %s", r, ep.Address, testServerIP)
		}
	}
	if len(cf.sslPatches) != 1 || cf.sslPatches[0] != "full" {
		t.Fatalf("SSL mode patches = %v, want flexible raised to full once", cf.sslPatches)
	}

	u := parseLineURL(t, lineLink(t, svc, ep))
	q := u.Query()
	want := map[string]string{"type": "ws", "security": "tls", "sni": ep.Address, "host": ep.Address, "fp": "chrome"}
	for k, v := range want {
		if q.Get(k) != v {
			t.Fatalf("link %s=%q, want %q (link %q)", k, q.Get(k), v, u)
		}
	}
	if u.Hostname() != ep.Address || u.Port() != "443" || !regexp.MustCompile(`^/[0-9a-z]{10}$`).MatchString(q.Get("path")) {
		t.Fatalf("link %q, want the CDN host on 443 with a random path", u)
	}

	var in model.Inbound
	database.GetDB().First(&in, ep.InboundId)
	var stream struct {
		TLSSettings struct {
			Certificates []struct {
				Certificate []string `json:"certificate"`
			} `json:"certificates"`
		} `json:"tlsSettings"`
	}
	if err := json.Unmarshal([]byte(in.StreamSettings), &stream); err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode([]byte(strings.Join(stream.TLSSettings.Certificates[0].Certificate, "\n")))
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil || leaf.CheckSignatureFrom(cf.caCert) != nil || leaf.DNSNames[0] != ep.Address {
		t.Fatalf("inbound certificate is not the Origin CA one for %s: %v", ep.Address, err)
	}
	var row model.EasyboxEndpoint
	database.GetDB().First(&row, ep.Id)
	if row.CdnHost != ep.Address || row.CdnRecordID == "" || row.CdnCertID == "" || row.CdnZoneID != testCFZone {
		t.Fatalf("row = %+v, want the Cloudflare resources recorded", row)
	}
}

func TestCDNLine_LeavesFullOrStrictModeAlone(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	cf := newFakeCloudflare(t)
	cf.sslMode = "strict"
	svc := cdnTestService(nil)
	configureCloudflare(t, svc)
	createLine(t, svc, TemplateVlessCDN, "a")
	if len(cf.sslPatches) != 0 {
		t.Fatalf("SSL mode patched %v; strict applies to the whole domain and must stay", cf.sslPatches)
	}
}

func TestCDNLine_SkipsPortsHeldByOtherPrograms(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	newFakeCloudflare(t)
	svc := cdnTestService(nil, 443)
	configureCloudflare(t, svc)
	if ep := createLine(t, svc, TemplateVlessCDN, "a"); ep.Port != 8443 {
		t.Fatalf("port = %d, want 8443 when 443 is busy", ep.Port)
	}
	// 2053 is also the panel's default port, so the next free one is 2083.
	if ep := createLine(t, svc, TemplateVlessCDN, "b"); ep.Port != 2083 {
		t.Fatalf("second line port = %d, want 2083", ep.Port)
	}
}

func TestCDNLine_FailedHealthCheckRemovesCloudflareResources(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	cf := newFakeCloudflare(t)
	svc := cdnTestService(errors.New("port not accepting"))
	configureCloudflare(t, svc)
	ep, err := svc.CreateEndpoint(CreateEndpointRequest{Template: TemplateVlessCDN})
	if err == nil || ep.FailedStep != StepHealthCheck {
		t.Fatalf("err = %v, line = %+v; want a health-check failure", err, ep)
	}
	recs, certs := cf.snapshot()
	if len(recs) != 0 || len(certs) != 0 || len(cf.revoked) != 1 {
		t.Fatalf("records %v certs %v revoked %v; want everything removed", recs, certs, cf.revoked)
	}
	if n := countInbounds(t); n != 0 {
		t.Fatalf("inbounds = %d, want 0", n)
	}
}

func TestCDNLine_CertificateFailureRemovesRecord(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	cf := newFakeCloudflare(t)
	cf.failCert = true
	svc := cdnTestService(nil)
	configureCloudflare(t, svc)
	ep, err := svc.CreateEndpoint(CreateEndpointRequest{Template: TemplateVlessCDN})
	if err == nil || ep.FailedStep != StepPrepare || !strings.Contains(err.Error(), "issue origin certificate: the API token lacks a permission") {
		t.Fatalf("err = %v, line = %+v; want a prepare failure naming the certificate permission", err, ep)
	}
	if recs, _ := cf.snapshot(); len(recs) != 0 {
		t.Fatalf("records %v left behind", recs)
	}
}

func TestDeleteCDNLine_RemovesRecordAndRevokesCertificate(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	cf := newFakeCloudflare(t)
	svc := cdnTestService(nil)
	configureCloudflare(t, svc)
	ep := createLine(t, svc, TemplateVlessCDN, "a")
	if err := svc.DeleteEndpoint(ep.Id); err != nil {
		t.Fatalf("DeleteEndpoint: %v", err)
	}
	recs, certs := cf.snapshot()
	if len(recs) != 0 || len(certs) != 0 {
		t.Fatalf("records %v certs %v left after delete", recs, certs)
	}
}

func TestDeleteCDNLine_WithoutTokenNamesTheLeftoverRecord(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	cf := newFakeCloudflare(t)
	svc := cdnTestService(nil)
	configureCloudflare(t, svc)
	ep := createLine(t, svc, TemplateVlessCDN, "a")
	if err := svc.ClearCloudflare(); err != nil {
		t.Fatal(err)
	}
	err := svc.DeleteEndpoint(ep.Id)
	if err == nil || !strings.Contains(err.Error(), ep.Address) {
		t.Fatalf("err = %v, want it to name the DNS record %s left behind", err, ep.Address)
	}
	if _, lerr := loadEndpoint(ep.Id); lerr == nil {
		t.Fatal("line still listed after delete")
	}
	if recs, _ := cf.snapshot(); len(recs) != 1 {
		t.Fatalf("records = %v; without a token nothing can be removed", recs)
	}
}

func TestRegenerateCDNLine_ReplacesRecordAndCertificate(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	cf := newFakeCloudflare(t)
	svc := cdnTestService(nil)
	configureCloudflare(t, svc)
	ep := createLine(t, svc, TemplateVlessCDN, "a")
	next, err := svc.RegenerateEndpoint(ep.Id, false)
	if err != nil {
		t.Fatalf("RegenerateEndpoint: %v", err)
	}
	if next.Address == ep.Address || !cdnHost.MatchString(next.Address) {
		t.Fatalf("regenerated address %q, want a new cdn host (old %q)", next.Address, ep.Address)
	}
	recs, certs := cf.snapshot()
	if len(recs) != 1 || len(certs) != 1 {
		t.Fatalf("records %v certs %v, want only the new line's", recs, certs)
	}
	for _, r := range recs {
		if r.Name != next.Address {
			t.Fatalf("record %s left, want %s", r.Name, next.Address)
		}
	}
}

func TestSingboxConfig_RendersCDNLineOverWebSocket(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	newFakeCloudflare(t)
	svc := cdnTestService(nil)
	configureCloudflare(t, svc)
	ep := createLine(t, svc, TemplateVlessCDN, "line-cdn")
	body, skipped := renderSingbox(t)
	if len(skipped) != 0 {
		t.Fatalf("skipped = %v, want none", skipped)
	}
	var cfg singboxConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		t.Fatal(err)
	}
	out := outboundByTag(cfg, "line-cdn")
	tr, _ := out["transport"].(map[string]any)
	headers, _ := tr["headers"].(map[string]any)
	tls, _ := out["tls"].(map[string]any)
	if out["server"] != ep.Address || out["server_port"] != float64(443) || tr["type"] != "ws" ||
		headers["Host"] != ep.Address || tls["server_name"] != ep.Address || tls["reality"] != nil {
		t.Fatalf("outbound = %v", out)
	}
	checkWithSingbox(t, body)
}
