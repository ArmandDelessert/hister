// SPDX-FileContributor: Adam Tauber <asciimoo@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
