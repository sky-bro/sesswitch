# Sesswitch

[English](README.md) | [简体中文](README.zh-CN.md)

[![CI](https://github.com/sky-bro/sesswitch/actions/workflows/ci.yml/badge.svg)](https://github.com/sky-bro/sesswitch/actions/workflows/ci.yml)
[![许可证：MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

找到需要你处理的 AI 会话，快速切回现场。

Sesswitch 将 Codex 和 Claude Code 会话汇总到一个可搜索的列表中，显示工作、等待审批和待查阅状态，定位到经过验证的 tmux / WezTerm 窗格；原终端不可用时，也可以恢复已保存的会话。

## 快速开始

目前支持 **macOS**，需要 Go 1.24+ 和对应的 AI CLI。终端定位与恢复使用 tmux / WezTerm。可选的 Vicinae 扩展需要 Node.js / npm，安装 hooks 需要 `jq`。

```sh
git clone https://github.com/sky-bro/sesswitch.git
cd sesswitch
make install
sesswitch doctor
```

按需安装提供方的 hooks，再安装启动器扩展：

```sh
make install-codex-hook       # Codex
make install-claude-hook      # Claude Code
make install-vicinae-extension
```

在 Vicinae 中打开 **AI Sessions**，并在设置中绑定快捷键。Codex 可能要求你在 `/hooks` 中信任安装的条目。安装和日常操作详见[使用指南](docs/zh-CN/user-guide.md)。

## 支持情况

| 功能 | Codex | Claude Code |
| --- | --- | --- |
| 会话发现 | 通过 app-server 读取历史 | 通过 hooks 记录观察到的会话 |
| 精确定位终端与恢复会话 | 支持 | 支持 |
| 重命名 | 支持 | 暂无受支持的 API |
| 桌面应用 / 浏览器路由 | Codex Desktop 和 Chrome | 在终端恢复 |

项目目前提供 Vicinae 启动器。尚不支持精确定位 VS Code 面板，也未提供 Raycast 或 Rofi 适配器。

## 文档

- [使用指南](docs/zh-CN/user-guide.md)：安装、快捷键、分组和状态含义。
- [CLI 参考](docs/zh-CN/cli.md)：命令与 JSON 集成。
- [配置](docs/zh-CN/configuration.md)：工具路径和本地存储。
- [故障排查](docs/zh-CN/troubleshooting.md)：会话缺失和打开失败。
- [开发指南](docs/zh-CN/development.md)：检查、扩展开发和代码目录。
- [架构](docs/zh-CN/architecture.md)：状态、归属、路由和恢复机制。
- [贡献指南](CONTRIBUTING.zh-CN.md)：提交要求和贡献约定。
- [安全说明](SECURITY.zh-CN.md)：私密报告漏洞和本地数据边界。

## 许可证

采用 [MIT](LICENSE) 许可证。提供方名称和标识仅用于说明兼容的服务；Sesswitch 与 OpenAI、Anthropic、tmux、WezTerm 或 Vicinae 没有隶属关系。
