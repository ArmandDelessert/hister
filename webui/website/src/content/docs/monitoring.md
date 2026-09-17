---
date: '2026-09-17T00:00:00+02:00'
draft: false
title: 'Monitoring'
description: 'Monitor Hister with Prometheus using authenticated scrapes, search and indexing metrics, and storage gauges.'
---

Hister exposes optional Prometheus metrics at `GET /metrics`. Prometheus periodically fetches this endpoint and stores the measurements. Enabling metrics does not automatically send statistics to an external service.

## Enable Metrics

Metrics are disabled by default. Add this setting to your Hister configuration, then restart the server:

```yaml
server:
  metrics: true
```

Alternatively, enable metrics with an environment variable when starting the server:

```bash
HISTER__SERVER__METRICS=true hister listen
```

For a local instance without authentication, check the endpoint with:

```bash
curl --fail http://127.0.0.1:4433/metrics
```

The endpoint uses the same address and base path as the rest of Hister. If `server.base_url` is `https://hister.example.com/hister`, fetch `https://hister.example.com/hister/metrics`.

## Authentication

Metrics describe the whole Hister instance, including document counts and activity totals across all users. Access follows these rules:

| Hister configuration                                          | Credentials for scraping                 |
| ------------------------------------------------------------- | ---------------------------------------- |
| Neither `app.access_token` nor `app.user_handling` is enabled | No credentials required                  |
| `app.access_token` is set and `app.user_handling` is disabled | The configured access token              |
| `app.user_handling: true`                                     | An administrator's personal access token |

Public mode (`app.public: true`) keeps the metrics endpoint protected. It does not grant anonymous access to metrics.

Send the token using the `Authorization: Bearer` header:

```bash
curl --fail \
  --header 'Authorization: Bearer YOUR_TOKEN' \
  http://127.0.0.1:4433/metrics
```

Replace `YOUR_TOKEN` with the appropriate token from the table. In multiple user mode, an administrator can obtain their [personal access token](user-handling#personal-access-tokens) from Hister's profile page. Hister also accepts the `X-Access-Token` header.

## Configure Prometheus

Add a scrape job to your Prometheus configuration:

```yaml
scrape_configs:
  - job_name: hister
    scrape_interval: 30s
    scheme: http
    metrics_path: /metrics
    authorization:
      type: Bearer
      credentials_file: /etc/prometheus/hister.token
    static_configs:
      - targets: ['127.0.0.1:4433']
```

Put the token in `/etc/prometheus/hister.token` on the Prometheus host, with permissions that allow Prometheus to read it. The file contains only the token, without the `Bearer` prefix. Omit the `authorization` block if Hister has no authentication configured.

This example assumes Prometheus can reach Hister at `127.0.0.1:4433`. If Prometheus runs on another host or in a container, use an address reachable from there. See [Server Setup](server-setup) for network access and reverse proxy configuration.

For `server.base_url: https://hister.example.com/hister`, replace the job's connection settings with the following, keeping its authentication settings:

```yaml
scheme: https
metrics_path: /hister/metrics
static_configs:
  - targets: ['hister.example.com:443']
```

Reload or restart Prometheus after updating its configuration. The target should appear as up, and Hister's metrics have names beginning with `hister_`. See the [Prometheus configuration reference](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config) for additional scrape settings.

## Available Metrics

| Metric                             | Type      | Meaning                                                                                                                                                           |
| ---------------------------------- | --------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `hister_queries_total`             | Counter   | Searches executed, labeled by `result`: `hit`, `miss`, or `error`.                                                                                                |
| `hister_search_duration_seconds`   | Histogram | Search latency in seconds, including failed searches.                                                                                                             |
| `hister_documents_indexed_total`   | Counter   | Successful document indexing operations, labeled by `type`: `web`, `local`, or `remote`. Updates to existing documents also count.                                |
| `hister_indexing_duration_seconds` | Histogram | Elapsed indexing time per document, from preparation through a successful index write.                                                                            |
| `hister_datastore_size_bytes`      | Gauge     | Sum of file sizes under Hister's data directory (`app.directory`), including indexes and stored content. External PostgreSQL storage is outside this measurement. |
| `hister_index_document_count`      | Gauge     | Current number of documents in the search index across all users.                                                                                                 |

For batch indexing, counters and duration samples are recorded after the batch's index writes succeed. Each duration includes time spent waiting for the rest of the batch and the batch write itself, so batch size affects the measured latency. Asynchronous semantic embedding work completes separately.

Counters and histograms start fresh when the Hister server starts. Counter series with labels appear after the corresponding activity first occurs. Histograms expose `_bucket`, `_sum`, and `_count` series.

The document count and storage size gauges refresh in the background at startup and every 30 seconds. They can lag behind recent changes even if Prometheus scrapes more frequently. If a storage size measurement fails, the previous value is retained.

The endpoint also includes Go runtime and process metrics, such as garbage collection, memory usage, and CPU time, where supported by the host platform.

## Troubleshooting

| Response                      | What to check                                                                                         |
| ----------------------------- | ----------------------------------------------------------------------------------------------------- |
| `403 Forbidden`               | Supply the configured token. In multiple user mode, use an administrator's token.                     |
| `404 Not Found`               | Enable `server.metrics`, restart Hister, and include any path prefix from `server.base_url`.          |
| Connection refused or timeout | Check the target address from the Prometheus host or container and confirm Hister is listening there. |
