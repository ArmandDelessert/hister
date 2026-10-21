package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v3"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/files"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/extractor"
	"github.com/asciimoo/hister/server/indexer"
	"github.com/asciimoo/hister/server/model"
	"github.com/asciimoo/hister/server/types"
)

func runtimeTestAddress(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func startRuntimeTestServer(t *testing.T) (*config.Config, func(), func(string, string, string, string) (int, string)) {
	t.Helper()
	candidate := config.CreateDefaultConfig()
	candidate.App.Directory = t.TempDir()
	candidate.App.AccessToken = "secret"
	candidate.Indexer.DetectLanguages = false
	candidate.Server.Address = runtimeTestAddress(t)
	candidate.Server.BaseURL = "http://" + candidate.Server.Address
	filename := filepath.Join(candidate.App.Directory, "config.yml")
	write := func() {
		t.Helper()
		raw, err := yaml.Marshal(candidate)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write()
	active, err := config.Load(filename)
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Init(active); err != nil {
		t.Fatal(err)
	}
	if err := extractor.Init(active.Extractors); err != nil {
		t.Fatal(err)
	}
	idx, err := indexer.New(active)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Listen(ctx, active, idx) }()
	hc := &http.Client{Timeout: 10 * time.Second}
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error("server failed: ", err)
			}
		case <-time.After(15 * time.Second):
			t.Error("server did not stop")
		}
		hc.CloseIdleConnections()
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := hc.Get(active.BaseURL("/health"))
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("server did not start: ", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	request := func(baseURL, path, token, body string) (int, string) {
		t.Helper()
		method := http.MethodGet
		if body != "" || path == "/api/reload" {
			method = http.MethodPost
		}
		req, err := http.NewRequest(method, baseURL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", "hister://")
		req.Header.Set("X-Access-Token", token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := hc.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Error(err)
			}
		}()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, string(raw)
	}
	return candidate, write, request
}

func TestFullReloadReopensIndexerAndAuthentication(t *testing.T) {
	c, write, request := startRuntimeTestServer(t)
	base := c.Server.BaseURL
	if status, body := request(base, "/api/add", "secret", `{"url":"https://example.com/saved","title":"Saved","text":"Retained content"}`); status != http.StatusCreated {
		t.Fatalf("add: %d %s", status, body)
	}
	c.App.DisablePreviews = true
	c.App.AccessToken = "replacement"
	c.Server.Metrics = true
	write()
	if status, body := request(base, "/api/reload", "secret", ""); status != http.StatusOK {
		t.Fatalf("reload: %d %s", status, body)
	}
	if status, _ := request(base, "/api/stats", "secret", ""); status != http.StatusForbidden {
		t.Fatalf("old token still accepted: %d", status)
	}
	if status, body := request(base, "/metrics", "replacement", ""); status != http.StatusOK {
		t.Fatalf("new server metrics: %d %s", status, body)
	}
	status, body := request(base, "/search?format=json&q=Retained", "replacement", "")
	var result indexer.Results
	if err := json.Unmarshal([]byte(body), &result); status != http.StatusOK || err != nil || result.Total != 1 {
		t.Fatalf("saved index unavailable after reload: %d %s (%v)", status, body, err)
	}
	if status, body := request(base, "/api/add", "replacement", `{"url":"https://example.com/after","title":"After","text":"New content","html":"<p>New content</p>"}`); status != http.StatusCreated {
		t.Fatalf("add after reload: %d %s", status, body)
	}
	status, body = request(base, "/api/document?url=https://example.com/after", "replacement", "")
	var saved document.Document
	if err := json.Unmarshal([]byte(body), &saved); status != http.StatusOK || err != nil || saved.HTMLKey != "" {
		t.Fatalf("indexer did not apply disable_previews: %d %s (%v)", status, body, err)
	}
}

