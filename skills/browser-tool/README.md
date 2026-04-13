## browser-tool（项目级 Skill）

本目录是 `browser-tool` 的**全量结构**与**配置/示例资产**，用于把 GoClaw 内置 `browser` 工具（以及可选的 `agent-browser` CLI）组织成可复用的“采集/操作工作流”。

本仓库（`icerain_ai`）主要承载 **skill 资产**；最小可运行执行器实现位于 `goclaw` 仓库源码中。

### 你应该从哪里开始

- **入口与 DSL**：`SKILL.md`
- **配置与规则详解**：`reference.md`
- **可直接拷贝的示例**：`examples.md` + `examples/*.yaml`
- **场景库**：`config/scenarios.yaml`
- **默认值**：`config/defaults.yaml`
- **站点覆盖**：`config/sites/<siteKey>.yaml`
- **可插拔流程（flows）**：`flows/README.md`
- **执行器文档**：`docs/executor.md`
- **模板（Markdown）**：`templates/`
- **Schema（JSON Schema）**：`schemas/`
- **检查清单**：`checklists/`
- **贡献指南**：`CONTRIBUTING.md`
- **变更记录**：`CHANGELOG.md`

schemas 目录包含：
- `schemas/run.schema.json`
- `schemas/config.schema.json`
- `schemas/flow.schema.json`
- `schemas/state.schema.json`

### 与最小执行器的关系

本 skill 配套了一个最小可运行执行器（Go 代码，位于 `goclaw` 仓库）：
- 命令：`goclaw browser-tool run --file <run.yaml> --skill-dir skills/browser-tool`
- 状态保存：`goclaw browser-tool state save ...`

执行器负责把 `run.yaml` + 本目录配置解析并落地执行，产物写入 `artifacts/`（路径可在配置中覆盖）。
