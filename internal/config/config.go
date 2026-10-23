// Package config reads the site server's settings from environment variables.
package config

// Config holds the server's settings. See the spec's "Configuration" table.
type Config struct {
	ListenAddr       string // LISTEN_ADDR, e.g. 127.0.0.1:8080
	SiteURL          string // SITE_URL, e.g. https://cranlab.ca
	DID              string // DID
	PDSURL           string // PDS_URL, e.g. http://127.0.0.1:3000
	PDSPublicURL     string // PDS_PUBLIC_URL, e.g. https://weslandia.ca
	PublicationURI   string // PUBLICATION_URI
	WebmentionDomain string // WEBMENTION_DOMAIN, e.g. cranlab.ca
	AppPassword      string // APP_PASSWORD (optional; empty disables syndication)
}

// Load reads the configuration through getenv (os.Getenv in main, a map in
// tests).
//
// Rules:
//   - Every variable except APP_PASSWORD is required. A missing or blank one is
//     an error naming the variable. Report every problem at once
//     (errors.Join), not just the first.
//   - Values are trimmed of surrounding whitespace.
//   - SITE_URL, PDS_URL and PDS_PUBLIC_URL must be absolute http or https URLs
//     with a host; a trailing slash is removed.
//   - DID must start with "did:", and PUBLICATION_URI with "at://".
//   - Error messages never include APP_PASSWORD's value.
func Load(getenv func(string) string) (Config, error) {
	panic("not implemented")
}
