package client

import (
	"context"
	"net/http"
	"net/url"
)

func (c *Client) FetchRules() (_ *RulesResponse, err error) {
	var data RulesResponse
	err = c.requestJSON(context.Background(), http.MethodGet, "/api/rules", nil, &data)
	return &data, err
}

// SaveRules saves skip and priority patterns, followed by optional versioning
// and allow patterns. Omitting allow leaves the server's allow rules unchanged.
func (c *Client) SaveRules(skip, priority string, patterns ...string) error {
	versioningRules := ""
	if len(patterns) > 0 {
		versioningRules = patterns[0]
	}
	formData := url.Values{"skip": {skip}, "priority": {priority}, "versioning": {versioningRules}}
	if len(patterns) > 1 {
		formData.Set("allow", patterns[1])
	}
	return c.postForm("/api/rules", formData)
}

func (c *Client) AddAlias(keyword, value string) error {
	formData := url.Values{"alias-keyword": {keyword}, "alias-value": {value}}
	return c.postForm("/api/add_alias", formData)
}

func (c *Client) DeleteAlias(alias string) error {
	formData := url.Values{"alias": {alias}}
	return c.postForm("/api/delete_alias", formData)
}
