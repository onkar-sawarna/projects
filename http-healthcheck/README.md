# http-healthcheck

Small CLI that sends HTTP requests to one or more URLs and reports **OK** when the response status is in an allowed set (default **200**). Useful for scripts, cron, or quick manual checks.

Built with [Cobra](https://github.com/spf13/cobra).

## Requirements

- Go **1.26**+ (see `go.mod`)

## Build

```bash
cd http-healthcheck
go build -o http-healthcheck .
```

Run `./http-healthcheck --help` for flags and usage.

## Usage

```text
http-healthcheck [flags] <url> [url...]
```

Only **`http://`** and **`https://`** URLs are accepted.

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--timeout` | `-t` | `10s` | Per-request timeout |
| `--method` | `-m` | `GET` | HTTP method (e.g. `GET`, `HEAD`) |
| `--expect` | `-e` | `200` | Comma-separated acceptable status codes (e.g. `200,204`) |
| `--insecure` | `-k` | off | Skip TLS certificate verification (see below) |

Long options use **double dash** (`--timeout`). Use **`-t`**, not a single `-timeout` (pflag treats that as shorthand letters).

### TLS and `-k`

By default the client **verifies** TLS certificates. Use **`-k`** or **`--insecure`** only when you must hit endpoints with self-signed or otherwise untrusted certs (e.g. local dev). Prefer **`--insecure=false`** or omit `-k` for normal verification.

The forms **`-k true`** and **`-k false`** as two separate words are supported and match **`-k=true`** / **`-k=false`**.

### Examples

```bash
# Single URL, default GET + expect 200
./http-healthcheck https://example.com

# Multiple URLs and status codes
./http-healthcheck -e 200,204 https://example.com https://httpbin.org/status/204

# HEAD request, 5s timeout
./http-healthcheck -t 5s -m HEAD https://example.com

# Flags may appear after URLs; use --timeout or -t
./http-healthcheck https://example.com --timeout 5s
```

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | All URLs passed (status in `--expect`, no transport error) |
| `1` | Any check failed, invalid URL/expect, or other error |

## Tests

From this directory (where `go.mod` lives):

```bash
go test -short ./...    # fast, no outbound network
go test ./...           # includes real HTTP checks (needs network)
```

Skip network tests without `-short`:

```bash
HTTP_HEALTHCHECK_SKIP_NET=1 go test ./...
```

## Run from a parent folder

If your shell is in a directory **without** `go.mod`, either `cd` into `http-healthcheck` or:

```bash
go -C /path/to/http-healthcheck test ./...
```
