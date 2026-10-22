package easybox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// cloudflareAPI is the Cloudflare v4 API base; tests point it at a fake server.
var cloudflareAPI = "https://api.cloudflare.com/client/v4"

var ErrCloudflareNotConfigured = errors.New("Cloudflare is not configured: add an API token and domain first")

// cfClient calls the few Cloudflare endpoints a CDN line needs. The token is
// sent as a bearer header and never logged or returned.
type cfClient struct {
	token string
	http  *http.Client
}

func newCFClient(token string) *cfClient {
	return &cfClient{token: token, http: &http.Client{Timeout: 20 * time.Second}}
}

type cfError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cfEnvelope struct {
	Success bool            `json:"success"`
	Errors  []cfError       `json:"errors"`
	Result  json.RawMessage `json:"result"`
}

// CFAPIError is a failed Cloudflare call, worded for the admin.
type CFAPIError struct {
	Op     string
	Status int
	Errors []cfError
}

// cfTokenPermissions is what a CDN line needs from the API token.
const cfTokenPermissions = "Zone Read, DNS Edit, SSL and Certificates Edit, Zone Settings Edit"

// Cloudflare answers a malformed or unknown token with these codes.
var cfBadTokenCodes = map[int]bool{6003: true, 6111: true, 6103: true, 9106: true, 9109: true, 1000: true}

func (e *CFAPIError) Error() string {
	msgs := make([]string, 0, len(e.Errors))
	badToken := false
	for _, x := range e.Errors {
		msgs = append(msgs, fmt.Sprintf("%s (%d)", x.Message, x.Code))
		badToken = badToken || cfBadTokenCodes[x.Code]
	}
	if len(msgs) == 0 {
		msgs = append(msgs, http.StatusText(e.Status))
	}
	hint := ""
	switch {
	case e.Status == http.StatusForbidden:
		hint = ": the API token lacks a permission for this step (needed: " + cfTokenPermissions + ")"
	case badToken || e.Status == http.StatusUnauthorized:
		hint = ": Cloudflare rejected the API token, copy the whole token again"
	}
	return fmt.Sprintf("Cloudflare %s%s [%s]", e.Op, hint, strings.Join(msgs, "; "))
}

func (c *cfClient) do(op, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, cloudflareAPI+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("Cloudflare %s: %w", op, err)
	}
	defer resp.Body.Close()
	var env cfEnvelope
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&env); err != nil {
		return &CFAPIError{Op: op, Status: resp.StatusCode}
	}
	if !env.Success || resp.StatusCode >= 300 {
		return &CFAPIError{Op: op, Status: resp.StatusCode, Errors: env.Errors}
	}
	if out != nil && len(env.Result) > 0 {
		return json.Unmarshal(env.Result, out)
	}
	return nil
}

// zoneID finds the zone for a domain the token can see.
func (c *cfClient) zoneID(domain string) (string, error) {
	var zones []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if err := c.do("find domain", http.MethodGet, "/zones?name="+url.QueryEscape(domain), nil, &zones); err != nil {
		return "", err
	}
	for _, z := range zones {
		if strings.EqualFold(z.Name, domain) {
			if z.Status != "active" {
				return "", fmt.Errorf("Cloudflare domain %s is %q, not active yet: finish moving its nameservers to Cloudflare", domain, z.Status)
			}
			return z.ID, nil
		}
	}
	return "", fmt.Errorf("Cloudflare has no domain %s that this token can access", domain)
}

func (c *cfClient) createProxiedRecord(zoneID, host, ipv4 string) (string, error) {
	var rec struct {
		ID string `json:"id"`
	}
	err := c.do("add DNS record", http.MethodPost, "/zones/"+zoneID+"/dns_records", map[string]any{
		"type": "A", "name": host, "content": ipv4, "proxied": true, "ttl": 1,
		"comment": "EasyBox CDN line",
	}, &rec)
	return rec.ID, err
}

func (c *cfClient) deleteRecord(zoneID, recordID string) error {
	return c.do("delete DNS record", http.MethodDelete, "/zones/"+zoneID+"/dns_records/"+recordID, nil, nil)
}

func (c *cfClient) sslMode(zoneID string) (string, error) {
	var s struct {
		Value string `json:"value"`
	}
	err := c.do("read SSL/TLS mode", http.MethodGet, "/zones/"+zoneID+"/settings/ssl", nil, &s)
	return s.Value, err
}

func (c *cfClient) setSSLMode(zoneID, mode string) error {
	return c.do("set SSL/TLS mode", http.MethodPatch, "/zones/"+zoneID+"/settings/ssl", map[string]string{"value": mode}, nil)
}

// originCertValidityDays is the longest Origin CA validity (15 years), so a
// CDN line never needs a renewal.
const originCertValidityDays = 5475

func (c *cfClient) createOriginCert(csrPEM, host string) (id, certPEM string, err error) {
	var cert struct {
		ID          string `json:"id"`
		Certificate string `json:"certificate"`
	}
	err = c.do("issue origin certificate", http.MethodPost, "/certificates", map[string]any{
		"csr": csrPEM, "hostnames": []string{host}, "request_type": "origin-ecc",
		"requested_validity": originCertValidityDays,
	}, &cert)
	return cert.ID, cert.Certificate, err
}

func (c *cfClient) revokeOriginCert(id string) error {
	return c.do("revoke origin certificate", http.MethodDelete, "/certificates/"+id, nil, nil)
}
