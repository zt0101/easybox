package easybox

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type singboxConfig struct {
	Outbounds []map[string]any `json:"outbounds"`
	Route     struct {
		Final string `json:"final"`
	} `json:"route"`
}

// assertReferencesResolve: sing-box check only decodes; references to outbound
// tags (selector, url-test, route.final) are resolved at runtime, so pin them.
func assertReferencesResolve(t *testing.T, cfg singboxConfig) {
	t.Helper()
	tags := map[string]bool{}
	for _, o := range cfg.Outbounds {
		tags[o["tag"].(string)] = true
	}
	refs := []string{cfg.Route.Final}
	for _, o := range cfg.Outbounds {
		if members, ok := o["outbounds"].([]any); ok {
			for _, m := range members {
				refs = append(refs, m.(string))
			}
		}
	}
	for _, r := range refs {
		if !tags[r] {
			t.Fatalf("config references outbound %q that does not exist (tags %v)", r, tags)
		}
	}
}

func renderSingbox(t *testing.T) ([]byte, []string) {
	t.Helper()
	tmpl, err := (&service.SettingService{}).GetRemarkTemplate()
	if err != nil {
		t.Fatal(err)
	}
	id, _ := EnsureDefaultSubscription()
	body, skipped, _, err := sub.NewSubService(tmpl).GetSingboxConfig(id.SubID, "203.0.113.10")
	if err != nil {
		t.Fatalf("GetSingboxConfig: %v", err)
	}
	return body, skipped
}

// checkWithSingbox validates the config with the real binary when
// SINGBOX_BINARY is set (devenv: /root/.cache/clients/sing-box).
func checkWithSingbox(t *testing.T, body []byte) {
	t.Helper()
	bin := os.Getenv("SINGBOX_BINARY")
	if bin == "" {
		t.Log("SINGBOX_BINARY unset: structural checks only")
		return
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bin, "check", "-c", path).CombinedOutput(); err != nil {
		t.Fatalf("sing-box check rejected the config: %v\n%s\n%s", err, out, body)
	}
}

func outboundByTag(cfg singboxConfig, tag string) map[string]any {
	for _, o := range cfg.Outbounds {
		if o["tag"] == tag {
			return o
		}
	}
	return nil
}

// EasyBox.md §25-§28, §34: one sing-box config carries every line sing-box can
// use; a self-signed Hysteria2 line embeds the certificate sing-box must trust,
// and the REALITY line (X25519MLKEM768, measured to fail) is left out with a reason.
func TestSingboxConfig_RendersUsableLines(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	svc := newTestService(nil, nil)
	createLine(t, svc, TemplateVlessReality, "line-vless")
	createLine(t, svc, TemplateHysteria2, "line-hy2")
	usePanelCert(t, "node.example.com")
	createLine(t, svc, TemplateTuic, "line-tuic")

	body, skipped := renderSingbox(t)
	if len(skipped) != 1 || !strings.Contains(skipped[0], "line-vless") || !strings.Contains(skipped[0], "X25519MLKEM768") {
		t.Fatalf("skipped = %v, want only the REALITY line, with the MLKEM reason", skipped)
	}
	var cfg singboxConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		t.Fatal(err)
	}
	id, _ := EnsureDefaultSubscription()
	if outboundByTag(cfg, "line-vless") != nil {
		t.Fatal("REALITY line shipped to sing-box although it cannot connect")
	}
	hy2 := outboundByTag(cfg, "line-hy2")
	cert, _ := hy2["tls"].(map[string]any)["certificate"].(string)
	if hy2["type"] != "hysteria2" || hy2["password"] != id.Auth || !strings.Contains(cert, "BEGIN CERTIFICATE") {
		t.Fatalf("hysteria2 outbound = %v, want auth and the embedded self-signed certificate", hy2)
	}
	if obfs, _ := hy2["obfs"].(map[string]any); obfs["type"] != "salamander" || obfs["password"] == "" {
		t.Fatalf("hysteria2 obfs = %v, want salamander", hy2["obfs"])
	}
	tuic := outboundByTag(cfg, "line-tuic")
	if tuic["type"] != "tuic" || tuic["uuid"] != id.UUID || tuic["password"] != id.Password {
		t.Fatalf("tuic outbound = %v", tuic)
	}
	selector := outboundByTag(cfg, "proxy")
	if members, _ := selector["outbounds"].([]any); len(members) != 3 {
		t.Fatalf("proxy selector = %v, want auto + 2 lines", selector["outbounds"])
	}
	assertReferencesResolve(t, cfg)
	checkWithSingbox(t, body)
}

// EasyBox.md §34: a line sing-box cannot express is left out, not shipped broken.
func TestSingboxConfig_SkipsUnsupportedLine(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	svc := newTestService(nil, nil)
	createLine(t, svc, TemplateHysteria2, "line-hy2")
	id, _ := EnsureDefaultSubscription()
	ws, err := BuildVlessRealityInbound("legacy-ws", 20050, id, testReality)
	if err != nil {
		t.Fatal(err)
	}
	ws.StreamSettings = `{"network":"ws","security":"none","wsSettings":{"path":"/ws"}}`
	ws.Settings = strings.Replace(ws.Settings, `"flow":"xtls-rprx-vision"`, `"flow":""`, 1)
	if _, _, err := (&service.InboundService{}).AddInbound(ws); err != nil {
		t.Fatalf("AddInbound(ws): %v", err)
	}

	body, skipped := renderSingbox(t)
	if len(skipped) != 1 || !strings.Contains(skipped[0], "legacy-ws") {
		t.Fatalf("skipped = %v, want the ws line", skipped)
	}
	var cfg singboxConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		t.Fatal(err)
	}
	if outboundByTag(cfg, "line-hy2") == nil || outboundByTag(cfg, "legacy-ws") != nil {
		t.Fatalf("outbounds = %v, want only the Hysteria2 line", cfg.Outbounds)
	}
	checkWithSingbox(t, body)
}

func TestSingboxConfig_NothingToRender(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	if body, _ := renderSingbox(t); body != nil {
		t.Fatalf("body = %s, want nil without lines", body)
	}
}
