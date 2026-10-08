// Package paginate computes pages for collage listings: the arithmetic of a
// pager, a numbered window of page links, path-paged links a static export can
// write (/blog, /blog/page/2, …) and query-paged links for server-only lists.
//
// It is a library, not a plugin: nothing is registered, and there is nothing to
// configure. A page uses it in its data handler and template.
package paginate

// Version is this library's release.
const Version = "0.1.0"
