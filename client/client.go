package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// targetUserIDHeader is sent by admin CLI callers to request a specific user_id
// for indexed documents. The server only honours it for admin users in
// multiuser mode.
const targetUserIDHeader = "X-Hister-Target-User-ID"

type Client struct {
	baseURL        string
	httpClient     *http.Client
	userAgent      string
	accessToken    string
	targetUserID   *uint
	allowSensitive bool
	ignoreRules    bool
	batchLimitOnce sync.Once
	batchBodyBytes int64
}

type HTTPError struct {
	StatusCode int
	Detail     string
	Message    string
}

func (e *HTTPError) Error() string {
	return e.Message
}

// HTTPStatusCode returns the response status associated with the error.
func (e *HTTPError) HTTPStatusCode() int {
	return e.StatusCode
}

type Option func(*Client)

func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = d }
}

func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

func WithAccessToken(token string) Option {
	return func(c *Client) { c.accessToken = token }
}

func WithAllowSensitive() Option {
	return func(c *Client) { c.allowSensitive = true }
}

// WithIgnoreRules marks submitted documents as explicitly saved, bypassing URL
// indexing rules on submission and on subsequent index rebuilds.
func WithIgnoreRules() Option {
	return func(c *Client) { c.ignoreRules = true }
}

// WithMaxBatchBodyBytes overrides batch capability discovery. It is primarily
// useful for clients that already obtained the server limit out of band.
func WithMaxBatchBodyBytes(limit int64) Option {
	return func(c *Client) {
		if limit > 0 {
			c.batchBodyBytes = limit
		}
	}
}

// WithTargetUserID instructs the server to index submitted documents under the
// given user ID instead of the authenticated caller's ID. The server only
// honours this for admin users in multiuser mode.
func WithTargetUserID(uid uint) Option {
	return func(c *Client) { c.targetUserID = &uid }
}

func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	if c.accessToken != "" {
		// Keep caller supplied HTTP clients and their redirect policies intact.
		// Go forwards custom authentication headers across origins by default.
		hc := *c.httpClient
		checkRedirect := hc.CheckRedirect
		hc.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if checkRedirect != nil {
				if err := checkRedirect(req, via); err != nil {
					return err
				}
			} else if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			if !sameOrigin(via[0].URL, req.URL) {
				return errors.New("refusing to send the Hister access token to a different origin")
			}
			return nil
		}
		c.httpClient = &hc
	}
	return c
}

func sameOrigin(a, b *url.URL) bool {
	port := func(u *url.URL) string {
		if p := u.Port(); p != "" {
			return p
		}
		switch strings.ToLower(u.Scheme) {
		case "http":
			return "80"
		case "https":
			return "443"
		}
		return ""
	}
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Hostname(), b.Hostname()) && port(a) == port(b)
}

// FetchConfig retrieves capabilities from the server the client is connected
// to. This avoids assuming that local configuration describes a remote server.
func (c *Client) FetchConfig() (*ServerConfig, error) {
	return c.FetchConfigContext(context.Background())
}

func (c *Client) FetchConfigContext(ctx context.Context) (*ServerConfig, error) {
	var serverConfig ServerConfig
	if err := c.requestJSON(ctx, http.MethodGet, "/api/config", nil, &serverConfig); err != nil {
		return nil, err
	}
	return &serverConfig, nil
}

const legacyMaxBatchBodyBytes int64 = 5 << 20

// MaxBatchBodyBytes returns the server advertised batch request limit. Servers
// that predate capability discovery use the former 5 MiB limit.
func (c *Client) MaxBatchBodyBytes() int64 {
	c.batchLimitOnce.Do(func() {
		if c.batchBodyBytes > 0 {
			return
		}
		c.batchBodyBytes = legacyMaxBatchBodyBytes
		_ = c.request(context.Background(), http.MethodGet, "/api/config", nil, "", func(resp *http.Response) error {
			if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
				return nil
			}
			var capabilities struct {
				MaxBatchBodyBytes int64 `json:"maxBatchBodyBytes"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&capabilities); err == nil && capabilities.MaxBatchBodyBytes > 0 {
				c.batchBodyBytes = capabilities.MaxBatchBodyBytes
			}
			return nil
		})
	})
	return c.batchBodyBytes
}
