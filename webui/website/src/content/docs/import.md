---
date: '2026-07-22T00:00:00+02:00'
draft: false
title: 'Importing Documents'
---

The `hister import` command collects related import tools under one command. Every import sends documents to a running Hister server.

## Available Import Sources

| Command                                     | Source                                     | Default label |
| ------------------------------------------- | ------------------------------------------ | ------------- |
| `hister import file INPUT...`               | Hister exports, archives, and saved pages  | `import`      |
| `hister import browser [BROWSER] [DB_PATH]` | Browser history databases                  | `browser`     |
| `hister import linkwarden INSTANCE_URL`     | A Linkwarden instance through its HTTP API | `linkwarden`  |
| `hister import karakeep INSTANCE_URL`       | A Karakeep instance through its HTTP API   | `karakeep`    |
| `hister import shaarli INSTANCE_URL`        | A Shaarli instance through its HTTP API    | `shaarli`     |

Use the global `--server-url` and `--token` flags when the destination Hister server differs from your configured server or requires authentication.

Use `--label LABEL` with any import source to attach the same label to every imported document. Without this flag, labels stored in imported documents or resumed browser jobs are preserved. The default shown above is applied only when no label was supplied by the user or the imported document.

## Importing Files

Use `hister import file` with any number of files or directories:

```bash
hister import file export.json page.html ~/Downloads/saved-pages
```

Supported inputs are:

| Input              | Behavior                                                                     |
| ------------------ | ---------------------------------------------------------------------------- |
| Hister JSON export | Restores serialized documents without extracting their stored content again  |
| 7z archive         | Reads the first JSON export inside the archive                               |
| HTML or HTM page   | Extracts the canonical URL and sends the saved page to Hister for processing |
| Directory          | Imports supported files directly inside it in filename order                 |

Directory imports are not recursive. Unsupported files and nested directories are ignored.

The following options apply to file imports:

| Flag                      | Purpose                                                     |
| ------------------------- | ----------------------------------------------------------- |
| `--skip-existing`         | Keep documents whose URL already exists in Hister           |
| `--label LABEL`           | Override stored labels and the default `import` label       |
| `--batch-size N`          | Submit from 1 through 100 documents per request             |
| `--start-date YYYY-MM-DD` | Import documents added on or after the date                 |
| `--end-date YYYY-MM-DD`   | Import documents added on or before the date                |
| `--global`                | Import for all users when authenticated as an administrator |
| `--user-id ID`            | Import for one user when authenticated as an administrator  |

## Importing Browser History

Browser history contains URLs and visit information, but it does not contain the page contents. Hister reads the URLs from the browser database and fetches the pages before indexing them.

### Automatic Detection

Run the command without arguments to detect supported browser databases in their standard locations:

```bash
hister import browser
```

Automatic detection supports Firefox, Firefox Developer Edition, Zen, Waterfox, Chrome, Chromium, Brave, Vivaldi, Edge, Opera, and Ladybird.

### Selecting a Browser or Database

You can provide a browser name, a database path, or both:

```bash
# Detect the Firefox database path
hister import browser firefox

# Detect the browser type from a database
hister import browser ~/.mozilla/firefox/example.default/places.sqlite

# Specify both values
hister import browser firefox ~/.mozilla/firefox/example.default/places.sqlite
```

Firefox stores history in `places.sqlite` inside its profile directory. Chromium based browsers usually store it in a file named `History` inside their profile directory.

Use `--min-visit N` to import only URLs that have at least `N` recorded visits.

Browser history documents receive the `browser` label by default. Use `--label LABEL` to replace it. Resumed browser import jobs reuse their stored label unless this flag is supplied again.

### Resume and Inspect a Browser Import

Browser imports use persistent crawl jobs named `browser-import-YYYY-MM-DD`. It is safe to interrupt the process and continue it later. Completed URLs remain completed, while pending and failed URLs remain available in the job.

