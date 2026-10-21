package sub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// A sing-box app pointed at the Clash address gets its own JSON config instead
// of YAML it cannot parse; Clash clients on that address still get YAML.
func TestClashAddressServesSingboxApps(t *testing.T) {
	initSubDB(t)
	subID := "clash-singbox"
	seedHysteriaSubInbound(t, subID)
	router := clashSubRouter(t)

	for _, path := range []string{"/clash/" + subID, "/mihomo/" + subID} {
		w := fetchClashSubUA(t, router, path, "SFA/1.14.3 (8; sing-box 1.14.3; language en_US)")
		if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Fatalf("%s: sing-box got Content-Type %q, body=%s", path, ct, w.Body.String())
		}
		var cfg map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
			t.Fatalf("%s: sing-box body is not JSON: %v", path, err)
		}
	}

	for _, ua := range []string{"mihomo/1.19.32", "SFI/1.11.4 (Build 1; sing-box 1.11.4; language zh_CN)"} {
		w := fetchClashSubUA(t, router, "/clash/"+subID, ua)
		if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/yaml") {
			t.Fatalf("%s: got Content-Type %q, want YAML", ua, ct)
		}
	}
}

func TestGenHysteriaLinkOmitsEmptyFingerprint(t *testing.T) {
	link := func(fp string) url.Values {
		in := &model.Inbound{
			Id: 910101, Listen: "203.0.113.1", Port: 443, Protocol: model.Hysteria,
			Settings:       `{"version":2,"clients":[{"auth":"secret","email":"user"}]}`,
			StreamSettings: `{"security":"tls","tlsSettings":{"serverName":"203.0.113.1","settings":{"fingerprint":"` + fp + `"}}}`,
		}
		u, err := url.Parse((&SubService{}).genHysteriaLink(in, "user"))
		if err != nil {
			t.Fatal(err)
		}
		return u.Query()
	}
	if q := link(""); q.Has("fp") {
		t.Fatalf("empty fingerprint still exported: fp=%q", q.Get("fp"))
	}
	if got := link("chrome").Get("fp"); got != "chrome" {
		t.Fatalf("fp = %q, want chrome", got)
	}
}

// seedHysteriaSubInbound seeds one Hysteria2 line with a trusted certificate,
// a protocol sing-box can express.
func seedHysteriaSubInbound(t *testing.T, subID string) {
	t.Helper()
	db := database.GetDB()
	ib := &model.Inbound{
		UserId: 1, Tag: "hy2", Enable: true, Listen: "203.0.113.5", Port: 20001,
		Protocol: model.Hysteria, Remark: "hy2",
		Settings:       `{"version":2,"clients":[{"auth":"hyauth","email":"hy2@e","subId":"` + subID + `","enable":true}]}`,
		StreamSettings: `{"security":"tls","tlsSettings":{"serverName":"hy.sni","alpn":["h3"]}}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	client := &model.ClientRecord{Email: "hy2@e", SubID: subID, Auth: "hyauth", Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client_inbound: %v", err)
	}
}

func fetchClashSubUA(t *testing.T, router http.Handler, path, ua string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = "sub.example.com"
	req.Header.Set("User-Agent", ua)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}
