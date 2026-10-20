package easybox

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/util/link"
)

// EasyBox.md §22: each line has its own share link (Copy / QR) that must match
// what the subscription serves: same name, address and identity.
func TestGetEndpointLink_MatchesSubscriptionLine(t *testing.T) {
	cases := []struct {
		name, address, host, wantAddr string
	}{
		{"custom server address", "node.example.com", "panel.example.com:2053", "node.example.com"},
		{"falls back to the panel host", "", "203.0.113.10:2053", "203.0.113.10"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			initTestDB(t)
			mustInit(t)
			svc := newTestService(nil, nil)
			ep, err := svc.CreateEndpoint(CreateEndpointRequest{Name: "US-Oracle-01", Address: tc.address})
			if err != nil {
				t.Fatal(err)
			}
			raw, err := svc.GetEndpointLink(ep.Id, tc.host)
			if err != nil {
				t.Fatalf("GetEndpointLink: %v", err)
			}
			res, err := link.ParseLink(raw)
			if err != nil {
				t.Fatalf("ParseLink(%q): %v", raw, err)
			}
			id, _ := EnsureDefaultSubscription()
			settings := res.Outbound["settings"].(map[string]any)
			if settings["address"] != tc.wantAddr || settings["port"] != ep.Port || settings["id"] != id.UUID {
				t.Fatalf("link settings = %v, want %s:%d with the subscription UUID", settings, tc.wantAddr, ep.Port)
			}
			if res.Outbound["tag"] != "US-Oracle-01" {
				t.Fatalf("link name = %v, want the line name", res.Outbound["tag"])
			}
		})
	}
}
