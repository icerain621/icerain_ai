# 参考：配置 schema 与管控模型

本文档较详细，核心用法见 `SKILL.md`。

## 1）内置 `browser` 工具接口（仓库现实）

网关注册了名为 `browser` 的单一工具，支持的 action 包括：
- `status`, `start`, `stop`, `tabs`
- `open`, `close`, `navigate`
- `snapshot`, `screenshot`, `console`
- `act`，其 `kind` 支持：`click|type|press|hover|wait|evaluate`

以 `pkg/browser/tool.go` 为准。

## 1.1 深度整合：外部 `agent-browser` 引擎（CLI）

当 `agent-browser` CLI 在 PATH 可用（或通过配置指定绝对路径）时，本编排器可将其作为优先引擎：
- 能力更完整：refs 快照、会话隔离、state 保存/加载、网络检查/路由、视频录制
- 不可用时再回退到 `goclaw-browser`

我们依赖的关键能力：
- 快照与 refs：`agent-browser snapshot -i -c`，交互使用 `@eN`
- 写操作：`click/fill/type/press/wait/...`（支持 `@eN` 或语义定位）
- 会话与 state：`--session`, `--session-name`, `--profile`, `state save/load`
- 网络：`network requests`, `network route/unroute`
- 录制证据：`record start/stop`（webm）

若你有本地仓库，可参考其内置文档（路径以实际为准）：
- `skills/agent-browser/references/commands.md`
- `skills/agent-browser/references/authentication.md`
- `skills/agent-browser/references/session-management.md`

### 预检建议

当 run 依赖 `agent-browser` 专属能力时，建议显式做 `preflight` 决策：
- `agent-browser` 不可用且允许回退：切换到 `goclaw-browser`
- 不允许回退（例如必须原生网络能力）：直接失败并给出安装提示

### 连接切换（CDP vs 本地）

运行时，连接模式由网关配置控制：
- 配置 `tools.browser.remote_url` 时，Rod 连接 **远程 CDP**
- 否则 Rod 启动 **本地 Chrome** 会话

对应代码：`cmd/gateway_setup.go` + `pkg/browser/browser.go`。

因此 `connection: cdp|local|auto` 是“意图”而非保证：
- `cdp` 只有在配置了 `remote_url` 时才可能生效
- 不匹配时应降级为 `local` 并记录原因

## 2）Skill 配置模型（defaults + per-site 覆盖）

编排器从以下位置读取配置：
- `config/defaults.yaml`
- `config/scenarios.yaml`（按场景套用配置）
- `config/sites/<siteKey>.yaml`（通常为 `<domain>.yaml`）

合并优先级（高 → 低）：
- run 中显式字段（例如 `run.scenario`、`run.output`、`run.login` 等）
- scenario（命中的场景）
- site（站点覆盖）
- defaults（全局默认）

## 2.0 场景选择算法（自动命中）

当 run 未显式指定 `run.scenario` 时，按以下规则选择场景：

1) 基于 `targetUrl` 提取 domain（作为站点识别与场景匹配输入）。
2) 遍历 `config/scenarios.yaml` 的 `scenarios[]`，对每个场景执行 match：
   - `domains`：domain 完全匹配
   - `urlPrefixes`：`targetUrl` 以该前缀开头
   - `urlRegex`：正则匹配 `targetUrl`
3) 取所有命中的场景中 `priority` 最大的作为最终场景。
4) 若无任何场景命中，则使用 `defaults.scenario.defaultScenarioId`。

冲突与可解释性：
- 多个场景同时命中时，只使用 `priority` 最大的一个（避免叠加导致不可预测）。
- 建议对“更具体”的场景使用更高 `priority`（例如 `api_capture_read` > `authenticated_read` > `public_read`）。

## 2.0.1 三种运行模式（read / write / mixed）

`mode` 既可以出现在 scenario 中，也可以在 run DSL 中显式指定：
- `read`：只读采集。禁止任何会改变页面状态的写操作（如 `write_actions`、`record_flow`）。
- `write`：允许写操作；同时允许插入读步骤用于证据与校验（如 `snapshot`、`extract`、`validate`）。
- `mixed`：读写混合。要求 **每个步骤** 标注 `step.mode: read|write`，用来明确该步骤按“读”还是“写”执行与计费/护栏。

