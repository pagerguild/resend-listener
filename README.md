# resend-listener

A small Go utility that polls [Resend](https://resend.com) for received emails and writes them to a local inbox directory as RFC 5322 `.eml` files.

## Install

```
go install github.com/pagerguild/resend-listener@latest
```

Or grab a binary from [Releases](https://github.com/pagerguild/resend-listener/releases).

## Usage

```
export RESEND_API_KEY=re_...
resend-listener [flags]
```

`RESEND_API_KEY` must be set in the environment. You can get one from the [Resend dashboard](https://resend.com/api-keys).

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-prefix` | | Filter recipients by address prefix |
| `-domain` | | Filter recipients by domain |
| `-path` | `./inbox` | Inbox directory path |
| `-since` | `0s` | Look back duration (e.g. `10000h` for ~1 year) |
| `-no-clear` | `false` | Don't empty the inbox directory on startup |
| `-no-create` | `false` | Fail if the inbox directory doesn't exist |

### Examples

Listen for all new emails:
```
export RESEND_API_KEY=re_...
resend-listener
```

Listen for emails to `alerts-*@example.com` and grab the last ~30 days:
```
resend-listener -prefix alerts- -domain example.com -since 720h
```

### File naming

Files are written as `YYYYMMDD-HHmmSS.eml`. If multiple emails share a timestamp, a numeric suffix is appended (e.g. `20260312-1532051.eml`).

## How it works

The listener polls the Resend receiving API every 5 seconds. Only emails with a `created_at` after the listener's start time (minus `-since`) are saved. When a raw email download URL is available from Resend, the original RFC 5322 message is fetched directly; otherwise one is constructed from the email fields.

On startup the inbox directory is created (unless `-no-create`) and cleared (unless `-no-clear`).
