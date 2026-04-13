## 变更记录

### Unreleased

- 新增最小可运行执行器（`goclaw browser-tool run`）
- 新增 `read/write/mixed` 三种模式，并在 mixed 模式要求 `steps[].mode`
- 新增场景自动命中（match + priority）
- 新增登录闭环：
  - `ensure_login` 支持 signals 判断
  - `manual` 手动确认
  - `auto_then_manual`（`storage_state/cookie/form`）→ 失败回退手动
- 新增 `storage_state` 策略与 state 保存命令（`goclaw browser-tool state save`）
- 补齐 skill 标准目录结构：`docs/`, `templates/`, `schemas/`, `checklists/`
