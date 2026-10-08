package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/indexer"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type trackedResponseBody struct {
	io.Reader
	closed   bool
	closeErr error
}

func (b *trackedResponseBody) Close() error {
	b.closed = true
	return b.closeErr
}

func TestRequestFormatsAndHeaders(t *testing.T) {
	const sourceURL = "https://example.com/a?x=1&y=two words"
	tests := []struct {
		name, method, path, contentType string
		payload                         string
		call                            func(*Client) error
	}{
		{"history", "POST", "/api/history", "application/json", `{"url":"` + sourceURL + `","title":"A title","query":"test"}`, func(c *Client) error { return c.PostHistory("test", sourceURL, "A title") }},
		{"delete history", "POST", "/api/history", "application/json", `{"url":"` + sourceURL + `","query":"test","delete":true}`, func(c *Client) error { return c.DeleteHistoryEntry("test", sourceURL) }},
		{"delete document", "POST", "/api/delete", "application/json", `{"query":"url:` + sourceURL + `"}`, func(c *Client) error { return c.DeleteDocument(sourceURL) }},
		{"reindex", "POST", "/api/reindex", "application/json", `{"skipSensitive":true,"detectLanguages":false}`, func(c *Client) error { return c.Reindex(true, false) }},
		{"add page", "POST", "/api/add", "application/x-www-form-urlencoded", url.Values{"url": {sourceURL}, "title": {"A title"}, "text": {"a+b & c"}}.Encode(), func(c *Client) error { return c.AddPage(sourceURL, "A title", "a+b & c") }},
		{"add alias", "POST", "/api/add_alias", "application/x-www-form-urlencoded", url.Values{"alias-keyword": {"test"}, "alias-value": {"a+b & c"}}.Encode(), func(c *Client) error { return c.AddAlias("test", "a+b & c") }},
		{"delete alias", "POST", "/api/delete_alias", "application/x-www-form-urlencoded", url.Values{"alias": {"a+b"}}.Encode(), func(c *Client) error { return c.DeleteAlias("a+b") }},
		{"cleanup", "POST", "/api/cleanup", "", "", func(c *Client) error {
			result, err := c.Cleanup()
			if err == nil && result.HTMLRemoved != 3 {
				t.Errorf("cleanup result = %+v", result)
			}
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &trackedResponseBody{Reader: strings.NewReader(`{"htmlRemoved":3}`)}
			calls := 0
			c := New("http://hister.test/prefix/", WithAccessToken("secret"), WithUserAgent("test-client"), WithTargetUserID(7), WithHTTPClient(&http.Client{
				Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					calls++
					if r.Method != tt.method || r.URL.Path != "/prefix"+tt.path {
						t.Errorf("request = %s %s", r.Method, r.URL)
					}
					for key, want := range map[string]string{"Content-Type": tt.contentType, "Origin": "hister://", "X-Access-Token": "secret", "User-Agent": "test-client", targetUserIDHeader: "7"} {
						if got := r.Header.Get(key); got != want {
							t.Errorf("header %s = %q, want %q", key, got, want)
						}
					}
					var data []byte
					if r.Body != nil {
						var err error
						data, err = io.ReadAll(r.Body)
						if err != nil {
							t.Fatal(err)
						}
					}
					if tt.contentType == "application/json" {
						var got, want any
						if err := json.Unmarshal(data, &got); err != nil {
							t.Fatal(err)
						}
						if err := json.Unmarshal([]byte(tt.payload), &want); err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(got, want) {
							t.Errorf("payload = %s, want %s", data, tt.payload)
						}
					} else if string(data) != tt.payload {
						t.Errorf("payload = %q, want %q", data, tt.payload)
					}
					return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
				}),
			}))
			if err := tt.call(c); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || !body.closed {
				t.Errorf("requests = %d, body closed = %t", calls, body.closed)
			}
		})
	}
}

