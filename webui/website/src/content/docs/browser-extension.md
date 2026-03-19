---
date: '2026-03-19T00:00:00+00:00'
draft: false
title: 'Browser Extension'
---

The Hister browser extension is the primary way to automatically index your browsing history. It runs silently in the background, sending page content to your Hister server as you browse.

## Installation

- **Chrome / Chromium / Edge**: [Install from Chrome Web Store](https://chromewebstore.google.com/detail/hister/cciilamhchpmbdnniabclekddabkifhb)
- **Firefox**: [Install from Firefox Add-ons](https://addons.mozilla.org/en-US/firefox/addon/hister/) (also works on Firefox for Android)

After installing, click the extension icon in your browser toolbar to open the popup and verify the server URL is correct.

> The extension only communicates with your Hister server, it never contacts any third-party services or the websites you visit (except for downloading the page's favicon while collecting page data).

## Features

### Automatic Page Indexing

The extension automatically captures page content every time you visit a URL. It extracts the page title, full text, HTML, and favicon, then sends them to your Hister server via its API.

After a page is successfully indexed, the extension continues monitoring it in the background and re-submits if the content changes (for example on single-page applications). The re-check interval starts at 10 seconds and doubles each time the page content is unchanged, reducing resource usage over time.

Automatic indexing can be paused at any time using the toggle in the popup.

### Manual Reindex

The **Reindex Page** button in the popup forces an immediate re-submission of the current page, regardless of whether it has changed. This is useful after clearing your server's index or when a page failed to index automatically (indicated by a `!` badge on the extension icon).

### Search Engine Result Tracking

The extension detects when you click on a search result in **Google** or **DuckDuckGo** and records the query alongside the result's title and URL to provide that result for the same query in the future.

## Popup

Clicking the extension icon opens the popup, which provides quick access to the most common controls.

| Control                       | Description                                                                                                     |
| ----------------------------- | --------------------------------------------------------------------------------------------------------------- |
| **Automatic indexing** toggle | Enable or disable automatic page indexing. The setting is saved immediately.                                    |
| **Reindex Page** button       | Force a re-submission of the current page to the server.                                                        |
| **Settings icon** (⚙)         | Expand an inline form to view and update the Server URL and Access Token without opening the full options page. |

A status banner appears at the bottom of the popup after any action, showing success or error feedback. If the server rejected the last submission, a `!` badge is shown on the extension icon; saving valid settings clears it.

## Options Page

The full options page is accessible via your browser's extension manager (right-click the icon → _Options_, or navigate to the extensions settings page). It provides all configuration in one place.

To open it directly, right click on the extension icon and select "Options", or with:

- **Chrome**: `chrome://extensions` → find Hister → click **Details** → **Extension options**
- **Firefox**: `about:addons` → find Hister → click the **…** menu → **Preferences**

### Connection Settings

| Setting            | Default                  | Description                                                                                                                                                      |
| ------------------ | ------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Server URL**     | `http://127.0.0.1:4433/` | The full URL of your Hister server, including scheme and port.                                                                                                   |
| **Access Token**   | _(empty)_                | Required if your server is configured with `app.access_token`. Sent as the `X-Access-Token` header.                                                              |
| **Custom Headers** | _(none)_                 | Additional HTTP headers included with every request. Useful for reverse-proxy authentication (e.g., `Authorization: Basic …`). Each header is a name/value pair. |

Click **Save Settings** to apply. The extension validates the connection by calling `GET /api/config` before saving; an invalid URL or wrong token will show an error instead.

## Troubleshooting

**The extension icon shows a `!` badge**

The last attempt to send page data to the server failed. Open the popup to see the error. Common causes:

- The Hister server is not running: start it with `hister listen`.
- The **Server URL** is wrong: confirm it matches the address printed when the server starts (default `http://127.0.0.1:4433/`).
- The **Access Token** is incorrect or missing: update it in the popup settings or the options page.

**Pages are not being indexed**

- Make sure **Automatic indexing** is enabled in the popup.
- Check that the server is reachable and the URL/token are correct (see above).
- Some pages (browser-internal pages like `chrome://…`, `about:…`) cannot be accessed by extensions and are silently skipped.
