package easybox

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

var ErrNoTrustedCertificate = errors.New("no trusted TLS certificate: set the panel certificate first (install.sh issues one for the server IP)")

// lineCertificate is a QUIC line's TLS certificate: either the panel's own
// files or a self-signed one stored inline, which clients must pin.
type lineCertificate struct {
	certFile, keyFile string
	certPEM, keyPEM   string
	serverName        string
	pins              []string
}

const selfSignedServerName = "easybox.internal"

// panelCertificate returns the panel's own TLS certificate (EasyBox.md §9.1);
// ok is false when none is configured. The SNI is the certificate's first name.
func panelCertificate() (lineCertificate, bool, error) {
	settings := &service.SettingService{}
	certFile, err := settings.GetCertFile()
	if err != nil {
		return lineCertificate{}, false, err
	}
	keyFile, err := settings.GetKeyFile()
	if err != nil {
		return lineCertificate{}, false, err
	}
	if strings.TrimSpace(certFile) == "" || strings.TrimSpace(keyFile) == "" {
		return lineCertificate{}, false, nil
	}
	raw, err := os.ReadFile(certFile)
	if err != nil {
		return lineCertificate{}, false, fmt.Errorf("read panel certificate: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return lineCertificate{}, false, errors.New("panel certificate is not PEM")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return lineCertificate{}, false, fmt.Errorf("parse panel certificate: %w", err)
	}
	name := leaf.Subject.CommonName
	switch {
	case len(leaf.DNSNames) > 0:
		name = leaf.DNSNames[0]
	case len(leaf.IPAddresses) > 0:
		name = leaf.IPAddresses[0].String()
	}
	return lineCertificate{certFile: certFile, keyFile: keyFile, serverName: name}, true, nil
}

// hysteriaCertificate prefers the panel's trusted certificate and otherwise
// mints a self-signed one that the subscription pins via pinSHA256.
func hysteriaCertificate() (lineCertificate, error) {
	if cert, ok, err := panelCertificate(); err != nil || ok {
		return cert, err
	}
	return selfSignedCertificate()
}

func selfSignedCertificate() (lineCertificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return lineCertificate{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	if err != nil {
		return lineCertificate{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: selfSignedServerName},
		DNSNames:     []string{selfSignedServerName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return lineCertificate{}, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return lineCertificate{}, err
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	pins, err := (&service.ServerService{}).GetCertHash("", certPEM)
	if err != nil {
		return lineCertificate{}, err
	}
	return lineCertificate{
		certPEM:    certPEM,
		keyPEM:     string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})),
		serverName: selfSignedServerName,
		pins:       pins,
	}, nil
}

// xrayTLSSettings mirrors upstream createHysteriaTlsSettingsWithDefaultCert
// (QUIC: ALPN h3, empty fingerprint) with this certificate filled in.
func xrayTLSSettings(c lineCertificate) map[string]any {
	cert := map[string]any{"oneTimeLoading": false, "usage": "encipherment", "buildChain": false}
	if c.certFile != "" {
		cert["certificateFile"], cert["keyFile"] = c.certFile, c.keyFile
	} else {
		cert["certificate"], cert["key"] = pemLines(c.certPEM), pemLines(c.keyPEM)
	}
	clientSettings := map[string]any{"fingerprint": "", "echConfigList": ""}
	if len(c.pins) > 0 {
		clientSettings["pinnedPeerCertSha256"] = c.pins
	}
	return map[string]any{
		"serverName": c.serverName, "minVersion": "1.2", "maxVersion": "1.3", "cipherSuites": "",
		"rejectUnknownSni": false, "disableSystemRoot": false, "enableSessionResumption": false,
		"certificates": []any{cert}, "alpn": []string{"h3"}, "echServerKeys": "",
		"settings": clientSettings,
	}
}

func pemLines(s string) []string {
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}
