# Security Policy

## Reporting a vulnerability

Please report security issues privately — do **not** open a public issue for a
vulnerability.

- **Preferred:** GitHub [private vulnerability reporting](https://github.com/Liqhtworks/sophon-sdk-go/security/advisories/new)
  (Security tab → "Report a vulnerability").
- **Email:** `support@sophon.rs`

We aim to acknowledge reports within a few business days. Please include a
description, affected version, and a reproduction if possible.

## Supported versions

This SDK is pre-1.0. Only the latest released `v0.1.x` line receives security
fixes. Pin to the `v0.1` minor line and upgrade promptly:

```bash
go get github.com/Liqhtworks/sophon-sdk-go@v0.1
```

## Trust boundary

This package is a **server-side** client for the SOPHON Encoding API. It is not
meant to run in untrusted clients (browsers, mobile apps, end-user binaries).

- **Credential:** a SOPHON API key (`xt_live_...`). Keep it on the server, in an
  environment variable or secret manager — never in client apps, public repos,
  logs, or analytics. The SDK sends it only as an `Authorization: Bearer` header
  over HTTPS; it is never placed in a URL.
- **Transport:** the default base URL is `https://api.liqhtworks.xyz`. TLS
  verification uses Go's defaults and is never disabled by the SDK.
- **Idempotency:** all job/upload create + complete calls carry an
  `Idempotency-Key`.
- **Webhooks:** `helpers.VerifyWebhookSignature` performs a constant-time
  HMAC-SHA256 check over `"{timestamp}.{raw_body}"` with a 5-minute replay
  window. Always verify inbound webhooks before acting on them, and bound the
  request body size (e.g. `http.MaxBytesReader`) before reading it.
- **Downloads:** encoded outputs are fetched from a presigned Backblaze B2 URL;
  the download helper follows at most one redirect and only to allowlisted
  hosts (`helpers.AllowedDownloadHosts`).

## Rotating an API key

1. Create a new key at <https://sophon.rs/account/general> → **API keys**.
2. Roll it out to your servers (update the secret manager / environment).
3. Revoke the old key from the same page once traffic has moved over.

Keys are shown only once at creation. Treat key exposure as a billing-impacting
incident — revoke and rotate immediately.

> **Note for maintainers:** this repository is generated from
> [`Liqhtworks/sophon-api`](https://github.com/Liqhtworks/sophon-api). Persist
> changes to this file in the upstream publish pipeline so they survive
> regeneration.
