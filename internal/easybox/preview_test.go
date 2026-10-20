package easybox

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// EasyBox.md §43: "Preview as" shows exactly what each client would receive,
// rendered by the same code the subscription server uses.
func TestPreviewSubscription_PerClientFormat(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	svc := newTestService(nil, nil)
	createLine(t, svc, TemplateVlessReality, "line-vless")
	createLine(t, svc, TemplateHysteria2, "line-hy2")

	for _, client := range []string{"v2rayn", "shadowrocket", "passwall"} {
		p, err := svc.PreviewSubscription(client, "203.0.113.10:2053")
		if err != nil {
			t.Fatalf("preview %s: %v", client, err)
		}
		if p.Format != "raw" || !strings.Contains(p.Body, "vless://") || !strings.Contains(p.Body, "hysteria2://") {
			t.Fatalf("preview %s = %+v, want raw links for both lines", client, p)
		}
	}

	mh, err := svc.PreviewSubscription("mihomo", "203.0.113.10:2053")
	if err != nil {
		t.Fatal(err)
	}
	if mh.Format != "clash" || !strings.Contains(mh.Body, "name: line-vless") || !strings.Contains(mh.Body, "name: line-hy2") {
		t.Fatalf("mihomo preview = %+v, want Clash YAML with both lines", mh)
	}

	sb, err := svc.PreviewSubscription("singbox", "203.0.113.10:2053")
	if err != nil {
		t.Fatal(err)
	}
	var cfg singboxConfig
	if sb.Format != "singbox" || json.Unmarshal([]byte(sb.Body), &cfg) != nil || outboundByTag(cfg, "line-hy2") == nil {
		t.Fatalf("singbox preview = %+v, want sing-box JSON with the Hysteria2 line", sb)
	}
	if len(sb.Skipped) != 1 || !strings.Contains(sb.Skipped[0], "line-vless") {
		t.Fatalf("singbox skipped = %v, want the REALITY line explained", sb.Skipped)
	}
}

func TestPreviewSubscription_UnknownClient(t *testing.T) {
	initTestDB(t)
	if _, err := newTestService(nil, nil).PreviewSubscription("netscape", "203.0.113.10"); !errors.Is(err, ErrUnknownPreviewClient) {
		t.Fatalf("err = %v, want ErrUnknownPreviewClient", err)
	}
}
