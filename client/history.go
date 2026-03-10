package client

import (
	"encoding/json"
	"strings"
)

func (c *Client) FetchHistory() (_ []HistoryItem, err error) {
	req, err := c.newRequest("GET", "/api/history", nil)
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
	var items []HistoryItem
	err = json.NewDecoder(resp.Body).Decode(&items)
	return items, err
}

func (c *Client) PostHistory(query, urlStr, title string) (err error) {
	body := historyRequest{URL: urlStr, Title: title, Query: query}
	data, _ := json.Marshal(body)
	req, err := c.newRequest("POST", "/api/history", strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer closeBody(resp, &err)
	return checkStatus(resp)
}

func (c *Client) DeleteHistoryEntry(query, urlStr string) (err error) {
	body := historyRequest{URL: urlStr, Query: query, Delete: true}
	data, _ := json.Marshal(body)
	req, err := c.newRequest("POST", "/api/history", strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer closeBody(resp, &err)
	return checkStatus(resp)
}
