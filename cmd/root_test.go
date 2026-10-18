// SPDX-FileContributor: 4evy <git@evy.pink>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"

	"github.com/asciimoo/hister/config"
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

func newShowUserTestCommand() *cobra.Command {
	cmd := &cobra.Command{Use: showUserCmd.Use, Args: showUserCmd.Args, Run: showUserCmd.Run, RunE: showUserCmd.RunE, SilenceUsage: true}
	cmd.Flags().Bool("show-token", false, "")
	cmd.SetErr(io.Discard)
	return cmd
}

func newCreateUserTestCommand() *cobra.Command {
	command := &cobra.Command{Use: createUserCmd.Use, Args: createUserCmd.Args, Run: createUserCmd.Run, RunE: createUserCmd.RunE, PreRunE: createUserCmd.PreRunE, SilenceUsage: true}
	command.Flags().Bool("admin", false, "")
	command.Flags().Bool("offline", false, "")
	command.SetErr(io.Discard)
	return command
}

func TestCreateUserUsesServer(t *testing.T) {
	previousConfig, previousDB, previousTransport, previousPrompt := cfg, model.DB, http.DefaultTransport, promptPassword
	t.Cleanup(func() {
		cfg, model.DB, http.DefaultTransport, promptPassword = previousConfig, previousDB, previousTransport, previousPrompt
	})
	cfg = testutil.InitModel(t)
	cfg.App.UserHandling = false
	cfg.App.AccessToken = "admin-token"
	if err := cfg.UpdateBaseURL("https://remote.example/hister"); err != nil {
		t.Fatal(err)
	}
	localDB := model.DB
	prompts := 0
	promptPassword = func(string) (string, error) {
		prompts++
		return "password123", nil
	}
	calls := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Method != http.MethodPost || req.URL.String() != "https://remote.example/hister/api/users" || req.Header.Get("X-Access-Token") != "admin-token" || req.Header.Get("Origin") != "hister://" {
			t.Errorf("unexpected creation request: %s %s", req.Method, req.URL)
		}
		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if len(payload) != 3 || payload["username"] != "alice" || payload["password"] != "password123" || payload["is_admin"] != true {
			t.Error("creation request has incorrect account fields")
		}
		return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(`{"id":77,"username":"alice","is_admin":true}`))}, nil
	})
	command := newCreateUserTestCommand()
	command.SetArgs([]string{"alice", "--admin"})
	var output bytes.Buffer
	command.SetOut(&output)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || prompts != 2 {
		t.Errorf("requests = %d, password prompts = %d, want 1 and 2", calls, prompts)
	}
	if model.DB != localDB {
		t.Fatal("remote creation opened a local database")
	}
	if _, err := model.GetUser("alice"); err == nil {
		t.Error("remote creation added a local account")
	}
	if !strings.Contains(output.String(), "User created: alice") || strings.Contains(output.String(), "password123") || strings.Contains(output.String(), "admin-token") {
		t.Errorf("unexpected command output: %q", output.String())
	}
}

func TestCreateUserConnectionFlags(t *testing.T) {
	if createUserCmd.Annotations[executionScopeAnnotation] != string(executionScopeHybrid) {
		t.Error("create-user is not classified as hybrid")
	}
	flags, _ := applicableGlobalFlags(createUserCmd)
	for _, name := range []string{"server-url", "token", "client-timeout"} {
		if !flags[name] {
			t.Errorf("connection flag %s is not applicable", name)
		}
	}
	if createUserCmd.PreRun != nil {
		t.Error("create-user still unconditionally initializes the local database")
	}
	if flag := createUserCmd.Flags().Lookup("offline"); flag == nil || flag.Value.Type() != "bool" || flag.DefValue != "false" {
		t.Error("create-user does not have an explicit --offline flag")
	}
}

