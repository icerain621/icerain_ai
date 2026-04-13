## 安全检查清单（必过）

### 凭据与敏感信息

- 不在任何 YAML/flow/template 中写入明文凭据
- 所有凭据必须通过引用：
  - `env:XXX`（环境变量）
  - `file:...`（状态文件，仅允许无敏感明文或可接受范围）
- 禁止将以下内容写入 artifacts / console：
  - `Authorization`、`Cookie`、`Set-Cookie`
  - token、password、验证码、人机验证信息

### URL 与脚本

- 仅允许 `https://`（必要时再加 `http://`）
- 禁止 `file://`、`chrome://`、`devtools://`
- `evaluate` 仅用于必要的采集/注入，避免执行站点提供的任意脚本片段

### state 复用边界

- `storage_state` 通过 `document.cookie` 注入，无法写入 HttpOnly cookie
- state 文件应视为敏感资产：纳入访问控制与清理策略
