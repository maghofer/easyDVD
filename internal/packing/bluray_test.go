package packing

import (
	"context"
	"testing"

	"easydvd/internal/model"
)

func TestBluRayPackingBudgetsMenusAndKeepsTitlesWhole(t *testing.T) {
	s := model.DefaultSettings(t.TempDir())
	s.DiscFormat = "Blu-ray"
	s.Media = "BD-25"
	items := []Item{{ID: "a", Bytes: 12500000000}, {ID: "b", Bytes: 12500000000, Order: 1}}
	r, err := PlanSettings(context.Background(), items, s, 0)
	if err != nil || len(r.Discs) != 2 {
		t.Fatal(r, err)
	}
	s.Media = "BD-50"
	r, err = PlanSettings(context.Background(), items, s, 0)
	if err != nil || len(r.Discs) != 1 || len(r.Discs[0].VideoIDs) != 2 {
		t.Fatal(r, err)
	}
	if r.Discs[0].Bytes <= items[0].Bytes+items[1].Bytes+(64<<20) {
		t.Fatal("menu reserve missing")
	}
	items[0].Bytes = 50000000000
	if _, err = PlanSettings(context.Background(), items, s, 0); err == nil {
		t.Fatal("oversized title accepted")
	}
}