func TestCreateUserWithoutLocalDatabase(t *testing.T) {
	previousConfig, previousDB, previousTransport, previousPrompt := cfg, model.DB, http.DefaultTransport, promptPassword
	t.Cleanup(func() {
		cfg, model.DB, http.DefaultTransport, promptPassword = previousConfig, previousDB, previousTransport, previousPrompt
	})
	cfg = config.CreateDefaultConfig()
	cfg.App.UserHandling = false
	cfg.Server.Database = "/does/not/exist/hister.db"
	model.DB = nil
	promptPassword = func(string) (string, error) { return "password123", nil }
	for _, tc := range []struct {
		name, body string
		status     int
		wantError  bool
	}{
		{"ordinary user", `{"id":77,"username":"alice","is_admin":false}`, http.StatusCreated, false},
		{"denied", "permission denied", http.StatusForbidden, true},
		{"old server", "not found", http.StatusNotFound, true},
		{"duplicate", `{"code":"username_exists","error":"username already exists"}`, http.StatusConflict, true},
		{"invalid response", `<html>old server</html>`, http.StatusOK, true},
		{"connection failure", "", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				var payload map[string]any
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload["is_admin"] != false {
					t.Error("creation requested administrator privileges by default")
				}
				if tc.status == 0 {
					return nil, io.ErrUnexpectedEOF
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})
			command := newCreateUserTestCommand()
			command.SetArgs([]string{"alice"})
			var output bytes.Buffer
			command.SetOut(&output)
			if err := command.Execute(); (err != nil) != tc.wantError {
				t.Fatalf("error = %v, want error = %t", err, tc.wantError)
			}
			if calls != 1 || model.DB != nil {
				t.Fatalf("requests = %d, opened local database = %t", calls, model.DB != nil)
			}
			if tc.wantError && output.Len() != 0 {
				t.Fatalf("reported success on failure: %s", &output)
			}
		})
	}
}

func TestCreateUserOffline(t *testing.T) {
	for _, isAdmin := range []bool{false, true} {
		t.Run(fmt.Sprintf("admin=%t", isAdmin), func(t *testing.T) {
			previousConfig, previousDB, previousTransport, previousPrompt := cfg, model.DB, http.DefaultTransport, promptPassword
			t.Cleanup(func() {
				cfg, model.DB, http.DefaultTransport, promptPassword = previousConfig, previousDB, previousTransport, previousPrompt
			})
			cfg = testutil.Config(t)
			cfg.App.UserHandling = true
			cfg.App.AccessToken = "remote-admin-token"
			model.DB = nil
			t.Cleanup(func() {
				if model.DB != nil {
					db, err := model.DB.DB()
					if err != nil {
						t.Error(err)
					} else if err := db.Close(); err != nil {
						t.Error(err)
					}
				}
			})
			promptPassword = func(string) (string, error) { return "password123", nil }
			http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Error("offline creation contacted the server")
				return nil, errors.New("unexpected request")
			})
			command := newCreateUserTestCommand()
			command.SetArgs([]string{"alice", "--offline", fmt.Sprintf("--admin=%t", isAdmin)})
			var output bytes.Buffer
			command.SetOut(&output)
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			if model.DB == nil {
				t.Fatal("offline creation did not open the database")
			}
			user, err := model.AuthenticateUser("alice", "password123")
			if err != nil || user.IsAdmin != isAdmin || user.Token == "" {
				t.Fatalf("offline account has incorrect credentials or privileges: %v", err)
			}
			if !strings.Contains(output.String(), "User created: alice") || strings.Contains(output.String(), user.Token) != isAdmin {
				t.Fatal("offline creation must print the token only for an administrator")
			}
			if strings.Contains(output.String(), "password123") || strings.Contains(output.String(), "remote-admin-token") {
				t.Fatal("offline output disclosed other credentials")
			}
		})
	}
}

func TestCreateUserInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		name, username, password, confirmation, wantError string
		offline, disabled, invalidDB                      bool
		promptError                                       error
	}{
		{name: "blank username", username: " ", wantError: "username"},
		{name: "control character", username: "ali\nce", wantError: "username"},
		{name: "short password", username: "alice", password: "1234567", wantError: "password"},
		{name: "long password", username: "alice", password: strings.Repeat("é", 37), wantError: "password"},
		{name: "mismatch", username: "alice", password: "password123", confirmation: "different", wantError: "match"},
		{name: "cancelled", username: "alice", promptError: errors.New("cancelled"), wantError: "cancelled"},
		{name: "offline disabled", username: "alice", offline: true, disabled: true, wantError: "user_handling"},
		{name: "offline cancelled", username: "alice", offline: true, promptError: errors.New("cancelled"), wantError: "cancelled"},
		{name: "offline database unavailable", username: "alice", offline: true, invalidDB: true, password: "password123", confirmation: "password123", wantError: "database"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previousConfig, previousDB, previousTransport, previousPrompt := cfg, model.DB, http.DefaultTransport, promptPassword
			t.Cleanup(func() {
				cfg, model.DB, http.DefaultTransport, promptPassword = previousConfig, previousDB, previousTransport, previousPrompt
			})
			cfg = testutil.Config(t)
			cfg.App.UserHandling = !tc.disabled
			if tc.invalidDB {
				cfg.App.Directory = filepath.Join(cfg.App.Directory, "missing")
			}
			model.DB = nil
			prompts := 0
			promptPassword = func(string) (string, error) {
				prompts++
				if prompts == 1 {
					return tc.password, tc.promptError
				}
				return tc.confirmation, nil
			}
			http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Error("invalid input contacted the server")
				return nil, errors.New("unexpected request")
			})
			command := newCreateUserTestCommand()
			command.SetArgs([]string{tc.username, fmt.Sprintf("--offline=%t", tc.offline)})
			var output bytes.Buffer
			command.SetOut(&output)
			if err := command.Execute(); err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error = %v, want %q", err, tc.wantError)
			}
			if !tc.invalidDB && model.DB != nil {
				t.Fatal("invalid input opened a local database")
			}
			if output.Len() != 0 {
				t.Fatalf("reported success on failure: %s", &output)
			}
		})
	}
}

func TestShowUserUsesRemoteAccount(t *testing.T) {
	for _, showToken := range []bool{false, true} {
		t.Run(fmt.Sprintf("show token=%t", showToken), func(t *testing.T) {
			previousConfig, previousDB, previousTransport := cfg, model.DB, http.DefaultTransport
			t.Cleanup(func() { cfg, model.DB, http.DefaultTransport = previousConfig, previousDB, previousTransport })
			cfg = testutil.InitModel(t)
			cfg.App.AccessToken = "admin-token"
			if err := cfg.UpdateBaseURL("https://remote.example/hister"); err != nil {
				t.Fatal(err)
			}
			localUser := testutil.CreateUser(t, "alice")
			const remoteUser = `{"id":77,"username":"alice","is_admin":true,"created_at":"2024-01-02T03:04:05Z","updated_at":"2025-06-07T08:09:10Z"}`
			var paths []string
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				paths = append(paths, req.URL.Path)
				if req.Method != http.MethodGet || req.URL.Host != "remote.example" || req.Header.Get("X-Access-Token") != "admin-token" {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL)
				}
				body := ""
				switch req.URL.Path {
				case "/hister/api/users":
					if req.URL.Query().Get("username") != "alice" || req.URL.Query().Get("limit") != "1" {
						t.Errorf("unexpected lookup query: %s", req.URL.RawQuery)
					}
					body = `{"users":[` + remoteUser + `]}`
				case "/hister/api/users/77/token":
					if !showToken {
						t.Error("retrieved a token without --show-token")
					}
					body = `{"token":"remote-user-token"}`
				default:
					t.Errorf("unexpected request path: %s", req.URL.Path)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})
			cmd := newShowUserTestCommand()
			cmd.SetArgs([]string{"alice", fmt.Sprintf("--show-token=%t", showToken)})
			var output bytes.Buffer
			cmd.SetOut(&output)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			wantCalls := 1
			if showToken {
				wantCalls++
			}
			if len(paths) != wantCalls {
				t.Errorf("requests = %v, want %d", paths, wantCalls)
			}
			for _, want := range []string{
				"Username:   alice", "ID:         77", "Admin:      yes",
				"Created at: " + formatLocalTimestamp(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)),
				"Updated at: " + formatLocalTimestamp(time.Date(2025, 6, 7, 8, 9, 10, 0, time.UTC)),
			} {
				if !strings.Contains(output.String(), want) {
					t.Errorf("output missing %q: %q", want, output.String())
				}
			}
			if strings.Contains(output.String(), "remote-user-token") != showToken || strings.Contains(output.String(), "Token:") != showToken {
				t.Error("token output did not follow --show-token")
			}
			if strings.Contains(output.String(), localUser.Token) || strings.Contains(output.String(), "admin-token") || strings.Contains(output.String(), "\x1b") {
				t.Error("output contains an unintended credential or terminal escape sequence")
			}
		})
	}
}

