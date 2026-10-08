package paginate

// Pager is one page of a listing: where it is, how many there are, and which
// items it holds.
type Pager struct {
	// Current is the page shown, clamped to [1, Last].
	Current int
	// Last is the number of pages, at least 1: an empty listing is one empty page.
	Last int
	// PerPage is how many items a full page holds.
	PerPage int
	// Total is how many items the listing has.
	Total int
	// Offset and Limit select the page's items: an SQL OFFSET and LIMIT, or a
	// slice of an in-memory list (see Items). The last page's Limit may be short.
	Offset, Limit int
	// Prev and Next are the neighbouring pages, or 0 where there is none.
	Prev, Next int
	// Window is the page numbers to show, with gaps: 1 … 4 5 6 … 20.
	Window []Slot
}

// Slot is one entry of a Pager's Window: a page number, or a gap standing for
// the numbers left out.
type Slot struct {
	N       int
	Gap     bool
	Current bool
}

// Default window: the first and last page, and two on each side of the current.
const (
	defaultEdges  = 1
	defaultAround = 2
)

// New computes the page current of a listing of total items, perPage to a
// page. current is clamped to [1, Last], so New never fails; deciding that a
// page out of range is not found belongs to FromPath. perPage below 1 is a
// programming error and panics.
func New(total, perPage, current int) Pager {
	if perPage < 1 {
		panic("paginate: perPage must be at least 1")
	}
	if total < 0 {
		total = 0
	}
	// Ceiling division without the overflow total+perPage-1 would risk.
	last := total / perPage
	if total%perPage != 0 {
		last++
	}
	if last < 1 {
		last = 1
	}
	current = max(1, min(current, last))

	// (current-1)*perPage <= (last-1)*perPage < total, so it cannot overflow.
	offset := (current - 1) * perPage
	limit := max(0, min(perPage, total-offset))

	p := Pager{
		Current: current,
		Last:    last,
		PerPage: perPage,
		Total:   total,
		Offset:  offset,
		Limit:   limit,
	}
	if current > 1 {
		p.Prev = current - 1
	}
	if current < last {
		p.Next = current + 1
	}
	return p.WithWindow(defaultEdges, defaultAround)
}

// WithWindow returns p with its Window recomputed: edges pages at each end and
// around pages on each side of the current one. A gap stands only where at
// least one number is left out, so a window never reads 1 … 2. With edges 0 the
// window can open and close on a gap. Negative values count as 0.
func (p Pager) WithWindow(edges, around int) Pager {
	edges, around = max(0, edges), max(0, around)

	// The candidates, as up to three sorted runs; never one entry per page, so
	// a huge Last costs nothing.
	runs := [3][2]int{
		{1, edges},
		{p.Current - around, p.Current + around},
		{p.Last - edges + 1, p.Last},
	}
	window := make([]Slot, 0, 2*edges+2*around+3)
	prev := 0
	for _, run := range runs {
		from, to := max(run[0], prev+1, 1), min(run[1], p.Last)
		for n := from; n <= to; n++ {
			if n > prev+1 {
				window = append(window, Slot{Gap: true})
			}
			window = append(window, Slot{N: n, Current: n == p.Current})
			prev = n
		}
	}
	if prev < p.Last {
		window = append(window, Slot{Gap: true})
	}
	p.Window = window
	return p
}

// Items returns p's page of an in-memory list: an empty, non-nil slice when
// the page holds nothing.
func Items[T any](items []T, p Pager) []T {
	if p.Offset >= len(items) {
		return items[:0:0]
	}
	return items[p.Offset:min(p.Offset+p.Limit, len(items))]
}
