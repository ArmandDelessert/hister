---
date: '2026-03-06T10:59:19+01:00'
draft: false
title: 'Importing Browser History'
---

You may want to index pages that you have visited before you've ever installed Hister.
The most obvious way to do so would be to re-visit them after setting up Hister in your browser; but because that's terribly tedious, Hister provides a mechanism to import your browsing history in bulk.

## Caveats

- Safari [is not supported yet](https://github.com/asciimoo/hister/issues/49); we welcome [contributions] that add support!
- Since browsing history only stores URLs and not contents, the command-line tool will need to fetch the contents of those pages.
  The command-line tool is more limited than a browser in a few ways:
  - It cannot run JavaScript, so dynamic elements will not load (and some sites are broken enough to be _completely empty_ without it)
  - Since the requests are made by an automated program and not a human, some sites will trip their anti-bot protections.
    Most of the time they merely refuse to serve you the page, but sometimes this can go further: out of the ~42000 URLs I imported, _one_ site decided to block my household, and even then the ban was lifted on its own the following day.
- The process can take a while: the aforementioned 42000 URLs took four hours on a decent connection (though [there are plans to improve this](https://codeberg.org/asciimoo/hister/issues/11)).
  Thankfully, it is perfectly fine to interrupt and then resume the process later!

## Overview

The procedure has two steps: first, you must locate where your browser stores its history (so the Hister client can process it); then, the client makes requests.

## Locating the History

The history is stored differently for each combination of browser and operating system.

Unfortunately, there doesn't seem to be ways to extract history out of mobile phones; consider using [Firefox Sync](https://www.firefox.com/en-GB/features/sync/), a Google account... to sync history to a computer, and proceed from the latter.

<details><summary><b>Firefox</b></summary>

Firefox supports separate [profiles](https://support.mozilla.org/en-US/kb/profiles-where-firefox-stores-user-data), which each have their own history.
Follow the "How do I find my profile?" procedure on that page to get the profile's directory; inside this directory is a `places.sqlite` file, which contains your history.

Examples: (note that some parts _will_ be different for you!)

- **Linux**: `/home/samantha/.mozilla/firefox/xm5axf8v.default-release`, to which you append `/places.sqlite`
- **Windows**: `C:\Users\Samantha\AppData\Roaming\Mozilla\Firefox\Profiles\6c3u6a3w.default-release`, to which you append `\places.sqlite`

You can also attempt to locate the file manually, patterning after the above paths.
(On Windows, the `AppData` directory is typically hidden; you should be able to access it by entering `%APPDATA%` into the file explorer's location bar.)

</details><details><summary>
	<b>Google Chrome</b> (and derivatives, like <b>Edge</b>, <b>Vivaldi</b>...)
</summary>

The file you're looking for is called `History`.

Examples for Chrome: (note that some parts _will_ be different for you!)

- **Windows**: `C:\Users\Samantha\AppData\Local\Google\Chrome\User Data\Default\History`

</details>

## Importing the History

Run `hister import <browser> <path>`, where `<browser>` is either `firefox` or `chrome` (depending on which section above you followed), and `<path>` is the path determined in the previous step.

This will print a count of how many (unique) URLs have been detected, and ask for confirmation before proceeding (press Enter to submit your choice, `Y` being the default).
Note that Hister doesn't print URLs it skips importing, which can happen if it is covered by a [skip rule] (TODO) or has already been indexed previously.

It is okay to interrupt the importing process in any way!
Since URLs previously indexed are not fetched again, it is possible to re-run the `hister import` command later, and it will roughly resume from where it left off.
(Pages that failed to be fetched won't be indexed on the server, so a new attempt will be made to fetch those.)

### Warnings

A lot of things can go wrong during the importing process!
In fact, it is rare for every page to be indexed without issues.
Note that only messages printed as `| ERROR |` are serious; messages printed as `| WARN  |` are mostly benign, and the most common are explained below.
(Unfortunately, due to a limitation of our logging library, even mere warnings print `error="..."` in red. [We hope to improve this](https://codeberg.org/asciimoo/hister/issues/11#issue-3701546) eventually, [contributions] are welcome!)

- **Failed to extract content**: This indicates that one of the [heuristics] Hister employs to extract the _most significant_ content out of a Web page has failed.
  This is benign by itself; though if all extractors fail, then this will also generate a "Failed to index URL" warning, mentioning `failed to process document: no extractor found`.

  In particular, this can happen for pages that use JavaScript to load _all_ of their content.

- **Failed to index URL**: This means that the Web page cannot be indexed by the Hister tool.
  This, in turn, can have a _ton_ of causes; check the `error=` field against the following list:
  - **failed to process document: no extractor found**: See `Failed to extract content` above.
  - **invalid response code: XXX**: This means that fetching the page failed, and the `XXX` code contains _some_ information as to why.
    You can look up the error code on http://http.cat and click the corresponding cat picture for a succinct explanation; follow the `Source:` link at the bottom for more technical information.
  - **failed to download file**: This means that the page couldn't be fetched because there was an error communicating with that Web server.
    The rest of the error message may have more details, but there's generally little you can do short of trying again.
  - **failed to send page to hister**: This means that the packaged-up page contents failed to reach the Hister server.
    This generally means that there was an error communicating with the server, except when the details contain `406 Not Acceptable`, which instead means that the server declined to add that page to the index (usually because Hister refuses it index its own pages 😛).

- **Failed to download favicon**: The [favicon] is the little icon shown in your browser tabs; Hister uses it to help illustrate search results.
  This error means that it failed to be fetched, but this is benign.

Any pages that fail to be imported like this, you can try visiting in your browser (if you're using the extension); this can succeed where the CLI tool failed.

[contributions]: https://github.com/asciimoo/hister#readme
[heuristics]: https://en.wikipedia.org/wiki/Heuristic
[favicon]: https://en.wikipedia.org/wiki/Favicon
