---
date: '2026-04-20T00:00:00+00:00'
draft: false
title: 'MCP Integration'
---

Hister exposes a [Model Context Protocol](https://modelcontextprotocol.io) (MCP)
endpoint that lets AI assistants read from your Hister index directly. Once
connected, the assistant can search indexed pages, retrieve stored previews,
and inspect Hister history through MCP tools.

## Endpoint

```
POST /mcp
```

The endpoint follows the MCP Streamable HTTP transport. Every interaction is a
`POST` request with a JSON-RPC 2.0 body. The server responds with a JSON object.

## Authentication

The default Hister configuration does not require authentication. Authentication
is required only when Hister is configured with `app.access_token` or
`app.user_handling`. The MCP endpoint uses the same token authentication as the
rest of the Hister API.

### Static access token

Pass the value of `app.access_token` from your config file:

```http
Authorization: Bearer <your-access-token>
```

Alternatively, use the `X-Access-Token` header with the same value.

### Multi-user mode

Generate a personal token on the profile page (`/profile`) or via:

```bash
hister update-user <username> --regen-token
```

Then pass it the same way:

```http
Authorization: Bearer <your-user-token>
```

### Public mode

When `app.public: true` is enabled, unauthenticated MCP access is allowed for
public routes. MCP tools can read public search results and previews for global
documents. The `get_history` tool is disabled in public mode, including for
authenticated callers.

## Available Tools

### `search`

Search your personal browsing history and indexed documents.

| Argument    | Type            | Required | Default | Description                                                                 |
| ----------- | --------------- | -------- | ------- | --------------------------------------------------------------------------- |
| `query`     | string          | yes      |         | Search query (see [Query Language](/docs/query-language))                   |
| `limit`     | integer         | no       | 10      | Maximum results to return. Values below 1 or above 50 use the default.      |
| `date_from` | string          | no       |         | Return only documents indexed on or after this date. Format: `YYYY-MM-DD`.  |
| `date_to`   | string          | no       |         | Return only documents indexed on or before this date. Format: `YYYY-MM-DD`. |
| `semantic`  | boolean         | no       | false   | Enable AI semantic search alongside keyword matching                        |
| `fields`    | array of string | no       | `[]`    | Extra fields to include in each result. See below.                          |

Semantic search is used only when it is enabled and available on the Hister
server. If the server does not have semantic search configured, `"semantic":
true` falls back to normal keyword search.

By default the response includes title, URL, date added, and a short text
snippet per result. Pass `fields` to include additional data:

| Field value | Description                                         |
| ----------- | --------------------------------------------------- |
| `text`      | Full stored article text instead of a short snippet |
| `html`      | Raw HTML                                            |
| `language`  | Detected language code (e.g. `en`, `de`)            |
| `label`     | User-defined label                                  |
| `domain`    | Domain name                                         |
| `score`     | Relevance score                                     |
| `type`      | Document type: `web` or `local`                     |

Example: to summarize articles on a topic without re-fetching any URLs:

```json
{ "query": "kubernetes networking", "limit": 5, "fields": ["text"] }
```

Example with a date range:

```json
{ "query": "postgres migration", "date_from": "2026-01-01", "date_to": "2026-01-31" }
```

### `get_preview`

Retrieve the stored preview for an indexed document by exact URL.

| Argument    | Type   | Required | Default | Description                                     |
| ----------- | ------ | -------- | ------- | ----------------------------------------------- |
| `url`       | string | yes      |         | Exact URL of the indexed document to preview    |
| `extractor` | string | no       |         | Extractor name to use for rendering the preview |

The response is a text block containing the document title, URL, indexing date,
available preview metadata, and extracted content. Metadata can include author,
published date, modified date, description, site name, type, language, image,
JSON LD structured data, and embedded video URLs. If stored HTML is unavailable,
or the requested extractor fails, Hister falls back to the stored text.

Example:

```json
{ "url": "https://example.com/article" }
```

### `get_history`

Retrieve items from the Hister history views. This tool is available only when
public mode is disabled.

| Argument   | Type    | Required | Default   | Description                                                                           |
| ---------- | ------- | -------- | --------- | ------------------------------------------------------------------------------------- |
| `mode`     | string  | no       | `indexed` | `indexed` returns recently indexed pages. `opened` returns opened result history.     |
| `limit`    | integer | no       | 20        | Maximum items to return. Values below 1 or above 100 use the default.                 |
| `page_key` | string  | no       |           | Pagination cursor for `indexed` mode. Use `next_page_key` from the previous response. |
| `last_id`  | integer | no       |           | Pagination cursor for `opened` mode. Use `next_last_id` from the previous response.   |

Indexed history results include title, URL, indexed time, indexed version count,
and `next_page_key` when another page is available. Opened history results
include title, URL, original query, opened time, indexed version count, and
`next_last_id` when another page is available.

Examples:

```json
{ "mode": "indexed", "limit": 20 }
```

```json
{ "mode": "opened", "limit": 20 }
```

## Client Configuration

### Claude Desktop

Add a `hister` entry to your Claude Desktop configuration file.

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`  
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`  
**Linux**: `~/.config/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "hister": {
      "url": "http://127.0.0.1:4433/mcp",
      "headers": {
        "Authorization": "Bearer <your-access-token>"
      }
    }
  }
}
```

Restart Claude Desktop after saving the file. The `search` tool will appear
in the tools panel when starting a new conversation.

### Cursor

Open **Settings** and locate the MCP servers section, or edit
`~/.cursor/mcp.json` directly:

```json
{
  "mcpServers": {
    "hister": {
      "url": "http://127.0.0.1:4433/mcp",
      "headers": {
        "Authorization": "Bearer <your-access-token>"
      }
    }
  }
}
```

### Remote or self-hosted server

Replace `http://127.0.0.1:4433` with your server's `base_url`. If you run
Hister behind a reverse proxy under a subpath (e.g. `https://example.com/hister`),
the endpoint is `https://example.com/hister/mcp`.

## Manual Testing with curl

You can verify the endpoint is working before configuring any client.

The authorization header is required only when authentication is enabled in
Hister's config.

**Handshake:**

```bash
curl -s -X POST http://127.0.0.1:4433/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-access-token>" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"curl","version":"0"}}}'
```

**List tools:**

```bash
curl -s -X POST http://127.0.0.1:4433/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-access-token>" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'
```

**Search:**

```bash
curl -s -X POST http://127.0.0.1:4433/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-access-token>" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search","arguments":{"query":"python async","limit":5}}}'
```

**Search with full text (no re-fetching):**

```bash
curl -s -X POST http://127.0.0.1:4433/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-access-token>" \
  -d '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"search","arguments":{"query":"python async","limit":5,"fields":["text","language"]}}}'
```

**Preview:**

```bash
curl -s -X POST http://127.0.0.1:4433/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-access-token>" \
  -d '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_preview","arguments":{"url":"https://example.com/article"}}}'
```

**Indexed history:**

```bash
curl -s -X POST http://127.0.0.1:4433/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-access-token>" \
  -d '{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"get_history","arguments":{"mode":"indexed","limit":10}}}'
```

## Example Interaction

Once connected, you can ask the assistant things like:

> "Search my history for anything about Rust error handling."

The assistant calls the `search` tool with `query: "rust error handling"` and
includes the results in its response, citing the specific pages you previously
read.

You can also ask:

> "Open the stored preview for the article I read about SQLite migrations."

The assistant can search first, then call `get_preview` with the selected URL.

Semantic search can be enabled per query by passing `"semantic": true` in the
tool arguments. This requires [semantic search to be configured](/docs/configuration#semantic-search)
on the server.

## Protocol Details

The endpoint implements MCP specification version `2024-11-05` with the
following methods:

| Method                      | Description                                                 |
| --------------------------- | ----------------------------------------------------------- |
| `initialize`                | Capability negotiation; required before any other call      |
| `ping`                      | Liveness check                                              |
| `tools/list`                | Returns the list of available tools and their input schemas |
| `tools/call`                | Executes a tool by name with the provided arguments         |
| `notifications/initialized` | Acknowledged with 202 when sent as a notification           |
| `notifications/cancelled`   | Acknowledged with 202 when sent as a notification           |
