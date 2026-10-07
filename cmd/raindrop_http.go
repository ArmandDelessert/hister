package cmd

import (
	"context"
	"net/url"
)

func (c *raindropClient) getJSON(ctx context.Context, endpoint string, query url.Values, target any) error {
	return c.getJSONWithRetry(ctx, endpoint, query, target, c.wait)
}
