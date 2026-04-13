## Flow 合入检查清单

- **安全**
  - 不包含任何敏感信息：cookie/token/password/验证码
  - `evaluate` 中不输出敏感字段到 console
- **稳定性**
  - `refHint` 优先使用 role+name 或稳定文本
  - 避免依赖易变文案、位置、nth
  - 写操作后有 `snapshot` 复核（除非明确不需要）
- **预算**
  - 不滥用 `screenshot`，优先 `snapshot`
  - 避免无界滚动/无限重试
- **可维护**
  - `flowId/siteKey/description` 清晰
  - 需要输入参数的地方使用 `${inputs.*}`，不要硬编码业务数据
