package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/extractor"
	"github.com/asciimoo/hister/server/model"
	"github.com/asciimoo/hister/server/testutil"
)

func TestPrepareConfigurationDraftValidatesFreshExtractors(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(fmt.Sprintf("invalid_%v", invalid), func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("HISTER_DATA_DIR", dir)
			filename := filepath.Join(dir, "config.yml")
			// A missing executable must not trigger runtime preparation or
			// execution during form validation.
			raw := fmt.Sprintf("extractors:\n  ytdlp:\n    enable: true\n    options:\n      binary: %q\n", filepath.Join(dir, "must-not-run"))
			if invalid {
				raw += "      max_concurrent_jobs: -1\n  basic:\n    options:\n      unknown: private-credential\nsensitive_content_patterns:\n  broken: '[private-credential'\n"
			}
			if err := os.WriteFile(filename, []byte(raw), 0o600); err != nil {
				t.Fatal(err)
			}
			c, _, err := config.LoadForInspection(filename, true)
			if err != nil {
				t.Fatal(err)
			}
			before, err := json.Marshal(extractor.DefaultRegistry().List())
			if err != nil {
				t.Fatal(err)
			}
			next, fields := prepareConfigurationDraft(c, []config.SettingEdit{{Path: "app.title", Action: "set", Value: json.RawMessage(`"Candidate"`)}})
			if invalid {
				if next != nil || len(fields) != 3 {
					t.Fatalf("invalid extractor settings accepted or errors not aggregated: %v", fields)
				}
				paths := map[string]bool{}
				for _, field := range fields {
					paths[field.Path] = true
				}
				if !paths["extractors.ytdlp"] || !paths["extractors.basic"] {
					t.Fatalf("extractor errors lack stable paths: %v", fields)
				}
				encoded, err := json.Marshal(fields)
				if err != nil || strings.Contains(string(encoded), "private-credential") {
					t.Fatal("extractor validation exposed option values")
				}
			} else if next == nil || len(fields) > 0 || next.App.Title != "Candidate" {
				t.Fatalf("valid draft rejected: %v", fields)
			}
			after, err := json.Marshal(extractor.DefaultRegistry().List())
			if err != nil || string(after) != string(before) {
				t.Fatal("draft validation changed the active extractor chain")
			}
			if c.App.Title != "Hister" {
				t.Fatal("draft validation mutated the active configuration")
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatal("draft validation created runtime files")
			}
		})
	}
}

func TestConfigurationCatalogKeepsUnsupportedCategoriesReadOnly(t *testing.T) {
	fields := configurationCatalog()
	var app, enable, option bool
	for _, field := range fields {
		switch field.Path {
		case "app.title":
			app = field.ReadOnlyReason == "" && field.Default == "Hister"
		case "extractors.ytdlp.enable":
			enable = field.ReadOnlyReason != "" && field.Default == false
		case "extractors.ytdlp.options.binary":
			option = field.ReadOnlyReason != "" && field.Schema.Type == "unknown" && field.Default == nil
		}
	}
	if !app || !enable || !option {
		t.Fatal("combined catalog lost defaults or enabled an unsupported setting")
	}
}

