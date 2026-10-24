// Package config reads the site server's settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

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

// validURL reports whether raw is an absolute http or https URL with a host.
func validURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
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

	c := Config{
		ListenAddr:       value("LISTEN_ADDR", true),
		SiteURL:          strings.TrimRight(value("SITE_URL", true), "/"),
		DID:              value("DID", true),
		PDSURL:           strings.TrimRight(value("PDS_URL", true), "/"),
		PDSPublicURL:     strings.TrimRight(value("PDS_PUBLIC_URL", true), "/"),
		PublicationURI:   value("PUBLICATION_URI", true),
		WebmentionDomain: value("WEBMENTION_DOMAIN", true),
		AppPassword:      value("APP_PASSWORD", false),
	}
	const httpURL = "an http or https URL with a host"
	check("SITE_URL", c.SiteURL, validURL(c.SiteURL), httpURL)
	check("PDS_URL", c.PDSURL, validURL(c.PDSURL), httpURL)
	check("PDS_PUBLIC_URL", c.PDSPublicURL, validURL(c.PDSPublicURL), httpURL)
	check("DID", c.DID, strings.HasPrefix(c.DID, "did:"), `a DID (starting with "did:")`)
	check("PUBLICATION_URI", c.PublicationURI, strings.HasPrefix(c.PublicationURI, "at://"), `an AT-URI (starting with "at://")`)

	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	return c, nil
}