mixed 的校验规则（建议）：
- 只要 run 或 scenario 的 `mode=mixed`，则 `steps[]` 内每个 step 都必须带 `mode` 字段。
- 写类 step（例如 `write_actions`、`record_flow`）必须声明 `mode: write`。
- 读类 step（例如 `snapshot`、`extract`、`capture_network`、`validate`、`report`）通常声明 `mode: read`。

### 2.0.2 mixed 的“步骤模式”配置表（推荐口径）

说明：
- 这张表用于在 `mode=mixed` 时统一团队口径，避免同一种 step 在不同 run 里被随意标注。
- “默认建议”并不强制；强制项以“必须”列为准。

| step.kind | 默认建议 step.mode | 允许 step.mode | 必须/禁止 | 备注 |
|---|---|---|---|---|
| `preflight` | `read` | `read` | 禁止 `write` | 预检不应改变站点状态。 |
| `connect` | `read` | `read` | 禁止 `write` | 连接/启动浏览器属于环境准备。 |
| `open` | `read` | `read` | 禁止 `write` | 打开页面本身归入读阶段（获取上下文与证据）。 |
| `navigate` | `read` | `read` | 禁止 `write` | 导航用于到达采集/操作位置。 |
| `snapshot` | `read` | `read` | 禁止 `write` | 快照是证据与 refs 来源。 |
| `screenshot` | `read` | `read` | 禁止 `write` | 截图是证据。 |
| `ensure_login` | `read` | `read` | 禁止 `write` | 该 step 是“登录控制器”，内部可包含必要写操作；对 mixed 外部标注保持 read。 |
| `extract` | `read` | `read` | 禁止 `write` | 采集步骤。 |
| `capture_network` | `read` | `read` | 禁止 `write` | 网络采集步骤（即便触发请求也按读处理）。 |
| `validate` | `read` | `read` | 禁止 `write` | 校验步骤。 |
| `report` | `read` | `read` | 禁止 `write` | 报告输出步骤。 |
| `paginate_or_scroll` | `read` | `read` 或 `write` | 取决于实现 | 如果实现仅滚动采集可标 read；若通过点击/加载更多等交互触发明显状态变化，建议标 write。 |
| `write_actions` | `write` | `write` | 必须 `write` | 一切显式 UI 交互都要标 write。 |
| `record_flow` | `write` | `write` | 必须 `write` | 录制阶段必然包含交互与状态变化。 |

### 2.1 根 schema

```yaml
version: 1

defaults:
  connection: auto               # auto|cdp|local
  scenario:
    defaultScenarioId: "public_read"
  output:
    format: both                 # markdown|json|both
    template: "default-report"   # 对应 `templates/default-report.md`
    reportPath: "artifacts/report.md"
    dataPath: "artifacts/data.json"
    artifactsDir: "artifacts"
  login:
    mode: auto_then_manual       # none|manual|auto|auto_then_manual
    manual:
      prompt: "请登录后确认。"
      confirm:
        signals: []              # 见 signals schema
    auto:
      strategies: []             # 顺序列表：cookie|form|oauth|storage_state
  budgets:
    timeoutMs: 180000
    maxSteps: 24
    maxSnapshots: 8
    maxScreenshots: 3
    maxActs: 40
  allow:
    actKinds: ["click","type","press","hover","wait","evaluate"]
    urlAllow:
      - "https://"
    urlDeny:
      - "file://"
      - "chrome://"
  extract:
    extractors: ["title","meta","mainText","links","tables","forms"]
    mainText:
      maxChars: 20000
  plugins:
    enabled: []                  # plugin ids
    registry: {}                 # optional inline plugin definitions

sites:
  - siteKey: example.com
    match:
      domains: ["example.com","www.example.com"]
      urlPrefixes: []
      urlRegex: []
    connection: auto
    login: {}                    # 覆盖 defaults.login
    detect:
      loginRequiredSignals: []   # signals
      loggedInSignals: []        # signals
      antiBotSignals: []         # signals
    selectors:
      hints: []                  # 快照中定位 refs 的提示（text/role/name）
    extract: {}                  # 覆盖 defaults.extract
    plugins:
      enabled: []
```

### 2.2 Signals schema

signals 用于：
- `loginRequiredSignals`：识别登录墙
- `loggedInSignals`：确认登录态
- `antiBotSignals`：识别挑战页/验证码/封禁

