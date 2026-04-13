# 可录制、可复用流程（Flows）

此目录存放可插拔到编排器中的用户录制流程（flows）。

## 什么是 flow？

flow 是一个可复用的步骤包（通常是 write-mode 操作），可用于：
- 在某个 step 前后插入（例如在 `ensure_login` 之后）
- 在 run DSL 中按名称调用
- 通过站点配置启用

## 目录结构

推荐：
- `flows/<siteKey>/<flowId>.yaml`

示例：
- `flows/example.com/create-ticket.yaml`

## Flow schema（最小）

```yaml
version: 1
flowId: "create-ticket"
siteKey: "example.com"
description: "创建工单并停留在确认页面。"

insertAfter: "ensureLogin"   # 可选

requires:
  mode: "write"
  allowActKinds: ["click","type","press","wait"]

steps:
  - id: snap0
    kind: snapshot
    with: { compact: true, interactive: true, maxChars: 8000 }
  - id: doWrites
    kind: write_actions
    with:
      ops:
        - { kind: click, refHint: { kind: by_text, value: "New ticket" } }
        - { kind: type,  refHint: { kind: by_text, value: "Subject" }, text: "${inputs.subject}" }
        - { kind: type,  refHint: { kind: by_text, value: "Description" }, text: "${inputs.description}" }
        - { kind: click, refHint: { kind: by_role_name, role: "button", name: "Submit" } }
        - { kind: wait, request: { kind: wait, text: "Ticket created" } }
  - id: snap1
    kind: snapshot
    with: { compact: true, interactive: true, maxChars: 8000 }
```

## 如何把 flow 插入 skill

flow 可通过站点配置（推荐）或 run DSL 启用：
- 站点配置可在 `plugins` 或 `flows.enabled`（如采用该约定）中启用
- run DSL 也可直接包含一个展开 flow steps 的 step（按约定实现）

## 录制 flow（用户驱动）

在没有浏览器事件录制 API 的情况下，编排器通过以下方式“录制”flow：
- 使用 `snapshot` 获取稳定的 refs
- 让用户一次只执行一个操作
- 再次 `snapshot` 确认状态

然后由 agent 生成基于 `refHint` 的 `write_actions` 操作列表，以提升跨会话稳定性。
