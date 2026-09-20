# Inca

[![SEGV 
LICENSE](https://img.shields.io/static/v1?label=SEGV%20LICENSE&message=1.1&labelColor=0060A8&color=ffffff)](https://xn--gckvb8fzb.com/segv/)

[<img src="https://xn--gckvb8fzb.com/images/chatroom.png" width="275">](https://xn--gckvb8fzb.com/contact/)

A command line client for CalDAV and CardDAV. Inca connects to a CalDAV/CardDAV
server and synchronizes your calendars, tasks and contacts for offline use.

Inca is the spiritual successor to [addrb][addrb] and [caldr][caldr].

[addrb]: https://github.com/mrusme/addrb
[caldr]: https://github.com/mrusme/caldr

**Note:** Inca depends on [Maya's][maya] hard-fork of `go-webdav`. Maya is not
(yet) publicly available, hence unless you have been given private beta-access
to Maya you won't be able to use Inca just yet.

## Installation

Build the binary with the provided Makefile:

```sh
make build
```

The result is written to `build/inca`.

## Configuration

If you're familiar with [Zeit] you probably know the drill. Inca reads a TOML
file. By default it looks in `$XDG_CONFIG_HOME/inca/config.toml`, and the `-c`
flag or the `INCA_CONFIG` environment variable point it elsewhere. Both a plain
path and a `file://` URL are accepted.

```toml
[database]
path = "/home/you/.local/share/inca/db"

[[account]]
name = "personal"
endpoint = "https://dav.example.com"
username = "alice"
password = "secret"
```

The `[database]` entry names the directory the badger database is stored in.
When `path` is empty, Inca falls back to `$XDG_DATA_HOME/inca/db`, and the
`INCA_DATABASE` environment variable overrides both.

Each `[[account]]` entry is one server. Most servers publish CalDAV and CardDAV
under a single `endpoint`, so that one value is enough. When a provider splits
the two services, set `caldav_endpoint` and `carddav_endpoint` instead. See
[`inca.example.toml`](inca.example.toml) for a fuller example.

[zeit]: https://zeit.observer

## Usage

Synchronize every configured account into the local database:

```sh
inca sync
```

For each account Inca discovers the calendars and address books, then pulls the
changes in each collection. Events, tasks and contacts are stored together,
keyed by account and remote path, so running `sync` again updates the existing
rows rather than duplicating them. An account that offers only one of the two
services, or a server that is unreachable, produces a warning and does not stop
the other accounts from syncing.

The first sync of a collection reads everything and each object line ends with
`(sync)` when the server supports incremental synchronization and `(full)` when
Inca fell back to reading the whole collection:

```
▶ Syncing account personal ...
● Calendar Personal: 12 updated, 0 removed (sync)
● Address book Contacts: 34 updated, 0 removed (sync)
```

On later runs Inca sends the sync token it stored last time, so the server
returns only what changed. Objects the server reports as deleted are removed
from the local database.

### Hosts

Inca sends the credentials of an account to the host of its endpoint, but some
providers however keep the data on a different host. iCloud, for example, refers
from `caldav.icloud.com` to a partition host such as `p42-caldav.icloud.com`.

Whenever this happens in a terminal, `sync` shows a prompt:

```
▲ caldav.icloud.com refers to p42-caldav.icloud.com for the data of this account.
  Send the credentials of personal there?
  [n] no   [h] this host   [d] every host under icloud.com
  >
```

`h` approves the one host and `d` every host under the domain. This is a safety
measure to protect your credentials from being sent to other systems.

Inca stores the approval for that account in its database and doesn't show the
prompt for it again in the future, unless the database is ever cleared/deleted.

Without a terminal (e.g. in a cron job) the host is rejected by default and the
sync aborts. You cen prevent this by using the `--trust-host` flag that takes
the account and a specific host (e.g. `p42-caldav.icloud.com`), or the account
and a wildcard (e.g. `*.icloud.com`).

**Note:** Inca never follows a reference from `https` to `http`, regardless of
whether it was approved or not.

### Accounts

List the configured accounts with the services Inca discovered for them:

```sh
inca accounts list
```

Inca stores the endpoint and the principal it discovered and skips the discovery
on later runs. It discovers again on its own when the stored endpoint returns a
404, a 405 or a 410, or when explicitly requested:

```sh
inca accounts discover personal
```

Without an account, `discover` goes through all of them.

To view and change the approved hosts use the following commands:

```sh
inca accounts hosts list
inca accounts hosts trust personal '*.icloud.com'
inca accounts hosts forget personal '*.icloud.com'
```

The approvals are in the database and if you delete the database, Inca shows the
prompts again.

### Contacts (a.k.a. _People_)

List every synced contact, sorted by name:

```sh
inca people list
```

The `people` command is also reachable as `person`, `p`, `contacts` and
`contact`, and `list` as `ls` and `l`. Running `inca people` with no subcommand
lists as well.

Search contacts with `find`:

```sh
inca people find tom ato
```

Each argument is a search term, and a contact is shown when every term is a
substring of any of its attributes. The terms may match different attributes, so
`tom ato` finds a contact named _Tom Ato_ as well as a _Tom_ whose address
contains _ATO Building_. The `find` command is also reachable as `fd` and `f`.

Both take `-f json` for machine-readable output.

### Version

Print version information:

```sh
inca version
```

### Flags

Every command takes `--color` (`always`, `auto`, `never`) and `--debug`.

## Compatibility

Inca finds the service the way RFC 6764 describes it. For an endpoint with a
path it asks that path first, then the well-known URI of the host, then `/`, and
it reads the account principal from `DAV:current-user-principal`. An account
without an endpoint is discovered through DNS from the domain of its username,
provided the username is an address.

Changes are pulled with a `sync-collection` report (RFC 6578), which is how Inca
gets both the initial state and later deltas, along with a sync token to store.
If a server does not support the report, Inca falls back to listing the ETags
and fetching what changed with a multiget. A stored token the server rejects as
stale will trigger a full sync of the collection from scratch, without
downloading what it already has. The sync report asks for the data of every
changed object, and if a server returns a changed object without its data, Inca
fetches that object with a multiget.

Inca is tested against [Radicale][radicale], against [Baïkal][baikal], against
[Nextcloud][nextcloud], against an in-memory go-webdav server, and against
Fastmail.

[radicale]: https://radicale.org
[baikal]: https://sabre.io/baikal/
[nextcloud]: https://nextcloud.com

However, the officially supported server-side for Inca is [Maya][maya].

[maya]: https://tty.fail/mrus/maya

## Development

TODO