func TestShowUserConnectionFlags(t *testing.T) {
	if showUserCmd.Annotations[executionScopeAnnotation] != string(executionScopeRemote) {
		t.Error("show-user is not classified as remote")
	}
	flags, _ := applicableGlobalFlags(showUserCmd)
	for _, name := range []string{"server-url", "token", "client-timeout"} {
		if !flags[name] {
			t.Errorf("connection flag %s is not applicable", name)
		}
	}
	if showUserCmd.PreRun != nil || showUserCmd.PreRunE != nil {
		t.Error("show-user still initializes local user handling")
	}
	showToken := showUserCmd.Flags().Lookup("show-token")
	if showToken == nil || showToken.Value.Type() != "bool" {
		t.Fatal("--show-token is not registered as a boolean")
	}
	authToken := rootCmd.PersistentFlags().Lookup("token")
	if flag := showUserCmd.Flags().Lookup("token"); flag != nil && flag != authToken {
		t.Fatal("local --token shadows the global authentication flag")
	}
	oldToken, oldTokenChanged := authToken.Value.String(), authToken.Changed
	oldShowToken, oldShowTokenChanged := showToken.Value.String(), showToken.Changed
	t.Cleanup(func() {
		if err := authToken.Value.Set(oldToken); err != nil {
			t.Error(err)
		}
		if err := showToken.Value.Set(oldShowToken); err != nil {
			t.Error(err)
		}
		authToken.Changed, showToken.Changed = oldTokenChanged, oldShowTokenChanged
	})
	if err := showUserCmd.ParseFlags([]string{"alice", "--token", "admin-token", "--show-token"}); err != nil {
		t.Fatal(err)
	}
	if authToken.Value.String() != "admin-token" || showToken.Value.String() != "true" {
		t.Error("authentication and token display flags were not parsed independently")
	}
}

func TestShowUserWithoutLocalDatabase(t *testing.T) {
	previousConfig, previousDB, previousTransport := cfg, model.DB, http.DefaultTransport
	t.Cleanup(func() { cfg, model.DB, http.DefaultTransport = previousConfig, previousDB, previousTransport })
	cfg = config.CreateDefaultConfig()
	cfg.App.UserHandling = false
	cfg.Server.Database = "/does/not/exist/hister.db"
	model.DB = nil
	const metadata = `{"users":[{"id":77,"username":"alice","is_admin":false}]}`
	for _, tc := range []struct {
		name, response          string
		status                  int
		wantError, tokenRequest bool
	}{
		{"remote account", metadata, http.StatusOK, false, false},
		{"denied", "permission denied", http.StatusForbidden, true, false},
		{"old server", "not found", http.StatusNotFound, true, false},
		{"server failure", "unavailable", http.StatusInternalServerError, true, false},
		{"missing user", `{"users":[]}`, http.StatusOK, true, false},
		{"invalid response", "<html>old server</html>", http.StatusOK, true, false},
		{"token denied", "permission denied", http.StatusForbidden, true, true},
		{"account deleted after lookup", "not found", http.StatusNotFound, true, true},
		{"invalid token response", `{}`, http.StatusOK, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if tc.tokenRequest && calls == 1 {
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(metadata))}, nil
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.response))}, nil
			})
			cmd := newShowUserTestCommand()
			cmd.SetArgs([]string{"alice", fmt.Sprintf("--show-token=%t", tc.tokenRequest)})
			var output bytes.Buffer
			cmd.SetOut(&output)
			if err := cmd.Execute(); (err != nil) != tc.wantError {
				t.Fatalf("error = %v, want error = %t", err, tc.wantError)
			}
			wantCalls := 1
			if tc.tokenRequest {
				wantCalls++
			}
			if calls != wantCalls || model.DB != nil {
				t.Errorf("requests = %d, local database opened = %t", calls, model.DB != nil)
			}
			if tc.wantError && output.Len() != 0 {
				t.Errorf("partial output on failure: %s", &output)
			}
			if !tc.wantError && !strings.Contains(output.String(), "Admin:      no") {
				t.Errorf("remote account output missing: %s", &output)
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
