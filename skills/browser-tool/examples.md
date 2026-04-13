# 可复制示例

这些示例关注：如何运行工作流与输出长什么样。按站点自定义请编辑 `config/sites/<domain>.yaml`。

## 示例 1：无需登录（公开页面）

### 输入
- targetUrl: `https://example.com/pricing`
- login：`none`
- 目标：提取价格表与关键说明
- scenario：`public_read`

### 运行 DSL
```yaml
run:
  scenario: "public_read"
  engine: "agent-browser"
  mode: "read"
  targetUrl: "https://example.com/pricing"
  siteKey: "example.com"
  connection: "auto"
  login: { mode: "none" }
  output:
    format: both
    reportPath: "artifacts/pricing-report.md"
    dataPath: "artifacts/pricing-data.json"
  budget:
    timeoutMs: 120000
    maxSteps: 16
    maxSnapshots: 6
    maxScreenshots: 2
    maxActs: 20
  steps:
    - id: connect
      kind: connect
    - id: open
      kind: open
      with: { url: "${run.targetUrl}" }
    - id: snapshot
      kind: snapshot
      with: { compact: true, interactive: true, maxChars: 8000 }
    - id: extract
      kind: extract
      with: { extractors: ["title","mainText","tables","links"] }
    - id: report
      kind: report
      with: { template: "default-report" }
```

### 预期报告结构（示意）

- `## 摘要`：价格概览
- `## 关键信息`：title / url / login_mode / connection
- `## 证据`：snapshot_stats / 关键引用
- `## 数据（JSON）`：tables 等结构化字段

## 示例 2：手动登录（用户确认闭环）

适用场景：
- 站点经常出现验证码/二次验证
- 登录流程过于动态，不适合稳定自动化

### 运行 DSL
```yaml
run:
  scenario: "authenticated_read"
  engine: "agent-browser"
  mode: "read"
  targetUrl: "https://example.com/account/billing"
  siteKey: "example.com"
  connection: "auto"
  login: { mode: "manual" }
  steps:
    - id: connect
      kind: connect
    - id: open
      kind: open
      with: { url: "${run.targetUrl}" }
    - id: snapshot0
      kind: snapshot
      with: { compact: true, interactive: true, maxChars: 8000 }
    - id: ensureLogin
      kind: ensure_login
    - id: snapshot1
      kind: snapshot
      with: { compact: true, interactive: true, maxChars: 8000 }
    - id: extract
      kind: extract
      with: { extractors: ["title","tables","mainText"] }
    - id: report
      kind: report
      with: { template: "default-report" }
```

## 示例 0：不写 `run.scenario`，按 URL 自动命中场景

适用场景：
- 你希望“默认一把梭”：只给 URL，让系统自动选择 public/auth/api/write 等场景
- 你在 `config/scenarios.yaml` 中已经为某些域名或 URL 前缀配置了 match

### 先配置场景命中规则（示例）

在 `config/scenarios.yaml` 中，为 `authenticated_read` 增加 domains 匹配（示例域名）：

```yaml
scenarios:
  - scenarioId: authenticated_read
    priority: 20
    match:
      domains: ["app.example.com"]
      urlPrefixes: []
      urlRegex: []
    # ... 其余保持不变 ...
```

### 运行 DSL（不写 scenario）

```yaml
run:
  engine: "auto"
  mode: "read"
  targetUrl: "https://app.example.com/dashboard"
  siteKey: "app.example.com"
  connection: "auto"
  # 不写 scenario：由 URL 自动命中 authenticated_read
  steps:
    - id: connect
      kind: connect
    - id: open
      kind: open
      with: { url: "${run.targetUrl}" }
    - id: ensureLogin
      kind: ensure_login
    - id: extract
      kind: extract
      with: { extractors: ["title","mainText","links"] }
    - id: report
      kind: report
      with: { template: "default-report" }
```

### 预期行为
- `targetUrl` 的 domain 为 `app.example.com`
- `authenticated_read.match.domains` 命中 → 选中 `authenticated_read`
- 合并优先级：run 显式字段 > scenario > site > defaults

