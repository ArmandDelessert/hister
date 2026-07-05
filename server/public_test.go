package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/model"
	"github.com/asciimoo/hister/server/testutil"

	"github.com/gorilla/sessions"
)

func newPublicTokenTestServer(t *testing.T) (*config.Config, http.Handler) {
	return newTokenTestServer(t, true)
}

func newTokenTestServer(t *testing.T, public bool) (*config.Config, http.Handler) {
	return newTokenTestServerWithLogLevel(t, public, "info")
}

func newTokenTestServerWithLogLevel(t *testing.T, public bool, logLevel string) (*config.Config, http.Handler) {
	t.Helper()
	cfg := testutil.Config(t)
	cfg.App.AccessToken = "secret"
	cfg.App.Public = public
	cfg.App.LogLevel = logLevel
	cfg.Server.Address = "127.0.0.1:4433"
	if err := cfg.UpdateBaseURL("http://127.0.0.1:4433"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveRules(); err != nil {
		t.Fatal(err)
	}
	sessionStore = sessions.NewCookieStore([]byte(strings.Repeat("x", 32)))
	sessionStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 365,
		HttpOnly: true,
	}
	return cfg, registerEndpoints(cfg)
}

func TestPublicModeConfigResponse(t *testing.T) {
	_, handler := newPublicTokenTestServer(t)
	rec := testutil.ServeHTTP(t, handler, http.MethodGet, "/api/config", nil, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/config status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body struct {
		Title          string `json:"title"`
		Subtitle       string `json:"subtitle"`
		Public         bool   `json:"public"`
		Authenticated  bool   `json:"authenticated"`
		CanWrite       bool   `json:"canWrite"`
		HistoryEnabled bool   `json:"historyEnabled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Title != "Hister" {
		t.Fatalf("title = %q, want %q", body.Title, "Hister")
	}
	if body.Subtitle != "Your own search engine" {
		t.Fatalf("subtitle = %q, want %q", body.Subtitle, "Your own search engine")
	}
	if !body.Public {
		t.Fatal("public = false, want true")
	}
	if body.Authenticated {
		t.Fatal("authenticated = true, want false")
	}
	if body.CanWrite {
		t.Fatal("canWrite = true, want false")
	}
	if body.HistoryEnabled {
		t.Fatal("historyEnabled = true, want false")
	}
}

func TestPublicModeAllowsDocumentedPublicRoutes(t *testing.T) {
	cfg, handler := newPublicTokenTestServer(t)
	dir := t.TempDir()
	filePath := testutil.WriteFile(t, dir, "note.txt", []byte("public file"))
	cfg.Indexer.Directories = []*config.Directory{{Path: dir}}

	tests := []struct {
		name   string
		method string
		target string
		body   string
		want   int
	}{
		{name: "api docs", method: http.MethodGet, target: "/api", want: http.StatusOK},
		{name: "search", method: http.MethodGet, target: "/search?format=json", want: http.StatusBadRequest},
		{name: "file", method: http.MethodGet, target: "/api/file?path=" + filePath, want: http.StatusOK},
		{name: "mcp tools list", method: http.MethodPost, target: "/mcp", body: `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, want: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := testutil.ServeHTTP(t, handler, tt.method, tt.target, strings.NewReader(tt.body), nil)

			if rec.Code != tt.want {
				t.Fatalf("%s %s status = %d, want %d; body=%s", tt.method, tt.target, rec.Code, tt.want, rec.Body.String())
			}
		})
	}
}

func TestPublicModeProtectsWriteRoutes(t *testing.T) {
	_, handler := newPublicTokenTestServer(t)
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{name: "delete", method: http.MethodPost, target: "/api/delete", body: `{"query":"*"}`},
		{name: "add", method: http.MethodPost, target: "/api/add", body: `{"url":"https://example.com"}`},
		{name: "rules", method: http.MethodGet, target: "/api/rules"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := testutil.ServeHTTP(t, handler, tt.method, tt.target, strings.NewReader(tt.body), nil)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s %s status = %d, want %d", tt.method, tt.target, rec.Code, http.StatusForbidden)
			}
		})
	}
}

