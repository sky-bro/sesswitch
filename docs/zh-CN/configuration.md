# 配置

[English](../configuration.md) | [简体中文](configuration.md)

[首页](../../README.zh-CN.md) · [使用指南](user-guide.md) · [故障排查](troubleshooting.md)

## 工具路径

通常无需额外配置。运行 `sesswitch doctor` 可以查看实际选择的工具路径及其来源。解析顺序为：环境变量覆盖、配置文件、PATH、标准安装目录。显式配置的路径无效时会直接报错。

配置文件默认为 `~/.config/sesswitch/config.json`，遵循 `XDG_CONFIG_HOME`。所有配置项均可省略：

```json
{
  "tools": {
    "codex": "~/.local/bin/codex",
    "claude": "~/.local/bin/claude",
    "node": "/opt/homebrew/bin/node",
    "tmux": "/opt/homebrew/bin/tmux",
    "wezterm": "/Applications/WezTerm.app/Contents/MacOS/wezterm"
  }
}
```

`SESSWITCH_CONFIG` 用于选择其他配置文件。`SESSWITCH_CODEX`、`SESSWITCH_CLAUDE`、`SESSWITCH_NODE`、`SESSWITCH_TMUX`、`SESSWITCH_WEZTERM` 和 `SESSWITCH_VICINAE` 可以分别覆盖对应工具的位置。路径支持 `~/`；不含目录的名称会作为命令解析。旧的 `AGENT_LOCATOR_CODEX` 覆盖变量仍然受支持。

备用查找目录涵盖 Homebrew、用户和系统二进制目录、npm 前缀、WezTerm、Volta、mise、nvm、asdf 和 fnm。不会加载 shell 启动文件。npm 安装的 AI CLI 入口使用选定的 Node，原生 AI CLI 则无需 Node。通过已有 WezTerm 启动的子进程会继承显式的工具和配置路径。

<a id="vicinae-shortcuts"></a>

## Vicinae 快捷键

在 Vicinae 设置中为 **AI Sessions** 绑定直接快捷键。扩展的 **Sesswitch Binary** 配置项默认为 `~/.local/bin/sesswitch`。

`make install-vicinae-extension` 也会安装原生 **Repair AI Sessions** 脚本。如只需安装恢复入口，可运行 `make install-vicinae-repair`。建议在 Vicinae 设置中为它绑定 **⌘⌥⇧Space（Cmd+Option+Shift+Space）**。扩展管理器崩溃时，此入口会重启 Vicinae 并重新打开 AI Sessions。

要使用使用指南中的 `⌘P` 下拉菜单和 `⌘B` 操作面板，通过 Vicinae 的 **Open Config File** 命令打开配置，并合并以下键位设置：

```json
{
  "keybinds": {
    "open-search-filter": "control+P",
    "toggle-action-panel": "control+B"
  }
}
```

Vicinae 会在 macOS 上将这些 `control` 绑定映射为 Command。将操作面板绑定到 `⌘B`，可以避开其 `⌘J/K` 导航键。

## 存储与隐私

私有记录保存在 `~/.local/state/sesswitch/`，遵循 `XDG_STATE_HOME`。其中包含 ID、标题、路径、进程身份、宿主坐标、生命周期观察记录和任务标记；文件权限为 `0600`。

Sesswitch 不需要提供方 API 密钥。Chrome 路由会按需从本地 Codex rollout 中读取选中标签页上下文；恢复终端绑定时按需读取进程参数。浏览器 URL、会话正文和进程参数都不会存入本地记录。漏洞报告方式见[安全说明](../../SECURITY.zh-CN.md)。