func TestRequestResponseErrorsAndCleanup(t *testing.T) {
	closeErr := errors.New("body close failed")
	for _, tt := range []struct {
		name     string
		status   int
		body     string
		closeErr error
		wantErr  string
	}{
		{"success", 200, `{"title":"Preview"}`, nil, ""},
		{"close error", 200, `{"title":"Preview"}`, closeErr, "closing response body"},
		{"decode error", 200, `{"title":`, closeErr, "unexpected EOF"},
		{"unauthorized", 401, " access denied \n", closeErr, "authentication required"},
		{"forbidden", 403, " access denied \n", closeErr, "access denied"},
		{"not found", 404, " missing \n", closeErr, "resource not found"},
		{"server error", 503, " unavailable \n", closeErr, "server error"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := &trackedResponseBody{Reader: strings.NewReader(tt.body), closeErr: tt.closeErr}
			c := New("http://hister.test", WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tt.status, Body: body}, nil
			})}))
			preview, err := c.FetchPreview("https://example.com")
			if tt.wantErr == "" {
				if err != nil || preview == nil || preview.Title != "Preview" {
					t.Fatalf("preview = %+v, error = %v", preview, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
			if tt.status >= 400 {
				var httpErr *HTTPError
				if !errors.As(err, &httpErr) || httpErr.HTTPStatusCode() != tt.status || httpErr.Detail != strings.TrimSpace(tt.body) {
					t.Errorf("HTTP error lost its status or detail: %v", err)
				}
			}
			if tt.name == "close error" && !errors.Is(err, closeErr) {
				t.Errorf("close error was not wrapped: %v", err)
			}
			if !body.closed {
				t.Error("response body was not closed")
			}
		})
	}
}

func TestRequestCancellation(t *testing.T) {
	for name, call := range map[string]func(*Client, context.Context) error{
		"config":      func(c *Client, ctx context.Context) error { _, err := c.FetchConfigContext(ctx); return err },
		"diagnostics": func(c *Client, ctx context.Context) error { _, err := c.FetchDiagnostics(ctx); return err },
		"exists": func(c *Client, ctx context.Context) error {
			_, err := c.DocumentExistsContext(ctx, "https://example.com")
			return err
		},
		"add": func(c *Client, ctx context.Context) error {
			return c.AddDocumentJSONContext(ctx, &document.Document{URL: "https://example.com"})
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			c := New("http://hister.test", WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Context().Err() == nil {
					return nil, errors.New("request lost caller cancellation")
				}
				return nil, r.Context().Err()
			})}))
			if err := call(c, ctx); !errors.Is(err, context.Canceled) {
				t.Errorf("error = %v, want context.Canceled", err)
			}
		})
	}
}

func TestDocumentExistsResponseHandling(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNoContent, http.StatusNotFound, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			body := &trackedResponseBody{Reader: strings.NewReader("")}
			c := New("http://hister.test", WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodHead || r.URL.Query().Get("url") != "https://example.com/?a=1&b=2" {
					t.Errorf("request = %s %s", r.Method, r.URL)
				}
				return &http.Response{StatusCode: status, Body: body}, nil
			})}))
			exists, err := c.DocumentExists("https://example.com/?a=1&b=2")
			if exists != (status == http.StatusOK) || (err != nil) != (status == http.StatusForbidden) {
				t.Errorf("exists = %t, error = %v", exists, err)
			}
			if !body.closed {
				t.Error("response body was not closed")
			}
		})
	}
}

func TestSearchRequestAndResponse(t *testing.T) {
	query := &indexer.Query{Text: `label:"a & b"`, Limit: 3, PageKey: "next+page"}
	for _, response := range []string{`{"total":2}`, `null`, `{"total":2}invalid`} {
		t.Run(response, func(t *testing.T) {
			body := &trackedResponseBody{Reader: strings.NewReader(response)}
			c := New("http://hister.test", WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				var got indexer.Query
				if err := json.Unmarshal([]byte(r.URL.Query().Get("query")), &got); err != nil {
					t.Fatal(err)
				}
				if r.Method != http.MethodGet || r.URL.Path != "/search" || !reflect.DeepEqual(got, *query) {
					t.Errorf("request = %s %s, query = %+v", r.Method, r.URL, got)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
			})}))
			result, err := c.Search(query)
			switch response {
			case `null`:
				if result != nil || err != nil {
					t.Errorf("result = %+v, error = %v", result, err)
				}
			case `{"total":2}`:
				if err != nil || result == nil || result.Total != 2 {
					t.Errorf("result = %+v, error = %v", result, err)
				}
			default:
				if err == nil {
					t.Error("accepted a response with trailing invalid data")
				}
			}
			if !body.closed {
				t.Error("response body was not closed")
			}
		})
	}
}

