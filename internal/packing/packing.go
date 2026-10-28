// Package packing assigns whole titles to discs with a bounded, cancellable search.
package packing

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"easydvd/internal/model"
)

const BaseReserve int64 = 32 * 1024 * 1024
const TitleReserve int64 = 256 * 1024
const MaxTitles = 99

// NaturalLess compares digit runs numerically without integer overflow.
func NaturalLess(a, b string) bool {
	aa, bb := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))
	i, j := 0, 0
	for i < len(aa) && j < len(bb) {
		if aa[i] >= '0' && aa[i] <= '9' && bb[j] >= '0' && bb[j] <= '9' {
			x, y := i, j
			for i < len(aa) && aa[i] >= '0' && aa[i] <= '9' {
				i++
			}
			for j < len(bb) && bb[j] >= '0' && bb[j] <= '9' {
				j++
			}
			av, bv := strings.TrimLeft(string(aa[x:i]), "0"), strings.TrimLeft(string(bb[y:j]), "0")
			if len(av) != len(bv) {
				return len(av) < len(bv)
			}
			if av != bv {
				return av < bv
			}
			continue
		}
		if aa[i] != bb[j] {
			return unicode.ToLower(aa[i]) < unicode.ToLower(bb[j])
		}
		i++
		j++
	}
	if i != len(aa) || j != len(bb) {
		return i == len(aa)
	}
	return a < b
}

func SortVideos(v []model.Video) {
	sort.SliceStable(v, func(i, j int) bool {
		a, b := base(v[i].Relative), base(v[j].Relative)
		if strings.EqualFold(a, b) {
			return NaturalLess(v[i].Relative, v[j].Relative)
		}
		return NaturalLess(a, b)
	})
}
func base(s string) string {
	s = strings.ReplaceAll(s, `\`, "/")
	p := strings.LastIndex(s, "/")
	return s[p+1:]
}

type Item struct {
	ID    string
	Bytes int64
	Order int
}
type Result struct {
	Discs  []model.Disc
	Proven bool
}
type bin struct {
	used  int64
	items []Item
}

func Plan(ctx context.Context, items []Item, capacity int64, efficient bool, budget time.Duration) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	limit := capacity - BaseReserve
	if limit <= TitleReserve {
		return Result{}, fmt.Errorf("disc capacity is too small")
	}
	items = append([]Item(nil), items...)
	for i := range items {
		if items[i].Bytes < 0 {
			return Result{}, fmt.Errorf("negative title size")
		}
		if items[i].Bytes > limit-TitleReserve {
			return Result{}, fmt.Errorf("video %s cannot fit on one disc at the chosen quality", items[i].ID)
		}
		items[i].Bytes += TitleReserve
	}
	if len(items) == 0 {
		return Result{}, nil
	}
	var bins []bin
	if !efficient {
		for _, it := range items {
			if err := ctx.Err(); err != nil {
				return Result{}, err
			}
			if len(bins) == 0 || bins[len(bins)-1].used+it.Bytes > limit || len(bins[len(bins)-1].items) >= MaxTitles {
				bins = append(bins, bin{})
			}
			k := len(bins) - 1
			bins[k].used += it.Bytes
			bins[k].items = append(bins[k].items, it)
		}
		return result(bins, false), nil
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Bytes == items[j].Bytes {
			return items[i].Order < items[j].Order
		}
		return items[i].Bytes > items[j].Bytes
	})
	var sum int64
	for _, it := range items {
		sum += it.Bytes
		best := -1
		for k, b := range bins {
			if len(b.items) < MaxTitles && b.used+it.Bytes <= limit && (best < 0 || b.used > bins[best].used) {
				best = k
			}
		}
		if best < 0 {
			bins = append(bins, bin{})
			best = len(bins) - 1
		}
		bins[best].used += it.Bytes
		bins[best].items = append(bins[best].items, it)
	}
	lower := int((sum + limit - 1) / limit)
	if t := (len(items) + MaxTitles - 1) / MaxTitles; t > lower {
		lower = t
	}
	if len(bins) == lower {
		return result(bins, true), nil
	}
	// Search each smaller target count. Exhaustion proves infeasibility for that count.
	deadline := time.Now().Add(budget)
	for target := lower; target < len(bins); target++ {
		work := make([]bin, target)
		nodes := 0
		timedOut := false
		var search func(int) bool
		search = func(n int) bool {
			nodes++
			if nodes%256 == 0 || n == 0 {
				if ctx.Err() != nil || !time.Now().Before(deadline) {
					timedOut = true
					return false
				}
			}
			if n == len(items) {
				return true
			}
			seen := map[[2]int64]bool{}
			for k := range work {
				key := [2]int64{work[k].used, int64(len(work[k].items))}
				if seen[key] {
					continue
				}
				seen[key] = true
				if work[k].used+items[n].Bytes > limit || len(work[k].items) >= MaxTitles {
					continue
				}
				work[k].used += items[n].Bytes
				work[k].items = append(work[k].items, items[n])
				if search(n + 1) {
					return true
				}
				work[k].used -= items[n].Bytes
				work[k].items = work[k].items[:len(work[k].items)-1]
				if timedOut {
					return false
				}
				if work[k].used == 0 {
					break
				}
			}
			return false
		}
		if search(0) {
			return result(work, true), nil
		}
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		if timedOut {
			return result(bins, false), nil
		}
	}
	return result(bins, true), nil
}

func result(bins []bin, proven bool) Result {
	for k := range bins {
		sort.Slice(bins[k].items, func(i, j int) bool { return bins[k].items[i].Order < bins[k].items[j].Order })
	}
	sort.SliceStable(bins, func(i, j int) bool { return bins[i].items[0].Order < bins[j].items[0].Order })
	r := Result{Proven: proven}
	for _, b := range bins {
		d := model.Disc{Number: len(r.Discs) + 1, Bytes: b.used + BaseReserve}
		for _, it := range b.items {
			d.VideoIDs = append(d.VideoIDs, it.ID)
		}
		r.Discs = append(r.Discs, d)
	}
	return r
}
