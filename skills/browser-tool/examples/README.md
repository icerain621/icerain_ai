## examples 目录

这里放“可直接运行”的 `run.yaml` 示例，便于用最小执行器快速验证。

### 示例清单

- `run-minimal.yaml`：最小 read 采集闭环（connect/open/snapshot/extract/report）
- `run-manual-login.yaml`：手动登录闭环（ensure_login + signals 校验）
- `run-auto-then-manual.yaml`：自动优先（storage_state/cookie/form）→ 失败回退手动
