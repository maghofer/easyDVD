package packing

import (
	"context"
	"fmt"
	"math/rand/v2"
	"reflect"
	"sort"
	"testing"
	"time"

	"easydvd/internal/model"
)

func TestNaturalOrder(t *testing.T) {
	names := []string{"5694-010", "5694-2", "5694-001", "5694-100000000000000000000", "5694-9"}
	sort.Slice(names, func(i, j int) bool { return NaturalLess(names[i], names[j]) })
	want := []string{"5694-001", "5694-2", "5694-9", "5694-010", "5694-100000000000000000000"}
	if !reflect.DeepEqual(names, want) {
		t.Fatal(names)
	}
}
func TestNestedNames(t *testing.T) {
	v := []model.Video{{Relative: "B/5694-2.mp4"}, {Relative: "A/5694-10.mp4"}, {Relative: "A/5694-2.mp4"}}
	SortVideos(v)
	if v[0].Relative != "A/5694-2.mp4" || v[1].Relative != "B/5694-2.mp4" {
		t.Fatal(v)
	}
}
func TestStrictAndEfficient(t *testing.T) {
	cap := BaseReserve + 10*TitleReserve
	items := []Item{{"a", 5 * TitleReserve, 0}, {"b", 5 * TitleReserve, 1}, {"c", 3 * TitleReserve, 2}, {"d", 3 * TitleReserve, 3}}
	strict, err := Plan(context.Background(), items, cap, false, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	efficient, err := Plan(context.Background(), items, cap, true, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(strict.Discs) != 3 || len(efficient.Discs) != 2 || !efficient.Proven {
		t.Fatalf("strict=%+v optimized=%+v", strict, efficient)
	}
	if !reflect.DeepEqual(efficient.Discs[0].VideoIDs, []string{"a", "c"}) {
		t.Fatal(efficient)
	}
}
func TestCapacityAndTitleLimit(t *testing.T) {
	items := make([]Item, 100)
	for i := range items {
		items[i] = Item{fmt.Sprint(i), 1, i}
	}
	r, err := Plan(context.Background(), items, 4700000000, true, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Discs) != 2 || !r.Proven {
		t.Fatal(r)
	}
	for _, d := range r.Discs {
		if len(d.VideoIDs) > 99 {
			t.Fatal("too many titles")
		}
	}
	_, err = Plan(context.Background(), []Item{{"oversized", 4700000000, 0}}, 4700000000, false, 0)
	if err == nil {
		t.Fatal("oversized video accepted")
	}
}
func TestPackingProperties(t *testing.T) {
	for trial := 0; trial < 80; trial++ {
		n := 1 + rand.IntN(60)
		items := make([]Item, n)
		sizes := map[string]int64{}
		for i := range items {
			size := int64(rand.IntN(9)+1) * TitleReserve
			id := fmt.Sprint(i)
			items[i] = Item{id, size, i}
			sizes[id] = size
		}
		capacity := BaseReserve + 15*TitleReserve
		for _, efficient := range []bool{false, true} {
			r, err := Plan(context.Background(), items, capacity, efficient, 2*time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			seen := map[string]bool{}
			last := -1
			for _, d := range r.Discs {
				used := BaseReserve
				for _, id := range d.VideoIDs {
					if seen[id] {
						t.Fatal("duplicate", id)
					}
					seen[id] = true
					used += sizes[id] + TitleReserve
					if !efficient {
						var num int
						fmt.Sscan(id, &num)
						if num <= last {
							t.Fatal("strict order changed")
						}
						last = num
					}
				}
				if used > capacity || used != d.Bytes {
					t.Fatalf("invalid size %d %d", used, d.Bytes)
				}
			}
			if len(seen) != n {
				t.Fatal("lost titles")
			}
		}
	}
}
func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Plan(ctx, []Item{{"a", 1, 0}}, 4700000000, false, time.Second)
	if err != context.Canceled {
		t.Fatal(err)
	}
}
