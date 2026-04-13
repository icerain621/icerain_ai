## 站点接入检查清单

### 站点配置（`config/sites/<siteKey>.yaml`）

- **基本字段**
  - `siteKey` 正确（通常为域名）
  - `match.domains/urlPrefixes/urlRegex`（至少配置 domains）
- **登录检测 signals**
  - `detect.loginRequiredSignals` 至少 1 条（避免误判不登录）
  - `detect.loggedInSignals` 至少 1 条（强烈建议，避免无法判定成功）
  - `detect.antiBotSignals` 至少 1 条（建议，避免自动化死循环）
- **登录策略**
  - `login.mode` 选择正确（推荐 `auto_then_manual`）
  - `login.auto.strategies` 顺序建议：
    1) `storage_state`
    2) `cookie`
    3) `form`
- **mixed 模式（如使用）**
  - `run.mode=mixed` 时，所有步骤均设置 `step.mode`

### 场景（`config/scenarios.yaml`）

- `scenarioId` 唯一
- `priority` 设计合理（越具体越高）
- `match` 不要过宽（避免误命中）
- `output.reportPath/dataPath` 不要覆盖其他场景产物

### 测试（最小）

- 先跑 `examples/run-minimal.yaml` 验证基础能力
- 再跑登录示例（manual 或 auto_then_manual）验证 signals 与策略链
