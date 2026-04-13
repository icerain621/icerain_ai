## 配置目录说明

本目录定义 `browser-tool` 的“全局默认 + 场景库 + 站点覆盖”三层配置。

### 文件清单

- `defaults.yaml`：全局默认值（预算、允许动作、登录默认、signals、输出路径等）
- `scenarios.yaml`：场景库（可自动命中 + priority 冲突规则）
- `sites/`：站点覆盖（按 `siteKey`，通常等于域名）

### 合并优先级（高 → 低）

- run 中显式字段（例如 `run.scenario/run.output/run.login/...`）
- scenario（命中的场景）
- site（`sites/<siteKey>.yaml`）
- defaults（`defaults.yaml`）

### mode（三种运行模式）

- `read`：只采集/观察，不允许写操作
- `write`：允许交互操作
- `mixed`：混合读写；run DSL 必须为每个 step 标注 `step.mode`