## 示例 0.1：读写混合模式（mixed），逐步标注读/写

适用场景：
- 先读：采集页面关键信息/定位元素
- 再写：执行一次交互（点击/填写/提交）
- 再读：校验结果并输出报告

### 运行 DSL（mixed + step.mode）

```yaml
run:
  scenario: "mixed_read_write" # 可选；也可不写 scenario 仅用 mode=mixed
  engine: "auto"
  mode: "mixed"
  targetUrl: "https://example.com/settings"
  siteKey: "example.com"
  login: { mode: "auto_then_manual" }
  steps:
    - id: connect
      kind: connect
      mode: read
    - id: open
      kind: open
      mode: read
      with: { url: "${run.targetUrl}" }
    - id: snap0
      kind: snapshot
      mode: read
      with: { compact: true, interactive: true, maxChars: 8000 }
    - id: ensureLogin
      kind: ensure_login
      mode: read
    - id: doWrites
      kind: write_actions
      mode: write
      with:
        ops:
          - { kind: click, refHint: { kind: by_text, value: "保存" } }
          - { kind: wait, request: { kind: wait, text: "已保存" } }
    - id: snap1
      kind: snapshot
      mode: read
      with: { compact: true, interactive: true, maxChars: 8000 }
    - id: extract
      kind: extract
      mode: read
      with: { extractors: ["title","mainText"] }
    - id: report
      kind: report
      mode: read
      with: { template: "default-report" }
```

### 手动登录提示词（agent 应该怎么说）
```text
我已打开登录页面，请现在在浏览器标签页中完成登录。
登录完成后，请回复：CONFIRMED
```

确认后：
- 重新执行 `snapshot`
- 验证 `loggedInSignals`（站点配置）
- 继续采集

## 示例 3：自动登录（cookie → 表单 → 回退手动）

这是推荐的默认策略：`auto_then_manual`。

### 站点配置片段
位于 `config/sites/example.com.yaml`：
```yaml
login:
  mode: auto_then_manual
  auto:
    strategies:
      - kind: cookie
        credentialRef: "env:EXAMPLE_COOKIE"
        apply: "header"
      - kind: form
        credentialRef:
          username: "env:EXAMPLE_USER"
          password: "env:EXAMPLE_PASS"
        fields:
          usernameRefHint: { kind: by_text, value: "Email" }
          passwordRefHint: { kind: by_text, value: "Password" }
          submitRefHint:   { kind: by_role_name, role: "button", name: "Sign in" }
```

### 运行 DSL
```yaml
run:
  engine: "agent-browser"
  mode: "read"
  targetUrl: "https://example.com/app"
  siteKey: "example.com"
  connection: "cdp"
  login: { mode: "auto_then_manual" }
  steps:
    - id: connect
      kind: connect
    - id: open
      kind: open
      with: { url: "${run.targetUrl}" }
    - id: snapshot0
      kind: snapshot
      with: { compact: true, interactive: true, maxChars: 8000 }
    - id: ensureLogin
      kind: ensure_login
    - id: extract
      kind: extract
      with: { extractors: ["title","mainText","links"] }
    - id: report
      kind: report
      with: { template: "default-report" }
```

### 自动登录规则
- cookie 注入成功则跳过表单步骤
- 触发反爬信号时停止自动化并请求手动确认
- 自动动作后必须重新检查 `loggedInSignals`

## 示例 4：插件插入（无限滚动）

在 `config/defaults.yaml` 中启用插件：
```yaml
defaults:
  plugins:
    enabled: ["infinite_scroll"]
    registry:
      infinite_scroll:
        description: "无限滚动采集"
        insertAfter: "firstSnapshot"
        steps:
          - id: scrollLoop
            kind: paginate_or_scroll
            with: { mode: "infinite_scroll", maxScrolls: 6, waitMs: 800 }
```

## 示例 5：读模式——采集指定网络响应

