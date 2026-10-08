package paginate_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Elagoht/collage-paginate"
	"github.com/Elagoht/collage/pkg/collage"
)

func TestPath(t *testing.T) {
	l := paginate.Path("/blog", "/blog/page/{n}")
	if got := l.URL(1); got != "/blog" {
		t.Errorf("URL(1) = %q", got)
	}
	if got := l.URL(7); got != "/blog/page/7" {
		t.Errorf("URL(7) = %q", got)
	}
	defer func() {
		if recover() == nil {
			t.Error("a pattern without {n} did not panic")
		}
	}()
	paginate.Path("/blog", "/blog/page")
}

func TestFor(t *testing.T) {
	l := paginate.Path("/blog", "/blog/page/{n}")
	first := l.For(paginate.New(30, 10, 1))
	if first.Prev != "" || first.Next != "/blog/page/2" {
		t.Errorf("page 1: Prev %q, Next %q", first.Prev, first.Next)
	}
	last := l.For(paginate.New(30, 10, 3))
	if last.Prev != "/blog/page/2" || last.Next != "" {
		t.Errorf("page 3: Prev %q, Next %q", last.Prev, last.Next)
	}
	mid := l.For(paginate.New(200, 10, 10))
	for _, s := range mid.Window {
		if s.Gap && s.URL != "" {
			t.Errorf("a gap has a URL: %q", s.URL)
		}
		if !s.Gap && s.URL != l.URL(s.N) {
			t.Errorf("slot %d has URL %q", s.N, s.URL)
		}
	}
}

func TestStaticParams(t *testing.T) {
	count := func(total int, err error) func(context.Context, string) (int, error) {
		return func(context.Context, string) (int, error) { return total, err }
	}
	got, err := paginate.StaticParams(count(25, nil), 10)(context.Background(), "en")
	if err != nil || len(got) != 2 || got[0]["n"] != "2" || got[1]["n"] != "3" {
		t.Errorf("25 items: %v, %v", got, err)
	}
	got, err = paginate.StaticParams(count(5, nil), 10)(context.Background(), "en")
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("one page: %#v, %v", got, err)
	}
	boom := errors.New("boom")
	if _, err := paginate.StaticParams(count(0, boom), 10)(context.Background(), "en"); !errors.Is(err, boom) {
		t.Errorf("count error: %v", err)
	}
}

// rcFor builds a RenderContext for a request carrying params, enough for the
// functions that read only the request and the path parameters.
func rcFor(target string, params map[string]string) *collage.RenderContext {
	return &collage.RenderContext{Request: httptest.NewRequest(http.MethodGet, target, nil), PathParams: params}
}

// Other query parameters stay, in their order; page 1 drops the key; a repeated
// key keeps its first position and loses the rest.
func TestQuery_KeepsOthers(t *testing.T) {
	l := paginate.Query(rcFor("/search?q=go&sort=new&page=3", nil), "page")
	if got := l.URL(4); got != "/search?q=go&sort=new&page=4" {
		t.Errorf("URL(4) = %q", got)
	}
	if got := l.URL(1); got != "/search?q=go&sort=new" {
		t.Errorf("URL(1) = %q", got)
	}
	l = paginate.Query(rcFor("/search?page=3&q=go&page=9", nil), "page")
	if got := l.URL(2); got != "/search?page=2&q=go" {
		t.Errorf("repeated key: URL(2) = %q", got)
	}
	l = paginate.Query(rcFor("/search?q=a+b%26c", nil), "page")
	if got := l.URL(2); got != "/search?q=a+b%26c&page=2" {
		t.Errorf("encoded value: URL(2) = %q", got)
	}
	l = paginate.Query(rcFor("/search?page=5", nil), "page")
	if got := l.URL(1); got != "/search" {
		t.Errorf("only the page key: URL(1) = %q", got)
	}
}

func TestFromPath_Table(t *testing.T) {
	n, err := paginate.FromPath(rcFor("/blog/page/2", map[string]string{"n": "2"}), "n", 5)
	if err != nil || n != 2 {
		t.Errorf(`"2": %d, %v`, n, err)
	}
	for _, raw := range []string{"1", "01", "0", "-2", "+2", "2a", "", "99999999999999999999", "6", " 2", "２"} {
		_, err := paginate.FromPath(rcFor("/x", map[string]string{"n": raw}), "n", 5)
		if !errors.Is(err, collage.ErrNotFound) {
			t.Errorf("%q: %v, want ErrNotFound", raw, err)
		}
	}
}

func TestFromQuery(t *testing.T) {
	for target, want := range map[string]int{
		"/s?page=3":                     3,
		"/s?page=1":                     1,
		"/s":                            1,
		"/s?page=0":                     1,
		"/s?page=9":                     1,
		"/s?page=abc":                   1,
		"/s?page=03":                    1,
		"/s?page=2&page=4":              2,
		"/s?page=-1":                    1,
		"/s?page=999999999999999999999": 1,
	} {
		if got := paginate.FromQuery(rcFor(target, nil), "page", 5); got != want {
			t.Errorf("%s: %d, want %d", target, got, want)
		}
	}
}

// Query links keep the path as the request spelled it, so an escaped "?" or "#"
// in a path parameter does not end the path early.
func TestQuery_EscapedPath(t *testing.T) {
	for target, want := range map[string]string{
		"/s/a%3Fb?q=1": "/s/a%3Fb?q=1&page=2",
		"/s/a%23b?q=1": "/s/a%23b?q=1&page=2",
	} {
		if got := paginate.Query(rcFor(target, nil), "page").URL(2); got != want {
			t.Errorf("%s: URL(2) = %q, want %q", target, got, want)
		}
	}
}