func TestReloadPDFParserProcess(t *testing.T) {
	marker := slices.Index(os.Args, "--pdf-reload-parser-test")
	if marker < 0 {
		return
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil || string(data) != "%PDF-1.4 reload fixture" || marker+1 >= len(os.Args) {
		os.Exit(2)
	}
	_, _ = io.WriteString(os.Stdout, os.Args[marker+1])
	os.Exit(0)
}

func TestFullReloadChangesPDFParser(t *testing.T) {
	c, write, request := startRuntimeTestServer(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"document": document.Document{URL: "https://example.com/reload.pdf"},
		"pdf":      base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 reload fixture")),
	})
	if err != nil {
		t.Fatal(err)
	}
	upload := func(want int) {
		t.Helper()
		if status, body := request(c.Server.BaseURL, "/api/add_pdf", "secret", string(payload)); status != want {
			t.Fatalf("PDF upload: %d %s, want status %d", status, body, want)
		}
	}
	upload(http.StatusInternalServerError) // The built in parser rejects the fake PDF.
	for _, text := range []string{"First parser text", "Replacement parser text"} {
		c.Indexer.PDFParser = config.PDFParser{
			Command: executable,
			Args:    []string{"-test.run=^TestReloadPDFParserProcess$", "--", "--pdf-reload-parser-test", text},
			Timeout: 5,
		}
		write()
		if status, body := request(c.Server.BaseURL, "/api/reload", "secret", ""); status != http.StatusOK {
			t.Fatalf("reload PDF parser: %d %s", status, body)
		}
		upload(http.StatusCreated)
		status, body := request(c.Server.BaseURL, "/api/document?url=https://example.com/reload.pdf", "secret", "")
		var saved document.Document
		if err := json.Unmarshal([]byte(body), &saved); status != http.StatusOK || err != nil || saved.Text != text {
			t.Fatalf("reloaded parser output: %d %s (%v), want text %q", status, body, err, text)
		}
	}
	c.Indexer.PDFParser = config.PDFParser{Timeout: 30}
	write()
	if status, body := request(c.Server.BaseURL, "/api/reload", "secret", ""); status != http.StatusOK {
		t.Fatalf("restore built in PDF parser: %d %s", status, body)
	}
	upload(http.StatusInternalServerError)
}

func TestFullReloadChangesListenerAndDatabase(t *testing.T) {
	c, write, request := startRuntimeTestServer(t)
	oldBase := c.Server.BaseURL
	c.Server.Address = runtimeTestAddress(t)
	c.Server.BaseURL = "http://" + c.Server.Address + "/new"
	c.Server.Database = "replacement.sqlite3"
	c.App.Title = "Replacement"
	write()
	if status, body := request(oldBase, "/api/reload", "secret", ""); status != http.StatusOK {
		t.Fatalf("reload: %d %s", status, body)
	}
	if status, body := request(c.Server.BaseURL, "/api/config", "secret", ""); status != http.StatusOK || !strings.Contains(body, `"title":"Replacement"`) {
		t.Fatalf("new listener unavailable: %d %s", status, body)
	}
	if _, err := os.Stat(filepath.Join(c.App.Directory, c.Server.Database)); err != nil {
		t.Fatal("new database was not opened: ", err)
	}
}

func TestFullReloadRestoresPreviousRuntimeOnOpenFailure(t *testing.T) {
	c, write, request := startRuntimeTestServer(t)
	base := c.Server.BaseURL
	c.Server.Database = filepath.Join("missing-parent", "db.sqlite3")
	c.App.Title = "Must not apply"
	write()
	if status, body := request(base, "/api/reload", "secret", ""); status != http.StatusInternalServerError || !strings.Contains(body, "restored") {
		t.Fatalf("failed reload: %d %s", status, body)
	}
	if status, body := request(base, "/api/config", "secret", ""); status != http.StatusOK || strings.Contains(body, "Must not apply") {
		t.Fatalf("previous runtime not restored: %d %s", status, body)
	}
	c.Server.Database = "db.sqlite3"
	c.App.Title = "Recovered"
	write()
	if status, body := request(base, "/api/reload", "secret", ""); status != http.StatusOK {
		t.Fatalf("retry failed: %d %s", status, body)
	}
}

func TestFullReloadConcurrentRequests(t *testing.T) {
	c, _, request := startRuntimeTestServer(t)
	var workers sync.WaitGroup
	for worker := range 4 {
		workers.Go(func() {
			for range 8 {
				path := "/api/stats"
				if worker < 2 {
					path = "/api/reload"
				}
				status, body := request(c.Server.BaseURL, path, "secret", "")
				if status != http.StatusOK && status != http.StatusConflict && status != http.StatusServiceUnavailable {
					t.Errorf("%s: %d %s", path, status, body)
				}
			}
		})
	}
	workers.Wait()
	if status, body := request(c.Server.BaseURL, "/api/stats", "secret", ""); status != http.StatusOK {
		t.Fatalf("server unavailable after concurrent reloads: %d %s", status, body)
	}
}

