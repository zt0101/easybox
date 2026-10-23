package easybox

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

var ErrNoRealityTarget = errors.New("no usable REALITY target")

const (
	// realityPoolMax caps how many of the fastest usable targets a new line
	// picks from at random.
	realityPoolMax = 8
	// realityPoolSlackMs: a target joins the pool while its handshake takes at
	// most twice the fastest one plus this, so a far one never wins by chance.
	realityPoolSlackMs = 20
)

// pickIndex chooses within the pool; tests replace it.
var pickIndex = random.Num

// GenerateRealityParams uses xray x25519 keys and upstream's ranked target scan
// over the realityScanCandidates setting, then picks the target (EasyBox.md §89.1).
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
	target, names, err := chooseRealityTarget(results, usedRealityTargets())
	if err != nil {
		return RealityParams{}, err
	}
	shortIDs, err := newShortIDs()
	if err != nil {
		return RealityParams{}, err
	}
	return RealityParams{PrivateKey: priv, PublicKey: pub, Target: target, ServerNames: names, ShortIDs: shortIDs}, nil
}

// chooseRealityTarget picks at random among the fastest usable targets of
// upstream's ranked scan, preferring one no existing inbound uses, so lines and
// installs don't all share one camouflage site.
func chooseRealityTarget(results []*service.RealityScanResult, used map[string]bool) (string, []string, error) {
	var pool []*service.RealityScanResult
	var reasons []string
	for _, r := range results {
		if r == nil {
			continue
		}
		if r.Feasible && !r.PrivateTarget {
			// Upstream sorts usable targets by latency, so pool[0] is the fastest.
			if len(pool) < realityPoolMax && (len(pool) == 0 || r.LatencyMs <= 2*pool[0].LatencyMs+realityPoolSlackMs) {
				pool = append(pool, r)
			}
			continue
		}
		if r.Reason != "" {
			reasons = append(reasons, r.Target+": "+r.Reason)
		}
	}
	if len(pool) == 0 {
		return "", nil, fmt.Errorf("%w (%s)", ErrNoRealityTarget, strings.Join(reasons, "; "))
	}
	var fresh []*service.RealityScanResult
	for _, r := range pool {
		if !used[r.Target] {
			fresh = append(fresh, r)
		}
	}
	if len(fresh) > 0 {
		pool = fresh
	}
	r := pool[pickIndex(len(pool))]
	// Only the probed name: subscriptions pick the client SNI at random from
	// serverNames, and a certificate's other names (www.apple.com.cn, 80 Sony
	// country sites) would put an unrelated or Chinese domain in links.
	names := []string{r.Host}
	if r.Host == "" && len(r.ServerNames) > 0 {
		names = r.ServerNames[:1]
	}
	return r.Target, names, nil
}

// usedRealityTargets lists the REALITY targets of the inbounds already on this
// panel; a read error only loses the preference for an unused target.
func usedRealityTargets() map[string]bool {
	used := map[string]bool{}
	var streams []string
	if err := database.GetDB().Model(&model.Inbound{}).Where("stream_settings LIKE ?", `%realitySettings%`).Pluck("stream_settings", &streams).Error; err != nil {
		logger.Warning("easybox: REALITY targets in use:", err)
		return used
	}
	for _, raw := range streams {
		var st struct {
			RealitySettings struct {
				Target string `json:"target"`
				Dest   string `json:"dest"`
			} `json:"realitySettings"`
		}
		if json.Unmarshal([]byte(raw), &st) != nil {
			continue
		}
		if t := st.RealitySettings.Target; t != "" {
			used[t] = true
		} else if t := st.RealitySettings.Dest; t != "" {
			used[t] = true
		}
	}
	return used
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
