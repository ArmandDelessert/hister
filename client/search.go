// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

func (c *Client) Search(q *SearchQuery) (result *SearchResults, err error) {
	qJSON, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}
	u := "/search?query=" + url.QueryEscape(string(qJSON))
	err = c.request(context.Background(), http.MethodGet, u, nil, "", func(resp *http.Response) error {
		if err := checkStatus(resp); err != nil {
			return err
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		var decoded *SearchResults
		if err := json.Unmarshal(body, &decoded); err != nil {
			return err
		}
		result = decoded
		return nil
	})
	return result, err
}