```bash
hister crawl list
hister crawl show browser-import-YYYY-MM-DD
hister crawl errors browser-import-YYYY-MM-DD
hister crawl queue browser-import-YYYY-MM-DD
```

Add `--count` to `hister crawl queue` when only the number of tracked URLs is needed.

### Browser Import Backends

The default HTTP backend is fast, but it cannot execute JavaScript. Select a browser based backend when a site requires client side rendering:

```bash
hister import browser --backend chromedp
```

The supported backends are `http`, `chromedp`, and `bidi`. Backend options, request headers, and cookies can be supplied when necessary:

```bash
hister import browser \
  --backend chromedp \
  --backend-option exec_path=/usr/bin/chromium \
  --header "Accept-Language=en" \
  --cookie "session=abc; Domain=example.com"
```

The `--backend-option`, `--header`, and `--cookie` flags can be repeated. Cookies use `Set-Cookie` syntax and require a `Domain` attribute. See [Website Crawler](crawler) for all crawler settings and backend limitations.

Automated requests can be rejected by bot protection, expired sessions, removed pages, or network failures. Failed URLs remain visible through `hister crawl errors` and can be retried by continuing the job.

## Importing from Linkwarden

Create an API token in Linkwarden, then store it in the environment before running the import:

```bash
export HISTER_IMPORT_LINKWARDEN_TOKEN='your-linkwarden-token'
hister import linkwarden https://links.example.com
```

You can use `--api-token` as a temporary override. The Linkwarden API token is separate from the global `--token` flag, which authenticates with the destination Hister server. Prefer the environment variable so the Linkwarden token does not appear in shell history or process listings.

### Incremental Linkwarden Imports

Every imported Linkwarden document receives `source: linkwarden` metadata. Before requesting Linkwarden records, Hister searches for `metadata.source:linkwarden`, sorts the results by update date, and reads the newest document's `updated` timestamp.

When a previous result exists, the importer adds an `after:` filter to the Linkwarden search request so only newer records are fetched. When no previous result exists, it performs a complete import. Repeating the command therefore continues from the most recent Linkwarden import automatically.

### Linkwarden Data Mapping

| Linkwarden value                       | Hister value            |
| -------------------------------------- | ----------------------- |
| URL                                    | Normalized document URL |
| Name                                   | Title                   |
| Description and extracted text content | Searchable text         |
| Import date, then creation date        | Added timestamp         |
| Update date                            | Updated timestamp       |
| Tags, collection, source type, and ID  | Document metadata       |

Records without a URL are skipped because every Hister document requires a URL. Pagination and batch submission are automatic.

If a Linkwarden URL record has no extracted text content, Hister downloads the page and extracts its contents before importing it. The configured crawler backend is used for these downloads. The crawler is initialized only when missing content is encountered and is reused for the rest of the import.

## Importing from Karakeep

Karakeep was formerly named Hoarder. Create an API key in Karakeep, then store it in the environment before running the import:

```bash
export HISTER_IMPORT_KARAKEEP_TOKEN='your-karakeep-token'
hister import karakeep https://karakeep.example.com
```

You can use `--api-token` as a temporary override. The Karakeep API token is separate from the global `--token` flag used for the destination Hister server.

### Incremental Karakeep Imports

Every imported Karakeep document receives `source: karakeep` metadata. Hister searches for `metadata.source:karakeep` and reads the newest imported document timestamp before calling Karakeep. If a previous import exists, the importer uses Karakeep search with an `after:` date. Otherwise, it requests every bookmark.

Karakeep applies the `after:` search qualifier to the bookmark creation date. A bookmark created before the checkpoint and edited afterward might therefore require a complete import to refresh it.

### Karakeep Data Mapping

| Karakeep value                             | Hister value            |
| ------------------------------------------ | ----------------------- |
| Link URL or source URL                     | Normalized document URL |
| Bookmark title, content title, or filename | Title                   |
| Note, summary, description, and content    | Searchable text         |
| Creation date                              | Added timestamp         |
| Modification date                          | Updated timestamp       |
| Tags, statuses, source, assets, and ID     | Document metadata       |

