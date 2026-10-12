package paginate_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Elagoht/collage-paginate"
	"github.com/Elagoht/collage/pkg/collage"
)

// listing is what a paged fragment renders.
type listing struct {
	Current int
	Links   paginate.Links
}

const perPage = 10

func count(total int) func(context.Context, string) (int, error) {
	return func(context.Context, string) (int, error) { return total, nil }
}

// blogApp registers the two-page pattern: "blog" at /blog serves page 1, and
// "blog-page" at /blog/page/{n} serves the rest. Both render the same
// template; the first page's links come from PathNamed, so a locale prefix is
// right.
func blogApp(t *testing.T, total int, locales collage.LocaleConfig) *collage.App {
	t.Helper()
	app, err := collage.New(&collage.Config{
		Server: collage.ServerConfig{Host: "localhost", Port: 3000},
		Template: collage.TemplateConfig{FS: fstest.MapFS{
			"t/list.html": {Data: []byte(`<p>page {{.Current}} next {{.Links.Next}} prev {{.Links.Prev}}</p>`)},
		}, Root: "t"},
		Locale: locales,
	})
	if err != nil {
		t.Fatal(err)
	}
	last := paginate.New(total, perPage, 1).Last
	handler := func(fromPath bool) collage.Data {
		return collage.LoadTagged(func(_ context.Context, rc *collage.RenderContext) (listing, []string, error) {
			n := 1
			if fromPath {
				var err error
				if n, err = paginate.FromPath(rc, "n", last); err != nil {
					return listing{}, nil, err
				}
			}
			links, err := paginate.PathNamed(rc, "blog", "blog-page")
			if err != nil {
				return listing{}, nil, err
			}
			return listing{Current: n, Links: links.For(paginate.New(total, perPage, n))}, nil, nil
		})
	}
	page := func(name string, fromPath bool) *collage.PageBuilder {
		b := collage.NewPage(name).
			WithContent(collage.NewFragment(name+"-list", "list.html").WithData(handler(fromPath)).Required().Build())
		def := locales.Default
		if def == "" {
			def = "en"
		}
		seen := map[string]bool{}
		for _, locale := range append([]string{def}, locales.Supported...) {
			if seen[locale] {
				continue
			}
			seen[locale] = true
			if fromPath {
				b = b.WithPath(locale, "/blog/page/{n}")
			} else {
				b = b.WithPath(locale, "/blog")
			}
		}
		return b
	}
	if err := app.RegisterPage(page("blog", false).Static().Build()); err != nil {
		t.Fatal(err)
	}
	if err := app.RegisterPage(page("blog-page", true).WithStaticParams(paginate.StaticParams(count(total), perPage)).Static().Build()); err != nil {
		t.Fatal(err)
	}
	return app
}

func get(t *testing.T, h http.Handler, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	body, _ := io.ReadAll(rec.Body)
	return rec.Code, string(body)
}

// Page 1 has exactly one URL: /blog. /blog/page/1 and /blog/page/02 are not
// found, and nothing links to them.
func TestApp_PageOneHasOneURL(t *testing.T) {
	app := blogApp(t, 25, collage.LocaleConfig{})
	h := app.Handler()
	if code, body := get(t, h, "/blog"); code != http.StatusOK || !strings.Contains(body, "page 1 next /blog/page/2 prev </p>") {
		t.Errorf("/blog = %d %q", code, body)
	}
	if code, body := get(t, h, "/blog/page/2"); code != http.StatusOK || !strings.Contains(body, "page 2 next /blog/page/3 prev /blog</p>") {
		t.Errorf("/blog/page/2 = %d %q", code, body)
	}
	for _, path := range []string{"/blog/page/1", "/blog/page/02", "/blog/page/4", "/blog/page/99999999999999999999"} {
		if code, _ := get(t, h, path); code != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", path, code)
		}
	}
}