func TestFullReloadClosesWebSockets(t *testing.T) {
	c, _, request := startRuntimeTestServer(t)
	conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(c.Server.BaseURL, "http")+"/search", http.Header{"Origin": {c.Server.BaseURL}, "X-Access-Token": {"secret"}})
	if resp != nil {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Error(err)
			}
		}()
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	}()
	if status, body := request(c.Server.BaseURL, "/api/reload", "secret", ""); status != http.StatusOK {
		t.Fatalf("reload with open WebSocket: %d %s", status, body)
	}
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("old WebSocket survived reload")
	} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		t.Fatal("WebSocket was not closed")
	}
}

func TestFullReloadRetainsRuntimeWhenListenerUnavailable(t *testing.T) {
	c, write, request := startRuntimeTestServer(t)
	base := c.Server.BaseURL
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := occupied.Close(); err != nil {
			t.Error(err)
		}
	}()
	c.Server.Address = occupied.Addr().String()
	c.App.AccessToken = "replacement"
	write()
	if status, body := request(base, "/api/reload", "secret", ""); status != http.StatusBadRequest || !strings.Contains(body, "server.address") {
		t.Fatalf("occupied listener: %d %s", status, body)
	}
	if status, body := request(base, "/api/stats", "secret", ""); status != http.StatusOK {
		t.Fatalf("old runtime changed: %d %s", status, body)
	}
}

func TestFullReloadSwitchesDataDirectories(t *testing.T) {
	c, write, request := startRuntimeTestServer(t)
	base, original := c.Server.BaseURL, c.App.Directory
	if status, body := request(base, "/api/add", "secret", `{"url":"https://example.com/original","text":"Original content"}`); status != http.StatusCreated {
		t.Fatalf("seed index: %d %s", status, body)
	}
	c.App.Directory = filepath.Join(t.TempDir(), "new-data")
	write()
	if status, body := request(base, "/api/reload", "secret", ""); status != http.StatusOK {
		t.Fatalf("new directory: %d %s", status, body)
	}
	if status, body := request(base, "/api/document?url=https://example.com/original", "secret", ""); status != http.StatusNotFound {
		t.Fatalf("new directory unexpectedly contains old document: %d %s", status, body)
	}
	status, body := request(base, "/api/diagnostics", "secret", "")
	var checks []types.DiagnosticCheck
	if err := json.Unmarshal([]byte(body), &checks); status != http.StatusOK || err != nil {
		t.Fatalf("diagnostics: %d %s", status, body)
	}
	for _, check := range checks {
		if strings.HasPrefix(check.Name, "index.") && check.Status != "ok" {
			t.Fatalf("new index metadata: %+v", check)
		}
	}
	c.App.Directory = original
	write()
	if status, body := request(base, "/api/reload", "secret", ""); status != http.StatusOK {
		t.Fatalf("restore directory: %d %s", status, body)
	}
	if status, body := request(base, "/api/document?url=https://example.com/original", "secret", ""); status != http.StatusOK {
		t.Fatalf("original data lost: %d %s", status, body)
	}
}

func TestFullReloadDrainsInFlightRequests(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	oldEndpoints := Endpoints
	Endpoints = append(append([]*Endpoint(nil), Endpoints...), &Endpoint{
		Path: "/api/test-drain", Method: GET,
		Handler: func(c *webContext) {
			close(entered)
			<-release
			if _, _, err := c.Indexer.GetMetadata(); err != nil {
				http.Error(c.Response, "index closed before request finished", 500)
				return
			}
			serve200(c)
		},
	})
	t.Cleanup(func() { Endpoints = oldEndpoints })
	c, _, request := startRuntimeTestServer(t)
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	slowDone := make(chan int, 1)
	go func() { status, _ := request(c.Server.BaseURL, "/api/test-drain", "secret", ""); slowDone <- status }()
	<-entered
	reloadDone := make(chan int, 1)
	go func() { status, _ := request(c.Server.BaseURL, "/api/reload", "secret", ""); reloadDone <- status }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		status, _ := request(c.Server.BaseURL, "/health", "", "")
		if status == http.StatusServiceUnavailable {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("reload never paused incoming requests")
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case status := <-reloadDone:
		t.Fatalf("reload returned before draining: %d", status)
	default:
	}
	close(release)
	if status := <-slowDone; status != http.StatusOK {
		t.Fatalf("in flight request failed: %d", status)
	}
	if status := <-reloadDone; status != http.StatusOK {
		t.Fatalf("reload failed: %d", status)
	}
}

