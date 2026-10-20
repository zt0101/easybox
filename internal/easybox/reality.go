package easybox

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

var ErrNoRealityTarget = errors.New("no usable REALITY target")

// GenerateRealityParams uses only upstream capabilities: xray x25519 keys and the
// ranked target scan over the realityScanCandidates setting (EasyBox.md §89.1).
func GenerateRealityParams(server *service.ServerService) (RealityParams, error) {
	pair, err := server.GetNewX25519Cert()
	if err != nil {
		return RealityParams{}, fmt.Errorf("generate REALITY keys: %w", err)
	}
	keys, _ := pair.(map[string]any)
	priv, _ := keys["privateKey"].(string)
	pub, _ := keys["publicKey"].(string)
	if priv == "" || pub == "" {
		return RealityParams{}, errors.New("generate REALITY keys: empty key pair")
	}
	results, err := server.ScanRealityTargets("")
	if err != nil {
		return RealityParams{}, fmt.Errorf("scan REALITY targets: %w", err)
	}
	target, names, err := chooseRealityTarget(results)
	if err != nil {
		return RealityParams{}, err
	}
	shortIDs, err := newShortIDs()
	if err != nil {
		return RealityParams{}, err
	}
	return RealityParams{PrivateKey: priv, PublicKey: pub, Target: target, ServerNames: names, ShortIDs: shortIDs}, nil
}

func chooseRealityTarget(results []*service.RealityScanResult) (string, []string, error) {
	var reasons []string
	for _, r := range results {
		if r == nil {
			continue
		}
		if r.Feasible && !r.PrivateTarget {
			names := r.ServerNames
			if len(names) == 0 {
				names = []string{r.Host}
			}
			return r.Target, names, nil
		}
		if r.Reason != "" {
			reasons = append(reasons, r.Target+": "+r.Reason)
		}
	}
	return "", nil, fmt.Errorf("%w (%s)", ErrNoRealityTarget, strings.Join(reasons, "; "))
}

// newShortIDs mirrors upstream RandomUtil.randomShortIds: one hex id of each
// even length 2..16 in random order.
func newShortIDs() ([]string, error) {
	lengths := []int{2, 4, 6, 8, 10, 12, 14, 16}
	for i := len(lengths) - 1; i > 0; i-- {
		j := random.Num(i + 1)
		lengths[i], lengths[j] = lengths[j], lengths[i]
	}
	ids := make([]string, 0, len(lengths))
	for _, n := range lengths {
		b := make([]byte, n/2)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		ids = append(ids, hex.EncodeToString(b))
	}
	return ids, nil
}
