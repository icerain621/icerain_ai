---
name: browser-tool
description: 面向指定 URL 的浏览器编排器：把内置 `browser`（以及可选的 `agent-browser` CLI）封装成受控、可配置的采集/操作工作流。支持站点级登录策略（none/manual/auto）、CDP 远程/本地会话切换、可自定义步骤 DSL、Markdown 模板输出、JSON 数据输出、可插拔插件与预算/超时护栏。用于“采集网页信息/抓取指定请求响应/自动化页面操作/登录后采集/录制流程”等需求。
---

# 浏览器编排器

## 适用场景

用这个 skill 在**指定 URL** 上运行**可重复、可控**的浏览器工作流，支持：
- **登录模式**: `none`, `manual`, `auto`（支持策略回退）
- **站点兼容规则**: selectors、登录检测、反爬信号、采集策略
- **连接切换**: **CDP remote** vs **local session**（由网关配置 + 工作流决定）
- **流程管控**: 步数预算、超时、允许动作、重试、降级
- **可定制输出**: Markdown 报告 + JSON 结构化数据

本 skill 假设网关暴露了名为 `browser` 的内置工具（见 `pkg/browser/tool.go`），动作包括：`start/open/navigate/snapshot/screenshot/console/act/tabs/close`。

## 引擎选择（深度整合 `agent-browser`）

本编排器支持两种执行引擎：
- **agent-browser（优先）**：使用外部 `agent-browser` CLI（PATH 可用时）获得更强能力：sessions、state 保存/加载、网络检查/路由、视频录制。
- **goclaw-browser（回退）**：使用本仓库内置 `browser` 工具。

实践建议：
- 需要网络采集、state 保存/加载、视频录制时优先选 **agent-browser**。
- 外部 CLI 不可用时自动回退到 **goclaw-browser**（能力降级）。

### 健壮性规则（必须）
永远不要假设 `agent-browser` 已安装。`run.engine=auto` 时必须：
- 先尝试 `agent-browser`（配置启用且 preflight 通过时）。
- 否则回退到 `goclaw-browser`。
- 两者都不可用则降级为非交互方案（例如 `web_fetch`）或请求用户协助。

## 快速开始（默认采集）

更多目录结构与入口索引见：`README.md`。最小执行器用法见：`docs/executor.md`。

### 需要向用户收集的输入
- **targetUrl**：要采集/操作的页面 URL。
- **goal**：目标信息或要完成的操作（表格、价格、提交表单等）。
- **mode**：`read | write | mixed`（读=采集，写=交互操作，mixed=同一次 run 内混合读写）。
- **scenario（推荐）**：场景 ID，用于一键套用登录、连接方式、输出模板、输出格式等（见 `config/scenarios.yaml`）。
- **login**：`none | manual | auto`（或 `auto_then_manual`）。
- **site**：URL 的域名，用于加载站点覆盖配置。
- **recording（可选）**：开启后可录制流程并合入 skill。

### 默认执行清单
- 启动/连接浏览器（CDP 远程或本地会话）。
- 打开 targetUrl。
- `snapshot`（compact + interactive），必要时 `screenshot`。
- 如需登录则确保登录完成。
- 采集/执行目标操作。
- 校验输出是否齐全。
- 生成 Markdown 报告与/或 JSON 数据。

## 流程录制（自定义流程采集）

当开启录制（见 `config/defaults.yaml` → `defaults.recording.enabled`）时，用户可快速录入操作流程并生成可复用的 flow YAML（输出到 `flows/`）。

录制模型（适配当前 `browser` 工具能力约束）：
- Agent 用 `snapshot` 获取稳定的 refs。
- 用户在浏览器中执行 **一个 UI 动作**。
- 用户回复确认 token（默认 `REC_NEXT`）。
- Agent 再次 `snapshot`，推断变化并把操作追加到 `write_actions`（使用 ref hints）。
- 重复直到用户回复结束 token（默认 `REC_DONE`）。

录制产物：
- `flows/<siteKey>/<flowId>.yaml`（可插拔到 run 或站点配置）。

flow schema 与合入规范见 `flows/README.md`。

## 工作流 DSL（最小可复制）

用 YAML 表示一次 run。步骤按顺序执行，每步受预算与护栏约束。

```yaml
run:
  scenario: "public_read"     # 场景 ID（见 config/scenarios.yaml）
  engine: "agent-browser"     # agent-browser|goclaw-browser|auto
  mode: "read"              # read|write|mixed
  targetUrl: "https://example.com/path"
  siteKey: "example.com"
  connection: "auto"         # auto|cdp|local
  login:
    mode: "auto_then_manual" # none|manual|auto|auto_then_manual
  budget:
    timeoutMs: 180000
    maxSteps: 24
    maxSnapshots: 8
    maxScreenshots: 3
    maxActs: 40
  allow:
    actKinds: ["click","type","press","hover","wait","evaluate"]
  steps:
    - id: connect
      kind: connect
    - id: open
      kind: open
      with:
        url: "${run.targetUrl}"
    - id: firstSnapshot
      kind: snapshot
      with:
        compact: true
        interactive: true
        maxChars: 8000
    - id: ensureLogin
      kind: ensure_login
    - id: extract
      kind: extract
      with:
        extractors: ["title","meta","mainText","links","tables","forms"]
    - id: validate
      kind: validate
      with:
        require:
          - "data.title"
          - "data.url"
    - id: report
      kind: report
      with:
        template: "default-report"
```

