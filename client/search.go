package client

import (
	"encoding/json"
	"io"
	"net/url"

	"github.com/asciimoo/hister/server/indexer"
)

func (c *Client) Search(query string) (_ *indexer.Results, err error) {
	return c.SearchPage(query, "")
}

func (c *Client) SearchPage(query, pageKey string) (_ *indexer.Results, err error) {
	u := "/search?q=" + url.QueryEscape(query)
	if pageKey != "" {
		u += "&page_key=" + url.QueryEscape(pageKey)
	}
	req, err := c.newRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer closeBody(resp, &err)
	if err := checkStatus(resp); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var res *indexer.Results
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return res, nil
}
