# 贡献指南

[English](CONTRIBUTING.md) | [简体中文](CONTRIBUTING.zh-CN.md)

环境准备和必需检查见[开发指南](docs/zh-CN/development.md)；提供方、宿主与启动器的职责边界见[架构](docs/zh-CN/architecture.md)。

提交前请确认：

- 按开发指南运行 Go 竞态测试、`go vet`、构建、扩展检查和 shell 脚本检查。
- 为行为变更添加回归测试，尤其是进程归属、路由、状态转换和持久化。
- 在 0.x 系列中保留已有的 JSON 字段；消费者应忽略新增的未知字段。
- 使用合成测试数据。不要提交会话记录、提示词、API 密钥、私有路径或凭据。
- 修改用户可见行为时，同步更新相关文档的中英文版本。

报告问题时，请提供 Sesswitch / macOS 版本、提供方和宿主、预期行为、复现步骤，以及脱敏后的诊断信息。
安全问题请按[安全说明](SECURITY.zh-CN.md)中的方式私密报告。
