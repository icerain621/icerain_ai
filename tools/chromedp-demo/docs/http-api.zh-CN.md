## HTTP API（Gin）

启动服务：

```bash
HTTP_ADDR=127.0.0.1:19090 go run ./cmd/http-server
```

OpenAPI：

- `GET /openapi.yaml`

### 接口列表

- **GET** `/healthz`
- **POST** `/api/v0/run`
- **POST** `/api/v1/capture/run`
- **POST** `/api/v2/run`
- **POST** `/api/v2/queue/start`（异步，返回 `jobId`）
- **GET** `/api/jobs/:id`

### POST /api/v0/run

通过 HTTP 执行 v0 单页抓取（可选表单登录）。

请求示例：

```json
{
  "url": "http://127.0.0.1:18080/app",
  "outDir": "artifacts_http/v0_1",
  "policy": { "allowDomains": ["127.0.0.1"], "timeoutMs": 90000 },
  "login": {
    "loginUrl": "http://127.0.0.1:18080/login",
    "username": "demo",
    "password": "demo",
    "usernameSelector": "#login_field",
    "passwordSelector": "#password",
    "submitSelector": "#sign_in",
    "afterSelector": "#app_home"
  }
}
```

### POST /api/v1/capture/run

按 URL 正则抓取 XHR/JSON 响应体（可选登录 + 触发 XHR）。

```json
{
  "url": "http://127.0.0.1:18080/app",
  "outDir": "artifacts_http/cap_1",
  "policy": { "allowDomains": ["127.0.0.1"], "timeoutMs": 120000 },
  "login": {
    "loginUrl": "http://127.0.0.1:18080/login",
    "username": "demo",
    "password": "demo",
    "usernameSelector": "#login_field",
    "passwordSelector": "#password",
    "submitSelector": "#sign_in",
    "afterSelector": "#app_home"
  },
  "capture": {
    "captureUrlRegex": "/api/profile",
    "maxBodyBytes": 1000000,
    "triggerEval": "fetch('/api/profile').then(r=>r.json()).then(_=>0)",
    "triggerWaitMs": 1200
  }
}
```

### POST /api/v2/run

对单个任务执行 workflow 文件：

```json
{
  "url": "http://127.0.0.1:18080/app",
  "outDir": "artifacts_http/v2_1",
  "workflowPath": "workflows/mocksite_login_capture.json",
  "policy": { "allowDomains": ["127.0.0.1"] }
}
```

### POST /api/v2/queue/start

启动“队列 + workflow”的异步任务：

```json
{
  "seedFile": "seed.txt",
  "dbPath": "queue_v2.db",
  "outDir": "artifacts_http/v2q_1",
  "workers": 2,
  "maxAttempts": 3,
  "workflowPath": "workflows/mocksite_login_capture.json",
  "policy": { "allowDomains": ["127.0.0.1"] }
}
```

返回：

```json
{ "jobId": "job-...", "outDir": "artifacts_http/v2q_1" }
```

查询状态：

```bash
curl http://127.0.0.1:19090/api/jobs/<jobId>
```

