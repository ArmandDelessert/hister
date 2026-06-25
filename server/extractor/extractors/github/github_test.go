package github

import (
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
	// Metadata checks.
	if d.Metadata["description"] != "Your own search engine" {
		t.Errorf("Metadata[description] = %v, want Your own search engine", d.Metadata["description"])
	}
	if d.Metadata["topics"] != "search, go, golang, search-engine, privacy, web, mcp, history, index, semantic-search, browser-history, personal-search, personal-search-engine, mcp-server" {
		t.Errorf("Metadata[topics] = %q", d.Metadata["topics"])
	}
	if d.Metadata["languages"] != "Go, Svelte, TypeScript, Shell, CSS, Nix" {
		t.Errorf("Metadata[languages] = %q", d.Metadata["languages"])
	}
	if d.Metadata["stars"] != "1255" {
		t.Errorf("Metadata[stars] = %q", d.Metadata["stars"])
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
	  <div data-testid="markdown-body">
		<h1 dir="auto">This is a meta issue raising awareness to contribute to existing extractors or add new ones</h1>
		<p dir="auto">[...]</p>
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
	// Metadata checks.
	if d.Metadata["type"] != "Issue" {
		t.Errorf("Metadata[issue] = %v, want Issue", d.Metadata["issue"])
	}
	if d.Metadata["title"] != "Extractors wanted!" {
		t.Errorf("Metadata[title] = %v, want Extractors wanted!", d.Metadata["title"])
	}
	if d.Metadata["dateOpened"] != "2026-04-09T07:47:32.000Z" {
		t.Errorf("Metadata[title] = %v, want 2026-04-09T07:47:32.000Z", d.Metadata["dateOpened"])
	}
	if d.Metadata["comments"] != "hey bruhh i like to work on reddit post extractor..., Thanks bro, is there any deadline for this ???, @dinzz005 No deadlines" {
		t.Errorf("Metadata[comments] = %s, want hey bruhh i like to work on reddit post extractor..., Thanks bro, is there any deadline for this ???, @dinzz005 No deadlines", d.Metadata["comments"])
	}
}
