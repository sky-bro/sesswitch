# 开发指南

[English](../development.md) | [简体中文](development.md)

[首页](../../README.zh-CN.md) · [架构](architecture.md) · [贡献指南](../../CONTRIBUTING.zh-CN.md)

需要 Go 1.24+、Node.js 24+ / npm、`jq` 和 ShellCheck。本地宿主集成支持 macOS；单元测试使用模拟提供方、进程和命令，不需要提供方凭据。

## 检查

在仓库根目录执行：

```sh
make test                         # Go、hook 安装器和恢复脚本测试
go test -race ./...
go vet ./...
make build
shellcheck scripts/*.sh
```

首次检查扩展前，按锁文件安装依赖：

```sh
cd extensions/vicinae
npm ci
```

回到仓库根目录后执行：

```sh
make check-vicinae-extension      # 测试、类型检查，并构建到 bin/vicinae
```

`make check` 还会运行 `sesswitch doctor`，因此需要本地已安装的工具和可用的提供方。CI 在 macOS 上检查 Go，在使用 Node.js 24 的 Linux 上检查扩展，并用 ShellCheck 检查 shell 脚本。

## 运行与安装

```sh
./bin/sesswitch list --json
./bin/sesswitch watch
make install                     # 将 ~/.local/bin/sesswitch 链接到当前检出目录
make install-vicinae-extension    # 替换本地已安装的扩展
```

开发扩展时，在 `extensions/vicinae` 中运行 `npm run dev`。如需独立构建而不替换已安装的扩展：

```sh
npm run build -- --out /tmp/sesswitch-vicinae
```

`vicinae-env.d.ts` 根据清单自动生成，不要手动修改。更新 CLI / 扩展后，请重新打开 AI Sessions。如果需要 Vicinae 独立于开发 shell 持续运行，请从“应用程序”启动它。

<a id="code-layout"></a>

## 代码目录

| 目录 | 职责 |
| --- | --- |
| `cmd/sesswitch` | 命令、会话目录快照、hook 处理和事件流 |
| `internal/providers` | 提供方会话发现与规范化 |
| `internal/hosts` | 运行中宿主的归属发现与经过验证的定位 |
| `internal/process` | 进程身份、TTY 查找和显式恢复证据 |
| `internal/open` | 备用路由和脱离请求生命周期的 GUI 启动 |
| `internal/registry`、`internal/state`、`internal/session` | 持久化、状态解析和共享模型 |
| `internal/toolenv` | 共用的工具选择和执行环境 |
| `internal/presentation`、`extensions/vicinae` | 文本与原生启动器适配器 |

修改相关行为时，请为归属歧义、进程替换 / 退出、备用路由和状态转换添加回归测试。测试数据应为合成数据，避免存储提示词、会话正文、私有路径或凭据。
