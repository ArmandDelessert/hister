// SPDX-FileContributor: Adam Tauber <asciimoo@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/indexer"
	"github.com/asciimoo/hister/server/model"
	"github.com/asciimoo/hister/server/testutil"
)

func TestSessionCookieSecurityFollowsBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		secure  bool
	}{
		{name: "HTTP", baseURL: "http://127.0.0.1:4433", secure: false},
		{name: "HTTPS", baseURL: "https://hister.example", secure: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newSessionStore([]byte(strings.Repeat("x", 32)), tt.baseURL, sessionMaxAge)
			if store.Options.Secure != tt.secure {
				t.Fatalf("Secure = %v, want %v", store.Options.Secure, tt.secure)
			}
			if !store.Options.HttpOnly {
				t.Fatal("HttpOnly = false, want true")
			}
			if store.Options.SameSite != http.SameSiteLaxMode {
				t.Fatalf("SameSite = %v, want %v", store.Options.SameSite, http.SameSiteLaxMode)
			}
		})
	}
}

func TestTokenSessionRotationAndLogoutRevokeSession(t *testing.T) {
	_, handler := newTokenTestServer(t, false)
	firstCookie := tokenLoginCookie(t, handler, nil)
	secondCookie := tokenLoginCookie(t, handler, firstCookie)
	if firstCookie.Value == secondCookie.Value {
		t.Fatal("token login did not rotate the session identifier")
	}
	assertSessionRejected(t, handler, firstCookie)
	assertSessionAccepted(t, handler, secondCookie)

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	logoutReq.Header.Set("Origin", "hister://")
	logoutReq.AddCookie(secondCookie)
	logoutRec := httptest.NewRecorder()
	handler.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("POST /api/logout status = %d, want %d", logoutRec.Code, http.StatusOK)
	}
	var expired bool
	for _, cookie := range logoutRec.Result().Cookies() {
		if cookie.Name == storeName && cookie.MaxAge < 0 {
			expired = true
		}
	}
	if !expired {
		t.Fatal("logout did not expire the session cookie")
	}
	if _, err := model.GetWebSession(sessionTokenHash(secondCookie.Value)); !errors.Is(err, model.ErrWebSessionNotFound) {
		t.Fatalf("logged out session lookup error = %v, want %v", err, model.ErrWebSessionNotFound)
	}
	assertSessionRejected(t, handler, secondCookie)
}

func TestTokenSessionExpirationRefreshesOnValidRequest(t *testing.T) {
	_, handler := newTokenTestServer(t, false)
	cookie := tokenLoginCookie(t, handler, nil)
	shortExpiry := time.Now().Add(time.Hour)
	if err := model.DB.Model(&model.WebSession{}).
		Where("token_hash = ?", sessionTokenHash(cookie.Value)).
		Update("expires_at", shortExpiry).Error; err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/add", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("session status = %d, want %d", rec.Code, http.StatusOK)
	}
	refreshedCookie := responseSessionCookie(t, rec)
	if refreshedCookie.Value != cookie.Value {
		t.Fatal("session refresh changed the session identifier")
	}
	if refreshedCookie.MaxAge != sessionMaxAge {
		t.Fatalf("refreshed cookie MaxAge = %d, want %d", refreshedCookie.MaxAge, sessionMaxAge)
	}
	minimumCookieExpiry := time.Now().Add(time.Duration(sessionMaxAge)*time.Second - time.Minute)
	if !refreshedCookie.Expires.After(minimumCookieExpiry) {
		t.Fatalf("refreshed cookie expiry = %v, want after %v", refreshedCookie.Expires, minimumCookieExpiry)
	}
	storedSession, err := model.GetWebSession(sessionTokenHash(cookie.Value))
	if err != nil {
		t.Fatal(err)
	}
	minimumExpiry := time.Now().Add(time.Duration(sessionMaxAge)*time.Second - time.Minute)
	if !storedSession.ExpiresAt.After(minimumExpiry) {
		t.Fatalf("refreshed expiry = %v, want after %v", storedSession.ExpiresAt, minimumExpiry)
	}
	if !storedSession.ExpiresAt.After(shortExpiry) {
		t.Fatalf("refreshed expiry = %v, want after previous expiry %v", storedSession.ExpiresAt, shortExpiry)
	}
}

