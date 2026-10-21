package easybox

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestEnsureDefaultSubscription_StableAcrossCalls(t *testing.T) {
	initTestDB(t)
	first, err := EnsureDefaultSubscription()
	if err != nil {
		t.Fatalf("EnsureDefaultSubscription: %v", err)
	}
	second, err := EnsureDefaultSubscription()
	if err != nil {
		t.Fatalf("EnsureDefaultSubscription (2nd): %v", err)
	}
	if first != second {
		t.Fatalf("identity changed between calls: %+v vs %+v", first, second)
	}
	var rows int64
	database.GetDB().Model(&model.EasyboxSubscription{}).Count(&rows)
	if rows != 1 {
		t.Fatalf("easybox_subscriptions rows = %d, want 1", rows)
	}
}

// EasyBox.md §37: deleting every line drops the upstream ClientRecord, but the
// subscription URL (SubID) must not change when the next line is created.
func TestEnsureDefaultSubscription_SubIDSurvivesDeletingAllLines(t *testing.T) {
	initTestDB(t)
	id, err := EnsureDefaultSubscription()
	if err != nil {
		t.Fatal(err)
	}
	ib, err := BuildVlessRealityInbound("only-line", 20001, id, testReality)
	if err != nil {
		t.Fatal(err)
	}
	inbounds := &service.InboundService{}
	created, _, err := inbounds.AddInbound(ib)
	if err != nil {
		t.Fatalf("AddInbound: %v", err)
	}
	if _, err := inbounds.DelInbound(created.Id); err != nil {
		t.Fatalf("DelInbound: %v", err)
	}

	again, err := EnsureDefaultSubscription()
	if err != nil {
		t.Fatal(err)
	}
	if again != id {
		t.Fatalf("identity after deleting all lines = %+v, want %+v", again, id)
	}
}
