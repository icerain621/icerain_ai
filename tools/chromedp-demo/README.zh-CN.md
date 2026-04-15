# chromedp-demo

English: `README.md` · 中文：`README.zh-CN.md`（本文件）

本目录是一个独立 Go module，用 `github.com/chromedp/chromedp` 演示并封装“生产可用爬虫”能力。

## 快速开始

### 启动本地“假登录站点”（mocksite）

```bash
go run ./cmd/mocksite -addr 127.0.0.1:18080
```

### 基础抓取（v0）

```bash
go run ./cmd/crawl-demo v0 -url https://example.com -out artifacts
```

### 用 chromedp 跑通 mocksite 登录（v0 + 表单登录）

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

## v1 子命令

### v1：抓 XHR/JSON 响应（v1-capture）

```bash
go run ./cmd/crawl-demo v1-capture -url https://example.com -out artifacts_capture -capture-url "example\\.com"
```

### v1：对 mocksite 先登录再触发 XHR 抓包（v1-capture）

先启动 mocksite：

```bash
go run ./cmd/mocksite -addr 127.0.0.1:18080
```

再运行 v1（先登录，再触发 `fetch('/api/profile')`）：

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

### v1：任务队列 + 去重 + 断点续爬（v1-queue）

准备一个种子文件（每行一个 URL），例如 `seed.txt`：

```text
https://example.com
https://www.example.com
```

运行：

```bash
go run ./cmd/crawl-demo v1-queue -seed-file seed.txt -db queue.db -out artifacts_queue -workers 4
```

说明：
- 可随时 Ctrl+C 中断；再次运行会从 SQLite 中继续（已入队 URL 会去重）。
- 失败任务会记录为 failed；下次运行会在 `-max-attempts` 限制内自动回队重试。

## 生产化建议

- 生产使用请完善：站点 allowlist、敏感信息脱敏、动作白名单、预算与并发/重试策略。

## v2：workflow 模式（v2）

v2 用一个 JSON workflow 把“登录 / 抓包 / 触发请求”等能力编排起来（适合 SPA/登录站点的可复用流程）。

启动 mocksite：

```bash
go run ./cmd/mocksite -addr 127.0.0.1:18080
```

运行 v2（使用内置示例 workflow）：

```bash
go run ./cmd/crawl-demo v2 ^
  -workflow workflows/mocksite_login_capture.json ^
  -allow-domains 127.0.0.1 ^
  -url http://127.0.0.1:18080/app ^
  -out artifacts_v2
```

## v2：队列 + workflow（v2-queue）

该模式会从 SQLite 队列取任务（去重/断点续爬/失败重试），并对每个任务执行同一份 workflow（适合批量跑登录站点或 SPA）。

准备种子文件 `seed.txt`（每行一个 URL，例如都指向 `/app`）：

```text
http://127.0.0.1:18080/app
```

运行：

```bash
go run ./cmd/crawl-demo v2-queue ^
  -workflow workflows/mocksite_login_capture.json ^
  -seed-file seed.txt ^
  -db queue_v2.db ^
  -out artifacts_v2_queue ^
  -workers 2 ^
  -allow-domains 127.0.0.1
```


