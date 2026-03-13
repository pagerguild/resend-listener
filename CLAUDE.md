# Resend Listener

A small Go utility that listens to received emails within a resend account and
writes the emails to an inbox directory in RFC 5322 format.  See rfc5322.txt.

## Command-line arguments

```
resend-listener -p <prefix> -d <domain> -p <inbox_path> [--no-clear] [--no-create]
```

### Options

* **-p/--prefix** filters recipients having the given prefix on their address.
* **-d/--domain** filters for recipients at the given domain.
* **-p/--path**   inbox path.  Defaults to ./inbox.
* **--no-clear**  do not clean out inbox on startup.  Normal operation is to empty the inbox directory if it exists.
* **--no-create** fail if the inbox directory does not exist.  Normally it will be created on startup.

### Environment Variables

* RESEND_API_KEY -- MUST be present.

## File Names

Files will be `YYYYMMDD-HHmmSS[X].eml`, meaning that subsequent messages arriving
with the same timestamp will be given a sequential suffix to separate them.

## Libraries

Use github.com/resend/resend-go as the base client.

```
go get github.com/resend/resend-go/v3
```

See [this](https://github.com/resend/resend-go/raw/refs/heads/main/examples/receiving.go)
for an example of receiving emails.

There is a RESEND_API_KEY in your environment.  Don't leak it.  Just use it.

## Go Version

Go version is 1.26.1.  That is CORRECT.  DO NOT Downgrade Go or any libraries.
ALWAYS check for latest version of any library before installing or downgrading.