```yaml
signals:
  - kind: snapshot_text_includes
    value: "登录"
  - kind: snapshot_text_matches
    value: "验证码|人机验证"
  - kind: url_matches
    value: "/login"
  - kind: console_includes
    value: "拒绝访问"
```

支持的 `kind`：
- `snapshot_text_includes`
- `snapshot_text_matches`（regex）
- `url_includes`
- `url_matches` (regex)
- `console_includes`

说明：
- 这些规则基于现有 `browser snapshot/console` 输出设计（不依赖 DOM API）。

### 2.3 Selector hints schema（用于稳定定位 ref）

```yaml
selectors:
  hints:
    - kind: by_text
      value: "登录"
    - kind: by_role_name
      role: "button"
      name: "登录"
    - kind: by_text_regex
      value: "使用谷歌继续|继续使用谷歌"
```

本 skill 假设 agent 会：
1) 执行 `snapshot`（interactive=true, compact=true）
2) 在快照文本中根据 hints 查找目标元素
3) 选择正确的 `ref`（e1/e2/...）用于 `browser act`

## 3）登录编排规则

### 3.1 模式
- `none`：不尝试登录；检测到登录墙则失败或（如允许）转手动确认
- `manual`：始终要求用户完成登录并确认
- `auto`：按配置尝试自动登录；失败则失败或回退手动
- `auto_then_manual`：先自动登录，失败则回退手动

### 3.2 自动登录策略（按顺序执行）

在站点配置中定义：
```yaml
login:
  mode: auto_then_manual
  auto:
    strategies:
      - kind: cookie
        credentialRef: "env:EXAMPLE_COOKIE"
        apply: "header" # header|evaluate
      - kind: form
        fields:
          usernameRefHint: { kind: by_text, value: "Email" }
          passwordRefHint: { kind: by_text, value: "Password" }
          submitRefHint:   { kind: by_role_name, role: "button", name: "Sign in" }
        credentialRef:
          username: "env:EXAMPLE_USER"
          password: "env:EXAMPLE_PASS"
      - kind: oauth
        provider: google
        buttonHint: { kind: by_text_regex, value: "(?i)continue with google" }
      - kind: storage_state
        stateRef: "file:.goclaw/browser-states/example.com.json"
```

重要约束：
- 不要在 YAML 中写入任何明文敏感信息；必须通过 `credentialRef` 引用
- 触发 `antiBotSignals` 时，应停止自动登录并切换为手动确认（如模式允许）

## 4）预算与护栏

预算用于防止无限循环：
- `maxSteps`：步骤硬上限
- `maxSnapshots/maxScreenshots/maxActs`：工具调用硬上限
- `timeoutMs`：整体超时（建议由 agent 进行逐步跟踪）

允许动作：
- `allow.actKinds` 限制 `browser act` 的 kind（例如敏感站点禁用 `evaluate`）
- URL allow/deny 防止导航到内部/危险 scheme

## 4.1 Read vs write modes

本 skill 支持两种 run 模式：
- **read**：采集页面数据，或采集指定网络响应数据
- **write**：执行受控页面操作（click/fill/submit/wait）以完成任务或到达目标状态，并可选再做采集

模式对护栏的影响：
- **read** 模式优先 `snapshot` + `extract`，尽量少交互
- **write** 模式要求明确动作计划，并用 `snapshot` 进行事后校验

## 4.2 采集网络响应（read 模式）

内置 `browser` 工具不直接暴露 CDP network 事件。
如需采集特定请求/响应 payload，可使用 hook 策略：

### 策略 A（默认）：`evaluate` 安装 fetch/XHR 拦截器 + `console` 读取捕获数据

1) 使用 `browser act`（`kind=evaluate`）安装 hook（每个页面一次）。
2) 触发目标请求（navigate/click/typing）。
3) 通过 `browser` 的 `console` 读取捕获输出。

hook 设计目标：
- 按 URL pattern（allowlist）过滤
- 限制 payload 最大字节数与捕获条数
- 对敏感字段做脱敏
- 输出可机器解析的 console 行，例如：
  - `ABO_NET {"ts":..., "url":"...", "status":200, "body":"...base64...", "contentType":"application/json"}`

推荐的 hook（概念示意）：
- 对 `window.fetch` 与 `XMLHttpRequest` 做轻量封装（拦截 open/send）。
- 仅对命中的 URL：克隆响应（fetch）或读取 responseText（XHR），并以可解析格式输出到 console。

