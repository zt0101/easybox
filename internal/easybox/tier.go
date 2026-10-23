package easybox

import (
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
)

// Subscription nodes carry their line's Quick Create tier: the {{TIER}} token
// prints a tag in the colours of the panel's tier tags, and subscriptions list
// recommended lines first. The tier comes from the template at request time,
// so it is never part of the stored line name.

// tierLabelsKey switches the tags off ("false"); unset = on.
const tierLabelsKey = "subTierLabels"

var (
	tierRanks  = map[string]int{tierRecommended: 0, tierBackup: 1, tierRisky: 2}
	tierLabels = map[string]string{tierRecommended: "🟢推荐|", tierBackup: "🔵备用|", tierRisky: "🟠易识别|"}
)

// inboundTiers backs sub.InboundTiers: inbounds that belong to no EasyBox line
// are left out.
func inboundTiers(ids []int) map[int]sub.InboundTier {
	out := map[int]sub.InboundTier{}
	if len(ids) == 0 {
		return out
	}
	var rows []model.EasyboxEndpoint
	if err := database.GetDB().Where("inbound_id IN ?", ids).Find(&rows).Error; err != nil {
		logger.Warning("easybox: subscription tiers:", err)
		return out
	}
	labels, err := TierLabelsEnabled()
	if err != nil {
		logger.Warning("easybox: subscription tier tags:", err)
	}
	for _, r := range rows {
		t, ok := templateByID(r.Template)
		if !ok || t.tier == "" {
			continue
		}
		it := sub.InboundTier{Rank: tierRanks[t.tier]}
		if labels {
			it.Label = tierLabels[t.tier]
		}
		out[r.InboundId] = it
	}
	return out
}

// TierLabelsEnabled reports whether node names carry the tier tag.
func TierLabelsEnabled() (bool, error) {
	v, ok, err := getSetting(tierLabelsKey)
	if err != nil {
		return false, err
	}
	return !ok || v != "false", nil
}

// SetTierLabels turns the tier tag in node names on or off; the order by tier
// stays either way.
func (s *Service) SetTierLabels(on bool) error {
	err := setSetting(tierLabelsKey, strconv.FormatBool(on))
	action := "subscription.tier_labels_off"
	if on {
		action = "subscription.tier_labels_on"
	}
	auditObject(action, "subscription", 0, err)
	return err
}