目标：采集页面触发的某个已知 API 调用返回的 JSON。

### 运行 DSL
```yaml
run:
  scenario: "api_capture_read"
  engine: "agent-browser"
  mode: "read"
  targetUrl: "https://example.com/app"
  siteKey: "example.com"
  login: { mode: "auto_then_manual" }
  steps:
    - id: connect
      kind: connect
    - id: open
      kind: open
      with: { url: "${run.targetUrl}" }
    - id: snap0
      kind: snapshot
      with: { compact: true, interactive: true, maxChars: 8000 }
    - id: ensureLogin
      kind: ensure_login
    - id: hookNet
      kind: capture_network
      with:
        allowUrlPatterns:
          - "^https://example\\.com/api/v1/items\\b"
        maxCaptures: 3
        maxBodyBytes: 80000
    - id: clickLoad
      kind: write_actions
      with:
        ops:
          - { kind: click, refHint: { kind: by_text, value: "加载" } }
          - { kind: wait,  request: { kind: wait, timeMs: 800 } }
    - id: extract
      kind: extract
      with: { extractors: ["title","mainText"] }
    - id: report
      kind: report
      with: { template: "default-report" }
```

预期：agent 读取 `browser console` 并解析 `ABO_NET ...` 行，将 JSON payload 写入 `network[]`。

## 示例 6：写模式——填写并提交表单

目标：执行提交（写操作），并验证成功状态。

```yaml
run:
  scenario: "write_form_submit"
  engine: "agent-browser"
  mode: "write"
  targetUrl: "https://example.com/contact"
  siteKey: "example.com"
  login: { mode: "none" }
  steps:
    - id: connect
      kind: connect
    - id: open
      kind: open
      with: { url: "${run.targetUrl}" }
    - id: snap0
      kind: snapshot
      with: { compact: true, interactive: true, maxChars: 8000 }
    - id: fillAndSubmit
      kind: write_actions
      with:
        ops:
          - { kind: type, refHint: { kind: by_text, value: "Name" }, text: "Alice" }
          - { kind: type, refHint: { kind: by_text, value: "Email" }, text: "alice@example.com" }
          - { kind: type, refHint: { kind: by_text, value: "Message" }, text: "Hello" }
          - { kind: click, refHint: { kind: by_role_name, role: "button", name: "Submit" } }
          - { kind: wait, request: { kind: wait, text: "Thank you" } }
    - id: snap1
      kind: snapshot
      with: { compact: true, interactive: true, maxChars: 8000 }
    - id: report
      kind: report
      with: { template: "default-report" }
```

## 示例 7：录制 flow（用户驱动）→ 合入 skill

目标：录入可复用流程，并保存为 `flows/` 下的 flow 文件。

### 7.1 开启录制

在 `config/defaults.yaml` 中：
```yaml
defaults:
  recording:
    enabled: true
```

### 7.2 录制 run（协议）

```yaml
run:
  engine: "agent-browser"
  mode: "write"
  targetUrl: "https://example.com/app"
  siteKey: "example.com"
  login: { mode: "manual" }
  steps:
    - id: connect
      kind: connect
    - id: open
      kind: open
      with: { url: "${run.targetUrl}" }
    - id: ensureLogin
      kind: ensure_login
    - id: snap0
      kind: snapshot
      with: { compact: true, interactive: true, maxChars: 8000 }
    - id: recordLoop
      kind: record_flow
      with:
        flowId: "create-ticket"
        siteKey: "example.com"
        confirmToken: "REC_NEXT"
        finishToken: "REC_DONE"
```

执行过程：
- agent 每次让你执行一个 UI 动作
- 每个动作完成后回复 `REC_NEXT`
- 全部完成后回复 `REC_DONE`

预期输出：
- `skills/browser-tool/flows/example.com/create-ticket.yaml`

### 7.3 在该站点启用录制的 flow

按约定添加一个插件项用于展开 flow steps：
```yaml
plugins:
  enabled: ["flow:create-ticket"]
```
