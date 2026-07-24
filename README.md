# Inca

[![SEGV 
LICENSE](https://img.shields.io/static/v1?label=SEGV%20LICENSE&message=1.1&labelColor=0060A8&color=ffffff)](https://xn--gckvb8fzb.com/segv/)

[<img src="https://xn--gckvb8fzb.com/images/chatroom.png" width="275">](https://xn--gckvb8fzb.com/contact/)

A command line client for CalDAV and CardDAV. Inca connects to a CalDAV/CardDAV
server and synchronizes your calendars, tasks and contacts for offline use.

Inca is the spiritual successor to [addrb][addrb] and [caldr][caldr].

[addrb]: https://github.com/mrusme/addrb
[caldr]: https://github.com/mrusme/caldr

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

Inca discovers the account principal with `DAV:current-user-principal` rather
than guessing a path, which most servers support, and falls back to
`/principals/{username}/` when discovery is refused.

Changes are pulled with a `sync-collection` report (RFC 6578), which is how Inca
gets both the initial state and later deltas, along with a sync token to store.
A server that does not support the report makes Inca fall back to a
`calendar-query` filtered on `VCALENDAR` and an `addressbook-query` requesting
all properties, the widely implemented reports from RFC 4791 and RFC 6352. A
stored token the server rejects as stale makes Inca sync the collection from
scratch. The CardDAV sync report names the contacts that changed without their
data, so Inca follows it with a multiget to fetch them.

Inca is tested against [Radicale][radicale], against [Baïkal][baikal], against
an in-memory go-webdav server, and against _Fastmail_.

[radicale]: https://radicale.org
[baikal]: https://sabre.io/baikal/

However, the officially supported server-side for Inca is [Maya][maya].

[maya]: https://tty.fail/mrus/maya

## Development

TODO