func TestBatchRequestSplittingPreservesResults(t *testing.T) {
	var sizes []int
	var bodies []*trackedResponseBody
	c := New("http://hister.test", WithMaxBatchBodyBytes(1<<20), WithAllowSensitive(), WithIgnoreRules(), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var request struct {
			Ops []addDocumentOperation `json:"ops"`
		}
		if r.URL.Path != "/api/batch" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected batch request: %s", r.URL)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		sizes = append(sizes, len(request.Ops))
		for _, op := range request.Ops {
			if op.Op != "add" || !op.SkipSensitiveCheck || !op.IgnoreSkipRules() {
				t.Errorf("document options lost: %+v", op)
			}
		}
		status, response := http.StatusOK, `{"results":[{"status":201}]}`
		if len(request.Ops) > 1 || request.Ops[0].URL == "https://example.com/large" {
			status, response = http.StatusRequestEntityTooLarge, `{"error":"too large","limit_bytes":100}`
		}
		body := &trackedResponseBody{Reader: strings.NewReader(response)}
		bodies = append(bodies, body)
		return &http.Response{StatusCode: status, Body: body}, nil
	})}))
	results, err := c.AddDocumentsJSON([]*document.Document{{URL: "https://example.com/first"}, {URL: "https://example.com/large"}, {URL: "https://example.com/last"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 || results[0].Status != 201 || results[1].Status != 413 || results[2].Status != 201 || !strings.Contains(results[1].Error, "too large; encoded document request") {
		t.Errorf("batch results = %+v", results)
	}
	if !reflect.DeepEqual(sizes, []int{3, 1, 2, 1, 1}) {
		t.Errorf("batch sizes = %v", sizes)
	}
	for _, body := range bodies {
		if !body.closed {
			t.Error("batch response body was not closed")
		}
	}
}

func TestBatchCapabilityDiscovery(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		status     int
		want       int64
	}{
		{"advertised", `{"maxBatchBodyBytes":12345}`, 200, 12345},
		{"legacy", `{}`, 200, legacyMaxBatchBodyBytes},
		{"invalid limit", `{"maxBatchBodyBytes":-1}`, 200, legacyMaxBatchBodyBytes},
		{"invalid JSON", `{`, 200, legacyMaxBatchBodyBytes},
		{"HTTP error", `{"maxBatchBodyBytes":12345}`, 403, legacyMaxBatchBodyBytes},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := &trackedResponseBody{Reader: strings.NewReader(tt.body), closeErr: errors.New("close failed")}
			calls := 0
			c := New("http://hister.test", WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: tt.status, Body: body}, nil
			})}))
			for range 2 {
				if got := c.MaxBatchBodyBytes(); got != tt.want {
					t.Errorf("batch limit = %d, want %d", got, tt.want)
				}
			}
			if calls != 1 || !body.closed {
				t.Errorf("discovery requests = %d, body closed = %t", calls, body.closed)
			}
		})
	}
}

func TestRequestEncodingAndTransportErrors(t *testing.T) {
	transportErr := errors.New("transport unavailable")
	calls := 0
	c := New("http://hister.test", WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return nil, transportErr
	})}))
	d := &document.Document{URL: "https://example.com", Metadata: map[string]any{"invalid": func() {}}}
	var encodingErr *json.UnsupportedTypeError
	if err := c.AddDocumentJSON(d); !errors.As(err, &encodingErr) {
		t.Errorf("encoding error = %v", err)
	}
	if calls != 0 {
		t.Fatal("sent a document that could not be encoded")
	}
	if err := c.PostHistory("test", d.URL, "title"); !errors.Is(err, transportErr) {
		t.Errorf("transport error was not preserved: %v", err)
	}
	if calls != 1 {
		t.Errorf("requests = %d, want 1", calls)
	}
}

func TestBatchRequestResponseValidation(t *testing.T) {
	for _, tt := range []struct {
		name, response, wantError string
		status                    int
	}{
		{"missing results", `{"results":[]}`, "batch response contained 0 results for 1 documents", http.StatusOK},
		{"invalid JSON", `{"results":`, "unexpected EOF", http.StatusOK},
		{"forbidden", "denied", "access denied", http.StatusForbidden},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := &trackedResponseBody{Reader: strings.NewReader(tt.response), closeErr: errors.New("close failed")}
			calls := 0
			c := New("http://hister.test", WithMaxBatchBodyBytes(1<<20), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: tt.status, Body: body}, nil
			})}))
			results, err := c.AddDocumentsJSON([]*document.Document{{URL: "https://example.com"}})
			if len(results) != 0 || err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("results = %+v, error = %v, want %q", results, err, tt.wantError)
			}
			if calls != 1 || !body.closed {
				t.Errorf("requests = %d, body closed = %t", calls, body.closed)
			}
		})
	}
}
