package sub

import (
	"slices"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Lines sort by tier rank, untiered inbounds go last, and each tier keeps the
// order the query returned (the admin's sub_sort_index).
func TestSortByTier_RankThenOriginalOrder(t *testing.T) {
	saved := InboundTiers
	t.Cleanup(func() { InboundTiers = saved })
	InboundTiers = func(ids []int) map[int]InboundTier {
		return map[int]InboundTier{
			1: {Rank: 2, Label: "risky|"},
			2: {Rank: 0, Label: "rec|"},
			4: {Rank: 1, Label: "backup|"},
			5: {Rank: 0, Label: "rec|"},
		}
	}
	inbounds := []*model.Inbound{{Id: 1}, {Id: 2}, {Id: 3}, {Id: 4}, {Id: 5}}
	s := &SubService{}
	s.sortByTier(inbounds)
	var got []int
	for _, ib := range inbounds {
		got = append(got, ib.Id)
	}
	if want := []int{2, 5, 4, 1, 3}; !slices.Equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if l := s.tierFor(&model.Inbound{Id: 4}).Label; l != "backup|" {
		t.Fatalf("tierFor(4) = %q, want the cached label", l)
	}
	if l := s.tierFor(&model.Inbound{Id: 3}).Label; l != "" {
		t.Fatalf("untiered inbound label = %q, want none", l)
	}
}

// Without EasyBox the order and names stay upstream's.
func TestSortByTier_NoHookKeepsOrder(t *testing.T) {
	saved := InboundTiers
	t.Cleanup(func() { InboundTiers = saved })
	InboundTiers = nil
	inbounds := []*model.Inbound{{Id: 3}, {Id: 1}, {Id: 2}}
	(&SubService{}).sortByTier(inbounds)
	if inbounds[0].Id != 3 || inbounds[1].Id != 1 || inbounds[2].Id != 2 {
		t.Fatalf("order changed without the EasyBox hook")
	}
}

func TestRemarkVar_Tier(t *testing.T) {
	if got := remarkVarValue("TIER", remarkContext{tier: "🟢推荐|"}); got != "🟢推荐|" {
		t.Fatalf("TIER = %q", got)
	}
}
