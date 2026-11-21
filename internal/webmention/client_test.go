package webmention_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wescran/cranlab/internal/webmention"
)

func TestLookupBudgetAndFailureDiscardPartialResults(t *testing.T) {
	for _, failure := range []string{"timeout", "http", "malformed"} {
		t.Run(failure, func(t *testing.T) {
			const target = "https://site.test/notes/abc"
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("target") == target {
					fmt.Fprint(w, `{"children":[{"wm-property":"mention-of","wm-source":"https://reader.test/post"}]}`)
					return
				}
				switch failure {
				case "timeout":
					<-r.Context().Done()
				case "http":
					w.WriteHeader(503)
				case "malformed":
					fmt.Fprint(w, `{"children":`)
				}
			}))
			defer api.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 6*time.Second)
			defer cancel()
			start := time.Now()
			got, err := webmention.New(api.URL, api.Client()).Interactions(ctx, target)
			if err == nil || len(got) != 0 {
				t.Fatalf("got %+v, %v after failure", got, err)
			}
			if failure == "timeout" {
				elapsed := time.Since(start)
				if !errors.Is(err, context.DeadlineExceeded) || elapsed < 2900*time.Millisecond || elapsed > 4*time.Second {
					t.Fatalf("lookup took %s: %v", elapsed, err)
				}
			}
		})
	}
}

func TestInteractionsMergeBothCanonicalTargetsBySource(t *testing.T) {
	const target = "https://site.test/articles/caf%C3%A9"
	const like = `{"wm-property":"like-of","wm-source":"https://sender.test/like","author":{"name":"Ann","photo":"https://sender.test/me.jpg"}}`
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/mentions.jf2" {
			http.NotFound(w, r)
			return
		}
		switch r.URL.Query().Get("target") {
		case target:
			fmt.Fprintf(w, `{"children":[%s]}`, like)
		case target + "/":
			fmt.Fprintf(w, `{"children":[%s,{"wm-property":"in-reply-to","wm-source":"https://sender.test/reply","content":{"text":"Thanks!"}}]}`, like)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()
	items, err := webmention.New(api.URL, api.Client()).Interactions(t.Context(), target)
	if err != nil || len(items) != 2 || items[0].AuthorName != "Ann" || items[0].PhotoURL != "https://sender.test/me.jpg" || items[1].Text != "Thanks!" {
		t.Fatalf("interactions = %+v, %v", items, err)
	}
}