func TestFullReloadRestartsDirectoryWatchers(t *testing.T) {
	c, write, request := startRuntimeTestServer(t)
	dir := t.TempDir()
	filename := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(filename, []byte("Initial watched content"), 0o600); err != nil {
		t.Fatal(err)
	}
	// File indexing compares modification times at second precision.
	initialTime := time.Now().Add(-2 * time.Second)
	if err := os.Chtimes(filename, initialTime, initialTime); err != nil {
		t.Fatal(err)
	}
	c.Indexer.Directories = []*config.Directory{{Path: dir, Filetypes: []string{"txt"}, Label: "watched"}}
	write()
	if status, body := request(c.Server.BaseURL, "/api/reload", "secret", ""); status != http.StatusOK {
		t.Fatalf("reload: %d %s", status, body)
	}
	path := "/api/document?url=" + url.QueryEscape(files.PathToFileURL(filename))
	waitText := func(text string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			status, body := request(c.Server.BaseURL, path, "secret", "")
			if status == http.StatusOK && strings.Contains(body, text) && strings.Contains(body, `"label":"watched"`) {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("watcher did not index %q: %d %s", text, status, body)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	waitText("Initial watched content")
	if err := os.WriteFile(filename, []byte("Updated watched content"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitText("Updated watched content")
	c.Indexer.Directories = nil
	write()
	if status, body := request(c.Server.BaseURL, "/api/reload", "secret", ""); status != http.StatusOK {
		t.Fatalf("remove watcher: %d %s", status, body)
	}
	if err := os.WriteFile(filename, []byte("Must not index"), 0o600); err != nil {
		t.Fatal(err)
	}
	laterTime := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(filename, laterTime, laterTime); err != nil {
		t.Fatal(err)
	}
	// Allow the former watcher's debounce interval to elapse.
	time.Sleep(350 * time.Millisecond)
	if status, body := request(c.Server.BaseURL, path, "secret", ""); status != http.StatusOK || !strings.Contains(body, "Updated watched content") {
		t.Fatalf("removed watcher still active: %d %s", status, body)
	}
}

func TestFullReloadRestartsSemanticIndexing(t *testing.T) {
	embeddings := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"embedding":[1,0,0],"index":0}]}`)
	}))
	defer embeddings.Close()
	c, write, request := startRuntimeTestServer(t)
	c.SemanticSearch.Enable = true
	c.SemanticSearch.Dimensions = 3
	c.SemanticSearch.EmbeddingEndpoint = embeddings.URL
	write()
	if status, body := request(c.Server.BaseURL, "/api/reload", "secret", ""); status != http.StatusOK {
		t.Fatalf("enable semantic search: %d %s", status, body)
	}
	if status, body := request(c.Server.BaseURL, "/api/config", "secret", ""); status != http.StatusOK || !strings.Contains(body, `"semanticEnabled":true`) {
		t.Fatalf("semantic search unavailable: %d %s", status, body)
	}
	if status, body := request(c.Server.BaseURL, "/api/add", "secret", `{"url":"https://example.com/semantic","text":"Semantic queue work"}`); status != http.StatusCreated {
		t.Fatalf("add: %d %s", status, body)
	}
	c.SemanticSearch.Enable = false
	write()
	if status, body := request(c.Server.BaseURL, "/api/reload", "secret", ""); status != http.StatusOK {
		t.Fatalf("stop semantic search: %d %s", status, body)
	}
	if status, body := request(c.Server.BaseURL, "/api/config", "secret", ""); status != http.StatusOK || !strings.Contains(body, `"semanticEnabled":false`) {
		t.Fatalf("semantic search still enabled: %d %s", status, body)
	}
}
