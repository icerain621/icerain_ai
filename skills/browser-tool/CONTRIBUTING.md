## 贡献指南（browser-tool）

本 skill 的目标是：用“配置 + 可复用资产（scenarios/sites/flows/templates）”把浏览器采集/操作流程标准化，并且保持可审计、可回退、可扩展。

### 新增一个站点（sites）

1) 在 `config/sites/` 新建 `<siteKey>.yaml`（通常为域名）
2) 至少配置：
   - `login.mode`
   - `detect.loginRequiredSignals`
   - `detect.loggedInSignals`（强烈建议）
   - `detect.antiBotSignals`（建议）
3) 如果启用自动登录：
   - 优先 `storage_state`（stateRef 指向 `.goclaw/browser-states/<siteKey>.json`）
   - 再尝试 `cookie` / `form`

### 新增一个场景（scenarios）

1) 在 `config/scenarios.yaml` 增加一个 `scenarioId`
2) 设置：
   - `priority`（越具体越高）
   - `match`（domains/urlPrefixes/urlRegex）
   - `mode`（read/write/mixed）
   - `output`（建议为不同场景分开 report/data 路径）

### 新增一个 flow（flows）

1) 放在 `flows/<siteKey>/<flowId>.yaml`
2) 参照 `flows/README.md` 的最小 schema
3) 提交前用 `checklists/flow-review.md` 自检

### 修改模板（templates）

- 模板一律使用 Markdown
- 不要在模板里包含敏感信息
- 允许根据场景拆分多个模板文件（例如 `default-report.md`、`api-capture.md`）
