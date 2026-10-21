package easybox

import (
	"encoding/json"
	"errors"
	"net/url"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/link"
)

func parseLineURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	// Upstream's parser has no tuic:// support; every other EasyBox link must parse.
	if _, err := link.ParseLink(raw); err != nil && u.Scheme != "tuic" {
		t.Fatalf("upstream link parser rejects %q: %v", raw, err)
	}
	return u
}

func TestTLSLines_RequireTrustedCertificate(t *testing.T) {
	for _, tpl := range []string{TemplateVlessTLS, TemplateTrojan} {
		t.Run(tpl, func(t *testing.T) {
			initTestDB(t)
			mustInit(t)
			ep, err := newTestService(nil, nil).CreateEndpoint(CreateEndpointRequest{Template: tpl})
			if !errors.Is(err, ErrNoTrustedCertificate) || ep.FailedStep != StepPrepare {
				t.Fatalf("err = %v, line = %+v; want ErrNoTrustedCertificate at prepare", err, ep)
			}
			if n := countInbounds(t); n != 0 {
				t.Fatalf("inbounds = %d, want 0", n)
			}
		})
	}
}

func TestVlessTLSLine_UsesPanelCertificateAndVision(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	usePanelCert(t, "203.0.113.10")
	svc := newTestService(nil, nil)
	ep := createLine(t, svc, TemplateVlessTLS, "HK-VLESS-TLS")
	if ep.Protocol != "vless" || ep.Transport != "tcp" || ep.Status != StatusRunning {
		t.Fatalf("line = %+v, want a RUNNING vless line over tcp", ep)
	}
	u := parseLineURL(t, lineLink(t, svc, ep))
	id, _ := EnsureDefaultSubscription()
	q := u.Query()
	if u.Scheme != "vless" || u.User.Username() != id.UUID {
		t.Fatalf("link %q, want vless for the subscription user's uuid", u)
	}
	want := map[string]string{"security": "tls", "type": "tcp", "sni": "203.0.113.10", "flow": "xtls-rprx-vision", "fp": "chrome"}
	for k, v := range want {
		if q.Get(k) != v {
			t.Fatalf("link %s=%q, want %q (link %q)", k, q.Get(k), v, u)
		}
	}
	if q.Get("pbk") != "" || q.Get("allowInsecure") == "1" {
		t.Fatalf("link %q carries REALITY keys or allows insecure TLS", u)
	}
}

func TestTrojanLine_UsesPanelCertificate(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	usePanelCert(t, "node.example.com")
	svc := newTestService(nil, nil)
	ep := createLine(t, svc, TemplateTrojan, "HK-TROJAN")
	if ep.Protocol != "trojan" || ep.Transport != "tcp" {
		t.Fatalf("line = %+v, want trojan over tcp", ep)
	}
	u := parseLineURL(t, lineLink(t, svc, ep))
	id, _ := EnsureDefaultSubscription()
	if u.Scheme != "trojan" || u.User.Username() != id.Password || id.Password == "" {
		t.Fatalf("trojan link %q, want the subscription user's password", u)
	}
	if q := u.Query(); q.Get("security") != "tls" || q.Get("sni") != "node.example.com" || q.Get("type") != "tcp" {
		t.Fatalf("trojan link query = %v, want tls over tcp with the panel certificate's name", q)
	}
}

func TestShadowsocksLine_NeedsNoCertificate(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	svc := newTestService(nil, nil)
	ep := createLine(t, svc, TemplateShadowsocks, "HK-SS")
	if ep.Protocol != "shadowsocks" || ep.Transport != "tcp+udp" || ep.Status != StatusRunning {
		t.Fatalf("line = %+v, want a RUNNING shadowsocks line over tcp+udp", ep)
	}
	u := parseLineURL(t, lineLink(t, svc, ep))
	id, _ := EnsureDefaultSubscription()
	keys, _ := u.User.Password()
	if u.Scheme != "ss" || u.User.Username() != ss2022Method {
		t.Fatalf("link %q, want ss with %s", u, ss2022Method)
	}
	serverKey, userKey, ok := cutLast(keys)
	if !ok || !isSS2022Password(serverKey) || userKey != id.Password || !isSS2022Password(userKey) {
		t.Fatalf("ss keys %q, want <16-byte server key>:<subscription password>", keys)
	}

	var ib model.Inbound
	if err := database.GetDB().First(&ib, ep.InboundId).Error; err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Network string `json:"network"`
	}
	if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil || settings.Network != "tcp,udp" {
		t.Fatalf("inbound network = %q (%v), want tcp,udp", settings.Network, err)
	}
}

func cutLast(s string) (string, string, bool) {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}

func TestNewSubscription_PasswordIsShadowsocksKey(t *testing.T) {
	initTestDB(t)
	id, err := EnsureDefaultSubscription()
	if err != nil {
		t.Fatal(err)
	}
	if !isSS2022Password(id.Password) {
		t.Fatalf("password %q is not a 16-byte base64 key", id.Password)
	}
	rotated, err := newTestService(nil, nil).RotateSubscription()
	if err != nil {
		t.Fatal(err)
	}
	if !isSS2022Password(rotated.Password) || rotated.Password == id.Password {
		t.Fatalf("rotated password %q, want a new 16-byte base64 key", rotated.Password)
	}
}

