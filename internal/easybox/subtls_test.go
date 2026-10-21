package easybox

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func subTLS(t *testing.T) (string, string) {
	t.Helper()
	settings := &service.SettingService{}
	certFile, err := settings.GetSubCertFile()
	if err != nil {
		t.Fatal(err)
	}
	keyFile, err := settings.GetSubKeyFile()
	if err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile
}

// EasyBox.md §9.1: the subscription carries every line's credentials, so it
// must be served over HTTPS as soon as the panel has a certificate.
func TestInit_SubscriptionUsesPanelCertificate(t *testing.T) {
	initTestDB(t)
	usePanelCert(t, "203.0.113.10")
	mustInit(t)
	settings := &service.SettingService{}
	panelCert, _ := settings.GetCertFile()
	panelKey, _ := settings.GetKeyFile()
	if c, k := subTLS(t); c != panelCert || k != panelKey {
		t.Fatalf("sub cert/key = %q/%q, want the panel's %q/%q", c, k, panelCert, panelKey)
	}
}

// install.sh may issue the panel certificate after the first start; the
// default must wait for it instead of being spent on an HTTP-only panel.
func TestInit_SubscriptionTLSWaitsForPanelCertificate(t *testing.T) {
	initTestDB(t)
	mustInit(t)
	if c, _ := subTLS(t); c != "" {
		t.Fatalf("sub cert = %q without a panel certificate, want empty", c)
	}
	usePanelCert(t, "203.0.113.10")
	mustInit(t)
	if c, _ := subTLS(t); c == "" {
		t.Fatal("sub cert still empty after the panel certificate appeared")
	}
}

func TestInit_SubscriptionTLSRespectsAdmin(t *testing.T) {
	initTestDB(t)
	settings := &service.SettingService{}
	if err := settings.SetSubCertFile("/etc/ssl/sub.crt"); err != nil {
		t.Fatal(err)
	}
	if err := settings.SetSubKeyFile("/etc/ssl/sub.key"); err != nil {
		t.Fatal(err)
	}
	usePanelCert(t, "203.0.113.10")
	mustInit(t)
	if c, k := subTLS(t); c != "/etc/ssl/sub.crt" || k != "/etc/ssl/sub.key" {
		t.Fatalf("sub cert/key = %q/%q, want the admin's own kept", c, k)
	}

	// Applied once: an admin who later turns sub TLS off is not overridden.
	initTestDB(t)
	usePanelCert(t, "203.0.113.10")
	mustInit(t)
	if err := settings.SetSubCertFile(""); err != nil {
		t.Fatal(err)
	}
	if err := settings.SetSubKeyFile(""); err != nil {
		t.Fatal(err)
	}
	mustInit(t)
	if c, _ := subTLS(t); c != "" {
		t.Fatalf("sub cert = %q after the admin cleared it, want it left empty", c)
	}
}
