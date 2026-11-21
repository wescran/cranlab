// Package weburl validates URLs used in site config and rendered links.
package weburl

import "net/url"

// Absolute reports whether raw is an HTTP(S) URL with a hostname.
func Absolute(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != ""
}
