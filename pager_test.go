package paginate_test

import (
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"

	"github.com/Elagoht/collage-paginate"
)

func TestNew_Table(t *testing.T) {
	cases := []struct {
		total, perPage, current              int
		last, cur, offset, limit, prev, next int
	}{
		{0, 10, 1, 1, 1, 0, 0, 0, 0},
		{25, 10, 1, 3, 1, 0, 10, 0, 2},
		{25, 10, 3, 3, 3, 20, 5, 2, 0},
		{25, 10, 9, 3, 3, 20, 5, 2, 0},
		{25, 10, 0, 3, 1, 0, 10, 0, 2},
		{30, 10, 2, 3, 2, 10, 10, 1, 3},
		{-5, 10, 1, 1, 1, 0, 0, 0, 0},
	}
	for _, c := range cases {
		p := paginate.New(c.total, c.perPage, c.current)
		got := [6]int{p.Last, p.Current, p.Offset, p.Limit, p.Prev, p.Next}
		want := [6]int{c.last, c.cur, c.offset, c.limit, c.prev, c.next}
		if got != want {
			t.Errorf("New(%d, %d, %d): Last,Current,Offset,Limit,Prev,Next = %v, want %v",
				c.total, c.perPage, c.current, got, want)
		}
	}
}

// The last page of a total that is not a multiple of perPage is short, and
// Items returns exactly what is left.
func TestNew_LastPageShort(t *testing.T) {
	items := make([]int, 23)
	for i := range items {
		items[i] = i
	}
	p := paginate.New(len(items), 5, 5)
	if p.Last != 5 || p.Limit != 3 {
		t.Fatalf("Last, Limit = %d, %d; want 5, 3", p.Last, p.Limit)
	}
	got := paginate.Items(items, p)
	if len(got) != 3 || got[0] != 20 || got[2] != 22 {
		t.Fatalf("Items = %v, want [20 21 22]", got)
	}
}

func TestNew_PanicsOnPerPage(t *testing.T) {
	for _, perPage := range []int{0, -1} {
		func() {
			defer func() {
				r := recover()
				if r != "paginate: perPage must be at least 1" {
					t.Errorf("perPage %d: recovered %v", perPage, r)
				}
			}()
			paginate.New(10, perPage, 1)
		}()
	}
}

func TestNew_NoOverflow(t *testing.T) {
	p := paginate.New(math.MaxInt, 7, math.MaxInt)
	wantLast := math.MaxInt/7 + 1 // MaxInt is not a multiple of 7
	if math.MaxInt%7 == 0 {
		wantLast = math.MaxInt / 7
	}
	if p.Last != wantLast || p.Current != wantLast {
		t.Fatalf("Last, Current = %d, %d; want %d", p.Last, p.Current, wantLast)
	}
	if p.Offset < 0 || p.Limit < 1 || p.Limit > 7 {
		t.Fatalf("Offset, Limit = %d, %d", p.Offset, p.Limit)
	}
}

// render writes a window as "1 … 4 5 [6] 7 8 … 20".
func render(w []paginate.Slot) string {
	parts := make([]string, 0, len(w))
	for _, s := range w {
		switch {
		case s.Gap:
			parts = append(parts, "…")
		case s.Current:
			parts = append(parts, "["+strconv.Itoa(s.N)+"]")
		default:
			parts = append(parts, strconv.Itoa(s.N))
		}
	}
	return strings.Join(parts, " ")
}

func TestWindow_Table(t *testing.T) {
	pager := func(last, current int) paginate.Pager { return paginate.New(last*10, 10, current) }
	cases := []struct {
		p    paginate.Pager
		want string
	}{
		{pager(20, 6), "1 … 4 5 [6] 7 8 … 20"},
		{pager(20, 1), "[1] 2 3 … 20"},
		{pager(20, 20), "1 … 18 19 [20]"},
		{pager(5, 3), "1 2 [3] 4 5"},
		{pager(1, 1), "[1]"},
		{pager(20, 4), "1 2 3 [4] 5 6 … 20"},
		// With no edges, the window can open and close on a gap.
		{pager(20, 10).WithWindow(0, 0), "… [10] …"},
	}
	for _, c := range cases {
		if got := render(c.p.Window); got != c.want {
			t.Errorf("Last %d, Current %d: window %q, want %q", c.p.Last, c.p.Current, got, c.want)
		}
	}
}

// Over every page, Items covers the whole list exactly once.
func TestProperty_ItemsCoverAll(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for range 2000 {
		total, perPage := r.IntN(501), 1+r.IntN(50)
		items := make([]int, total)
		for i := range items {
			items[i] = i
		}
		last := paginate.New(total, perPage, 1).Last
		var all []int
		for n := 1; n <= last; n++ {
			all = append(all, paginate.Items(items, paginate.New(total, perPage, n))...)
		}
		if len(all) != total {
			t.Fatalf("total %d perPage %d: %d items over all pages", total, perPage, len(all))
		}
		for i, v := range all {
			if v != i {
				t.Fatalf("total %d perPage %d: item %d is %d", total, perPage, i, v)
			}
		}
	}
}

// A window is strictly increasing over its numbers, has no two gaps in a row,
// and marks exactly one page as current.
func TestProperty_WindowSorted(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	for range 2000 {
		last := 1 + r.IntN(200)
		p := paginate.New(last*3, 3, 1+r.IntN(last)).WithWindow(r.IntN(4), r.IntN(4))
		prev, current, gap := 0, 0, false
		for _, s := range p.Window {
			if s.Gap {
				if gap {
					t.Fatalf("two gaps in a row: %s", render(p.Window))
				}
				gap = true
				continue
			}
			gap = false
			if s.N <= prev {
				t.Fatalf("not increasing: %s", render(p.Window))
			}
			if s.Current {
				current++
			}
			prev = s.N
		}
		if current != 1 {
			t.Fatalf("%d current slots: %s", current, render(p.Window))
		}
	}
}

func TestItems_EmptyIsNotNil(t *testing.T) {
	got := paginate.Items([]int{}, paginate.New(0, 10, 1))
	if got == nil || len(got) != 0 {
		t.Fatalf("Items of an empty page = %#v, want an empty non-nil slice", got)
	}
}
