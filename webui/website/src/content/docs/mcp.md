---
date: '2026-04-20T00:00:00+00:00'
draft: false
title: 'MCP Integration'
---

Hister exposes a [Model Context Protocol](https://modelcontextprotocol.io) (MCP)
endpoint that lets AI assistants search your personal index directly. Once
connected, the assistant can call a `search` tool to retrieve relevant pages
from your browsing history and local files as part of any conversation.

## Endpoint

```
POST /mcp
```

The endpoint follows the MCP Streamable HTTP transport. Every interaction is a
`POST` request with a JSON-RPC 2.0 body; the server responds with a JSON object.

## Authentication

The default Hister configuration does not require authentication. Authentication required only if the server requires an `access_token` or if it is configured as a multi-user server.
The MCP endpoint uses the same authentication as the rest of the Hister API.

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

### Origin header

All requests from external clients must include:

```http
Origin: hister://
```

Without this header the server's CSRF protection will reject the request.

## Available Tools

### `search`

Search your personal browsing history and indexed documents.

| Argument   | Type    | Required | Default | Description                                               |
| ---------- | ------- | -------- | ------- | --------------------------------------------------------- |
| `query`    | string  | yes      |         | Search query (see [Query Language](/docs/query-language)) |
| `limit`    | integer | no       | 10      | Maximum results to return (max 50)                        |
| `semantic` | boolean | no       | false   | Enable AI semantic search alongside keyword matching      |

The response is plain text listing matching results with their title, URL,
date added, and a short text snippet.

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
        "Authorization": "Bearer <your-access-token>",
        "Origin": "hister://"
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
        "Authorization": "Bearer <your-access-token>",
        "Origin": "hister://"
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

**Handshake:**

```bash
curl -s -X POST http://127.0.0.1:4433/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-access-token>" \
  -H "Origin: hister://" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"curl","version":"0"}}}'
```

**List tools:**

```bash
curl -s -X POST http://127.0.0.1:4433/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-access-token>" \
  -H "Origin: hister://" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'
```

**Search:**

```bash
curl -s -X POST http://127.0.0.1:4433/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-access-token>" \
  -H "Origin: hister://" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search","arguments":{"query":"python async","limit":5}}}'
```

## Example Interaction

Once connected, you can ask the assistant things like:

> "Search my history for anything about Rust error handling."

The assistant calls the `search` tool with `query: "rust error handling"` and
includes the results in its response, citing the specific pages you previously
read.

Semantic search can be enabled per query by passing `"semantic": true` in the
tool arguments. This requires [semantic search to be configured](/docs/configuration#semantic-search)
on the server.

## Protocol Details

The endpoint implements MCP specification version `2024-11-05` with the
following methods:

| Method            | Description                                                 |
| ----------------- | ----------------------------------------------------------- |
| `initialize`      | Capability negotiation; required before any other call      |
| `ping`            | Liveness check                                              |
| `tools/list`      | Returns the list of available tools and their input schemas |
| `tools/call`      | Executes a tool by name with the provided arguments         |
| `notifications/*` | Acknowledged silently (202 response, no body)               |