// A subscription user from before Shadowsocks support: the first Shadowsocks
// line upgrades its password, and the TUIC line that shares it follows.
func TestShadowsocksLine_UpgradesLegacyPasswordAcrossLines(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	usePanelCert(t, "node.example.com")
	const legacy = "k3v9x1m2p8q4r7s6"
	useLegacyPassword(t, legacy)
	svc := newTestService(nil, nil)
	tuic := createLine(t, svc, TemplateTuic, "HK-TUIC")
	if pass, _ := parseLineURL(t, lineLink(t, svc, tuic)).User.Password(); pass != legacy {
		t.Fatalf("tuic password before = %q, want the legacy %q", pass, legacy)
	}
	if !templateFlag(svc, TemplateShadowsocks) {
		t.Fatal("ListTemplates: shadowsocks passwordUpgrade = false, want true while a TUIC line holds the legacy password")
	}

	ss := createLine(t, svc, TemplateShadowsocks, "HK-SS")
	id, _ := EnsureDefaultSubscription()
	if id.Password == legacy || !isSS2022Password(id.Password) {
		t.Fatalf("subscription password = %q, want an upgraded 16-byte key", id.Password)
	}
	if pass, _ := parseLineURL(t, lineLink(t, svc, tuic)).User.Password(); pass != id.Password {
		t.Fatalf("tuic password after = %q, want the upgraded %q", pass, id.Password)
	}
	keys, _ := parseLineURL(t, lineLink(t, svc, ss)).User.Password()
	if _, userKey, _ := cutLast(keys); userKey != id.Password {
		t.Fatalf("ss user key = %q, want the upgraded password", userKey)
	}
	if templateFlag(svc, TemplateShadowsocks) {
		t.Fatal("ListTemplates: passwordUpgrade still true after the upgrade")
	}
}

func TestShadowsocksTemplate_NoUpgradeNoticeWithoutPasswordLines(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	useLegacyPassword(t, "k3v9x1m2p8q4r7s6")
	svc := newTestService(nil, nil)
	createLine(t, svc, TemplateVlessReality, "HK-VLESS")
	if templateFlag(svc, TemplateShadowsocks) {
		t.Fatal("passwordUpgrade = true, but no TUIC / Trojan line would change")
	}
}

// useLegacyPassword gives the default subscription user (no lines yet) a
// password in the pre-Shadowsocks format.
func useLegacyPassword(t *testing.T, legacy string) {
	t.Helper()
	if _, err := EnsureDefaultSubscription(); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Model(&model.EasyboxSubscription{}).
		Where("is_default = ?", true).Update("password", legacy).Error; err != nil {
		t.Fatal(err)
	}
}

func templateFlag(svc *Service, id string) bool {
	for _, tpl := range svc.ListTemplates() {
		if tpl.Id == id {
			return tpl.PasswordUpgrade
		}
	}
	return false
}

// A TUIC line's traffic enters Xray through a relay inbound that exists only
// after an Xray restart, so Quick Create applies it at once instead of leaving
// a RUNNING line that cannot carry traffic until upstream's 30-second job.
func TestTuicLine_AppliesXrayRestartAtOnce(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	usePanelCert(t, "node.example.com")
	restarts := 0
	svc := newTestService(nil, nil)
	svc.restart = func() error { restarts++; return nil }
	createLine(t, svc, TemplateTuic, "HK-TUIC")
	if restarts != 1 {
		t.Fatalf("Xray restarts while creating a TUIC line = %d, want 1", restarts)
	}
}

func TestListTemplates_TLSLinesNeedPanelCertificate(t *testing.T) {
	initTestDB(t)
	svc := newTestService(nil, nil)
	avail := func() map[string]bool {
		out := map[string]bool{}
		for _, tpl := range svc.ListTemplates() {
			out[tpl.Id] = tpl.Available
		}
		return out
	}
	if got := avail(); got[TemplateVlessTLS] || got[TemplateTrojan] || !got[TemplateShadowsocks] {
		t.Fatalf("availability without panel cert = %v, want shadowsocks but not vless-tls / trojan", got)
	}
	usePanelCert(t, "node.example.com")
	if got := avail(); !got[TemplateVlessTLS] || !got[TemplateTrojan] {
		t.Fatalf("availability with panel cert = %v, want vless-tls and trojan", got)
	}
}

// sing-box gets every new TCP line: VLESS + TLS (no REALITY, so no MLKEM filter),
// Trojan, and Shadowsocks 2022 with the "serverKey:userKey" password.
func TestSingboxConfig_RendersTCPLines(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	usePanelCert(t, "node.example.com")
	svc := newTestService(nil, nil)
	for _, tpl := range []string{TemplateVlessTLS, TemplateTrojan, TemplateShadowsocks} {
		createLine(t, svc, tpl, "line-"+tpl)
	}
	body, skipped := renderSingbox(t)
	if len(skipped) != 0 {
		t.Fatalf("skipped = %v, want none", skipped)
	}
	var cfg singboxConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		t.Fatal(err)
	}
	assertReferencesResolve(t, cfg)
	id, _ := EnsureDefaultSubscription()

	vless := outboundByTag(cfg, "line-"+TemplateVlessTLS)
	tls, _ := vless["tls"].(map[string]any)
	if vless["type"] != "vless" || vless["flow"] != "xtls-rprx-vision" || tls["server_name"] != "node.example.com" || tls["reality"] != nil {
		t.Fatalf("vless outbound = %v, want TLS + Vision without REALITY", vless)
	}
	trojan := outboundByTag(cfg, "line-"+TemplateTrojan)
	if tls, _ := trojan["tls"].(map[string]any); trojan["type"] != "trojan" || trojan["password"] != id.Password || tls["enabled"] != true {
		t.Fatalf("trojan outbound = %v", trojan)
	}
	ss := outboundByTag(cfg, "line-"+TemplateShadowsocks)
	pw, _ := ss["password"].(string)
	if _, userKey, _ := cutLast(pw); ss["type"] != "shadowsocks" || ss["method"] != ss2022Method || userKey != id.Password || ss["tls"] != nil {
		t.Fatalf("shadowsocks outbound = %v", ss)
	}
	checkWithSingbox(t, body)
}
