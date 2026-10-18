// SPDX-FileContributor: 4evy <git@evy.pink>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"

	"github.com/asciimoo/hister/server/indexer"
	"github.com/asciimoo/hister/server/model"
	"github.com/asciimoo/hister/server/testutil"
)

func TestDeleteUserUsesRemoteAccount(t *testing.T) {
	previousConfig, previousDB, previousTransport := cfg, model.DB, http.DefaultTransport
	t.Cleanup(func() { cfg, model.DB, http.DefaultTransport = previousConfig, previousDB, previousTransport })
	cfg = testutil.InitModel(t)
	cfg.App.AccessToken = "admin-token"
	if err := cfg.UpdateBaseURL("https://remote.example/hister"); err != nil {
		t.Fatal(err)
	}
	localUser := testutil.CreateUser(t, "alice")
	var paths []string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		paths = append(paths, req.URL.Path)
		body := `{}`
		switch req.URL.Path {
		case "/hister/api/delete_user":
			if req.Method != http.MethodPost || req.URL.Host != "remote.example" || req.Header.Get("X-Access-Token") != "admin-token" {
				t.Errorf("unexpected request: %s %s", req.Method, req.URL)
			}
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if len(payload) != 2 || payload["username"] != "alice" || payload["purge"] != true {
				t.Errorf("payload = %v, want username and purge only", payload)
			}
			body = `{"username":"alice","deleted_documents":3}`
		case "/hister/search":
			// Allow the former workflow to complete so the test reproduces its local deletion.
			body = `{"total":1}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	cmd := &cobra.Command{Use: deleteUserCmd.Use, Args: deleteUserCmd.Args, Run: deleteUserCmd.Run, RunE: deleteUserCmd.RunE}
	cmd.Flags().Bool("purge", false, "")
	cmd.SetArgs([]string{"alice", "--purge"})
	var output bytes.Buffer
	cmd.SetOut(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := model.GetUserByID(localUser.ID); err != nil {
		t.Errorf("remote deletion changed the local account: %v", err)
	}
	if len(paths) != 1 || paths[0] != "/hister/api/delete_user" {
		t.Errorf("requests = %v, want one remote user deletion", paths)
	}
	if text := ansi.Strip(output.String()); !strings.Contains(text, "Purged 3 document(s)") || !strings.Contains(text, "User deleted: alice") {
		t.Errorf("output = %q", output.String())
	}
}

func TestDeleteUserRemoteErrorsDoNotFallBack(t *testing.T) {
	previousConfig, previousDB, previousTransport := cfg, model.DB, http.DefaultTransport
	t.Cleanup(func() { cfg, model.DB, http.DefaultTransport = previousConfig, previousDB, previousTransport })
	cfg = testutil.InitModel(t)
	// Remote deletion must work without enabling local user handling or opening its database.
	cfg.App.UserHandling = false
	cfg.Server.Database = "/does/not/exist/hister.db"
	localUser := testutil.CreateUser(t, "alice")
	if deleteUserCmd.Annotations[executionScopeAnnotation] != string(executionScopeRemote) {
		t.Error("delete-user is not classified as remote")
	}
	for _, tc := range []struct {
		name, body string
		status     int
		wantError  bool
	}{
		{"empty account", `{"username":"alice","deleted_documents":0}`, http.StatusOK, false},
		{"denied", "permission denied", http.StatusForbidden, true},
		{"missing endpoint", "not found", http.StatusNotFound, true},
		{"unsupported method", "method not allowed", http.StatusMethodNotAllowed, true},
		{"purge required", "use --purge", http.StatusConflict, true},
		{"failed purge", "internal server error", http.StatusInternalServerError, true},
		{"invalid response", "<html>old server</html>", http.StatusOK, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.URL.Path != "/api/delete_user" {
					t.Errorf("unexpected request: %s", req.URL)
				}
				var payload map[string]any
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload["purge"] != false {
					t.Errorf("purge was not explicitly requested: %v", payload)
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})
			cmd := &cobra.Command{Use: deleteUserCmd.Use, Args: deleteUserCmd.Args, PreRun: deleteUserCmd.PreRun, PreRunE: deleteUserCmd.PreRunE, RunE: deleteUserCmd.RunE}
			cmd.Flags().Bool("purge", false, "")
			cmd.SetArgs([]string{"alice"})
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(io.Discard)
			if err := cmd.Execute(); (err != nil) != tc.wantError {
				t.Fatalf("error = %v, want error = %t", err, tc.wantError)
			}
			if calls != 1 {
				t.Errorf("requests = %d, want one", calls)
			}
			if tc.wantError && strings.Contains(output.String(), "User deleted:") {
				t.Errorf("reported success after failure: %s", &output)
			}
			if _, err := model.GetUserByID(localUser.ID); err != nil {
				t.Errorf("remote command changed the local account: %v", err)
			}
		})
	}
}

func TestInitialAnalyzerFingerprint(t *testing.T) {
	tests := []struct {
		name            string
		indexerVersion  int
		detectLanguages bool
		keepStopwords   bool
		want            string
	}{
		{
			name:            "fresh index uses active configuration",
			indexerVersion:  -1,
			detectLanguages: true,
			keepStopwords:   true,
			want:            indexer.AnalyzerFingerprint(true, true),
		},
		{
			name:            "upgraded index with defaults matches active configuration",
			indexerVersion:  indexer.Version,
			detectLanguages: true,
			keepStopwords:   false,
			want:            indexer.AnalyzerFingerprint(true, false),
		},
		{
			name:            "upgraded index uses legacy configuration",
			indexerVersion:  indexer.Version,
			detectLanguages: true,
			keepStopwords:   true,
			want:            indexer.AnalyzerFingerprint(true, false),
		},
		{
			name:            "upgraded index retains disabled language detection",
			indexerVersion:  indexer.Version,
			detectLanguages: false,
			keepStopwords:   true,
			want:            indexer.AnalyzerFingerprint(false, false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := initialAnalyzerFingerprint(tt.indexerVersion, tt.detectLanguages, tt.keepStopwords)
			if got != tt.want {
				t.Fatalf("initialAnalyzerFingerprint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEmbeddingConfigWarning(t *testing.T) {
	tests := []struct {
		name   string
		stored string
		active string
		want   string
	}{
		{
			name:   "matching configuration",
			stored: "same",
			active: "same",
		},
		{
			name:   "semantic search disabled",
			stored: "stored",
		},
		{
			name:   "legacy indexed configuration",
			active: "active",
		},
		{
			name:   "changed configuration",
			stored: "stored",
			active: "active",
			want:   "The semantic search embedding configuration differs from the indexed configuration. Run `hister reindex` to update your embeddings.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := embeddingConfigWarning(tt.stored, tt.active); got != tt.want {
				t.Fatalf("embeddingConfigWarning() = %q, want %q", got, tt.want)
			}
		})
	}
}
