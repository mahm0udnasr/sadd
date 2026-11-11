# sadd

**sadd** blocks distracting websites on Linux by managing its own small
section of `/etc/hosts`. It's a single binary with no daemon and no database.

```console
$ sudo sadd block youtube.com --for 2h
Blocked until Tue 23:15: youtube.com
Auto-unblock scheduled (systemd timer sadd-expire-20261110-231500-4242).

$ sudo sadd block --group social
$ sadd list
youtube.com    until Tue 23:15 (1h59m left)
facebook.com   forever
instagram.com  forever
…
```

## Features

- **`block` / `unblock` / `list`** take domains or full URLs:
  `https://www.YouTube.com/watch?v=…` becomes `youtube.com`.
- **Timed blocks:** `--for 30m`, `--for 2h`, `--for 1d`. A systemd timer
  removes the block automatically, even if the computer restarts in between.
- **Groups:** `--group social`, `video`, `news`, `shopping`. Run
  `sadd groups` to see what each one contains.
- **`check`** asks the system resolver (the one your browser uses) whether a
  site really resolves to `0.0.0.0` / `::`.
- **Interactive TUI:** just run `sudo sadd` (or `sudo sadd tui`). It has tabs for blocked sites and groups.
  Changes happen in centered dialogs, and adding sites has a live preview.
  `enter` shows a site's details, `space` selects several sites to unblock or extend at once, and
  `e` adds time to a timed block. Any duration picker has a **Custom…** option (`45m`, `1h30m`, `2d`).
  Without sudo it opens read-only.
- **Safe by design:**
  - Lines outside its own section are never touched.
  - Every write is atomic.
  - A backup is saved before each change, and `sadd restore` undoes the last one.

## Requirements

- Linux with systemd. It's built and tested on Ubuntu, and uses `systemd-resolved`
  to flush the DNS cache.
- Go 1.27 or newer, to build it (see `go.mod`).

## Install

```bash
git clone https://github.com/mahm0udnasr/sadd.git
cd sadd
make                 # build as your normal user
sudo make install    # copies to /usr/local/bin and enables the boot service
```

`make install` also enables `sadd-expire.service`. At boot, this service removes
timed blocks that ended while the computer was off, and re-creates timers for
the ones still running.

Uninstall:

```bash
sudo sadd unblock $(sadd list | awk '{print $1}')   # optional: remove your blocks first
sudo make uninstall
```

## Usage

| Command | What it does |
|---|---|
| `sudo sadd block <domain>...` | Block one or more sites |
| `sudo sadd block <domain> --for 2h` | Block for a while (`30m`, `1h30m`, `1d`; minimum `1m`) |
| `sudo sadd block --group social` | Block a built-in group (`-g` for short, can be repeated) |
| `sudo sadd unblock <domain>...` | Unblock sites (also takes `--group`) |
| `sadd list` | Show what is blocked and for how long (no sudo needed) |
| `sadd groups` | Show the built-in groups |
| `sudo sadd restore` | Undo the last change (run it again to redo) |
| `sadd check [domain]...` | Ask the system what a site resolves to; exit code 1 if any is not blocked |
| `sudo sadd` | Interactive screen (same as `sadd tui`): Blocked · Groups (`tab` or `1`/`2` to switch) |

Each domain is blocked for IPv4 and IPv6, with and without `www.`:

```
0.0.0.0 youtube.com
:: youtube.com
0.0.0.0 www.youtube.com
:: www.youtube.com
```

> **Subdomains are not blocked automatically.** `/etc/hosts` has no
> wildcards, so blocking `youtube.com` does not block `m.youtube.com`.
> Block it explicitly, or use a group, which already includes common ones.

## How it works

sadd keeps everything between two markers in `/etc/hosts`:

```
127.0.0.1 localhost          ← your lines: never modified
# >>> sadd start
# until 2026-11-10T23:15:00+02:00 youtube.com
0.0.0.0 youtube.com
:: youtube.com
0.0.0.0 www.youtube.com
:: www.youtube.com
# <<< sadd end
```

- **No database.** The hosts file is the only source of truth. A timed
  block's end time is stored as a comment inside the sadd section.
- **Timed blocks** create a systemd transient timer that runs `sadd expire`.
  It doesn't unblock a specific site: it removes only entries whose end time
  has passed. So if you change your mind and block a site forever, the
  pending timer leaves it alone.
- **Atomic writes.** sadd writes to a temp file in `/etc`, calls `fsync`, then
  `rename`s it over `/etc/hosts`. The file is never half-written, and its
  permissions and owner are kept.
- **Backup.** Before each change, the current file is copied to `/etc/hosts.bak`.
  `sadd restore` swaps the two files.
- **Refuses damaged files.** If the markers are broken (for example a start
  without an end), sadd stops with an error instead of guessing.
- **DNS flush.** After every change sadd runs `resolvectl flush-caches`.

## Testing without touching /etc/hosts

Point sadd at any file with `HOSTS_FILE`. No sudo is needed for a file you own:

```bash
cp /etc/hosts /tmp/hosts
HOSTS_FILE=/tmp/hosts sadd block youtube.com
HOSTS_FILE=/tmp/hosts sadd
```

`sudo` drops environment variables. If you really need both, write
`sudo HOSTS_FILE=/tmp/hosts sadd ...`.

Run the test suite with:

```bash
make test
```

## Limitations

- **Browser caches.** A browser that already has the site open keeps its
  connections, and browsers keep their own DNS cache. Some use DNS-over-HTTPS,
  which ignores `/etc/hosts`. See below.
- **Willpower not included.** Anyone with sudo can undo a block. sadd is a
  speed bump, not a lock.

## A blocked site still loads?

1. Run `sadd check example.com`.
   - **✗ not blocked:** it isn't in the list. Run `sudo sadd block example.com`.
   - **✗ in sadd's list, but…:** something before sadd wins. Look for an
     earlier line for the same name in `/etc/hosts`, and make sure the `hosts:`
     line in `/etc/nsswitch.conf` contains `files`.
   - **✓ blocked:** the system is fine. The browser is the problem (step 2).
2. Close the browser completely (every window) and open it again. Or clear
   its cache: `chrome://net-internals/#dns` → *Clear host cache*, then
   `#sockets` → *Flush socket pools* (Chrome, Brave, Edge), or
   `about:networking#dns` → *Clear DNS Cache* (Firefox).
3. Turn off *Secure DNS* / *DNS over HTTPS* in the browser settings.

## Project layout

```
cmd/               Cobra commands (CLI only: parse input, print output)
internal/hosts/    parse, render and atomically write the hosts file
internal/groups/   built-in site groups
internal/timer/    durations and systemd timers
internal/dns/      DNS cache flush and system lookups (getent)
internal/tui/      Bubble Tea interface
packaging/         systemd unit installed by "make install"
testdata/          sample hosts file for tests
```

## License

[MIT](LICENSE)