func newReloadTestServer(t *testing.T, multiUser bool) (*config.Config, http.Handler, func(string)) {
	t.Helper()
	dir := t.TempDir()
	filename := filepath.Join(dir, "config.yml")
	write := func(extra string) {
		t.Helper()
		content := fmt.Sprintf("app:\n  directory: %q\n  access_token: secret\n  user_handling: %t\n  public: true\n  title: Original\nserver:\n  address: 127.0.0.1:0\n  base_url: http://127.0.0.1:4433/hister\nindexer:\n  detect_languages: false\n", dir, multiUser)
		content = strings.Replace(content, "  title: Original\n", extra, 1)
		if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("  title: Original\n")
	cfg, err := config.Load(filename)
	if err != nil {
		t.Fatal(err)
	}
	testutil.InitModelWithConfig(t, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	runtime := &serverRuntime{ctx: ctx, failures: make(chan error, 1)}
	listener, err := newListener(cfg.Server)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	runtime.current, err = runtime.generation(cfg, newServerTestIndexer(t, cfg))
	if err != nil {
		cancel()
		_ = listener.Close()
		t.Fatal(err)
	}
	runtime.serving = runtime.startListener(listener)
	t.Cleanup(func() { cancel(); runtime.close() })
	return cfg, runtime, write
}

func reloadRequest(t *testing.T, handler http.Handler, token string) int {
	t.Helper()
	rec := testutil.ServeHTTP(t, handler, http.MethodPost, "/hister/api/reload", nil, map[string]string{"Origin": "hister://", "X-Access-Token": token})
	return rec.Code
}

func TestReloadAppliesConfigurationAndRules(t *testing.T) {
	cfg, handler, write := newReloadTestServer(t, false)
	write("  title: Reloaded\n  color_scheme: dark\n")
	if err := os.WriteFile(cfg.RulesPath(), []byte(`{"skip":["private"],"aliases":{"docs":"documentation"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if status := reloadRequest(t, handler, "secret"); status != http.StatusOK {
		t.Fatalf("reload status = %d, want 200", status)
	}
	rec := testutil.ServeHTTP(t, handler, http.MethodGet, "/hister/api/config", nil, nil)
	var result struct{ Title, ColorScheme string }
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Title != "Reloaded" || result.ColorScheme != "dark" {
		t.Fatalf("configuration was not reloaded: %+v", result)
	}
	rec = testutil.ServeHTTP(t, handler, http.MethodGet, "/hister/api/rules", nil, map[string]string{"X-Access-Token": "secret"})
	if !strings.Contains(rec.Body.String(), "private") || !strings.Contains(rec.Body.String(), "documentation") {
		t.Fatalf("rules were not reloaded: %s", rec.Body)
	}
	// Subsequent rule writes must update the active generation and its file.
	headers := map[string]string{"Origin": "hister://", "X-Access-Token": "secret", "Content-Type": "application/x-www-form-urlencoded"}
	rec = testutil.ServeHTTP(t, handler, http.MethodPost, "/hister/api/rules", strings.NewReader(url.Values{"skip": {"updated"}}.Encode()), headers)
	if rec.Code != http.StatusOK {
		t.Fatalf("rule update failed: %d %s", rec.Code, rec.Body)
	}
	write("")
	if status := reloadRequest(t, handler, "secret"); status != http.StatusOK {
		t.Fatalf("second reload status = %d", status)
	}
	rec = testutil.ServeHTTP(t, handler, http.MethodGet, "/hister/api/config", nil, nil)
	if !strings.Contains(rec.Body.String(), `"title":"Hister"`) {
		t.Fatalf("removed settings did not revert to defaults: %s", rec.Body)
	}
	rec = testutil.ServeHTTP(t, handler, http.MethodGet, "/hister/api/rules", nil, headers)
	if !strings.Contains(rec.Body.String(), "updated") || strings.Contains(rec.Body.String(), "private") {
		t.Fatalf("reload lost a rule update: %s", rec.Body)
	}
}

func TestReloadRejectsInvalidConfigurationWithoutChangingServer(t *testing.T) {
	for _, failure := range []string{"invalid YAML", "missing config", "invalid rules", "missing rules", "log file", "log format", "invalid extractor", "invalid sensitive pattern"} {
		t.Run(failure, func(t *testing.T) {
			cfg, handler, write := newReloadTestServer(t, false)
			write("  title: Must not apply\n")
			var err error
			switch failure {
			case "invalid YAML":
				err = os.WriteFile(cfg.Filename(), []byte("app: ["), 0o600)
			case "missing config":
				err = os.Remove(cfg.Filename())
			case "invalid rules":
				err = os.WriteFile(cfg.RulesPath(), []byte(`{"skip":["["]}`), 0o600)
			case "missing rules":
				err = os.Remove(cfg.RulesPath())
			case "log file":
				write(fmt.Sprintf("  title: Must not apply\n  log_file: %q\n", filepath.Join(t.TempDir(), "missing", "hister.log")))
			case "log format":
				write("  title: Must not apply\n  log_format: invalid\n")
			case "invalid extractor":
				write("  title: Must not apply\nextractors:\n  nonexistent:\n    enabled: true\n")
			case "invalid sensitive pattern":
				write("  title: Must not apply\nsensitive_content_patterns:\n  test: '['\n")
			}
			if err != nil {
				t.Fatal(err)
			}
			rec := testutil.ServeHTTP(t, handler, http.MethodPost, "/hister/api/reload", nil, map[string]string{"Origin": "hister://", "X-Access-Token": "secret"})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("reload status = %d, want 400: %s", rec.Code, rec.Body)
			}
			if strings.HasPrefix(failure, "log ") && !strings.Contains(rec.Body.String(), "app.log_") {
				t.Fatalf("missing logging error explanation: %s", rec.Body)
			}
			rec = testutil.ServeHTTP(t, handler, http.MethodGet, "/hister/api/config", nil, nil)
			if !strings.Contains(rec.Body.String(), `"title":"Original"`) {
				t.Fatalf("failed reload changed active configuration: %s", rec.Body)
			}
			if failure == "missing rules" {
				if _, err := os.Stat(cfg.RulesPath()); !os.IsNotExist(err) {
					t.Fatalf("reload recreated missing rules: %v", err)
				}
			}
		})
	}
}

func TestReloadConcurrentRequestsAndRuleUpdates(t *testing.T) {
	_, handler, _ := newReloadTestServer(t, false)
	var workers sync.WaitGroup
	for worker := range 3 {
		workers.Go(func() {
			for range 8 {
				method, path, body := http.MethodGet, "/api/rules", ""
				switch worker {
				case 1:
					method, path = http.MethodPost, "/api/reload"
				case 2:
					method, path, body = http.MethodPost, "/api/add_alias", "alias-keyword=docs&alias-value=documentation"
				}
				rec := testutil.ServeHTTP(t, handler, method, "/hister"+path, strings.NewReader(body), map[string]string{"Origin": "hister://", "X-Access-Token": "secret", "Content-Type": "application/x-www-form-urlencoded"})
				if rec.Code != http.StatusOK && rec.Code != http.StatusConflict && rec.Code != http.StatusServiceUnavailable {
					t.Errorf("%s: status %d, body %s", path, rec.Code, rec.Body)
				}
			}
		})
	}
	workers.Wait()
}

func TestReloadAuthorization(t *testing.T) {
	for _, multiUser := range []bool{false, true} {
		t.Run(fmt.Sprint(multiUser), func(t *testing.T) {
			_, handler, _ := newReloadTestServer(t, multiUser)
			adminToken := "secret"
			forbiddenTokens := []string{"", "wrong"}
			if multiUser {
				user := testutil.CreateUser(t, "reader")
				admin, err := model.CreateUser("admin", "password123", true)
				if err != nil {
					t.Fatal(err)
				}
				adminToken = admin.Token
				forbiddenTokens = append(forbiddenTokens, user.Token, "secret")
			}
			for _, token := range forbiddenTokens {
				if status := reloadRequest(t, handler, token); status != http.StatusForbidden {
					t.Fatalf("unauthorized reload status = %d, want 403", status)
				}
			}
			rec := testutil.ServeHTTP(t, handler, http.MethodPost, "/hister/api/reload", nil, map[string]string{"X-Access-Token": adminToken, "Origin": "https://other.example"})
			if rec.Code != http.StatusForbidden {
				t.Fatalf("cross origin reload status = %d, want 403", rec.Code)
			}
			if status := reloadRequest(t, handler, adminToken); status != http.StatusOK {
				t.Fatalf("authorized reload status = %d, want 200", status)
			}
		})
	}
}

func TestReloadPreservesAuthenticatedSession(t *testing.T) {
	_, handler, write := newReloadTestServer(t, false)
	login := testutil.ServeHTTP(t, handler, http.MethodPost, "/hister/api/token-login", strings.NewReader(`{"token":"secret"}`), map[string]string{"Origin": "hister://", "Content-Type": "application/json"})
	if login.Code != http.StatusOK {
		t.Fatalf("login: %d %s", login.Code, login.Body)
	}
	cookie := responseSessionCookie(t, login)
	write("  title: Reloaded\n")
	if status := reloadRequest(t, handler, "secret"); status != http.StatusOK {
		t.Fatalf("reload: %d", status)
	}
	req := httptest.NewRequest(http.MethodGet, "/hister/api/history", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("session lost after reload: %d %s", rec.Code, rec.Body)
	}
}
