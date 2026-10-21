package sub

import (
	"errors"
	"strings"
	"testing"
)

// EasyBox.md §30, §76: official sing-box apps get sing-box JSON, but a known
// version below 1.12 falls back to raw because the config format changed.
func TestSingboxClient(t *testing.T) {
	cases := []struct {
		ua                string
		isSingbox, tooOld bool
	}{
		{"SFA/1.12.4 (8; sing-box 1.12.4; language zh_CN)", true, false},
		{"SFI/1.14.3 (iOS 18.1; sing-box 1.14.3)", true, false},
		{"SFA/1.11.9 (8; sing-box 1.11.9; language en_US)", true, true},
		{"SFM/1.13.0", true, false},
		{"sing-box 1.10.1", true, true},
		{"v2rayN/7.10.5", false, false},
		{"clash.meta/1.19.2", false, false},
	}
	for _, tc := range cases {
		isSingbox, tooOld := SingboxClient(tc.ua)
		if isSingbox != tc.isSingbox || tooOld != tc.tooOld {
			t.Errorf("SingboxClient(%q) = %v, %v; want %v, %v", tc.ua, isSingbox, tooOld, tc.isSingbox, tc.tooOld)
		}
	}
}

// A REALITY line renders for sing-box unless the server advertises
// X25519MLKEM768, which sing-box 1.14.3 measurably cannot verify.
func TestCanonicalVlessReality(t *testing.T) {
	const link = "vless://11111111-2222-3333-4444-555555555555@203.0.113.10:20042?type=tcp&security=reality&pbk=PUBKEY&sid=ab12&sni=www.example.com&fp=chrome&flow=xtls-rprx-vision#line"
	n, err := canonicalFromLink(link)
	if err != nil {
		t.Fatalf("canonicalFromLink: %v", err)
	}
	out := singboxOutbound(n)
	tls := out["tls"].(map[string]any)
	reality := tls["reality"].(map[string]any)
	if out["type"] != "vless" || out["uuid"] != "11111111-2222-3333-4444-555555555555" || out["flow"] != "xtls-rprx-vision" ||
		out["server_port"] != 20042 || tls["server_name"] != "www.example.com" ||
		reality["public_key"] != "PUBKEY" || reality["short_id"] != "ab12" {
		t.Fatalf("sing-box outbound = %v", out)
	}
	mlkem := strings.Replace(link, "#line", "&support-x25519mlkem768=true#line", 1)
	if _, err := canonicalFromLink(mlkem); !errors.Is(err, errSingboxRealityMLKEM) {
		t.Fatalf("err = %v, want errSingboxRealityMLKEM", err)
	}
}
