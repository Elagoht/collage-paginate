package paginate

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Elagoht/collage/pkg/collage"
)

// StaticParams lists a path-paged listing's later pages for WithStaticParams:
// {"n": "2"} … {"n": "<Last>"}. Page 1 is not listed, because it lives at the
// listing's own path. count reports, per locale, how many items there are; its
// error stops the build, as any static params error does.
func StaticParams(count func(ctx context.Context, locale string) (int, error), perPage int) collage.StaticParamsFunc {
	return func(ctx context.Context, locale string) ([]map[string]string, error) {
		total, err := count(ctx, locale)
		if err != nil {
			return nil, err
		}
		last := New(total, perPage, 1).Last
		params := make([]map[string]string, 0, max(0, last-1))
		for n := 2; n <= last; n++ {
			params = append(params, map[string]string{"n": strconv.Itoa(n)})
		}
		return params, nil
	}
}

// FromPath reads a path-paged page number from the path parameter "n", the one
// PathNamed and StaticParams write. It
// accepts only 2…last, spelled the one canonical way: "1" (page 1 is the
// listing's own path), a leading zero, a sign, anything but ASCII digits and
// any number out of range are not found, so every page has one address.
func FromPath(rc *collage.RenderContext, last int) (int, error) {
	raw := rc.Param("n")
	n, ok := parse(raw)
	if !ok || n < 2 || n > last {
		return 0, fmt.Errorf("%w: page %q", collage.ErrNotFound, raw)
	}
	return n, nil
}

// FromQuery reads a query-paged page number from the request's first value of
// key. Anything that is not a canonical number in 1…last is page 1: a search
// whose page went stale shows its first page rather than an error.
func FromQuery(rc *collage.RenderContext, key string, last int) int {
	n, ok := parse(rc.Request.URL.Query().Get(key))
	if !ok || n < 1 || n > last {
		return 1
	}
	return n
}

// parse reads a page number spelled canonically: ASCII digits, no leading zero,
// no sign, within an int.
func parse(raw string) (int, bool) {
	if raw == "" || len(raw) > 19 || raw[0] == '0' {
		return 0, false
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] < '0' || raw[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(raw)
	return n, err == nil
}
