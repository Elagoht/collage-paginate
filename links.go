package paginate

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/Elagoht/collage/pkg/collage"
)

// Linker builds the URL of each page of a listing: Path and PathNamed for
// path-paged listings a static export can write, Query for server-only ones.
type Linker struct {
	url func(n int) string
}

// URL returns page n's address.
func (l Linker) URL(n int) string { return l.url(n) }

// LinkSlot is a Window slot with its address; a gap has none.
type LinkSlot struct {
	Slot
	URL string
}

// Links is what a template needs to draw a pager: the neighbours' addresses, ""
// where there is none, and the numbered window with an address per number. It
// is a plain struct, so it goes into a fragment's data and the template checker
// sees its fields.
type Links struct {
	Prev, Next string
	Window     []LinkSlot
}

// For returns p's links.
func (l Linker) For(p Pager) Links {
	links := Links{Window: make([]LinkSlot, len(p.Window))}
	if p.Prev > 0 {
		links.Prev = l.url(p.Prev)
	}
	if p.Next > 0 {
		links.Next = l.url(p.Next)
	}
	for i, s := range p.Window {
		links.Window[i] = LinkSlot{Slot: s}
		if !s.Gap {
			links.Window[i].URL = l.url(s.N)
		}
	}
	return links
}

// Path links a listing whose first page is at first and whose later pages are
// at pattern, with "{n}" standing for the page number: Path("/blog",
// "/blog/page/{n}"). Page 1 is always first, so it has one address. A pattern
// without "{n}" is a programming error and panics.
func Path(first, pattern string) Linker {
	if !strings.Contains(pattern, "{n}") {
		panic(fmt.Sprintf("paginate: pattern %q has no {n}", pattern))
	}
	return Linker{url: func(n int) string {
		if n <= 1 {
			return first
		}
		return strings.Replace(pattern, "{n}", strconv.Itoa(n), 1)
	}}
}

// PathNamed links a listing through its pages' names, in the render's locale:
// first is the page serving page 1, rest the page whose path carries {n}. Both
// names are checked here, so a misspelled one is an error rather than an empty
// link later.
func PathNamed(rc *collage.RenderContext, first, rest string) (Linker, error) {
	firstURL, err := rc.URL(first, nil)
	if err != nil {
		return Linker{}, fmt.Errorf("paginate: page %q: %w", first, err)
	}
	if _, err := rc.URL(rest, map[string]string{"n": "2"}); err != nil {
		return Linker{}, fmt.Errorf("paginate: page %q: %w", rest, err)
	}
	return Linker{url: func(n int) string {
		if n <= 1 {
			return firstURL
		}
		u, err := rc.URL(rest, map[string]string{"n": strconv.Itoa(n)})
		if err != nil {
			return "" // unreachable: the name resolved for n=2 above
		}
		return u
	}}, nil
}

// Query links a server-only listing paged by the query key: the request's
// other parameters stay, in their order and spelling; page 1 drops the key, so
// its address is the one without it. A key given more than once keeps its first
// position and loses the rest.
func Query(rc *collage.RenderContext, key string) Linker {
	// As the request spelled it: the decoded path would turn an escaped "?" or
	// "#" in a path parameter into the end of the path.
	path := rc.Request.URL.EscapedPath()
	var parts []string
	if raw := rc.Request.URL.RawQuery; raw != "" {
		parts = strings.Split(raw, "&")
	}
	at := -1 // the key's first position among the kept parts
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		name, _, _ := strings.Cut(part, "=")
		if unescaped, err := url.QueryUnescape(name); err == nil && unescaped == key {
			if at < 0 {
				at = len(kept)
				kept = append(kept, "") // the key's slot, filled per page
			}
			continue
		}
		kept = append(kept, part)
	}
	encodedKey := url.QueryEscape(key)
	return Linker{url: func(n int) string {
		out := make([]string, 0, len(kept)+1)
		for i, part := range kept {
			if i == at {
				if n > 1 {
					out = append(out, encodedKey+"="+strconv.Itoa(n))
				}
				continue
			}
			out = append(out, part)
		}
		if at < 0 && n > 1 {
			out = append(out, encodedKey+"="+strconv.Itoa(n))
		}
		if len(out) == 0 {
			return path
		}
		return path + "?" + strings.Join(out, "&")
	}}
}
