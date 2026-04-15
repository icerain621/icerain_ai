# chromedp-demo

English: see `README.md` (this file) · 中文：见 `README.zh-CN.md`

This is an independent Go module that demonstrates and packages a production-ready crawler toolkit using `github.com/chromedp/chromedp`.

## Quick start

### Start local mock login site (`mocksite`)

```bash
go run ./cmd/mocksite -addr 127.0.0.1:18080
```

### Basic crawl (`v0`)

```bash
go run ./cmd/crawl-demo v0 -url https://example.com -out artifacts
```

### Mocksite form login (`v0` + form login)

```bash
go run ./cmd/crawl-demo v0 ^
  -allow-domains 127.0.0.1 ^
  -url http://127.0.0.1:18080/app ^
  -out artifacts_login ^
  -login-url http://127.0.0.1:18080/login ^
  -login-user-sel "#login_field" ^
  -login-pass-sel "#password" ^
  -login-submit-sel "#sign_in" ^
  -login-after-sel "#app_home" ^
  -login-user demo ^
  -login-pass demo
```

### v1: capture XHR/JSON responses (`v1-capture`)

```bash
go run ./cmd/crawl-demo v1-capture -url https://example.com -out artifacts_capture -capture-url "example\\.com"
```

### v1: login then trigger XHR on mocksite (`v1-capture`)

Start mocksite first:

```bash
go run ./cmd/mocksite -addr 127.0.0.1:18080
```

Then run v1 (login first, then trigger `fetch('/api/profile')`):

```bash
go run ./cmd/crawl-demo v1-capture ^
  -allow-domains 127.0.0.1 ^
  -url http://127.0.0.1:18080/app ^
  -out artifacts_capture_login ^
  -capture-url "/api/profile" ^
  -login-url http://127.0.0.1:18080/login ^
  -login-user-sel "#login_field" ^
  -login-pass-sel "#password" ^
  -login-submit-sel "#sign_in" ^
  -login-after-sel "#app_home" ^
  -login-user demo ^
  -login-pass demo ^
  -trigger-eval "fetch('/api/profile').then(r=>r.json()).then(_=>0)"
```

### v1: task queue + dedup + resume (`v1-queue`)

Prepare a seed file (one URL per line), e.g. `seed.txt`:

```text
https://example.com
https://www.example.com
```

Run:

```bash
go run ./cmd/crawl-demo v1-queue -seed-file seed.txt -db queue.db -out artifacts_queue -workers 4
```

Notes:
- You can stop anytime (Ctrl+C) and rerun to resume from SQLite (queued URLs are deduped).
- Failed tasks are marked as `failed` and will be re-queued automatically on next run (limited by `-max-attempts`).

### Block images to speed up (`v0`/`v1-capture`)

```bash
go run ./cmd/crawl-demo v0 -url https://example.com -out artifacts -block-images
```

## Production notes

- For real production, harden: domain allowlist, sensitive-data redaction, action allowlist, budgets, and concurrency/retry policies.

## v2: workflow mode (`v2`)

v2 loads a JSON workflow to orchestrate login / capture / trigger steps (useful for SPA and authenticated sites).

Start mocksite:

```bash
go run ./cmd/mocksite -addr 127.0.0.1:18080
```

Run v2 (using the built-in workflow example):

```bash
go run ./cmd/crawl-demo v2 ^
  -workflow workflows/mocksite_login_capture.json ^
  -allow-domains 127.0.0.1 ^
  -url http://127.0.0.1:18080/app ^
  -out artifacts_v2
```

## v2: queue + workflow (`v2-queue`)

This mode runs a SQLite-backed queue (dedup + resume + retry) and applies the same workflow to every task.

Prepare a seed file `seed.txt` (one URL per line, e.g. all pointing to `/app`):

```text
http://127.0.0.1:18080/app
```

Run:

```bash
go run ./cmd/crawl-demo v2-queue ^
  -workflow workflows/mocksite_login_capture.json ^
  -seed-file seed.txt ^
  -db queue_v2.db ^
  -out artifacts_v2_queue ^
  -workers 2 ^
  -allow-domains 127.0.0.1
```


