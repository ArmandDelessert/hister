// SPDX-License-Identifier: AGPL-3.0-or-later

// Package server MCP endpoint implements the Model Context Protocol (MCP)
// Streamable HTTP transport so that AI assistants (Claude Desktop, Cursor,
// etc.) can search the Hister index directly.
//
// Specification: https://modelcontextprotocol.io/specification/2024-11-05
//
// Only the search tool is exposed. The handler lives at POST /mcp and uses
// the same authentication as the rest of the API. Bearer tokens are accepted
// via the standard Authorization header and are resolved by the global auth
// middleware (withTokenAuth / populateUserContext).
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/asciimoo/hister/server/indexer"

	"github.com/rs/zerolog/log"
)

// mcpProtocolVersion is the MCP specification version this server targets.
const mcpProtocolVersion = "2024-11-05"

// JSON-RPC 2.0 error codes defined by the MCP specification.
const (
	mcpErrParse        = -32700
	mcpErrInvalidReq   = -32600
	mcpErrNotFound     = -32601
	mcpErrInvalidParam = -32602
	mcpErrInternal     = -32603
)

// mcpRequest is a JSON-RPC 2.0 request envelope.
// ID is kept as raw JSON so that its type (string / number / null) is
// reflected verbatim in the response.
type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// mcpResponse is a JSON-RPC 2.0 response envelope.
type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpRPCError    `json:"error,omitempty"`
}

type mcpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// mcpTextContent is an MCP text content block returned inside a tools/call result.
type mcpTextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// serveMCP handles POST /mcp requests using the MCP Streamable HTTP transport.
func serveMCP(c *webContext) {
	var req mcpRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		mcpWriteError(c, nil, mcpErrParse, "parse error: "+err.Error())
		return
	}
	if req.JSONRPC != "2.0" {
		mcpWriteError(c, req.ID, mcpErrInvalidReq, `invalid jsonrpc version, expected "2.0"`)
		return
	}

	// Notifications carry no id. Acknowledge them with 202 and no body.
	isNotification := len(req.ID) == 0 || string(req.ID) == "null"

	switch req.Method {
	case "initialize":
		mcpWriteResult(c, req.ID, map[string]any{
			"protocolVersion": mcpProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "hister", "version": Version},
		})

	case "notifications/initialized", "notifications/cancelled":
		if isNotification {
			c.Response.WriteHeader(http.StatusAccepted)
			return
		}
		mcpWriteResult(c, req.ID, map[string]any{})

	case "ping":
		mcpWriteResult(c, req.ID, map[string]any{})

	case "tools/list":
		mcpWriteResult(c, req.ID, map[string]any{"tools": mcpToolList()})

	case "tools/call":
		mcpCallTool(c, req)

	default:
		if isNotification {
			c.Response.WriteHeader(http.StatusAccepted)
			return
		}
		mcpWriteError(c, req.ID, mcpErrNotFound, "unknown method: "+req.Method)
	}
}

// mcpToolList returns the list of tools this MCP server exposes.
func mcpToolList() []map[string]any {
	return []map[string]any{
		{
			"name":        "search",
			"description": "Search your personal browsing history and indexed documents. Returns titles, URLs, and text snippets for matching pages.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type": "string",
						"description": `Search query. Supports plain keywords, "exact phrases", ` +
							`field filters (url:, domain:, title:, text:, language:, type:web/local), ` +
							`negation (-term), wildcards (term*), and disjunction (a|b|c).`,
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of results to return (default: 10, max: 50).",
					},
					"semantic": map[string]any{
						"type":        "boolean",
						"description": "Enable AI semantic similarity search alongside keyword matching. Only effective when the server has semantic search configured.",
					},
					"fields": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
							"enum": []string{"text", "html", "language", "label", "domain", "score", "type"},
						},
						"description": "Extra document fields to include in the response. " +
							`"text" returns the full stored article text instead of a short snippet. ` +
							`"html" returns the raw HTML. ` +
							`"language" returns the detected language code. ` +
							`"label" returns the user-defined label. ` +
							`"domain" returns the domain name. ` +
							`"score" returns the relevance score. ` +
							`"type" returns the document type (web or local).`,
					},
				},
				"required": []string{"query"},
			},
		},
	}
}

