// Package config reads the site server's settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wescran/cranlab/internal/weburl"
)

// Config holds the server's settings.
type Config struct {
	ListenAddr       string // LISTEN_ADDR, e.g. 127.0.0.1:8080
	SiteURL          string // SITE_URL, e.g. https://cranlab.ca
	DID              string // DID
	PDSURL           string // PDS_URL, e.g. http://127.0.0.1:3000
	PDSPublicURL     string // PDS_PUBLIC_URL, e.g. https://weslandia.ca
	PublicationURI   string // PUBLICATION_URI
	WebmentionDomain string // WEBMENTION_DOMAIN, e.g. cranlab.ca
	CacheDBPath      string // CACHE_DB_PATH (optional; defaults to ./cache.db)
	AppPassword      string // APP_PASSWORD (optional; empty disables syndication)
}

// Load reads and validates the configuration through getenv. Every variable
// except APP_PASSWORD and CACHE_DB_PATH is required; all problems are reported together. URLs
// lose any trailing slash.
func Load(getenv func(string) string) (Config, error) {
	var errs []error
	// value returns a trimmed variable, recording an error if it's required and blank.
	value := func(name string, required bool) string {
		v := strings.TrimSpace(getenv(name))
		if v == "" && required {
			errs = append(errs, fmt.Errorf("%s is required", name))
		}
		return v
	}
	// check records an error if a present value fails a validity test.
	check := func(name, v string, ok bool, want string) {
		if v != "" && !ok {
			errs = append(errs, fmt.Errorf("%s must be %s", name, want))
		}
	}

	requiredURL := func(name string) string {
		v := strings.TrimRight(strings.TrimSpace(getenv(name)), "/")
		if !weburl.Absolute(v) {
			errs = append(errs, fmt.Errorf("%s must be an http or https URL with a host", name))
		}
		return v
	}

	c := Config{
		ListenAddr:       value("LISTEN_ADDR", true),
		SiteURL:          requiredURL("SITE_URL"),
		DID:              value("DID", true),
		PDSURL:           requiredURL("PDS_URL"),
		PDSPublicURL:     requiredURL("PDS_PUBLIC_URL"),
		PublicationURI:   value("PUBLICATION_URI", true),
		WebmentionDomain: value("WEBMENTION_DOMAIN", true),
		AppPassword:      value("APP_PASSWORD", false),
		CacheDBPath:      value("CACHE_DB_PATH", false),
	}
	if c.CacheDBPath == "" {
		c.CacheDBPath = "./cache.db"
	}
	check("DID", c.DID, strings.HasPrefix(c.DID, "did:"), `a DID (starting with "did:")`)
	check("PUBLICATION_URI", c.PublicationURI, strings.HasPrefix(c.PublicationURI, "at://"), `an AT-URI (starting with "at://")`)

	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	return c, nil
}
