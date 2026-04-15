## HTTP API (Gin)

Run server:

```bash
HTTP_ADDR=127.0.0.1:19090 go run ./cmd/http-server
```

OpenAPI:

- `GET /openapi.yaml`

### Endpoints

- **GET** `/healthz`
- **POST** `/api/v0/run`
- **POST** `/api/v1/capture/run`
- **POST** `/api/v2/run`
- **POST** `/api/v2/queue/start` (async, returns `jobId`)
- **GET** `/api/jobs/:id`

### POST /api/v0/run

Basic crawl (v0) via HTTP.

Request:

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

Response:

```json
{ "outDir": "artifacts_http/v0_1", "result": { "taskId": "...", "htmlPath": "...", "shotPath": "..." } }
```

### POST /api/v1/capture/run

Capture XHR/JSON by URL regex.

Request:

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

Run a workflow file for one task.

```json
{
  "url": "http://127.0.0.1:18080/app",
  "outDir": "artifacts_http/v2_1",
  "workflowPath": "workflows/mocksite_login_capture.json",
  "policy": { "allowDomains": ["127.0.0.1"] }
}
```

### POST /api/v2/queue/start

Starts an async queue+workflow job.

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

Returns:

```json
{ "jobId": "job-...", "outDir": "artifacts_http/v2q_1" }
```

Check job status:

```bash
curl http://127.0.0.1:19090/api/jobs/<jobId>
```