### Step 模型
- **id**：步骤唯一标识
- **kind**：内置 step 种类之一
- **with**：步骤参数
- **mode（可选）**：`read|write`。当 `run.mode=mixed` 时必填，用于标注该步骤按读或写执行。
- **when（可选）**：对 run state 求值的条件表达式
- **onError（可选）**：`fail | skip | retry | fallback:<stepId>`
- **budgetCost（可选）**：覆盖默认预算计费

模式约束：
- `run.mode=read`：禁止执行写步骤（例如 `write_actions`、`record_flow`）。
- `run.mode=write`：允许写步骤；读步骤可用于证据与校验（如 `snapshot`、`extract`、`validate`）。
- `run.mode=mixed`：必须为每个 step 显式标注 `step.mode`（read 或 write），并且写步骤必须标为 write。

## 内置 step 种类

### `preflight`（预检）
在开始浏览前验证引擎可用性与能力要求。

适用场景：
- 依赖 `agent-browser` 专属能力（network tools、state、video）
- 希望“明确失败”而不是静默回退

### `connect`
选择浏览器连接方式：
- **cdp**：优先使用 CDP 远程 Chrome（需要网关配置 `tools.browser.remote_url`）
- **local**：本地启动会话
- **auto**：按站点配置/默认配置自动选择

护栏：
- 如果 run 指定 `cdp` 但未配置 `remote_url`，则降级为 `local` 并记录到 `actions_taken`。

引擎差异：
- **agent-browser**：映射为 `agent-browser connect <port>`（或 `--auto-connect`），并结合 session/profile 选择
- **goclaw-browser**：由内置工具的 RemoteURL/launcher 行为决定

### `open`
在 `url` 打开新标签页，并返回/记录 `targetId`。

映射到工具调用：
- `browser` 的 `open`（参数 `targetUrl`）

### `navigate`
在已有标签页中导航（需要 `targetId`）。

### `snapshot`
获取可访问性快照与 element refs（用于稳定交互）。

映射到：
- `browser` 的 `snapshot`（`targetId`, `maxChars`, `interactive`, `compact`, `depth`）

### `screenshot`
截图作为证据（谨慎使用，受预算约束）。

### `ensure_login`
标准登录控制器：
- 检测是否需要登录（站点 signals + 规则）
- 路由到 `none/manual/auto` 行为
- 复检登录态 signals，继续或安全失败

手动登录约束：
- 必须明确提示用户在浏览器中完成登录并确认
- 必须重新 `snapshot` 并验证登录态 signals

自动登录约束：
- 只允许执行策略与站点配置允许的自动登录步骤
- 检测到反爬/验证码后应停止自动尝试并切换为手动确认

### `extract`
对当前页面进行结构化采集。

默认 extractors：
- `title`, `meta`, `headings`, `mainText`, `links`, `tables`, `forms`, `entities`

### `capture_network`
读模式：采集当前页面触发的指定网络响应。

典型流程：
- 通过 `evaluate` 安装 fetch/XHR hook
- 通过导航/点击触发目标请求
- 通过 `console` 读取捕获的 payload

该 step 受预算约束，并且必须限制：
- 允许的 URL pattern
- 最大 payload 字节数
- 脱敏规则（禁止泄露敏感信息）

引擎差异：
- **agent-browser**：优先使用 `agent-browser network requests --filter ...` 和/或 `agent-browser network route ...` 直接采集/观察请求
- **goclaw-browser**：回退为 JS hook + console 解析

### `write_actions`
写模式：基于 `snapshot` refs 的受控 UI 操作（click/fill/submit/wait）。

用途：
- 点击按钮/菜单
- 填写输入并提交表单
- 等待 URL/文本变化

护栏：
- 只允许使用已允许的 `actKinds`
- 在 `actions_taken` 中保留审计记录
- 用 `snapshot` 校验操作后的页面状态

### `record_flow`
录制：把用户驱动的操作流程转换为可复用 flow 文件。

输入：
- `flowId`, `siteKey`
- `confirmToken`（默认 `REC_NEXT`）
- `finishToken`（默认 `REC_DONE`）

行为：
- 循环：`snapshot` → 提示用户执行一个动作 → 等待确认 token → `snapshot`
- 使用 `refHint` 选择器把推断出的操作追加到 `write_actions`
- 输出与 `flows/README.md` 兼容的 flow YAML

引擎差异：
- **agent-browser**：可选在录制过程中使用 `agent-browser record start/stop` 生成视频证据，并按需使用其网络/会话能力

### `paginate_or_scroll`
可选策略 step；站点配置可用于无限滚动或多页表格场景。

### `validate`
检查必填字段是否存在；缺失时可触发受控的“补采”小循环：
- snapshot → 定向交互 → snapshot → extract

### `report`
使用指定模板格式化最终输出。

## 输出约定（默认模板）

输出格式由配置控制：
- `defaults.output.format: markdown|json|both`

规则：
- 模板一律采用 Markdown，报告由 Markdown 模板渲染生成。
- `format` 包含 `markdown` 时输出 Markdown 报告。
- `format` 包含 `json` 时输出 JSON 数据文件（结构化 payload）。

默认模板：`default-report`（文件：`templates/default-report.md`）

示意结构（不必与真实模板逐字一致）：

    ## 摘要
    [1 段简短总结]

    ## 关键信息
    - title / url / login_mode / connection / mode

    ## 证据
    - snapshot_stats / screenshots / 关键引用

    ## 数据（JSON）
    { ... }

## 插件（动态插拔）

插件是通过配置启用的命名步骤包：
- 插件可在某个 step 前后插入步骤，或按约定替换某个 kind 的实现
- 插件必须声明预算影响与所需的允许动作（actKinds）

完整配置 schema 与插件约定见 `reference.md`。

## 更多资源
- 完整 schema 与高级用法：`reference.md`
- 可复制示例：`examples.md`

