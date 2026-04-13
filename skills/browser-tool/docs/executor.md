## 最小执行器（goclaw CLI）

本 skill 配套一个最小可运行执行器，用于把 `run.yaml` + 本 skill 配置真正执行起来，并写出 `artifacts` 产物。

### 运行一个工作流

```bash
goclaw browser-tool run --file skills/browser-tool/examples/run-minimal.yaml --timeout 2m --skill-dir skills/browser-tool
```

常用参数：
- `--headless`：本地 Chrome 以 headless 模式运行（手动登录不可用）
- `--remote-cdp ws://127.0.0.1:9222`：连接远程 CDP
- `--skill-dir <dir>`：指定 skill 目录（在 `icerain_ai` 中通常为 `skills/browser-tool`）

### 保存登录态（storage_state）

```bash
goclaw browser-tool state save --site-key example.com --url https://example.com --timeout 3m
```

默认输出：
- `.goclaw/browser-states/<siteKey>.json`

### 已支持（最小版）

- 基础 steps：`connect/open/navigate/snapshot/screenshot/extract/write_actions/validate/report`
- 登录控制：`ensure_login`（signals 判断 + auto_then_manual（storage_state/cookie/form）+ manual 回退）

### 报告模板（templates）

`report` step 会选择模板：
- 优先 `step.with.template`
- 否则使用 `output.template`

模板查找规则：
- `default-report` → `templates/default-report.md`
- `api-capture-report` → `templates/api-capture-report.md`
- `write-report` → `templates/write-report.md`
- 也支持 `templates/<name>.md` 形式显式指定

占位符（最小版会填充）：
- `{{title}}`, `{{url}}`, `{{scenario}}`, `{{mode}}`
- `{{snapshot}}`（截断）
- `{{actions_taken}}`

### 重要限制

- `storage_state` 只能 best-effort 注入：
  - `document.cookie`（无法写入 HttpOnly）
  - `localStorage/sessionStorage`（evaluate 写入）
- 如果站点完全依赖 HttpOnly session cookie，建议切换 `engine=agent-browser` 走其原生 state save/load（后续可继续接入）。
