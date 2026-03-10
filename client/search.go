package client

import (
	"encoding/json"
	"io"
	"net/url"

	"github.com/asciimoo/hister/server/indexer"
)

func (c *Client) Search(query string) (_ *indexer.Results, err error) {
	req, err := c.newRequest("GET", "/search?q="+url.QueryEscape(query), nil)
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
