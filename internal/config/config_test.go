package config

import (
	"strings"
	"testing"
)

const password = "hunter2-app-password"

func env(overrides map[string]string) func(string) string {
	values := map[string]string{
		"LISTEN_ADDR":       "127.0.0.1:8080",
		"SITE_URL":          "https://cranlab.ca",
		"DID":               "did:plc:ogl52fc2rdooori2ayf74can",
		"PDS_URL":           "http://127.0.0.1:3000",
		"PDS_PUBLIC_URL":    "https://weslandia.ca",
		"PUBLICATION_URI":   "at://did:plc:ogl52fc2rdooori2ayf74can/site.standard.publication/3mxcsltpg2k26",
		"WEBMENTION_DOMAIN": "cranlab.ca",
		"APP_PASSWORD":      password,
	}
	for k, v := range overrides {
		values[k] = v
	}
	return func(key string) string { return values[key] }
}

func TestLoadAll(t *testing.T) {
	got, err := Load(env(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := Config{
		ListenAddr:       "127.0.0.1:8080",
		SiteURL:          "https://cranlab.ca",
		DID:              "did:plc:ogl52fc2rdooori2ayf74can",
		PDSURL:           "http://127.0.0.1:3000",
		PDSPublicURL:     "https://weslandia.ca",
		PublicationURI:   "at://did:plc:ogl52fc2rdooori2ayf74can/site.standard.publication/3mxcsltpg2k26",
		WebmentionDomain: "cranlab.ca",
		AppPassword:      password,
	}
	if got != want {
		t.Errorf("Load =\n%+v\nwant\n%+v", got, want)
	}
}

func TestLoadRequired(t *testing.T) {
	for _, name := range []string{
		"LISTEN_ADDR", "SITE_URL", "DID", "PDS_URL", "PDS_PUBLIC_URL", "PUBLICATION_URI", "WEBMENTION_DOMAIN",
	} {
		for _, value := range []string{"", "   "} {
			t.Run(name+"="+strings.TrimSpace(value), func(t *testing.T) {
				_, err := Load(env(map[string]string{name: value}))
				if err == nil {
					t.Fatalf("Load with %s=%q: want an error", name, value)
				}
				if !strings.Contains(err.Error(), name) {
					t.Errorf("error %q doesn't name %s", err, name)
				}
			})
		}
	}
}

func TestLoadReportsEveryProblem(t *testing.T) {
	_, err := Load(env(map[string]string{"DID": "", "SITE_URL": "ftp://cranlab.ca"}))
	if err == nil {
		t.Fatal("want an error")
	}
	for _, name := range []string{"DID", "SITE_URL"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q doesn't name %s", err, name)
		}
	}
}

func TestLoadAppPasswordOptional(t *testing.T) {
	got, err := Load(env(map[string]string{"APP_PASSWORD": ""}))
	if err != nil {
		t.Fatalf("Load without APP_PASSWORD: %v", err)
	}
	if got.AppPassword != "" {
		t.Errorf("AppPassword = %q, want empty", got.AppPassword)
	}
}

func TestLoadTrims(t *testing.T) {
	got, err := Load(env(map[string]string{
		"SITE_URL":          "  https://cranlab.ca/ ",
		"PDS_URL":           "http://127.0.0.1:3000/",
		"PDS_PUBLIC_URL":    "https://weslandia.ca//",
		"WEBMENTION_DOMAIN": " cranlab.ca\n",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.SiteURL != "https://cranlab.ca" || got.PDSURL != "http://127.0.0.1:3000" ||
		got.PDSPublicURL != "https://weslandia.ca" || got.WebmentionDomain != "cranlab.ca" {
		t.Errorf("not trimmed: %+v", got)
	}
}

func TestLoadRejectsBadValues(t *testing.T) {
	tests := []struct{ name, value string }{
		{"SITE_URL", "cranlab.ca"},
		{"SITE_URL", "ftp://cranlab.ca"},
		{"SITE_URL", "https://"},
		{"PDS_URL", "127.0.0.1:3000"},
		{"PDS_PUBLIC_URL", "/xrpc"},
		{"DID", "plc:ogl52fc2rdooori2ayf74can"},
		{"PUBLICATION_URI", "https://cranlab.ca"},
	}
	for _, tt := range tests {
		t.Run(tt.name+"="+tt.value, func(t *testing.T) {
			_, err := Load(env(map[string]string{tt.name: tt.value}))
			if err == nil {
				t.Fatalf("Load with %s=%q: want an error", tt.name, tt.value)
			}
			if !strings.Contains(err.Error(), tt.name) {
				t.Errorf("error %q doesn't name %s", err, tt.name)
			}
		})
	}
}

func TestLoadNeverLeaksPassword(t *testing.T) {
	_, err := Load(env(map[string]string{"SITE_URL": "", "DID": "x", "PDS_URL": "nope"}))
	if err == nil {
		t.Fatal("want an error")
	}
	if strings.Contains(err.Error(), password) {
		t.Errorf("error leaks the app password: %q", err)
	}
}
