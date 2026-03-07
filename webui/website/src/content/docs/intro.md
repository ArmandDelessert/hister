---
date: '2026-02-13T10:59:19+01:00'
draft: false
title: 'Overview of Hister'
---

Hister works with a _client-server_ architecture: a single long-running _server_ program stores all of the indexed pages, and one or more _clients_ connect to it to submit pages.
Note that the server _can_ run on the same machine as the client!

The `hister` program can function both as a server and as a client; the Hister browser extension is solely a client.

## Why a client-server architecture?

**Benefits**:

- Clients can be on a different machine than the server; this is especially useful so that...
- Multiple clients can connect to the same server (e.g. your phone and laptop, or your Firefox and Chrome, both feed into and search from the same history)

**Drawbacks**:

- The server must be started separately from any clients, and clients can't do anything if the server isn't up
- The server is a background process that consumes some resources (few, and this can be mitigated if desired)
- Slightly more complex setup

## Privacy

Hister clients only communicate with the designated server, and the server _does not_ “phone home” or share any of your browsing history with anyone else.
The source code is publicly accessible, so we can be audited by anyone who wants to check!

If you run the Hister server on the same machine as all clients, then there are no other concerns.
However, if the Hister traffic is sent over a network, two _potential_ concerns emerge:

- Hister _does not_ encrypt the history data it stores.
  This is only a problem if you don't trust the Hister server your clients are communicating with.
- Hister only encrypts data it transfers if you use HTTPS.
  Accessing the server over a network **should** be done exclusively via HTTPS and never plain HTTP.

## Technical details

The Hister server never makes any requests itself; it expects clients to provide the (extracted) contents of the pages they want indexed, not just their URLs.
In particular, this means that pages indexed via the browser extensions aren't affected by anti-bot checks... since they aren't performed by a bot!

Indexing operations via the command-line tool (`hister index`, `hister import`...), however, are clearly bot-like (they advertise themselves as such!) so they can end up being rejected.
The offending URLs can be visited manually from the browser afterwards.
