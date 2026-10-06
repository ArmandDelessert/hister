---
date: '2026-10-06T12:00:00+02:00'
draft: false
title: 'Search Your Saved Content'
description: 'Import bookmarks, reading lists, sitemaps, and local notes, then use labels and query aliases to reach the content you need.'
---

Import your browser bookmarks, saved articles, and project references into
Hister, and you can search them alongside your browsing history. Add a few query
aliases, and those collections become easy to reach: `!bm deadlock` for your
bookmarks, `!docs timeout` for reference material, `!notes migration` for your
own notes.

Since the [earlier post about import commands](/posts/new-import-command-structure-and-sources),
we have added browser bookmark imports, Raindrop.io, Readeck, sitemap imports,
and continuous file imports. Here are some ways to put them to use.

The examples below send documents to your configured Hister server. If you have
not set one up yet, start with the [quickstart guide](/docs/quickstart).

## Import Your Bookmarks

To import them, run:

```bash
hister import browser bookmarks
```

Hister detects supported bookmark stores on your machine and lets you choose
which to import. Firefox and Chromium families are supported, including browsers
such as Zen, Brave, and Vivaldi, as well as Ladybird. You can also select a
browser explicitly:

```bash
hister import browser bookmarks --browser firefox
```

Use `--db PATH` to select a particular profile's bookmark store. This is useful
when you keep work and personal browsing in separate profiles. Add a different
`--label` to each import if you want to search them separately later.

Imported pages receive the `bookmarks` label by default. You can search their
contents immediately:

```textplain
label:bookmarks deadlock
```

For repeated searches, open **Rules**, find **Search aliases**, and add `!bm`
in **Keyword** with `label:bookmarks` in **Expands to**. Now `!bm deadlock` runs
the query above. The `!` prefix keeps the shortcut distinct from ordinary words
you might search for.

## Add a Site Through Its Sitemap

To add a documentation site or blog archive, point Hister at its sitemap and
give the collection a label:

```bash
hister import sitemap https://docs.example.com/sitemap.xml --label reference
```

Sitemap indexes and gzip compressed files are supported. The import stays
within the listed URLs, so links to other sites do not expand the crawl.

Create an alias `!docs` that expands to `label:reference`, then use queries such
as `!docs timeout`. You can reuse the same label for several documentation sites
to search your reference collection with one shortcut.

Existing documents are skipped by default. Add `--force` when you want to fetch
fresh copies. Bookmark and sitemap page imports both use persistent crawl jobs.
To inspect failures or resume an interrupted job, use the ID printed by the
import:

```bash
hister crawl errors JOB_ID
hister index --job-id JOB_ID
```

Resume from the same local configuration that created the job. For sitemaps,
discovery must finish before the resumable page job is created. Supply any
crawler or ownership overrides again when resuming.

Sitemap imports were added after v0.20.0. At the time of writing, this example
requires a [rolling build or a build from source](/docs/installing).

## Bring Your Reading Lists

Raindrop.io and Readeck have joined the existing import sources: Linkding,
Linkwarden, Karakeep, Shaarli, and wallabag. You can keep using these tools to
organize your reading while making their content searchable through Hister.

For Raindrop, a CSV export is a simple starting point:

```bash
hister import raindrop --input Raindrop.io-Export.csv
```

You can also import directly from your account with a Raindrop API token:

```bash
export HISTER_IMPORT_RAINDROP_TOKEN='your-raindrop-token'
hister import raindrop
```

The API import preserves notes, highlights and their annotations, tags, and
collection paths, and downloads the linked pages. Notes and highlights become
searchable alongside the page text, so you can search for your own annotations
as well as the author's words.

If a linked page is no longer available, Hister still imports the Raindrop
bookmark details and reports the download failure.

When repeating a Raindrop import, add `--skip-existing` to avoid downloading
pages already in Hister. Leave it off when you want to refresh existing content
or changed bookmark details.

Readeck imports use the article content already saved in your instance:

```bash
export HISTER_IMPORT_READECK_TOKEN='your-readeck-token'
hister import readeck https://readeck.example.com
```

This can bring an older article into Hister even when the original website no
longer serves it. If Readeck has no usable saved content, Hister tries the
original URL. Repeating the import picks up new and updated bookmarks through
Readeck's sync API.

To search both services with one shortcut, create `!read` with this expansion:

```textplain
metadata.source:(raindrop|readeck)
```

Then `!read deadlock` searches both collections. Service imports retain their
source in metadata, so this alias still works if you change their labels to
group them by project. Add other imported services to the alternatives as
needed, for example `metadata.source:(raindrop|readeck|wallabag)`.

The source service tokens in these examples are separate from the token used
to connect to Hister. The [import documentation](/docs/import) covers credentials
and options for each service. Imports leave the source collections in place,
and deleting an item from a service does not automatically delete its Hister
copy.

## Keep Your Notes Searchable

If Hister runs on another machine, run the import on the computer that holds
your files:

```bash
hister import file --watch --source laptop --label notes ~/notes
```

Hister extracts the content locally and sends searchable snapshots to the
server. With `--watch`, the command performs an initial scan and keeps importing
new and changed files until you stop it. Removing a source file leaves its
previously imported snapshot in Hister.

Keep `--source laptop` stable between runs so updates replace earlier snapshots,
and use a different source name for each machine. Set up `!notes` to expand to
`label:notes` for searches such as `!notes migration`.

When the server can already access your files, use
[local directory indexing](/docs/configuration#local-directory-indexing) to have
it scan and watch them directly.

## Create Shortcuts for Projects

An alias can combine several filters. Suppose you want a shortcut for GitHub
pages among your imported bookmarks. Add `!code` with this expansion:

```textplain
label:bookmarks domain:github.com
```

Searching `!code retry` now limits results to bookmarked GitHub pages containing
`retry`. Use the same pattern for a documentation host or an internal wiki you
return to often.

For a project with references spread across sources, choose a project label
when importing. For example, these commands put a site's documentation and a
local notes directory under `atlas`:

```bash
hister import sitemap https://docs.example.com/sitemap.xml --label atlas
hister import file --watch --source laptop --label atlas ~/projects/atlas/notes
```

Add `!atlas` with the expansion `label:atlas`. Now `!atlas deployment` searches
both sources. The label replaces each import's default label, so use it when
project scope is more useful than the source grouping.

Choose shortcuts for the searches you repeat. A few collection aliases and one
or two project aliases are enough to make imports part of your daily workflow.
The [alias documentation](/docs/rules#aliases) and
[query reference](/docs/query-language) cover more combinations.