func TestExpiredTokenSessionIsNotRefreshed(t *testing.T) {
	_, handler := newTokenTestServer(t, false)
	cookie := tokenLoginCookie(t, handler, nil)
	if err := model.DB.Model(&model.WebSession{}).
		Where("token_hash = ?", sessionTokenHash(cookie.Value)).
		Update("expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}

	assertSessionRejected(t, handler, cookie)
	if _, err := model.GetWebSession(sessionTokenHash(cookie.Value)); !errors.Is(err, model.ErrWebSessionNotFound) {
		t.Fatalf("expired session lookup error = %v, want %v", err, model.ErrWebSessionNotFound)
	}
}

func TestTokenChangeInvalidatesBrowserSession(t *testing.T) {
	cfg, handler := newTokenTestServer(t, false)
	cookie := tokenLoginCookie(t, handler, nil)
	cfg.App.AccessToken = "replacement"
	assertSessionRejected(t, handler, cookie)
}

func TestHeaderTokenDoesNotCreateBrowserSession(t *testing.T) {
	_, handler := newTokenTestServer(t, false)
	rec := testutil.ServeHTTP(t, handler, http.MethodGet, "/api/add", nil, map[string]string{
		"Origin":         "hister://",
		"X-Access-Token": "secret",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/add status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatalf("header token response set cookies: %v", rec.Result().Cookies())
	}
	var count int64
	if err := model.DB.Model(&model.WebSession{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("web session count = %d, want 0", count)
	}
}

func TestTokenLoginAcceptsCSRFTokenFromConfigResponse(t *testing.T) {
	_, handler := newTokenTestServer(t, false)
	origin := "https://proxy.example"

	missingCSRFReq := httptest.NewRequest(http.MethodPost, "/api/token-login", strings.NewReader(`{"token":"secret"}`))
	missingCSRFReq.Header.Set("Content-Type", "application/json")
	missingCSRFReq.Header.Set("Origin", origin)
	missingCSRFRec := httptest.NewRecorder()
	handler.ServeHTTP(missingCSRFRec, missingCSRFReq)
	if missingCSRFRec.Code != http.StatusForbidden {
		t.Fatalf("POST /api/token-login without CSRF token status = %d, want %d", missingCSRFRec.Code, http.StatusForbidden)
	}

	configReq := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	configReq.Header.Set("Origin", origin)
	configRec := httptest.NewRecorder()
	handler.ServeHTTP(configRec, configReq)
	if configRec.Code != http.StatusOK {
		t.Fatalf("GET /api/config status = %d, want %d", configRec.Code, http.StatusOK)
	}
	csrfToken := configRec.Header().Get("X-CSRF-Token")
	if csrfToken == "" {
		t.Fatal("GET /api/config did not return a CSRF token")
	}
	csrfCookie := responseSessionCookie(t, configRec)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/token-login", strings.NewReader(`{"token":"secret"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("Origin", origin)
	loginReq.Header.Set("X-CSRF-Token", csrfToken)
	loginReq.AddCookie(csrfCookie)
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("POST /api/token-login status = %d, want %d; body=%s", loginRec.Code, http.StatusOK, loginRec.Body.String())
	}
	assertSessionAccepted(t, handler, responseSessionCookie(t, loginRec))
}

func TestUserSessionStoresOnlyUserIDAndRejectsDeletedUser(t *testing.T) {
	cfg := testutil.Config(t)
	cfg.App.UserHandling = true
	cfg.Server.Address = "127.0.0.1:4433"
	if err := cfg.UpdateBaseURL("http://127.0.0.1:4433"); err != nil {
		t.Fatal(err)
	}
	cfg.Server.Database = "file::memory:"
	if err := cfg.SaveRules(); err != nil {
		t.Fatal(err)
	}
	testutil.InitModelWithConfig(t, cfg)
	sessionStore = newSessionStore([]byte(strings.Repeat("x", 32)), cfg.BaseURL(""), sessionMaxAge)
	user := testutil.CreateUser(t, "alice")
	handler := registerEndpoints(cfg, newServerTestIndexer(t, cfg))

	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"alice","password":"password123"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("Origin", "hister://")
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("POST /api/login status = %d, want %d", loginRec.Code, http.StatusOK)
	}
	cookie := responseSessionCookie(t, loginRec)
	storedSession, err := model.GetWebSession(sessionTokenHash(cookie.Value))
	if err != nil {
		t.Fatal(err)
	}
	values := make(map[any]any)
	if err := decodeSessionValues(storedSession.Data, &values); err != nil {
		t.Fatal(err)
	}
	if values["user_id"] != user.ID {
		t.Fatalf("stored user_id = %v, want %d", values["user_id"], user.ID)
	}
	if _, ok := values["username"]; ok {
		t.Fatal("server side session stores the username")
	}

	profileReq := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	profileReq.AddCookie(cookie)
	profileRec := httptest.NewRecorder()
	handler.ServeHTTP(profileRec, profileReq)
	if profileRec.Code != http.StatusOK {
		t.Fatalf("GET /api/profile status = %d, want %d", profileRec.Code, http.StatusOK)
	}
	var profile struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(profileRec.Body.Bytes(), &profile); err != nil {
		t.Fatal(err)
	}
	if profile.Username != user.Username {
		t.Fatalf("profile username = %q, want %q", profile.Username, user.Username)
	}

	if err := model.DeleteUser(user.Username); err != nil {
		t.Fatal(err)
	}
	deletedReq := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	deletedReq.AddCookie(cookie)
	deletedRec := httptest.NewRecorder()
	handler.ServeHTTP(deletedRec, deletedReq)
	if deletedRec.Code != http.StatusForbidden {
		t.Fatalf("deleted user GET /api/profile status = %d, want %d", deletedRec.Code, http.StatusForbidden)
	}
}

func newUserDeletionTestServer(t *testing.T) (*config.Config, http.Handler, *indexer.Indexer, *model.User) {
	t.Helper()
	cfg := testutil.ModelConfig(t)
	cfg.App.UserHandling = true
	cfg.App.Public = true
	if err := cfg.UpdateBaseURL("http://127.0.0.1:4433"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveRules(); err != nil {
		t.Fatal(err)
	}
	testutil.InitModelWithConfig(t, cfg)
	sessionStore = newSessionStore([]byte(strings.Repeat("x", 32)), cfg.BaseURL(""), sessionMaxAge)
	admin, err := model.CreateUser("admin", "password123", true)
	if err != nil {
		t.Fatal(err)
	}
	idx := newServerTestIndexer(t, cfg)
	return cfg, registerEndpoints(cfg, idx), idx, admin
}

func TestDeleteUserAuthorizationAndPurge(t *testing.T) {
	_, handler, idx, admin := newUserDeletionTestServer(t)
	alice := testutil.CreateUser(t, "alice")
	bob := testutil.CreateUser(t, "bob")
	const sharedURL = "https://example.com/shared"
	for _, d := range []*document.Document{
		{URL: sharedURL, UserID: alice.ID, Text: "Alice's document", Processed: true},
		{URL: "https://example.com/private", UserID: alice.ID, Text: "Private", Processed: true},
		{URL: sharedURL, UserID: bob.ID, Text: "Bob's document", Processed: true},
		{URL: sharedURL, Text: "Global document", Processed: true},
	} {
		if err := idx.Add(d); err != nil {
			t.Fatal(err)
		}
	}
	if err := model.UpdateHistory(alice.ID, "test", sharedURL, "Alice"); err != nil {
		t.Fatal(err)
	}
	if err := model.UpdateHistory(bob.ID, "test", sharedURL, "Bob"); err != nil {
		t.Fatal(err)
	}
	login := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/login", strings.NewReader(`{"username":"alice","password":"password123"}`), map[string]string{"Origin": "hister://", "Content-Type": "application/json"})
	if login.Code != http.StatusOK {
		t.Fatalf("login: %d %s", login.Code, login.Body)
	}
	cookie := responseSessionCookie(t, login)

	for _, tc := range []struct {
		name, token, origin, body string
		status                    int
	}{
		{"anonymous", "", "hister://", `{"username":"alice","purge":true}`, http.StatusForbidden},
		{"invalid token", "wrong", "hister://", `{"username":"alice","purge":true}`, http.StatusForbidden},
		{"regular user", bob.Token, "hister://", `{"username":"alice","purge":true}`, http.StatusForbidden},
		{"cross origin", admin.Token, "https://other.example", `{"username":"alice","purge":true}`, http.StatusForbidden},
		{"invalid JSON", admin.Token, "hister://", `{`, http.StatusBadRequest},
		{"missing username", admin.Token, "hister://", `{}`, http.StatusBadRequest},
		{"unknown user", admin.Token, "hister://", `{"username":"missing","purge":true}`, http.StatusNotFound},
		{"purge required", admin.Token, "hister://", `{"username":"alice"}`, http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/delete_user", strings.NewReader(tc.body), map[string]string{
				"Origin": tc.origin, "X-Access-Token": tc.token, "Content-Type": "application/json",
			})
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.status, rec.Body)
			}
			if _, err := model.GetUser("alice"); err != nil {
				t.Fatalf("rejected request deleted the account: %v", err)
			}
			if count, err := idx.CountByQuery("*", &alice.ID); err != nil || count != 2 {
				t.Fatalf("rejected request changed documents: count = %d, error = %v", count, err)
			}
		})
	}

	rec := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/delete_user", strings.NewReader(`{"username":"alice","purge":true}`), map[string]string{
		"Origin": "hister://", "Authorization": "Bearer " + admin.Token, "Content-Type": "application/json",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("purge status = %d: %s", rec.Code, rec.Body)
	}
	var result struct {
		Username         string `json:"username"`
		DeletedDocuments int    `json:"deleted_documents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result.Username != "alice" || result.DeletedDocuments != 2 {
		t.Fatalf("purge response = %s, error = %v", rec.Body, err)
	}
	if _, err := model.GetUser("alice"); !errors.Is(err, model.ErrUserNotFound) {
		t.Fatalf("deleted account lookup = %v", err)
	}
	if count, err := idx.CountByQuery("*", &alice.ID); err != nil || count != 0 {
		t.Fatalf("owned documents remain: count = %d, error = %v", count, err)
	}
	if idx.GetByURLAndUser(sharedURL, bob.ID) == nil || idx.GetByURLAndUser(sharedURL, 0) == nil {
		t.Fatal("purge removed another owner's document")
	}
	for _, user := range []*model.User{admin, bob} {
		if _, err := model.GetUserByID(user.ID); err != nil {
			t.Fatalf("purge removed another account: %v", err)
		}
	}
	if history, err := model.GetURLsByQuery(alice.ID, "test"); err != nil || len(history) != 0 {
		t.Fatalf("deleted document history remains: %v, %v", history, err)
	}
	if history, err := model.GetURLsByQuery(bob.ID, "test"); err != nil || len(history) != 1 {
		t.Fatalf("another user's history changed: %v, %v", history, err)
	}
	for _, tokenAuth := range []bool{false, true} {
		req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
		if tokenAuth {
			req.Header.Set("X-Access-Token", alice.Token)
		} else {
			req.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != http.StatusForbidden {
			t.Errorf("deleted user still authenticated: token = %t, status = %d", tokenAuth, response.Code)
		}
	}
}

func TestDeleteUserWithAdminSession(t *testing.T) {
	_, handler, _, _ := newUserDeletionTestServer(t)
	testutil.CreateUser(t, "empty")
	login := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/login", strings.NewReader(`{"username":"admin","password":"password123"}`), map[string]string{"Origin": "hister://", "Content-Type": "application/json"})
	if login.Code != http.StatusOK {
		t.Fatalf("login: %d %s", login.Code, login.Body)
	}
	cookie := responseSessionCookie(t, login)
	for _, withToken := range []bool{false, true} {
		req := httptest.NewRequest(http.MethodPost, "/api/delete_user", strings.NewReader(`{"username":"empty"}`))
		req.Header.Set("Origin", "https://proxy.example")
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		if withToken {
			configReq := httptest.NewRequest(http.MethodGet, "/api/config", nil)
			configReq.Header.Set("Origin", "https://proxy.example")
			configReq.AddCookie(cookie)
			configRec := httptest.NewRecorder()
			handler.ServeHTTP(configRec, configReq)
			req.Header.Set("X-CSRF-Token", configRec.Header().Get("X-CSRF-Token"))
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		want := http.StatusForbidden
		if withToken {
			want = http.StatusOK
		}
		if rec.Code != want {
			t.Fatalf("with CSRF token = %t: status = %d, want %d: %s", withToken, rec.Code, want, rec.Body)
		}
	}
	if _, err := model.GetUser("empty"); !errors.Is(err, model.ErrUserNotFound) {
		t.Fatalf("empty account was not deleted: %v", err)
	}
}

func TestDeleteUserRequiresUserHandling(t *testing.T) {
	_, handler := newTokenTestServer(t, false)
	testutil.CreateUser(t, "alice")
	rec := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/delete_user", strings.NewReader(`{"username":"alice","purge":true}`), map[string]string{
		"Origin": "hister://", "X-Access-Token": "secret", "Content-Type": "application/json",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("single user deletion status = %d: %s", rec.Code, rec.Body)
	}
	if _, err := model.GetUser("alice"); err != nil {
		t.Fatalf("single user request deleted the account: %v", err)
	}
}

func TestDeleteUserRetainsAccountOnIndexFailure(t *testing.T) {
	_, handler, idx, admin := newUserDeletionTestServer(t)
	testutil.CreateUser(t, "alice")
	idx.Close()
	for _, body := range []string{`{"username":"alice"}`, `{"username":"alice","purge":true}`} {
		rec := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/delete_user", strings.NewReader(body), map[string]string{
			"Origin": "hister://", "X-Access-Token": admin.Token, "Content-Type": "application/json",
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("index failure status = %d: %s", rec.Code, rec.Body)
		}
		if _, err := model.GetUser("alice"); err != nil {
			t.Fatalf("index failure deleted the account: %v", err)
		}
	}
}

func TestUserReadAPI(t *testing.T) {
	cfg, _, idx, admin := newUserDeletionTestServer(t)
	if err := cfg.UpdateBaseURL("http://127.0.0.1:4433/hister"); err != nil {
		t.Fatal(err)
	}
	handler := registerEndpoints(cfg, idx)
	alice := testutil.CreateUser(t, "alice")
	bob := testutil.CreateUser(t, "bob")
	literal := testutil.CreateUser(t, "a%_' OR 1=1 --")
	deleted := testutil.CreateUser(t, "deleted")
	if err := model.DeleteUserByID(deleted.ID); err != nil {
		t.Fatal(err)
	}
	request := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		response := testutil.ServeHTTP(t, handler, http.MethodGet, "/hister"+path, nil, map[string]string{"Authorization": "Bearer " + admin.Token})
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s: %d %s", path, response.Code, response.Body)
		}
		return response
	}
	checkUser := func(data map[string]json.RawMessage, user *model.User) {
		t.Helper()
		if len(data) != 5 {
			t.Fatalf("user fields = %v, want only id, username, is_admin, created_at, updated_at", data)
		}
		var got struct {
			ID        uint      `json:"id"`
			Username  string    `json:"username"`
			IsAdmin   bool      `json:"is_admin"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
		}
		encoded, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(encoded, &got); err != nil {
			t.Fatal(err)
		}
		if got.ID != user.ID || got.Username != user.Username || got.IsAdmin != user.IsAdmin || !got.CreatedAt.Equal(user.CreatedAt) || !got.UpdatedAt.Equal(user.UpdatedAt) {
			t.Fatalf("user = %s, want metadata for %q", encoded, user.Username)
		}
	}
	var ids []uint
	path := "/api/users?limit=2"
	for page := 0; ; page++ {
		if page > 2 {
			t.Fatal("pagination did not terminate")
		}
		response := request(path)
		var result struct {
			Users       []map[string]json.RawMessage `json:"users"`
			NextAfterID uint                         `json:"next_after_id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Users) != 2 {
			t.Fatalf("page %d has %d users, want 2", page, len(result.Users))
		}
		for _, user := range result.Users {
			var id uint
			if err := json.Unmarshal(user["id"], &id); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
		if result.NextAfterID == 0 {
			break
		}
		if result.NextAfterID != ids[len(ids)-1] {
			t.Fatalf("cursor %d does not point at last returned user", result.NextAfterID)
		}
		path = fmt.Sprintf("/api/users?limit=2&after_id=%d", result.NextAfterID)
	}
	if !slices.Equal(ids, []uint{admin.ID, alice.ID, bob.ID, literal.ID}) {
		t.Fatalf("listed IDs = %v", ids)
	}
	for _, user := range []*model.User{admin, alice, literal} {
		response := request("/api/users?username=" + url.QueryEscape(user.Username))
		var result struct {
			Users []map[string]json.RawMessage `json:"users"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || len(result.Users) != 1 {
			t.Fatalf("exact username lookup: %s, error = %v", response.Body, err)
		}
		checkUser(result.Users[0], user)
		response = request(fmt.Sprintf("/api/users/%d", user.ID))
		var details map[string]json.RawMessage
		if err := json.Unmarshal(response.Body.Bytes(), &details); err != nil {
			t.Fatal(err)
		}
		checkUser(details, user)
	}
	for _, query := range []string{"username=ali", "username=Alice", "username=missing", "username=deleted", "after_id=999999"} {
		response := request("/api/users?" + query)
		var result map[string]json.RawMessage
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if string(result["users"]) != "[]" || result["next_after_id"] != nil {
			t.Fatalf("empty page = %s", response.Body)
		}
	}
	response := request(fmt.Sprintf("/api/users/%d/token", alice.ID))
	var token map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &token); err != nil || len(token) != 1 || token["token"] != alice.Token {
		t.Fatalf("token response invalid, error = %v", err)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("token response may be cached")
	}
	unchanged, err := model.GetUserByID(alice.ID)
	if err != nil || unchanged.Token != alice.Token {
		t.Fatal("reading the token changed it")
	}
}

