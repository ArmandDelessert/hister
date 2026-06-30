package github

import (
	"strings"
	"testing"

	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/types"
)

func TestMatchGitHubURLs(t *testing.T) {
	e := &GitHubExtractor{}
	cases := []struct {
		url  string
		want bool
	}{
		// only top-level is supported
		{"https://github.com/asciimoo", false},
		{"https://github.com/asciimoo/hister", true},
		{"https://github.com/asciimoo/hister/issues", false},
		{"https://github.com/asciimoo/hister/issues/305", true},
		{"https://github.com/asciimoo/hister/pulls", false},
		{"https://github.com/asciimoo/hister/settings", false},
		{"https://github.com/topics/react-native", false},
		{"https://de.wikipedia.org/wiki/Berlin", false},
		{"https://stackoverflow.com/questions/1234", false},
		{"https://example.com/wiki/Foo", false},
	}
	for _, tc := range cases {
		d := &document.Document{URL: tc.url}
		if got := e.Match(d); got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

const minimalRepoPage = `<html>
<head><title>asciimoo/hister: Your own search engine</title></head>
<body>
<div><p class="f4">Your own search engine</p></div>
<div>
<form class="js-social-form BtnGroup-parent flex-auto js-deferred-toggler-target" data-turbo="false" action="/asciimoo/hister/unstar" accept-charset="UTF-8" method="post">
<button>
<path d="star"></path>
</svg><span data-view-component="true" class="d-inline">Starred</span>
<span id="repo-stars-counter-unstar" aria-label="1255 users starred this repository" title="1,255">1.3k</span>
</button></form>

<h3 class="sr-only">Topics</h3>
<div class="tmp-my-3">
<div class="f6">
<a href="/topics/search" title="Topic: search" data-view-component="true" class="topic-tag topic-tag-link">
search
</a>
<a href="/topics/go" title="Topic: go" data-view-component="true" class="topic-tag topic-tag-link">
go
</a>
<a href="/topics/golang" title="Topic: golang" data-view-component="true" class="topic-tag topic-tag-link">
golang
</a>
<a href="/topics/search-engine" title="Topic: search-engine" data-view-component="true" class="topic-tag topic-tag-link">
search-engine
</a>
<a href="/topics/privacy" title="Topic: privacy" data-view-component="true" class="topic-tag topic-tag-link">
privacy
</a>
<a href="/topics/web" title="Topic: web" data-view-component="true" class="topic-tag topic-tag-link">
web
</a>
<a href="/topics/mcp" title="Topic: mcp" data-view-component="true" class="topic-tag topic-tag-link">
mcp
</a>
<a href="/topics/history" title="Topic: history" data-view-component="true" class="topic-tag topic-tag-link">
history
</a>
<a href="/topics/index" title="Topic: index" data-view-component="true" class="topic-tag topic-tag-link">
index
</a>
<a href="/topics/semantic-search" title="Topic: semantic-search" data-view-component="true" class="topic-tag topic-tag-link">
semantic-search
</a>
<a href="/topics/browser-history" title="Topic: browser-history" data-view-component="true" class="topic-tag topic-tag-link">
browser-history
</a>
<a href="/topics/personal-search" title="Topic: personal-search" data-view-component="true" class="topic-tag topic-tag-link">
personal-search
</a>
<a href="/topics/personal-search-engine" title="Topic: personal-search-engine" data-view-component="true" class="topic-tag topic-tag-link">
personal-search-engine
</a>
<a href="/topics/mcp-server" title="Topic: mcp-server" data-view-component="true" class="topic-tag topic-tag-link">
mcp-server
</a>
</div>
</div>

<ul class="list-style-none">
<li class="d-inline">
<a class="d-inline-flex flex-items-center flex-nowrap Link--secondary no-underline text-small tmp-mr-3" href="/asciimoo/hister/search?l=go" data-ga-click="Repository, language stats search click, location:repo overview">
<svg style="color:#00ADD8;" aria-hidden="true" data-component="Octicon" height="16" viewBox="0 0 16 16" version="1.1" width="16" data-view-component="true" class="octicon octicon-dot-fill mr-2 tmp-mr-2">
<path d="M8 4a4 4 0 1 1 0 8 4 4 0 0 1 0-8Z"></path>
</svg>
<span class="color-fg-default text-bold mr-1">Go</span>
<span>60.0%</span>
</a>
</li>
<li class="d-inline">
<a class="d-inline-flex flex-items-center flex-nowrap Link--secondary no-underline text-small tmp-mr-3" href="/asciimoo/hister/search?l=svelte" data-ga-click="Repository, language stats search click, location:repo overview">
<svg style="color:#ff3e00;" aria-hidden="true" data-component="Octicon" height="16" viewBox="0 0 16 16" version="1.1" width="16" data-view-component="true" class="octicon octicon-dot-fill mr-2 tmp-mr-2">
<path d="M8 4a4 4 0 1 1 0 8 4 4 0 0 1 0-8Z"></path>
</svg>
<span class="color-fg-default text-bold mr-1">Svelte</span>
<span>30.9%</span>
</a>
</li>
<li class="d-inline">
<a class="d-inline-flex flex-items-center flex-nowrap Link--secondary no-underline text-small tmp-mr-3" href="/asciimoo/hister/search?l=typescript" data-ga-click="Repository, language stats search click, location:repo overview">
<svg style="color:#3178c6;" aria-hidden="true" data-component="Octicon" height="16" viewBox="0 0 16 16" version="1.1" width="16" data-view-component="true" class="octicon octicon-dot-fill mr-2 tmp-mr-2">
<path d="M8 4a4 4 0 1 1 0 8 4 4 0 0 1 0-8Z"></path>
</svg>
<span class="color-fg-default text-bold mr-1">TypeScript</span>
<span>5.4%</span>
</a>
</li>
<li class="d-inline">
<a class="d-inline-flex flex-items-center flex-nowrap Link--secondary no-underline text-small tmp-mr-3" href="/asciimoo/hister/search?l=shell" data-ga-click="Repository, language stats search click, location:repo overview">
<svg style="color:#89e051;" aria-hidden="true" data-component="Octicon" height="16" viewBox="0 0 16 16" version="1.1" width="16" data-view-component="true" class="octicon octicon-dot-fill mr-2 tmp-mr-2">
<path d="M8 4a4 4 0 1 1 0 8 4 4 0 0 1 0-8Z"></path>
</svg>
<span class="color-fg-default text-bold mr-1">Shell</span>
<span>1.1%</span>
</a>
</li>
<li class="d-inline">
<a class="d-inline-flex flex-items-center flex-nowrap Link--secondary no-underline text-small tmp-mr-3" href="/asciimoo/hister/search?l=css" data-ga-click="Repository, language stats search click, location:repo overview">
<svg style="color:#663399;" aria-hidden="true" data-component="Octicon" height="16" viewBox="0 0 16 16" version="1.1" width="16" data-view-component="true" class="octicon octicon-dot-fill mr-2 tmp-mr-2">
<path d="M8 4a4 4 0 1 1 0 8 4 4 0 0 1 0-8Z"></path>
</svg>
<span class="color-fg-default text-bold mr-1">CSS</span>
<span>1.1%</span>
</a>
</li>
<li class="d-inline">
<a class="d-inline-flex flex-items-center flex-nowrap Link--secondary no-underline text-small tmp-mr-3" href="/asciimoo/hister/search?l=nix" data-ga-click="Repository, language stats search click, location:repo overview">
<svg style="color:#7e7eff;" aria-hidden="true" data-component="Octicon" height="16" viewBox="0 0 16 16" version="1.1" width="16" data-view-component="true" class="octicon octicon-dot-fill mr-2 tmp-mr-2">
<path d="M8 4a4 4 0 1 1 0 8 4 4 0 0 1 0-8Z"></path>
</svg>
<span class="color-fg-default text-bold mr-1">Nix</span>
<span>1.0%</span>
</a>
</li>
<li class="d-inline">
<span class="d-inline-flex flex-items-center flex-nowrap text-small tmp-mr-3">
<svg style="color:#ededed;" aria-hidden="true" data-component="Octicon" height="16" viewBox="0 0 16 16" version="1.1" width="16" data-view-component="true" class="octicon octicon-dot-fill mr-2 tmp-mr-2">
<path d="M8 4a4 4 0 1 1 0 8 4 4 0 0 1 0-8Z"></path>
</svg>
<span class="color-fg-default text-bold mr-1">Other</span>
<span>0.5%</span>
</span>
</li>
</ul>
<react-app app-name="code-view" initial-path="/asciimoo/hister">
  <script type="application/json" data-target="react-app.embeddedData">{"payload":{"codeViewRepoRoute":{"path":"/","refInfo":{"name":"master","listCacheKey":"v0:1782301410.0","canEdit":true,"refType":"branch","currentOid":"6a2fdecb5b7d20f1b214fdfa8bc9bf1dcebab86e"},"tree":{"items":[{"name":".forgejo/workflows","path":".forgejo/workflows","contentType":"directory","hasSimplifiedPath":true}],"totalCount":37,"templateDirectorySuggestionUrl":null,"readme":null,"showBranchInfobar":false},"overview":{"overviewFiles":[{"displayName":"README.md","repoName":"hister","refName":"master","path":"README.md","preferredFileType":"readme","tabName":"README","richText":"<article class=\"markdown-body entry-content container-lg\" itemprop=\"text\"><div class=\"markdown-heading\" dir=\"auto\"><h1 tabindex=\"-1\" class=\"heading-element\" dir=\"auto\">Hister</h1><a id=\"user-content-hister\" class=\"anchor\" aria-label=\"Permalink: Hister\" href=\"#hister\"><svg data-component=\"Octicon\" class=\"octicon octicon-link\" viewBox=\"0 0 16 16\" version=\"1.1\" width=\"16\" height=\"16\" aria-hidden=\"true\"><path d=\"m7.775 3.275 1.25-1.25a3.5 3.5 0 1 1 4.95 4.95l-2.5 2.5a3.5 3.5 0 0 1-4.95 0 .751.751 0 0 1 .018-1.042.751.751 0 0 1 1.042-.018 1.998 1.998 0 0 0 2.83 0l2.5-2.5a2.002 2.002 0 0 0-2.83-2.83l-1.25 1.25a.751.751 0 0 1-1.042-.018.751.751 0 0 1-.018-1.042Zm-4.69 9.64a1.998 1.998 0 0 0 2.83 0l1.25-1.25a.751.751 0 0 1 1.042.018.751.751 0 0 1 .018 1.042l-1.25 1.25a3.5 3.5 0 1 1-4.95-4.95l2.5-2.5a3.5 3.5 0 0 1 4.95 0 .751.751 0 0 1-.018 1.042.751.751 0 0 1-1.042.018 1.998 1.998 0 0 0-2.83 0l-2.5 2.5a1.998 1.998 0 0 0 0 2.83Z\"></path></svg></a></div>\n<p dir=\"auto\"><strong>Your own search engine</strong></p>\n<p dir=\"auto\">Hister is a general purpose web search engine providing automatic full-text indexing for visited websites.</p>\n<div class=\"markdown-heading\" dir=\"auto\"><h2 tabindex=\"-1\" class=\"heading-element\" dir=\"auto\">Features</h2><a id=\"user-content-features\" class=\"anchor\" aria-label=\"Permalink: Features\" href=\"#features\"><svg data-component=\"Octicon\" class=\"octicon octicon-link\" viewBox=\"0 0 16 16\" version=\"1.1\" width=\"16\" height=\"16\" aria-hidden=\"true\"><path d=\"m7.775 3.275 1.25-1.25a3.5 3.5 0 1 1 4.95 4.95l-2.5 2.5a3.5 3.5 0 0 1-4.95 0 .751.751 0 0 1 .018-1.042.751.751 0 0 1 1.042-.018 1.998 1.998 0 0 0 2.83 0l2.5-2.5a2.002 2.002 0 0 0-2.83-2.83l-1.25 1.25a.751.751 0 0 1-1.042-.018.751.751 0 0 1-.018-1.042Zm-4.69 9.64a1.998 1.998 0 0 0 2.83 0l1.25-1.25a.751.751 0 0 1 1.042.018.751.751 0 0 1 .018 1.042l-1.25 1.25a3.5 3.5 0 1 1-4.95-4.95l2.5-2.5a3.5 3.5 0 0 1 4.95 0 .751.751 0 0 1-.018 1.042.751.751 0 0 1-1.042.018 1.998 1.998 0 0 0-2.83 0l-2.5 2.5a1.998 1.998 0 0 0 0 2.83Z\"></path></svg></a></div>\n<ul dir=\"auto\">\n<li><strong>Privacy-focused</strong>: Keep your browsing history indexed locally - don't use remote search engines if it isn't necessary</li>\n<li><strong>Full-text indexing</strong>: Search through the actual content of web pages you've visited</li>\n<li><strong>Advanced search capabilities</strong>: Utilize a powerful <a href=\"https://hister.org/docs/query-language\" rel=\"nofollow\">query language</a> for precise results</li>\n<li><strong>Local file indexing</strong>: Index your local knowledge base</li>\n<li><strong>Crawler</strong>: Use a (headless) browser or a traditional crawler to extend your index fast</li>\n<li><strong>Multi-user support</li></ul></article>"}]}}}}</script>
</body>`

func TestExtractRepo(t *testing.T) {
	d := &document.Document{
		URL:  "https://github.com/asciimoo/hister",
		HTML: minimalRepoPage,
	}
	e := &GitHubExtractor{}
	state, err := e.Extract(d)
	if err != nil {
		t.Fatalf("Extract error: %v", err)
	}
	if state != types.ExtractorStop {
		t.Fatalf("state = %v, want Stop", state)
	}
	if d.Title != "asciimoo/hister: Your own search engine" {
		t.Errorf("Title = %q, want %q", d.Title, "asciimoo/hister: Your own search engine")
	}
	// Text checks.
	if !strings.Contains(d.Text, "description: Your own search engine") {
		t.Error("Text should contain description")
	}
	if !strings.Contains(d.Text, "topics: search, go, golang") {
		t.Error("Text should contain topics")
	}
	if !strings.Contains(d.Text, "languages: Go, Svelte, TypeScript") {
		t.Error("Text should contain languages")
	}
	if !strings.Contains(d.Text, "Multi-user support") {
		t.Error("Text should contain README")
	}
	if !strings.Contains(d.Text, "stars: 1255") {
		t.Error("Text should contain stars")
	}

	// Metadata checks.
	if d.Metadata["repo"] != "asciimoo/hister" {
		t.Errorf("Metadata[repo] = %v, want asciimoo/hister", d.Metadata["repo"])
	}
	if d.Metadata["description"] != "Your own search engine" {
		t.Errorf("Metadata[description] = %v, want Your own search engine", d.Metadata["description"])
	}
	if d.Metadata["topics"] != "search, go, golang, search-engine, privacy, web, mcp, history, index, semantic-search, browser-history, personal-search, personal-search-engine, mcp-server" {
		t.Errorf("Metadata[topics] = %q", d.Metadata["topics"])
	}
	if d.Metadata["languages"] != "Go, Svelte, TypeScript, Shell, CSS, Nix" {
		t.Errorf("Metadata[languages] = %q", d.Metadata["languages"])
	}
}

const issuePage = `<html>
<head><title>Extractors wanted! · Issue #305 · asciimoo/hister</title></head>
<body>
<div>
<div data-component="TitleArea" data-size-variant="medium">
  <h1 data-component="PH_Title" data-hidden="false">
  <bdi data-testid="issue-title">Extractors wanted!</bdi>
  </h1>
</div>
<div>
  <div data-testid="issue-viewer-issue-container">
	<div data-testid="issue-body">
	  <div>
		<h2>Description</h2>
		<a data-component="Link" href="https://github.com/asciimoo/hister/issues/305#issue-4230456940" data-testid="issue-body-header-link">
		<relative-time class="IssueBodyHeader-module__RelativeTime__xv0lw" datetime="2026-04-09T07:47:32.000Z" title="Apr 9, 2026, 09:47 GMT+2">on Apr 9, 2026</relative-time>
		</a>
	  </div>
      <div id="issue-body-viewer">
		<div data-testid="markdown-body">
		  <h1 dir="auto">This is a meta issue raising awareness to contribute to existing extractors or add new ones</h1>
		  <p dir="auto">Extractors are modules that provide custom, content-specific document parsing or rendering functions to enhance the data quality of Hister. [...]</p>
		</div>
      </div>
	</div>
  </div>
  <div data-testid="issue-viewer-comments-container">
	<div data-testid="markdown-body">
      <p dir="auto">hey bruhh i like to work on reddit post extractor...</p>
	</div>
	<div data-testid="markdown-body">
	  <p dir="auto">Thanks bro, is there any deadline for this ???</p>
	</div>
	<div data-testid="markdown-body"
	  <p dir="auto"><a class="user-mention notranslate" data-hovercard-type="user" data-hovercard-url="/users/dinzz005/hovercard" data-octo-click="hovercard-link-click" data-octo-dimensions="link_type:self" href="https://github.com/dinzz005" aria-keyshortcuts="Alt+ArrowUp">@dinzz005</a> No deadlines</p>
	</div>
  </div>
</div>
</body>
</html>`

func TestExtractIssuePage(t *testing.T) {
	d := &document.Document{
		URL:  "https://github.com/asciimoo/hister/issues/305",
		HTML: issuePage,
	}
	e := &GitHubExtractor{}
	state, err := e.Extract(d)
	if err != nil {
		t.Fatalf("Extract error: %v", err)
	}

	if state != types.ExtractorStop {
		t.Fatalf("state = %v, want Stop", state)
	}
	if d.Title != "Extractors wanted! · Issue #305 · asciimoo/hister" {
		t.Errorf("Title = %q, want %q", d.Title, "Extractors wanted! · Issue #305 · asciimoo/hister")
	}

	// Text checks.
	if !strings.Contains(d.Text, "comments: hey bruhh i like to work on reddit post extractor..., Thanks bro, is there any deadline for this ???") {
		t.Error("Text should contain comments")
	}
	if !strings.Contains(d.Text, "This is a meta issue raising awareness to contribute to existing extractors or add new ones") || !strings.Contains(d.Text, "Extractors are modules that provide custom") {
		t.Error("Text should contain the issue body")
	}

	// Metadata checks.
	if d.Metadata["type"] != "Issue" {
		t.Errorf("Metadata[issue] = %v, want Issue", d.Metadata["issue"])
	}
	if d.Metadata["repo"] != "asciimoo/hister" {
		t.Errorf("Metadata[repo] = %v, want asciimoo/hister", d.Metadata["repo"])
	}
	if d.Metadata["title"] != "Extractors wanted!" {
		t.Errorf("Metadata[title] = %v, want Extractors wanted!", d.Metadata["title"])
	}
	if d.Metadata["date"] != "2026-04-09T07:47:32.000Z" {
		t.Errorf("Metadata[title] = %v, want 2026-04-09T07:47:32.000Z", d.Metadata["dateOpened"])
	}
}
