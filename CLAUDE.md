# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

A Go CLI that polls the Resend receiving API every 5 seconds and writes incoming emails as RFC 5322 `.eml` files into an inbox directory. Intended to back automated test flows that need to consume real emails (verification codes, magic links, etc.).

## Commands

```
go build ./...           # build the binary (./resend-listener)
go run . [flags]         # run from source
go vet ./...             # static checks
go mod tidy              # sync deps
```

There is currently no test suite. `RESEND_API_KEY` must be in the environment for the binary to run.

## Architecture

The whole program is a single file (`main.go`, ~300 lines, package `main`). The shape:

1. **Flag parsing & defaults** — `-prefix` uses a sentinel (`"\x00"`) so an explicit empty string disables filtering, while an unset flag falls back to `defaultPrefix()` (git origin basename + `$USER`, joined with `-`). This three-state behavior is load-bearing for the README contract; preserve it if you touch flag handling.
2. **Inbox setup** (`setupInbox`) — creates or clears `./inbox` per `-no-create` / `-no-clear`.
3. **Filter advertisement** (`writeFilterFile`) — writes `<inbox>/filter.txt` with the current pattern (e.g. `myapp-tyler*@example.com`). External tools (see `llms.txt`) read this to know which addresses will be captured. Keep `filter.txt` in sync if filter semantics change.
4. **Poll loop** — 5-second ticker calls `poll()`, which lists emails via `client.Emails.Receiving.ListWithContext`, filters by `created_at > lastChecked` and `matchesFilter`, then fetches each match individually.
5. **Two write paths** in `poll()`:
   - If `full.Raw.DownloadUrl` is set, download the original RFC 5322 bytes verbatim (preferred, preserves MIME structure).
   - Otherwise reconstruct headers + body via `buildRFC5322` from parsed fields. This path is lossy (single-part text, no attachments) — only used as a fallback.
6. **Timestamp parsing** — `parseTime` tries multiple formats (`timeFormats` slice) because the Resend API has returned non-RFC3339 timestamps in the past (see commit `0c5185b`). Add new formats to the slice rather than changing existing parsers.
7. **Filenames** — `generateFilename` produces `YYYYMMDD-HHmmSS.eml`, appending an integer suffix for collisions within the same second.

The `lastChecked` watermark is advanced even when an email is filtered out, so non-matching mail does not get re-fetched on every tick.

## Constraints

- **Go version is 1.26.1 — do not downgrade.** Same for `github.com/resend/resend-go/v3` (currently v3.6.0). Check upstream for the latest version before changing either.
- **Do not leak `RESEND_API_KEY`** in logs, errors, or commits. It is read once from the environment.
- The Resend client library reference example: https://github.com/resend/resend-go/raw/refs/heads/main/examples/receiving.go
- `rfc5322.txt` in the repo root is the RFC for reference when touching `buildRFC5322`.
