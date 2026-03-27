---
date: '2026-02-25T00:00:00+00:00'
draft: false
title: 'Configuration Reference'
---

Hister is configured via a YAML file. The default location is:

- **Linux**: `~/.config/hister/config.yml` (or `$XDG_DATA_HOME/hister/config.yml`)
- **macOS**: `~/Library/Application Support/hister/config.yml`
- **Windows**: `%LOCALAPPDATA%\hister\config.yml`

Generate a config file with default values:

```bash
hister create-config ~/.config/hister/config.yml
```

**Important**: Restart the Hister server after modifying the configuration file.

## Full Example

```yaml
app:
  directory: '~/.config/hister'
  search_url: 'https://google.com/search?q={query}'
  log_level: 'info'
  open_results_on_new_tab: false

server:
  address: '127.0.0.1:4433'
  base_url: 'http://127.0.0.1:4433'
  database: 'db.sqlite3'

indexer:
  detect_languages: true
  directories:
    - path: '~/notes'
      filetypes: ['txt', 'md']

hotkeys:
  web:
    '/': 'focus_search_input'
    'enter': 'open_result'
    'alt+enter': 'open_result_in_new_tab'
    'alt+j': 'select_next_result'
    'alt+k': 'select_previous_result'
    'alt+o': 'open_query_in_search_engine'
    'alt+v': 'view_result_popup'
    'tab': 'autocomplete'
    '?': 'show_hotkeys'

sensitive_content_patterns:
  aws_access_key: 'AKIA[0-9A-Z]{16}'
  github_token: '(ghp|gho|ghu|ghs|ghr)_[a-zA-Z0-9]{36}'
```

## `app` Section