// A static export writes /blog and /blog/page/2…Last, never /blog/page/1, and
// PageURLs — what a sitemap reads — lists the same pages.
func TestApp_StaticExport(t *testing.T) {
	app := blogApp(t, 25, collage.LocaleConfig{})
	out := t.TempDir()
	if _, err := collage.Build(context.Background(), app, collage.BuildOptions{OutDir: out}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	for file, want := range map[string]bool{
		"blog/index.html":        true,
		"blog/page/2/index.html": true,
		"blog/page/3/index.html": true,
		"blog/page/1/index.html": false,
		"blog/page/4/index.html": false,
	} {
		_, err := os.Stat(filepath.Join(out, filepath.FromSlash(file)))
		if (err == nil) != want {
			t.Errorf("%s written: %v, want %v", file, err == nil, want)
		}
	}
	urls, err := app.PageURLs(context.Background(), "blog-page")
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, u := range urls {
		paths = append(paths, u.Path)
	}
	if strings.Join(paths, " ") != "/blog/page/2 /blog/page/3" {
		t.Errorf("PageURLs = %v", paths)
	}
}

// PathNamed builds links through the page's names, so a locale prefix is right.
func TestApp_PathNamedLocale(t *testing.T) {
	app := blogApp(t, 25, collage.LocaleConfig{Default: "en", Supported: []string{"en", "tr"}})
	h := app.Handler()
	if code, body := get(t, h, "/tr/blog"); code != http.StatusOK || !strings.Contains(body, "next /tr/blog/page/2") {
		t.Errorf("/tr/blog = %d %q", code, body)
	}
	if code, body := get(t, h, "/tr/blog/page/2"); code != http.StatusOK || !strings.Contains(body, "prev /tr/blog</p>") {
		t.Errorf("/tr/blog/page/2 = %d %q", code, body)
	}
}

// Query paging on a cached page: with the page key among its cache params, each
// page is cached apart. The core hands the handler the cache params in its own
// (sorted) order, so only their presence is checked here; TestQuery_KeepsOthers
// pins the order Query keeps.
func TestApp_QueryCacheParams(t *testing.T) {
	app, err := collage.New(&collage.Config{
		Server: collage.ServerConfig{Host: "localhost", Port: 3000},
		Template: collage.TemplateConfig{FS: fstest.MapFS{
			"t/list.html": {Data: []byte(`<p>page {{.Current}} next {{.Links.Next}}</p>`)},
		}, Root: "t"},
		Cache: collage.CacheConfig{Enabled: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	const total = 50
	last := paginate.New(total, perPage, 1).Last
	data := collage.LoadTagged(func(_ context.Context, rc *collage.RenderContext) (listing, []string, error) {
		n := paginate.FromQuery(rc, "page", last)
		return listing{Current: n, Links: paginate.Query(rc, "page").For(paginate.New(total, perPage, n))}, nil, nil
	})
	page := collage.NewPage("search").
		WithContent(collage.NewFragment("search-list", "list.html").WithData(data).Build()).
		WithPath("en", "/search").
		WithCacheParams("q", "page").
		Incremental(time.Hour).
		Build()
	if err := app.RegisterPage(page); err != nil {
		t.Fatal(err)
	}
	h := app.Handler()
	for _, n := range []int{2, 3, 2} {
		code, body := get(t, h, "/search?q=a&page="+strconv.Itoa(n))
		if code != http.StatusOK || !strings.Contains(body, "page "+strconv.Itoa(n)+" next /search?") ||
			!strings.Contains(body, "q=a") || !strings.Contains(body, "page="+strconv.Itoa(n+1)) {
			t.Errorf("page %d = %d %q", n, code, body)
		}
	}
}

// A listing whose path has its own parameters — /tag/{tag} and
// /tag/{tag}/page/{n} — links its pages within the same tag.
func TestApp_PathNamedCarriesOtherParams(t *testing.T) {
	app, err := collage.New(&collage.Config{
		Server: collage.ServerConfig{Host: "localhost", Port: 3000},
		Template: collage.TemplateConfig{FS: fstest.MapFS{
			"t/list.html": {Data: []byte(`<p>next {{.Links.Next}} prev {{.Links.Prev}}</p>`)},
		}, Root: "t"},
	})
	if err != nil {
		t.Fatal(err)
	}
	const total = 25
	last := paginate.New(total, perPage, 1).Last
	data := func(fromPath bool) collage.Data {
		return collage.LoadTagged(func(_ context.Context, rc *collage.RenderContext) (listing, []string, error) {
			n := 1
			if fromPath {
				var err error
				if n, err = paginate.FromPath(rc, "n", last); err != nil {
					return listing{}, nil, err
				}
			}
			links, err := paginate.PathNamed(rc, "tag", "tag-page")
			if err != nil {
				return listing{}, nil, err
			}
			return listing{Current: n, Links: links.For(paginate.New(total, perPage, n))}, nil, nil
		})
	}
	for _, page := range []*collage.Page{
		collage.NewPage("tag").WithContent(collage.NewFragment("tag-list", "list.html").WithData(data(false)).Required().Build()).
			WithPath("en", "/tag/{tag}").Build(),
		collage.NewPage("tag-page").WithContent(collage.NewFragment("tag-page-list", "list.html").WithData(data(true)).Required().Build()).
			WithPath("en", "/tag/{tag}/page/{n}").Build(),
	} {
		if err := app.RegisterPage(page); err != nil {
			t.Fatal(err)
		}
	}
	h := app.Handler()
	if code, body := get(t, h, "/tag/go"); code != http.StatusOK || !strings.Contains(body, "next /tag/go/page/2 prev </p>") {
		t.Errorf("/tag/go = %d %q", code, body)
	}
	if code, body := get(t, h, "/tag/go/page/2"); code != http.StatusOK || !strings.Contains(body, "next /tag/go/page/3 prev /tag/go</p>") {
		t.Errorf("/tag/go/page/2 = %d %q", code, body)
	}
}
