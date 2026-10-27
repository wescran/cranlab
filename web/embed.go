// Package web holds the site's templates and static files, embedded in the
// binary.
package web

import "embed"

// FS contains templates/ (html/template files) and static/ (CSS, fonts, icons,
// images), served under /static/.
//
//go:embed templates static
var FS embed.FS
