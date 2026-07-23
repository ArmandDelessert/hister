package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/testutil"

	"github.com/gorilla/sessions"
)

func newFormTokenTestHandler(t *testing.T, cfg *config.Config, called *bool) http.Handler {
	t.Helper()
	sessionStore = sessions.NewCookieStore([]byte(strings.Repeat("x", 32)))
	h := endpointHandler(func(c *webContext) {
		*called = true
		c.Response.WriteHeader(http.StatusNoContent)
	})
	h = withCSRF(h)
	h = withTokenAuth(h)
	return http.HandlerFunc(createHandler(cfg, h))
}

func TestAddFormAccessTokenAuthenticatesAndBypassesCSRF(t *testing.T) {
	cfg := testutil.Config(t)
	cfg.App.AccessToken = "secret"
	called := false
	handler := newFormTokenTestHandler(t, cfg, &called)
	body := url.Values{
		"access_token": {"secret"},
		"url":          {"https://example.com"},
	}.Encode()

	rec := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/add", strings.NewReader(body), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Origin":       "https://unrelated.example",
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("POST /api/add status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if !called {
		t.Fatal("POST /api/add did not reach the protected handler")
	}
}

func TestAddFormAccessTokenIsRouteAndEncodingScoped(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		contentType string
		body        string
	}{
		{
			name:        "invalid token",
			target:      "/api/add",
			contentType: "application/x-www-form-urlencoded",
			body:        url.Values{"access_token": {"invalid"}}.Encode(),
		},
		{
			name:        "missing token",
			target:      "/api/add",
			contentType: "application/x-www-form-urlencoded",
			body:        url.Values{"url": {"https://example.com"}}.Encode(),
		},
		{
			name:        "query token",
			target:      "/api/add?access_token=secret",
			contentType: "application/x-www-form-urlencoded",
		},
		{
			name:        "JSON token",
			target:      "/api/add",
			contentType: "application/json",
			body:        `{"access_token":"secret","url":"https://example.com"}`,
		},
		{
			name:        "plain text token",
			target:      "/api/add",
			contentType: "text/plain",
			body:        "access_token=secret",
		},
		{
			name:        "legacy route",
			target:      "/add",
			contentType: "application/x-www-form-urlencoded",
			body:        url.Values{"access_token": {"secret"}}.Encode(),
		},
		{
			name:        "different route",
			target:      "/api/delete",
			contentType: "application/x-www-form-urlencoded",
			body:        url.Values{"access_token": {"secret"}}.Encode(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testutil.Config(t)
			cfg.App.AccessToken = "secret"
			called := false
			handler := newFormTokenTestHandler(t, cfg, &called)

			rec := testutil.ServeHTTP(t, handler, http.MethodPost, tt.target, strings.NewReader(tt.body), map[string]string{
				"Content-Type": tt.contentType,
				"Origin":       "https://unrelated.example",
			})

			if rec.Code != http.StatusForbidden {
				t.Fatalf("POST %s status = %d, want %d", tt.target, rec.Code, http.StatusForbidden)
			}
			if called {
				t.Fatalf("POST %s reached the protected handler", tt.target)
			}
		})
	}
}

func TestAddFormAccessTokenRequiresConfiguredAuthentication(t *testing.T) {
	cfg := testutil.Config(t)
	sessionStore = sessions.NewCookieStore([]byte(strings.Repeat("x", 32)))
	called := false
	h := withCSRF(func(c *webContext) {
		called = true
		c.Response.WriteHeader(http.StatusNoContent)
	})
	handler := http.HandlerFunc(createHandler(cfg, h))
	body := url.Values{"access_token": {"secret"}}.Encode()

	rec := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/add", strings.NewReader(body), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Origin":       "https://unrelated.example",
	})

	if rec.Code == http.StatusNoContent {
		t.Fatal("POST /api/add without a configured access token was accepted")
	}
	if called {
		t.Fatal("POST /api/add without a configured access token reached the protected handler")
	}
}

func TestDecodeAddDocumentFromFormIncludesRenderedContent(t *testing.T) {
	body := url.Values{
		"access_token": {"secret"},
		"url":          {"https://example.com/article"},
		"title":        {"Rendered title"},
		"text":         {"Rendered text"},
		"html":         {`<html><body><main>Rendered content</main></body></html>`},
		"favicon":      {"data:image/png;base64,AA=="},
		"label":        {"qutebrowser"},
	}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/api/add", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	doc, jsonData, err := decodeAddDocument(req)
	if err != nil {
		t.Fatal(err)
	}
	if jsonData {
		t.Fatal("form document was identified as JSON")
	}
	if doc.URL != "https://example.com/article" {
		t.Errorf("URL = %q, want %q", doc.URL, "https://example.com/article")
	}
	if doc.Title != "Rendered title" {
		t.Errorf("Title = %q, want %q", doc.Title, "Rendered title")
	}
	if doc.Text != "Rendered text" {
		t.Errorf("Text = %q, want %q", doc.Text, "Rendered text")
	}
	if doc.HTML != `<html><body><main>Rendered content</main></body></html>` {
		t.Errorf("HTML = %q, want rendered HTML", doc.HTML)
	}
	if doc.Favicon != "data:image/png;base64,AA==" {
		t.Errorf("Favicon = %q, want submitted data URI", doc.Favicon)
	}
	if doc.Label != "qutebrowser" {
		t.Errorf("Label = %q, want %q", doc.Label, "qutebrowser")
	}
}
