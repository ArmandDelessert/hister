---
date: '2026-02-13T10:59:19+01:00'
draft: false
title: 'Overview of Hister'
---

Hister works with a *client-server* architecture: a single long-running *server* program stores all of the indexed pages, and one or more *clients* connect to it to submit pages.
Note that the server *can* run on the same machine as the client!

The `hister` program can function both as a server and as a client; the Hister browser extension is solely a client.

## Why this architecture?

**Benefits**:

- Clients can be on a different machine than the server; this is especially useful so that...
- Multiple clients can connect to the same server (e.g. your phone and laptop both feed into and search from the same history)

**Drawbacks**:

- The server must be started separately from any clients, and clients can't do anything if the server isn't up
- The server is a background process that consumes some resources (this can be mitigated, however)
- Slightly more complex setup

## Privacy

Hister clients only communicate with the designated server, and the server *does not* “phone home” or share any of your browsing history with anyone else.
The source code is publicly accessible, so we can be audited by anyone who wants to check!

Hister *does not* encrypt the history data it stores.
This is only a problem if you don't trust the Hister server your clients are communicating with.

Hister only encrypts data it transfers if you use HTTPS.
This is fine if running the client and server on the same machine, but accessing the server over a network **should** be done exclusively via HTTPS and not plain HTTP.
