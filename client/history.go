package client

import (
	"context"
	"net/http"
)

func (c *Client) FetchHistory() (_ []HistoryItem, err error) {
	var response struct {
		Documents []HistoryItem `json:"documents"`
	}
	err = c.requestJSON(context.Background(), http.MethodGet, "/api/history?opened=true", nil, &response)
	return response.Documents, err
}

func (c *Client) PostHistory(query, urlStr, title string) error {
	body := historyRequest{URL: urlStr, Title: title, Query: query}
	return c.requestJSON(context.Background(), http.MethodPost, "/api/history", body, nil)
}

func (c *Client) DeleteHistoryEntry(query, urlStr string) error {
	body := historyRequest{URL: urlStr, Query: query, Delete: true}
	return c.requestJSON(context.Background(), http.MethodPost, "/api/history", body, nil)
}