### 策略 B（回退）：使用 `web_fetch` 做服务端抓取

如果目标请求较简单且不依赖浏览器登录态：
- 使用 `web_fetch`（需域名在 allowlist 中）
- 当 `evaluate` 不允许或站点阻止 JS hook 时优先用该方式

### 策略 0（engine=agent-browser 时优先）：使用原生网络工具

当 `engine=agent-browser` 时，优先使用其内置能力：
- Observe: `agent-browser network requests --filter <pattern>`
- Intercept/mock/block: `agent-browser network route <url> [--abort|--body ...]`

该方式通常比 JS 拦截更可靠，并且在不改动站点代码的情况下适用于许多 SPA/XHR 场景。

### 何时停止并转手动登录

如果请求依赖登录态且无法安全采集：
- 切换到 `manual` 登录流程
- 在用户确认登录后重新执行采集

### 网络采集安全规则
- 禁止记录或存储任何 token、session cookie、密码、验证码等
- 将捕获的 JSON 视为潜在敏感信息；只包含用户需要的字段
- 强制上限：超过配置限制的 body 不采集，仅记录元数据

## 5）插件模型（动态插拔）

插件是配置驱动的步骤包，可用于：
- 在某个 step 前/后插入步骤
- 按约定提供某个 kind 的策略实现

最小插件定义：
```yaml
plugins:
  enabled: ["infinite_scroll","table_export_button"]
  registry:
    infinite_scroll:
      description: "Scroll-down collection for infinite lists."
      insertAfter: "firstSnapshot"
      steps:
        - id: scrollLoop
          kind: paginate_or_scroll
          with: { mode: "infinite_scroll", maxScrolls: 6, waitMs: 800 }
```

插件约束：
- 必须声明所需 `actKinds` 与预估预算影响
- 禁用时不应造成正确性退化

## 6）流程录制与合入

### 6.1 本仓库中的“录制”含义

`browser` 工具不提供原始点击流/DOM 事件录制 API。录制采用“人在回路”的协议：
- agent 执行 `snapshot`
- 用户手动执行下一步 UI 操作
- 用户确认已完成
- agent 再次 `snapshot` 并用 `refHint` 选择器追加操作

这样可生成“可移植”的 flow：基于稳定 hints（text/role/name），而不是脆弱的坐标。

当 `engine=agent-browser` 时，也可在录制过程中附带视频证据：
- `agent-browser record start ./flows/<siteKey>/<flowId>.webm`
- 执行步骤
- `agent-browser record stop`

### 6.2 录制协议（推荐）

默认配置：
- confirm token：`REC_NEXT`
- finish token：`REC_DONE`

循环：
1) agent 打开目标 URL，并在需要时确保登录
2) agent 执行 `snapshot`（interactive=true, compact=true）
3) agent 提示用户执行**一个动作**（click/type/submit），然后回复 `REC_NEXT`
4) agent 再次 `snapshot`；推断意图并记录到 flow 的 `write_actions.ops[]`。
5) 重复直到用户回复 `REC_DONE`

### 6.3 Flow 输出路径与命名

flow 文件写入：
- `skills/browser-tool/flows/<siteKey>/<flowId>.yaml`

命名规则：
- `flowId`：小写 + 连字符的动词短语（如 `create-ticket`, `export-csv`）
- `siteKey`：域名（如 `example.com`）

### 6.4 合入前校验清单

- **Safety**:
  - flow 文件中不得包含任何敏感信息（cookies/tokens/passwords 等）
 - **稳定性**：
  - `refHint` 优先使用稳定选择器（role+name 或稳定文本），避免脆弱标签
  - 每段写操作后应跟随 `snapshot` 校验（除非明确关闭）
- **预算**：
  - steps 需满足站点预算约束
  - 避免不必要的截图，优先用快照
- **兼容性**：
  - flow 需声明 `siteKey`、`requires.mode`，可选声明插入点

### 6.5 启用已录制的 flow（可插拔）

推荐约定（站点级）：
1) 添加一个指向 flow 的插件注册项，或将 flow 以内联步骤包形式加入插件
2) 在 `config/sites/<siteKey>.yaml` 中启用

如果希望纯文件引用（不内联重复），可把每个 flow 当作插件步骤包：
- agent 在运行时按插入点展开 flow 的 `steps`
