# elagoht/paginate

Pagination helpers for [collage](https://github.com/Elagoht/collage):

- the arithmetic of a pager;
- a numbered window (`1 … 4 5 [6] 7 8 … 20`);
- path-paged links a static export can write (`/blog`, `/blog/page/2`, …);
- query-paged links for listings only a server can answer (`/search?q=go&page=3`).

It is a **library, not a plugin**. There is nothing to register and nothing to
configure. A page uses it in its data handler and its template.

Requires collage v0.57.0 or later.

```sh
go get github.com/Elagoht/collage-paginate
```

## The pager

```go
p := paginate.New(total, perPage, current)
// p.Current, p.Last, p.Offset, p.Limit, p.Prev, p.Next, p.Window

rows := db.Query("… LIMIT ? OFFSET ?", p.Limit, p.Offset) // fetch only the page
page := paginate.Items(all, p)                             // or slice a list you already have
```

- **`current` is clamped to `[1, Last]`.** `New` never fails, so deciding that a
  page is out of range is the caller's job; `FromPath` does it for path-paged
  listings.
- **An empty listing is one empty page.** `Last` is 1, `Limit` is 0, and `Items`
  returns an empty, non-nil slice.
- **The last page can be short.** For 23 items at 5 per page, page 5 holds 3.
- **The window** keeps the first and last page and two pages on each side of the
  current one. It never reads `1 … 2`: a gap stands only where a number is left
  out. Use `p.WithWindow(edges, around)` to change it.
- **`perPage` below 1 panics.** It is a programming error, not input.

## Path paging: a listing a static export can write

A collage page has one path pattern per locale. A paged listing is therefore two
pages that share their fragment:

- one page serves page 1 at the listing's own path;
- the other serves the rest at `/…/page/{n}`.

```go
func listing(ctx context.Context, rc *collage.RenderContext) (BlogList, []string, error) {
	total, err := posts.Count(ctx)
	if err != nil {
		return BlogList{}, nil, err
	}
	last := paginate.New(total, perPage, 1).Last
	n := 1
	if rc.Param("n") != "" {
		if n, err = paginate.FromPath(rc, "n", last); err != nil {
			return BlogList{}, nil, err // wraps collage.ErrNotFound: a 404
		}
	}
	p := paginate.New(total, perPage, n)
	links, err := paginate.PathNamed(rc, "blog", "blog-page")
	if err != nil {
		return BlogList{}, nil, err
	}
	items, err := posts.List(ctx, p.Limit, p.Offset)
	return BlogList{Posts: items, Pager: p, Links: links.For(p)}, []string{"posts"}, err
}

list := func(name string) *collage.Fragment {
	return collage.NewFragment(name, "blog/list.html").WithData(collage.DataHandler(listing)).Required().Build()
}
app.RegisterPage(collage.NewPage("blog").
	WithContent(list("blog-list")).
	WithPath("en", "/blog").
	Build())
app.RegisterPage(collage.NewPage("blog-page").
	WithContent(list("blog-page-list")).
	WithPath("en", "/blog/page/{n}").
	WithStaticParams(paginate.StaticParams(posts.CountFor, perPage)).
	Build())
```

- **`StaticParams` lists pages 2…Last** for the export and for `PageURLs`, which
  the sitemap reads. Page 1 is not listed, because it is `/blog`.
- **Each page has one address.** `FromPath` accepts only page numbers spelled the
  canonical way, from 2 to `last`. It treats everything else as not found:
  `/blog/page/1`, `/blog/page/02`, a sign, anything other than ASCII digits, and
  any number out of range. A Required fragment whose data handler returns that
  error renders the page's not-found page with a 404.
- **`PathNamed` builds the links through the pages' names,** so locale prefixes
  come out right (`/tr/blog/page/2`). It checks both names up front, so a
  misspelled one is an error rather than an empty link. If the listing has no
  locales, `paginate.Path("/blog", "/blog/page/{n}")` does the same with fixed
  paths.

## Query paging: server-only listings

```go
n := paginate.FromQuery(rc, "page", last)                 // anything invalid is page 1
links := paginate.Query(rc, "page").For(paginate.New(total, perPage, n))
```

- **The request's other parameters stay,** in their order and spelling. Page 1
  drops the key, so its address is the one without it.
- **A cached page must name the key among its cache parameters:**
  `WithCacheParams("q", "page")`. Without that, every page shares one cache
  entry.
- **A static export cannot serve query-paged pages.** A static host answers
  `/search?page=2` with the `/search` file, and the export warns about pages that
  read query parameters.

## In the template

`Links` is a plain struct, so it goes into the fragment's data, and collage's
template checker sees `.Links.Next` and the rest.

```html
<nav class="pager" aria-label="Pages">
  {{with .Links.Prev}}<a href="{{.}}" rel="prev">Previous</a>{{end}}
  {{range .Links.Window}}
    {{if .Gap}}<span>…</span>
    {{else if .Current}}<span aria-current="page">{{.N}}</span>
    {{else}}<a href="{{.URL}}">{{.N}}</a>{{end}}
  {{end}}
  {{with .Links.Next}}<a href="{{.}}" rel="next">Next</a>{{end}}
</nav>
```

To put `rel="prev"` and `rel="next"` in the head, hoist them from the data
handler. The layout places them where it has `{{hoist "head"}}`:

```go
if l := links.For(p); l.Next != "" {
	rc.Hoist("head", "pager-next", template.HTML(`<link rel="next" href="`+template.HTMLEscapeString(l.Next)+`">`))
}
```

## Limits

- **Page totals must be known.** Cursor-based paging, infinite scroll and paging
  without a known total are out of scope.
- **One page can't serve both `/blog` and `/blog/page/{n}`.** A page has one
  pattern per locale, so the listing is two pages.
