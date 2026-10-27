# 使用指南

[English](../user-guide.md) | [简体中文](user-guide.md)

[首页](../../README.zh-CN.md) · [故障排查](troubleshooting.md) · [CLI](cli.md)

## 安装与更新

需要 macOS、Go 1.24+，以及 Codex 或 Claude Code。终端定位和恢复使用 tmux 与 WezTerm。Vicinae 扩展需要 Node.js / npm，hook 安装器需要 `jq`。请先自行安装这些工具，再按[快速开始](../../README.zh-CN.md)操作。

`make install` 会构建 CLI，并将 `~/.local/bin/sesswitch` 链接到当前检出目录。请保留该目录，并把 `~/.local/bin` 加入 PATH。使用其他路径时，需要修改 Vicinae 中的 **Sesswitch Binary** 配置项。

只需安装你使用的提供方 hooks：

```sh
make install-codex-hook
make install-claude-hook
```

安装器会保留无关设置，并在 `~/.local/state/sesswitch/backups/` 下保存带时间戳的备份。在 Codex 中打开 `/hooks`，如有提示，信任七个 Sesswitch 条目。安装 Claude Code hooks 后请重启 Claude Code。Claude 会话需要被 hooks 观察到后才能发现；Codex 还可以列出已保存的历史会话。

更新已有检出目录：

```sh
git pull --ff-only
make install
make install-vicinae-extension
```

更新后重新打开 AI Sessions。如果二进制路径变动，请重新安装 hooks。如果命令缺失，请彻底退出 Vicinae，再从“应用程序”重新打开。

## 选择与打开会话

在 Vicinae 中打开 **AI Sessions**，可在设置中为其绑定直接快捷键。支持按会话标题、项目路径、提供方或宿主搜索。按 Return 打开选中的会话，并关闭启动器。

使用搜索栏下拉菜单（`⌘P`）选择视图：

| 视图 | 排列方式 |
| --- | --- |
| Focus | Needs You、Active、Recent、Done |
| Recent | 按最近更新时间排序 |
| Project | 按完整项目路径分组，主目录下的路径会缩写 |
| Agent | 按 Codex 或 Claude 分组 |

Project 视图会把不活跃和已完成的历史会话折叠到 **Show past sessions**。按 Return 展开或折叠；搜索仍会覆盖折叠的历史会话。状态筛选位于操作菜单，与分组方式相互独立。两项选择都会保存；启用状态筛选时，底部标题会显示当前条件。

| 快捷键 | 操作 |
| --- | --- |
| Return | 打开选中的会话 |
| `⌘↓` / `⌘↑` | 在分组间跳转（macOS） |
| `⌘B` | 打开操作面板，需要按下述文档配置 |
| `⌘E` | 重命名 Codex 会话 |
| `⌘⇧R` | 将已结束的一轮结果标为已读 |
| `⌘⇧D` | 将任务标为已完成，或清除 Done 标记 |
| `⌘⇧G` | 打开 Organize By |
| `⌘⇧O` | 循环切换分组方式 |

分组导航不会隐藏任何会话：在 Agent 视图中，`⌘↓` 按显示顺序跳到下一提供方的第一个会话；在 Project 和 Focus 视图中，则在项目或状态组之间跳转。`⌘↑` 会先回到当前组开头，再跳到上一组，有时会先滚动以显示隐藏的组标题。在 Linux 上使用 `Ctrl+↓` / `Ctrl+↑`。这些是 Vicinae 内置的列表操作。

Vicinae 的操作面板和下拉菜单快捷键可以配置。如果与你的设置不一致，请参阅[配置](configuration.md#vicinae-shortcuts)。

## 理解状态

| 界面标签 | 含义 |
| --- | --- |
| Needs input / Action needed | 等待你的回复或审批 |
| Working | 观察到 AI CLI 正在活动 |
| Ready to review | 一轮对话已结束，但尚未标为已读 |
| Reviewed | 你已确认查阅这一轮结果 |
| Open | 已确认存在运行中的 CLI，但没有更明确的运行状态 |
| Idle | 提供方报告空闲 |
| Closed / Saved | CLI 已关闭，或属于已保存的历史会话 |
| Done | 你显式将任务标为已完成 |

打开会话不会自动标为已读。**Ready to review** 会保留，直到你标为已读，或新的运行事件改变其状态；它不代表整个任务已经完成。新的 AI CLI 活动会清除较早的 Done 标记。

列表在打开或执行 **Refresh Sessions** 时读取最新目录，随后持续接收本地 hook 和标记变更，不会轮询进程或提供方。宿主变化却没有产生 hook 时，请手动刷新。待查阅提醒可能在对应浏览器标签页关闭后仍保留，详见[已知限制](troubleshooting.md#known-limitations)。

**Background** 会话与其他线程共用一个 CLI，而 CLI 当前显示的是另一个线程；打开它时会使用精确恢复命令。**No saved data** 会话仅在等待输入或审批时显示，并提供 **Go to Original CLI**。此操作只定位到承载它的 CLI，需要在 CLI 内自行切换线程。