| Key                       | Type   | Default                               | Description                                                                    |
| ------------------------- | ------ | ------------------------------------- | ------------------------------------------------------------------------------ |
| `directory`               | string | platform default                      | Directory where Hister stores its data (index, rules, secret key).             |
| `search_url`              | string | `https://google.com/search?q={query}` | Fallback web search URL. Use `{query}` as the placeholder for the search term. |
| `access_token`            | string | (none)                                | Optional access token for securing the API. See [Access Token](#access-token). |
| `user_handling`           | bool   | `false`                               | Enable multi-user mode. See [User Handling](/docs/user-handling) for details.  |
| `log_level`               | string | `info`                                | Log verbosity. One of: `debug`, `info`, `warn`, `error`.                       |
| `debug_sql`               | bool   | `false`                               | Enable verbose SQL query logging.                                              |
| `open_results_on_new_tab` | bool   | `false`                               | Open search results in a new browser tab instead of the current tab.           |

## `server` Section

| Key        | Type   | Default                | Description                                                                                                  |
| ---------- | ------ | ---------------------- | ------------------------------------------------------------------------------------------------------------ |
| `address`  | string | `127.0.0.1:4433`       | Host and port to listen on. Use `[::]:4433` or `0.0.0.0:4433` to listen on all interfaces.                   |
| `base_url` | string | derived from `address` | Public URL of the Hister instance. Required when `address` uses `0.0.0.0`. Must match how you access Hister. |
| `database` | string | `db.sqlite3`           | SQLite database filename (relative to `app.directory`).                                                      |

## TUI Settings

TUI settings are configured in a separate `tui.yaml` file located in the same directory as your main config file. This file is automatically created with default values when you first run `hister search`.

### Theme Settings

| Key            | Type   | Default            | Description                                                                                             |
| -------------- | ------ | ------------------ | ------------------------------------------------------------------------------------------------------- |
| `dark_theme`   | string | `tokyonight`       | Theme to use in dark mode. Available themes: catppuccin, dracula, gruvbox, nord, rose-pine, tokyonight. |
| `light_theme`  | string | `catppuccin-latte` | Theme to use in light mode.                                                                             |
| `color_scheme` | string | `auto`             | Color scheme mode: `auto` (follow system), `dark`, or `light`.                                          |
| `themes_dir`   | string | (built-in themes)  | Custom directory for theme YAML files (optional).                                                       |

**Built-in themes**: catppuccin-mocha, catppuccin-frappe, catppuccin-macchiato, catppuccin-latte, dracula, gruvbox, nord, rose-pine, tokyonight.

## `indexer` Section

| Key                | Type        | Default | Description                                                                                                                                                        |
| ------------------ | ----------- | ------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `detect_languages` | bool        | `true`  | Enable automatic language detection for indexed pages. See [Language Detection](#language-detection) for details on memory/CPU impact and reindexing requirements. |
| `directories`      | Directory[] | (none)  | List of local directories to index. See [Local Directory Indexing](#local-directory-indexing) for details.                                                         |
| `max_file_size_mb` | int         | `1`     | Maximum file size (in MB) to index. Files larger than this value are skipped.                                                                                      |

### Directory Entry

Each entry in `directories` is an object with the following keys:

| Key              | Type     | Default | Description                                                                                                                                                                                                           |
| ---------------- | -------- | ------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `path`           | string   | ""      | **(required)** Directory path to index. Paths starting with `~/` are expanded to your home directory.                                                                                                                 |
| `filetypes`      | string[] | (none)  | Only index files with these extensions (without the dot). e.g. `['txt', 'md']`.                                                                                                                                       |
| `patterns`       | string[] | (none)  | Only index files whose names match at least one glob pattern. e.g. `['doc_*', 'README*']`.                                                                                                                            |
| `excludes`       | string[] | (none)  | Skip files whose names match any of these glob patterns. e.g. `['*secret*', '*.tmp']`.                                                                                                                                |
| `include_hidden` | bool     | `false` | When `true`, index hidden files/directories (starting with `.`) and well-known dependency/cache directories (`node_modules`, `__pycache__`, etc.) that are skipped by default. User-specified `excludes` still apply. |

When multiple filters are specified, they are applied in order: excludes first, then filetypes, then patterns. A file must pass all specified filters to be indexed. When a filter is omitted, it is not applied (all files pass).

## Local Directory Indexing

The `indexer.directories` option lets you index local files so they appear alongside your browser history in search results. Files are indexed automatically when the server starts, running in the background so the server is available immediately. A file watcher monitors configured directories for changes, so new and modified files are indexed automatically without needing to restart the server.

```yaml
indexer:
  directories:
    - path: '~/notes'
      filetypes: ['txt', 'md']
      patterns: ['doc_*']
      excludes: ['*secret*']
    - path: '~/Documents/wiki'
    - path: '/path/to/project'
      filetypes: ['go', 'py', 'js']
```

Files are indexed recursively, with the following rules:

- Hidden files and directories (starting with `.`) are skipped unless `include_hidden: true`
- Well-known dependency/cache directories (`node_modules`, `bower_components`, `jspm_packages`, `__pycache__`, `__pypackages__`) are skipped unless `include_hidden: true`
- Binary files are skipped
- Files larger than `indexer.max_file_size_mb` (default: 1 MB) are skipped
- Files matching `sensitive_content_patterns` are skipped

Changes to indexed directories are picked up automatically by the file watcher, no server restart is needed. On server start, only files that have been modified since they were last indexed are re-processed. File results appear with the domain `local` and are served through the Hister web interface directly.

No reindex is required when adding or removing files. Files are detected and indexed automatically.

## Access Token

The `app.access_token` setting provides a simple authentication mechanism to secure your Hister instance. When configured, clients must include the token in API requests using the `X-Access-Token` HTTP header. This is particularly useful when exposing Hister to the network or internet, preventing unauthorized access to your browsing history and search index.

To use the access token, set it in your configuration file:

```yaml
app:
  access_token: 'your-secret-token-here'
```

The web UI automatically prompts for and stores the access token when configured. The access token has to be added to the browser extension as well.

For command-line usage with `curl` or similar tools, include the header in your requests:

```bash
curl -H "X-Access-Token: your-secret-token-here" http://localhost:4433/api/config
```

**Security note**: The access token is transmitted in plain text with each request. When exposing Hister over the network, always use HTTPS (via reverse proxy) to encrypt the token in transit. The token provides basic access control but does not replace proper authentication systems for multi-user scenarios.

## Language Detection

The `indexer.detect_languages` option (default: `true`) controls automatic language detection for indexed pages. When enabled, Hister uses language detection libraries to identify the language of each page's content, creating separate language-specific indexes that improve search accuracy through language-aware tokenization and stemming.

**Performance considerations**: Language detection increases both CPU usage and memory consumption. Each document requires additional processing to analyze text and determine its language, and separate indexes are maintained for each detected language. If you're experiencing memory pressure or slow indexing performance, especially with large numbers of documents, consider disabling this feature.

**Important**: Changing this setting requires a full reindex to take effect. After enabling or disabling language detection in your config file, run:

```bash
hister reindex
```

The reindex operation will rebuild all indexes according to the new setting. With language detection disabled, all documents are indexed using a single default analyzer, reducing memory overhead and simplifying the indexing process at the cost of potentially less accurate search results.

## `hotkeys.web` Section

Defines keyboard shortcuts for the web interface. Each entry maps a key combination to an action.

**Key format**: `[modifier+]key` where modifier is `ctrl`, `alt`, or `meta`. Key can be a letter, digit, or special key (`enter`, `tab`, `arrowup`, `arrowdown`, etc.).

| Action                        | Description                                                     |
| ----------------------------- | --------------------------------------------------------------- |
| `focus_search_input`          | Move focus to the search input box                              |
| `open_result`                 | Open the selected (or first) result                             |
| `open_result_in_new_tab`      | Open the selected result in a new tab                           |
| `select_next_result`          | Move selection to the next result                               |
| `select_previous_result`      | Move selection to the previous result                           |
| `open_query_in_search_engine` | Open the current query in the configured fallback search engine |
| `view_result_popup`           | Open the offline preview popup for the selected result          |
| `autocomplete`                | Accept the autocomplete suggestion                              |
| `show_hotkeys`                | Show the keyboard shortcuts help overlay                        |

## TUI Configuration

TUI-specific settings are stored in a separate `tui.yaml` file in the same directory as your main config. This file is automatically created with defaults the first time you run `hister search`.

**Default location**: `~/.config/hister/tui.yaml` (or alongside your config file)

### tui.yaml Example

```yaml
dark_theme: 'tokyonight'
light_theme: 'catppuccin-latte'
color_scheme: 'auto'

hotkeys:
  ctrl+c: 'quit'
  f1: 'toggle_help'
  tab: 'toggle_focus'
  esc: 'toggle_focus'
  up: 'scroll_up'
  k: 'scroll_up'
  down: 'scroll_down'
  j: 'scroll_down'
  enter: 'open_result'
  ctrl+d: 'delete_result'
  ctrl+t: 'toggle_theme'
  ctrl+s: 'toggle_settings'
  ctrl+o: 'toggle_sort'
  alt+1: 'tab_search'
  alt+2: 'tab_history'
  alt+3: 'tab_rules'
  alt+4: 'tab_add'
```

### TUI Hotkeys

TUI keyboard shortcuts are configured in `tui.yaml` under the `hotkeys` section. See the [tui.yaml example](#tui-configuration) above.

| Action            | Description                                                     |
| ----------------- | --------------------------------------------------------------- |
| `quit`            | Exit the TUI                                                    |
| `toggle_help`     | Show/hide the keybindings help overlay                          |
| `toggle_focus`    | Switch between search input and results list                    |
| `scroll_up`       | Move selection up                                               |
| `scroll_down`     | Move selection down                                             |
| `open_result`     | Open the selected result in your browser                        |
| `delete_result`   | Delete the selected entry from the index                        |
| `toggle_theme`    | Open the interactive theme picker overlay                       |
| `toggle_settings` | Open the keybinding editor overlay                              |
| `toggle_sort`     | Toggle domain-based sorting for search results                  |
| `tab_search`      | Switch to the Search tab                                        |
| `tab_history`     | Switch to the History tab (view recent searches)                |
| `tab_rules`       | Switch to the Rules tab (manage blacklist/priority/alias rules) |
| `tab_add`         | Switch to the Add tab (manually add URLs to index)              |

## `sensitive_content_patterns` Section

A map of named regex patterns. Any indexed page containing a match will have that field redacted before storage. Useful for preventing accidental indexing of secrets, tokens, or credentials.

```yaml
sensitive_content_patterns:
  my_pattern: 'regex here'
```

Default patterns cover common secrets: AWS keys, GitHub tokens, SSH/PGP private keys.

## Environment Variables

You can override configuration values using environment variables. The naming convention is:

```textplain
HISTER__<SECTION>__<KEY>=value
```

For example:

- `HISTER__APP__LOG_LEVEL=debug` overrides `app.log_level`
- `HISTER__SERVER__ADDRESS=0.0.0.0:8080` overrides `server.address`

Two special-purpose variables are also supported:

| Variable          | Description                                                            |
| ----------------- | ---------------------------------------------------------------------- |
| `HISTER_PORT`     | Override the port only (keeps the existing host from `server.address`) |
| `HISTER_DATA_DIR` | Override `app.directory`                                               |