// mcpCallTool dispatches a tools/call request to the appropriate tool handler.
func mcpCallTool(c *webContext, req mcpRequest) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		mcpWriteError(c, req.ID, mcpErrInvalidParam, "invalid params: "+err.Error())
		return
	}
	switch params.Name {
	case "search":
		mcpToolSearch(c, req.ID, params.Arguments)
	default:
		mcpWriteError(c, req.ID, mcpErrNotFound, "unknown tool: "+params.Name)
	}
}

type mcpSearchArgs struct {
	Query    string   `json:"query"`
	Limit    int      `json:"limit"`
	Semantic bool     `json:"semantic"`
	Fields   []string `json:"fields"`
}

// mcpToolSearch executes a Hister search and formats the results as MCP content.
func mcpToolSearch(c *webContext, id json.RawMessage, rawArgs json.RawMessage) {
	var args mcpSearchArgs
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			mcpWriteError(c, id, mcpErrInvalidParam, "invalid search arguments: "+err.Error())
			return
		}
	}
	if args.Query == "" {
		mcpWriteError(c, id, mcpErrInvalidParam, "query is required")
		return
	}
	if args.Limit <= 0 || args.Limit > 50 {
		args.Limit = 10
	}

	q := &indexer.Query{
		Text:            args.Query,
		Limit:           args.Limit,
		SemanticEnabled: args.Semantic && c.Config.SemanticSearch.Enable,
	}
	res, err := doSearch(q, c.Config, c.effectiveRules(), c.UserID)
	if err != nil {
		log.Error().Err(err).Str("query", args.Query).Msg("MCP search failed")
		mcpWriteError(c, id, mcpErrInternal, "search failed")
		return
	}

	mcpWriteResult(c, id, map[string]any{
		"content": []mcpTextContent{
			{Type: "text", Text: mcpFormatResults(args.Query, res, args.Fields)},
		},
	})
}

// mcpFormatResults renders search results as a human-readable text block.
// fields is the optional list of extra document fields requested by the caller.
func mcpFormatResults(query string, res *indexer.Results, fields []string) string {
	fieldSet := make(map[string]bool, len(fields))
	for _, f := range fields {
		fieldSet[f] = true
	}

	total := int(res.Total) + len(res.History)
	if total == 0 {
		return fmt.Sprintf("No results found for %q.", query)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d result(s) for %q (%s)\n", total, query, res.SearchDuration)

	n := 1
	for _, h := range res.History {
		fmt.Fprintf(&b, "\n%d. %s\n   URL: %s\n", n, h.Title, h.URL)
		if t := strings.TrimSpace(h.Text); t != "" {
			if fieldSet["text"] {
				fmt.Fprintf(&b, "   Text: %s\n", t)
			} else {
				fmt.Fprintf(&b, "   %s\n", mcpTruncate(t, 300))
			}
		}
		n++
	}
	for _, d := range res.Documents {
		added := time.Unix(d.Added, 0).Format("2006-01-02")
		fmt.Fprintf(&b, "\n%d. %s\n   URL: %s\n   Added: %s\n", n, d.Title, d.URL, added)
		if t := strings.TrimSpace(d.Text); t != "" {
			if fieldSet["text"] {
				fmt.Fprintf(&b, "   Text: %s\n", t)
			} else {
				fmt.Fprintf(&b, "   %s\n", mcpTruncate(t, 300))
			}
		}
		if fieldSet["domain"] && d.Domain != "" {
			fmt.Fprintf(&b, "   Domain: %s\n", d.Domain)
		}
		if fieldSet["language"] && d.Language != "" {
			fmt.Fprintf(&b, "   Language: %s\n", d.Language)
		}
		if fieldSet["label"] && d.Label != "" {
			fmt.Fprintf(&b, "   Label: %s\n", d.Label)
		}
		if fieldSet["score"] {
			fmt.Fprintf(&b, "   Score: %.4f\n", d.Score)
		}
		if fieldSet["type"] {
			fmt.Fprintf(&b, "   Type: %s\n", d.Type.String())
		}
		if fieldSet["html"] && d.HTML != "" {
			fmt.Fprintf(&b, "   HTML: %s\n", d.HTML)
		}
		n++
	}
	return b.String()
}

// mcpTruncate truncates s at a rune boundary so that the result contains at most maxRunes runes.
func mcpTruncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

func mcpWriteResult(c *webContext, id json.RawMessage, result any) {
	c.JSON(mcpResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func mcpWriteError(c *webContext, id json.RawMessage, code int, message string) {
	c.JSON(mcpResponse{JSONRPC: "2.0", ID: id, Error: &mcpRPCError{Code: code, Message: message}})
}