func TestUserReadAPIAuthorization(t *testing.T) {
	_, handler, _, admin := newUserDeletionTestServer(t)
	user := testutil.CreateUser(t, "alice")
	login := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/login", strings.NewReader(`{"username":"admin","password":"password123"}`), map[string]string{"Origin": "hister://"})
	if login.Code != http.StatusOK {
		t.Fatalf("admin login: %d %s", login.Code, login.Body)
	}
	cookie := responseSessionCookie(t, login)
	for _, path := range []string{"/api/users", fmt.Sprintf("/api/users/%d", user.ID), fmt.Sprintf("/api/users/%d/token", user.ID)} {
		for _, token := range []string{"", "invalid", user.Token, admin.Token} {
			response := testutil.ServeHTTP(t, handler, http.MethodGet, path, nil, map[string]string{"X-Access-Token": token})
			want := http.StatusForbidden
			if token == admin.Token {
				want = http.StatusOK
			}
			if response.Code != want {
				t.Errorf("GET %s: status = %d, want %d", path, response.Code, want)
			}
		}
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(cookie)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != http.StatusOK {
			t.Errorf("admin session GET %s: %d", path, response.Code)
		}
	}
}

func TestUserReadAPIValidation(t *testing.T) {
	_, handler, _, admin := newUserDeletionTestServer(t)
	for _, path := range []string{
		"/api/users?limit=0", "/api/users?limit=-1", "/api/users?limit=101", "/api/users?limit=x", "/api/users?limit=",
		"/api/users?after_id=-1", "/api/users?after_id=18446744073709551615", "/api/users?after_id=x",
		"/api/users?username=", "/api/users?username=%20", "/api/users?limit=1&limit=2", "/api/users?name=alice", "/api/users?username=%zz",
		"/api/users/0", "/api/users/-1", "/api/users/nope", "/api/users/18446744073709551615", "/api/users/nope/token",
		"/api/users/999999", "/api/users/999999/token",
	} {
		t.Run(path, func(t *testing.T) {
			response := testutil.ServeHTTP(t, handler, http.MethodGet, path, nil, map[string]string{"X-Access-Token": admin.Token})
			wantStatus, wantCode := http.StatusBadRequest, "invalid_request"
			if strings.Contains(path, "999999") {
				wantStatus, wantCode = http.StatusNotFound, "user_not_found"
			}
			var result struct {
				Code  string `json:"code"`
				Error string `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != wantStatus || result.Code != wantCode || result.Error == "" {
				t.Fatalf("status = %d, body = %s, decode error = %v", response.Code, response.Body, err)
			}
		})
	}
}

func TestUserReadAPIRequiresUserHandling(t *testing.T) {
	_, handler := newTokenTestServer(t, true)
	user := testutil.CreateUser(t, "alice")
	for _, path := range []string{"/api/users", fmt.Sprintf("/api/users/%d", user.ID), fmt.Sprintf("/api/users/%d/token", user.ID)} {
		response := testutil.ServeHTTP(t, handler, http.MethodGet, path, nil, map[string]string{"X-Access-Token": "secret"})
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"user_handling_disabled"`) {
			t.Errorf("single user GET %s: %d %s", path, response.Code, response.Body)
		}
	}
}

func TestUserReadAPIPaginationLimits(t *testing.T) {
	_, handler, _, admin := newUserDeletionTestServer(t)
	for i := range 100 {
		name := fmt.Sprintf("user-%03d", i)
		if _, err := model.CreateOAuthUser(name, name); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		query string
		count int
	}{
		{"", 50},
		{"?limit=1", 1},
		{"?limit=100", 100},
	} {
		response := testutil.ServeHTTP(t, handler, http.MethodGet, "/api/users"+tc.query, nil, map[string]string{"X-Access-Token": admin.Token})
		var result struct {
			Users []struct {
				ID uint `json:"id"`
			} `json:"users"`
			NextAfterID uint `json:"next_after_id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != http.StatusOK || len(result.Users) != tc.count {
			t.Fatalf("page %s: status = %d, count = %d, error = %v", tc.query, response.Code, len(result.Users), err)
		}
		if result.NextAfterID != result.Users[len(result.Users)-1].ID {
			t.Fatalf("page %s: wrong cursor %d", tc.query, result.NextAfterID)
		}
	}
}

func TestUserReadAPIDatabaseErrors(t *testing.T) {
	cfg, _, _, _ := newUserDeletionTestServer(t)
	db, err := model.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	// Call the handlers after authentication to isolate failures during the account lookup.
	for _, handler := range []endpointHandler{serveUsers, serveUser, serveUserToken} {
		req := httptest.NewRequest(http.MethodGet, "/api/users/1", nil)
		req.SetPathValue("id", "1")
		response := httptest.NewRecorder()
		handler(&webContext{Request: req, Response: response, Config: cfg})
		var result map[string]string
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != http.StatusInternalServerError || result["code"] != "internal_error" {
			t.Fatalf("database error: status = %d, body = %s, decode error = %v", response.Code, response.Body, err)
		}
		if strings.Contains(result["error"], "sql:") {
			t.Fatal("response disclosed the database error")
		}
	}
}

func TestCreateUserAPI(t *testing.T) {
	cfg, _, idx, admin := newUserDeletionTestServer(t)
	if err := cfg.UpdateBaseURL("http://127.0.0.1:4433/hister"); err != nil {
		t.Fatal(err)
	}
	handler := registerEndpoints(cfg, idx)
	for _, tc := range []struct {
		username string
		password string
		isAdmin  bool
	}{
		{"ál ice@example.org", "12345678", false},
		{"another-admin", strings.Repeat("é", 36), true},
	} {
		t.Run(tc.username, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{"username": tc.username, "password": tc.password, "is_admin": tc.isAdmin})
			if err != nil {
				t.Fatal(err)
			}
			response := testutil.ServeHTTP(t, handler, http.MethodPost, "/hister/api/users", strings.NewReader(string(body)), map[string]string{
				"Authorization": "Bearer " + admin.Token, "Origin": "hister://", "Content-Type": "application/json",
			})
			if response.Code != http.StatusCreated {
				t.Fatalf("create status = %d, want 201: %s", response.Code, response.Body)
			}
			user, err := model.AuthenticateUser(tc.username, tc.password)
			if err != nil {
				t.Fatalf("created account cannot authenticate: %v", err)
			}
			if user.IsAdmin != tc.isAdmin || user.Token == "" || user.Token == admin.Token || user.Password == tc.password {
				t.Fatal("created account has incorrect privileges or credentials")
			}
			var fields map[string]json.RawMessage
			var metadata userResponse
			if err := json.Unmarshal(response.Body.Bytes(), &fields); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(response.Body.Bytes(), &metadata); err != nil {
				t.Fatal(err)
			}
			if len(fields) != 5 || metadata.ID != user.ID || metadata.Username != tc.username || metadata.IsAdmin != tc.isAdmin || !metadata.CreatedAt.Equal(user.CreatedAt) || !metadata.UpdatedAt.Equal(user.UpdatedAt) {
				t.Fatalf("response must contain only the account's metadata: %s", response.Body)
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("creation response may be cached")
			}
			location, err := url.Parse(response.Header().Get("Location"))
			if err != nil || location.Path != fmt.Sprintf("/hister/api/users/%d", user.ID) {
				t.Fatalf("incorrect resource location: %q", response.Header().Get("Location"))
			}
			lookup := testutil.ServeHTTP(t, handler, http.MethodGet, location.RequestURI(), nil, map[string]string{"Authorization": "Bearer " + admin.Token})
			if lookup.Code != http.StatusOK || lookup.Body.String() != response.Body.String() {
				t.Fatalf("resource location does not return the created account: %d %s", lookup.Code, lookup.Body)
			}
		})
	}
}

func TestCreateUserAPIValidation(t *testing.T) {
	_, handler, _, admin := newUserDeletionTestServer(t)
	for _, tc := range []struct {
		name string
		body string
	}{
		{"missing body", ""},
		{"null body", `null`},
		{"array", `[]`},
		{"missing username", `{"password":"password123"}`},
		{"blank username", `{"username":" \t ","password":"password123"}`},
		{"control in username", `{"username":"ali\nce","password":"password123"}`},
		{"null username", `{"username":null,"password":"password123"}`},
		{"missing password", `{"username":"alice"}`},
		{"null password", `{"username":"alice","password":null}`},
		{"short password", `{"username":"alice","password":"1234567"}`},
		{"long password", `{"username":"alice","password":"` + strings.Repeat("x", 73) + `"}`},
		{"long UTF8 password", `{"username":"alice","password":"` + strings.Repeat("é", 37) + `"}`},
		{"invalid admin", `{"username":"alice","password":"password123","is_admin":"true"}`},
		{"null admin", `{"username":"alice","password":"password123","is_admin":null}`},
		{"unknown field", `{"username":"alice","password":"password123","token":"chosen-token"}`},
		{"trailing object", `{"username":"alice","password":"password123"} {}`},
		{"trailing null", `{"username":"alice","password":"password123"} null`},
		{"trailing garbage", `{"username":"alice","password":"password123"} secret`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/users", strings.NewReader(tc.body), map[string]string{"X-Access-Token": admin.Token, "Origin": "hister://"})
			var result map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != http.StatusBadRequest || result["code"] != "invalid_request" || result["error"] == "" {
				t.Fatalf("invalid request: status = %d, body = %s, decode error = %v", response.Code, response.Body, err)
			}
			if strings.Contains(response.Body.String(), "password123") || strings.Contains(response.Body.String(), "chosen-token") {
				t.Fatal("validation response disclosed submitted credentials")
			}
		})
	}
	users, err := model.ListUsers("", 0, 100)
	if err != nil || len(users) != 1 || users[0].ID != admin.ID {
		t.Fatalf("invalid requests changed accounts: %v, error = %v", users, err)
	}
}

func TestCreateUserAPIDuplicate(t *testing.T) {
	_, handler, _, admin := newUserDeletionTestServer(t)
	response := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/users", strings.NewReader(`{"username":"admin","password":"replacement","is_admin":false}`), map[string]string{"X-Access-Token": admin.Token, "Origin": "hister://"})
	var result map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != http.StatusConflict || result["code"] != "username_exists" {
		t.Fatalf("duplicate request: status = %d, body = %s, decode error = %v", response.Code, response.Body, err)
	}
	unchanged, err := model.AuthenticateUser("admin", "password123")
	if err != nil || !unchanged.IsAdmin || unchanged.Token != admin.Token {
		t.Fatalf("duplicate creation changed the existing account: %v", err)
	}
}

func TestCreateUserAPIAuthorization(t *testing.T) {
	_, handler, _, admin := newUserDeletionTestServer(t)
	user := testutil.CreateUser(t, "alice")
	for _, token := range []string{"", "invalid", user.Token} {
		response := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/users", strings.NewReader(`{"username":"unauthorized","password":"password123","is_admin":true}`), map[string]string{"Authorization": "Bearer " + token, "Origin": "hister://"})
		if response.Code != http.StatusForbidden {
			t.Fatalf("unauthorized creation: status = %d, want 403: %s", response.Code, response.Body)
		}
	}
	if _, err := model.GetUser("unauthorized"); !errors.Is(err, model.ErrUserNotFound) {
		t.Fatalf("unauthorized creation changed accounts: %v", err)
	}
	// A session must pass CSRF validation before it can create even an ordinary account.
	login := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/login", strings.NewReader(`{"username":"admin","password":"password123"}`), map[string]string{"Origin": "hister://"})
	if login.Code != http.StatusOK {
		t.Fatalf("login: %d %s", login.Code, login.Body)
	}
	cookie := responseSessionCookie(t, login)
	for _, withToken := range []bool{false, true} {
		req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{"username":"bob","password":"password123"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://proxy.example")
		req.AddCookie(cookie)
		if withToken {
			configReq := httptest.NewRequest(http.MethodGet, "/api/config", nil)
			configReq.Header.Set("Origin", "https://proxy.example")
			configReq.AddCookie(cookie)
			configRec := httptest.NewRecorder()
			handler.ServeHTTP(configRec, configReq)
			req.Header.Set("X-CSRF-Token", configRec.Header().Get("X-CSRF-Token"))
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		want := http.StatusForbidden
		if withToken {
			want = http.StatusCreated
		}
		if response.Code != want {
			t.Fatalf("session creation with CSRF token = %t: status = %d, want %d: %s", withToken, response.Code, want, response.Body)
		}
		bob, err := model.GetUser("bob")
		if !withToken && !errors.Is(err, model.ErrUserNotFound) {
			t.Fatalf("failed CSRF check changed accounts: %v", err)
		}
		if withToken && (err != nil || bob.IsAdmin || bob.Token == admin.Token) {
			t.Fatalf("creation without is_admin did not create an ordinary account: %v", err)
		}
	}
}

func TestCreateUserAPIRequiresUserHandling(t *testing.T) {
	_, handler := newTokenTestServer(t, false)
	response := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/users", strings.NewReader(`{"username":"alice","password":"password123","is_admin":true}`), map[string]string{"X-Access-Token": "secret", "Origin": "hister://"})
	var result map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != http.StatusBadRequest || result["code"] != "user_handling_disabled" {
		t.Fatalf("single user creation: status = %d, body = %s, decode error = %v", response.Code, response.Body, err)
	}
	if _, err := model.GetUser("alice"); !errors.Is(err, model.ErrUserNotFound) {
		t.Fatalf("single user creation changed accounts: %v", err)
	}
}

func TestCreateUserAPIBodyLimit(t *testing.T) {
	_, handler, _, admin := newUserDeletionTestServer(t)
	for _, knownLength := range []bool{true, false} {
		// Trailing whitespace must still count toward the limit before an account is created.
		req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{"username":"alice","password":"password123"}`+strings.Repeat(" ", int(maxRequestBodyBytes))))
		req.Header.Set("X-Access-Token", admin.Token)
		req.Header.Set("Origin", "hister://")
		if !knownLength {
			req.ContentLength = -1
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		var result struct{ Code string }
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != http.StatusRequestEntityTooLarge || result.Code != "request_too_large" {
			t.Fatalf("oversized request: status = %d, body = %s, decode error = %v", response.Code, response.Body, err)
		}
	}
	if _, err := model.GetUser("alice"); !errors.Is(err, model.ErrUserNotFound) {
		t.Fatalf("oversized request created an account: %v", err)
	}
}

func TestCreateUserAPIDatabaseError(t *testing.T) {
	cfg, _, _, _ := newUserDeletionTestServer(t)
	db, err := model.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	// Isolate a database failure after administrator authentication has succeeded.
	for _, endpoint := range Endpoints {
		if endpoint.Path != "/api/users" || endpoint.Method != POST {
			continue
		}
		req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{"username":"alice","password":"password123"}`))
		response := httptest.NewRecorder()
		endpoint.Handler(&webContext{Request: req, Response: response, Config: cfg})
		var result map[string]string
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != http.StatusInternalServerError || result["code"] != "internal_error" {
			t.Fatalf("database failure: status = %d, body = %s, decode error = %v", response.Code, response.Body, err)
		}
		if strings.Contains(response.Body.String(), "sql:") || strings.Contains(response.Body.String(), "password123") {
			t.Fatal("creation response disclosed private details")
		}
		return
	}
	t.Fatal("user creation endpoint is missing")
}

func tokenLoginCookie(t *testing.T, handler http.Handler, current *http.Cookie) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/token-login", strings.NewReader(`{"token":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "hister://")
	if current != nil {
		req.AddCookie(current)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/token-login status = %d, want %d", rec.Code, http.StatusOK)
	}
	return responseSessionCookie(t, rec)
}

func responseSessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	var sessionCookie *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == storeName {
			sessionCookie = cookie
		}
	}
	if sessionCookie != nil {
		return sessionCookie
	}
	t.Fatalf("response did not set %q cookie", storeName)
	return nil
}

func assertSessionAccepted(t *testing.T, handler http.Handler, cookie *http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/add", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("session status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func assertSessionRejected(t *testing.T, handler http.Handler, cookie *http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/add", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("session status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
