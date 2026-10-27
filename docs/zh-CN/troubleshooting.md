# 故障排查

[English](../troubleshooting.md) | [简体中文](troubleshooting.md)

[首页](../../README.zh-CN.md) · [使用指南](user-guide.md) · [配置](configuration.md)

先运行 `sesswitch doctor`，检查工具路径、缺失的命令和 Codex app-server 连通性。报告问题时请提供脱敏后的输出。

## 无法打开 AI Sessions

1. 运行 `vicinae ping`。如果无法连接，请从“应用程序”启动 Vicinae。
2. 如果 Vicinae 正常运行，但 AI Sessions 打不开，请检查 Vicinae 日志中的扩展错误。
3. 如果缺少 `extension-manager.js` 或扩展管理器崩溃，从 Launcher 或绑定的快捷键运行 **Repair AI Sessions**。这是不依赖扩展管理器的原生脚本，会重启 Vicinae 并重新打开 AI Sessions。如果没有此入口，运行 `make install-vicinae-repair` 安装。
4. 检查 AI Sessions 的快捷键和 **Sesswitch Binary** 配置项。

macOS 上建议使用 **⌘⌥⇧Space（Cmd+Option+Shift+Space）** 作为修复快捷键。它会重启整个 Launcher，因此其他已打开的扩展视图也会关闭。恢复日志位于 `~/.local/state/sesswitch/vicinae-repair.log`，遵循 `XDG_STATE_HOME`。
如果修复失败，请彻底退出 Vicinae，再从“应用程序”重新打开。**AI Sessions (Legacy)** 或 `sesswitch pick` 提供不依赖扩展管理器的可搜索备用列表。

macOS 上应通过正常的应用启动方式运行常驻 Vicinae。前台执行 `vicinae server --replace` 会让它依附于启动该命令的终端或工具会话。

修改源码后，`make install` 更新 CLI，`make install-vicinae-extension` 更新界面。随后请重新打开 AI Sessions。

## 会话缺失或信息过期

- 确认 hooks 已安装、加载并受信任；二进制路径变动后重新运行安装器。安装 hooks 之前的 Claude 历史不会被索引。
- 将 **Filter Status** 重置为 **All**，展开项目历史，或直接搜索会话。
- 对不会产生 hook 的变更，或已停止的订阅，使用 **Refresh Sessions** 更新或重连。出现提供方警告时，列表使用的是缓存元数据。
- 提供方确认无法恢复的历史会话会被隐藏。仍在等待输入或审批的会话可能提供 **Go to Original CLI**，但不能单独恢复。

## 打开时新建了终端，或定位到错误的应用

查看 `sesswitch list --json` 中目标会话的 `location`、`host` 和 `display_state`。位置失效、在后台或存在歧义时，会使用提供方路由，避免定位到可能不相关的线程。CLI 关闭后仍保留终端来源，因此其已保存线程仍会在终端恢复。

CLI 重启后的自动重新绑定只支持显式的 `codex resume <thread-UUID>`，且必须能够验证新进程、当前标题、工作目录，以及是否存在竞争归属。不会根据会话选择器或 `--last` 推断恢复关系。

如果窗格已切换，但窗口仍在后台，请检查 macOS 对启动器 / 终端的自动化权限。定位 Chrome 标签页同样需要自动化权限；如果无法精确路由到标签页，Sesswitch 会尝试将 Chrome 置于前台。

<a id="known-limitations"></a>

## 已知限制

- 不进行轮询：进程退出或浏览器标签页关闭后，可能需要手动刷新。
- Chrome 标签页关闭后，**Ready to review** 提醒可能仍然保留，因为待查阅状态与标签页是否存活相互独立（[#6](https://github.com/sky-bro/sesswitch/issues/6)）。将其标为已读即可确认这一轮结果，仅打开会话不会清除此状态。
- VS Code 路由只能将应用置于前台，尚不能精确定位编辑器面板。
- Sesswitch 无法远程切换运行中 CLI 内的另一线程。后台线程可以通过精确恢复命令打开，或由你在原 CLI 中自行切换。