For link bookmarks, Hister extracts the stored Karakeep HTML when it is available. If stored HTML is absent or cannot be extracted, Hister downloads the page with the selected crawler backend. Text and asset bookmarks use their stored content when they have a source URL.

Text and asset bookmarks without a source URL are skipped because every Hister document requires a URL. Pagination and batch submission are automatic.

Consult the [Karakeep API documentation](https://docs.karakeep.app/api) when troubleshooting API access.

## Importing from Shaarli

Copy the API secret from the Shaarli administration page, then store it in the environment before running the import:

```bash
export HISTER_IMPORT_SHAARLI_SECRET='your-shaarli-api-secret'
hister import shaarli https://shaarli.example.com
```

You can use `--api-token` as a temporary override. For Shaarli, this option accepts the API secret from the administration page. Hister uses the secret to generate a short lived HS512 JWT for each API request. The secret itself is not sent to Shaarli. The global `--token` flag remains the access token for the destination Hister server.

### Incremental Shaarli Imports

Every imported Shaarli document receives `source: shaarli` metadata. Hister searches for `metadata.source:shaarli` and reads the newest imported document timestamp before calling Shaarli.

If a previous import exists, Hister requests Shaarli history since that timestamp and retrieves the current value of every created or updated Shaare. Repeated events for the same Shaare are combined. Deleted Shaares are ignored because an import does not remove existing Hister documents. Without a previous result, Hister requests every Shaare.

### Shaarli Data Mapping

| Shaarli value                                 | Hister value            |
| --------------------------------------------- | ----------------------- |
| Link URL or text note permalink               | Normalized document URL |
| Title                                         | Title                   |
| Description and downloaded page content       | Searchable text         |
| Creation date                                 | Added timestamp         |
| Update date                                   | Updated timestamp       |
| Tags, privacy, short URL, note status, and ID | Document metadata       |

Shaarli stores bookmark descriptions rather than complete copies of linked pages. Hister therefore downloads every ordinary bookmark with the configured crawler backend and combines its extracted content with the stored description. Text notes use their description directly and keep a stable Shaarli permalink, so they are not downloaded.

Pagination and batch submission are automatic. Consult the [Shaarli API documentation](https://shaarli.github.io/api-documentation/) and [Shaarli REST API authentication guide](https://shaarli.readthedocs.io/en/master/REST-API.html) when troubleshooting API access.

## Service Import Options

The following options apply to Linkwarden, Karakeep, and Shaarli imports:

| Flag                         | Purpose                                                      |
| ---------------------------- | ------------------------------------------------------------ |
| `--api-token TOKEN`          | Override the source service credential for this invocation   |
| `--backend BACKEND`          | Download missing content with `http`, `chromedp`, or `bidi`  |
| `--backend-option KEY=VALUE` | Set an option for the selected crawler backend               |
| `--header KEY=VALUE`         | Add a request header when downloading missing content        |
| `--cookie COOKIE`            | Add a cookie when downloading missing content                |
| `--skip-existing`            | Keep documents whose normalized URL already exists in Hister |
| `--label LABEL`              | Override the default service source label                    |
| `--batch-size N`             | Submit from 1 through 100 documents per request              |
| `--start-date YYYY-MM-DD`    | Import documents added on or after the date                  |
| `--end-date YYYY-MM-DD`      | Import documents added on or before the date                 |
| `--global`                   | Import for all users when authenticated as an administrator  |
| `--user-id ID`               | Import for one user when authenticated as an administrator   |

For example:

```bash
hister import linkwarden https://links.example.com \
	--backend chromedp \
	--backend-option exec_path=/usr/bin/chromium \
	--skip-existing \
  --start-date 2024-01-01 \
  --batch-size 25
```

Linkwarden provides `/api/v1/search` and Bearer token authentication. Consult the [Linkwarden API documentation](https://docs.linkwarden.app/api/search-links) when troubleshooting API access.