func TestPublicModeAllowsAuthenticatedProtectedRoutes(t *testing.T) {
	_, handler := newPublicTokenTestServer(t)
	rec := testutil.ServeHTTP(t, handler, http.MethodGet, "/api/add", nil, map[string]string{
		"Origin":         "hister://",
		"X-Access-Token": "secret",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/add status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestTokenLoginSetsHttpOnlySessionCookieAndAuthenticates(t *testing.T) {
	_, handler := newPublicTokenTestServer(t)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/token-login", strings.NewReader(`{"token":"secret"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("Origin", "hister://")
	loginRec := httptest.NewRecorder()

	handler.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("POST /api/token-login status = %d, want %d; body=%s", loginRec.Code, http.StatusOK, loginRec.Body.String())
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("POST /api/token-login did not set a cookie")
	}
	var sessionCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == storeName {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("POST /api/token-login did not set %q cookie", storeName)
	}
	if !sessionCookie.HttpOnly {
		t.Fatal("session cookie HttpOnly = false, want true")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/add", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/add with session cookie status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPublicModeDisablesHistoryForAuthenticatedCallers(t *testing.T) {
	_, handler := newPublicTokenTestServer(t)
	anonymousRec := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/history", strings.NewReader(`{"query":"q","url":"https://example.com"}`), map[string]string{
		"Origin": "hister://",
	})

	if anonymousRec.Code != http.StatusForbidden {
		t.Fatalf("anonymous POST /api/history status = %d, want %d", anonymousRec.Code, http.StatusForbidden)
	}

	readRec := testutil.ServeHTTP(t, handler, http.MethodGet, "/api/history", nil, map[string]string{
		"X-Access-Token": "secret",
	})

	if readRec.Code != http.StatusNotFound {
		t.Fatalf("authenticated GET /api/history status = %d, want %d", readRec.Code, http.StatusNotFound)
	}

	rec := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/history", strings.NewReader(`{"query":"q","url":"https://example.com"}`), map[string]string{
		"Origin":         "hister://",
		"X-Access-Token": "secret",
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("POST /api/history status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestMCPGetHistoryOpenedMode(t *testing.T) {
	cfg, handler := newTokenTestServer(t, false)
	cfg.Server.Database = "file::memory:"
	testutil.InitModelWithConfig(t, cfg)
	if err := model.UpdateHistory(0, "hister mcp", "https://example.com/mcp", "MCP result"); err != nil {
		t.Fatal(err)
	}
	if err := model.UpdateHistory(0, "history view", "https://example.com/history", "History result"); err != nil {
		t.Fatal(err)
	}

	rec := testutil.ServeHTTP(t, handler, http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_history","arguments":{"mode":"opened","limit":10}}}`), map[string]string{
		"X-Access-Token": "secret",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp get_history status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Result struct {
			Content []mcpTextContent `json:"content"`
		} `json:"result"`
		Error *mcpRPCError `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error != nil {
		t.Fatalf("MCP error = %+v", body.Error)
	}
	if len(body.Result.Content) != 1 {
		t.Fatalf("content length = %d, want 1", len(body.Result.Content))
	}
	text := body.Result.Content[0].Text
	for _, want := range []string{
		"Opened history items: 2",
		"Query: hister mcp",
		"URL: https://example.com/mcp",
		"Query: history view",
		"URL: https://example.com/history",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("history response missing %q in:\n%s", want, text)
		}
	}
}

func TestMCPGetHistoryDefaultsToIndexedMode(t *testing.T) {
	_, handler := newTokenTestServer(t, false)
	rec := testutil.ServeHTTP(t, handler, http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_history","arguments":{"limit":10}}}`), map[string]string{
		"X-Access-Token": "secret",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp get_history status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Result struct {
			Content []mcpTextContent `json:"content"`
		} `json:"result"`
		Error *mcpRPCError `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error != nil {
		t.Fatalf("MCP error = %+v", body.Error)
	}
	if len(body.Result.Content) != 1 {
		t.Fatalf("content length = %d, want 1", len(body.Result.Content))
	}
	if !strings.Contains(body.Result.Content[0].Text, "indexed history items") {
		t.Fatalf("default history response did not use indexed mode:\n%s", body.Result.Content[0].Text)
	}
}

func TestTokenAuthStillProtectsPublicRoutesWhenPublicModeDisabled(t *testing.T) {
	_, handler := newTokenTestServer(t, false)
	rec := testutil.ServeHTTP(t, handler, http.MethodGet, "/search?format=json", nil, nil)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("GET /search status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
