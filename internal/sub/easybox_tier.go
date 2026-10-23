package sub

import (
	"sort"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// InboundTiers lets EasyBox tag and order subscription nodes by the tier of the
// template each line was made from (recommended / backup / risky). It returns
// a tier per inbound id; inbounds it leaves out get no tag and sort last.
// nil = subscriptions stay as upstream builds them.
var InboundTiers func(inboundIDs []int) map[int]InboundTier

// InboundTier is one line's place in a subscription: Rank orders the nodes
// (lower first) and Label is what the {{TIER}} token prints ("" = no tag).
type InboundTier struct {
	Rank  int
	Label string
}

const untieredRank = 1 << 30

// tierFor returns the inbound's tier, asking EasyBox once per inbound and
// request (the panel's link/QR displays never load a whole subscription).
func (s *SubService) tierFor(inbound *model.Inbound) InboundTier {
	if InboundTiers == nil || inbound == nil {
		return InboundTier{Rank: untieredRank}
	}
	if s.tiers == nil {
		s.tiers = map[int]InboundTier{}
	}
	if t, ok := s.tiers[inbound.Id]; ok {
		return t
	}
	t, ok := InboundTiers([]int{inbound.Id})[inbound.Id]
	if !ok {
		t = InboundTier{Rank: untieredRank}
	}
	s.tiers[inbound.Id] = t
	return t
}

// sortByTier puts recommended lines first, then backup, then risky, then the
// rest; within a tier the admin's sub_sort_index order stays.
func (s *SubService) sortByTier(inbounds []*model.Inbound) {
	if InboundTiers == nil || len(inbounds) == 0 {
		return
	}
	ids := make([]int, len(inbounds))
	for i, ib := range inbounds {
		ids[i] = ib.Id
	}
	found := InboundTiers(ids)
	if s.tiers == nil {
		s.tiers = map[int]InboundTier{}
	}
	for _, id := range ids {
		t, ok := found[id]
		if !ok {
			t = InboundTier{Rank: untieredRank}
		}
		s.tiers[id] = t
	}
	sort.SliceStable(inbounds, func(i, j int) bool {
		return s.tiers[inbounds[i].Id].Rank < s.tiers[inbounds[j].Id].Rank
	})
}
