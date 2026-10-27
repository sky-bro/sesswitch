# CLI 参考

[English](../cli.md) | [简体中文](cli.md)

[首页](../../README.zh-CN.md) · [配置](configuration.md) · [架构](architecture.md)

会话键格式为 `codex:<thread-id>` 或 `claude:<session-id>`。可以从 Vicinae 的操作菜单或 `sesswitch list --json` 输出中复制。

| 命令 | 用途 |
| --- | --- |
| `sesswitch list [--json] [--limit N]` | 列出会话；历史记录默认上限为 100，最多 1000 |
| `sesswitch watch [--limit N]` | 本地记录变化时推送快照 |
| `sesswitch open [--target auto\|terminal\|app\|host] <key>` | 定位或恢复会话 |
| `sesswitch pick` | 通过 Vicinae dmenu 选择会话，作为文本备用入口 |
| `sesswitch rename <key> <name>` | 重命名 Codex 线程 |
| `sesswitch mark read\|done\|clear <key>` | 将一轮结果标为已读、结束任务或清除标记 |
| `sesswitch hook codex\|claude` | 从标准输入读取生命周期事件，由 hooks 调用 |
| `sesswitch doctor` | 检查工具解析结果和 Codex app-server 连通性 |
| `sesswitch version` | 输出 CLI 版本 |

## 打开目标

- `auto`（默认）：优先定位经过验证、当前正在显示会话的宿主，否则使用提供方路由。
- `terminal`：在 WezTerm 中精确恢复该会话。
- `app`：在 macOS 上打开 Codex Desktop 深链接；不支持 Claude。
- `host`：只定位到原 CLI，不选择线程，也不恢复会话。

```sh
sesswitch list --json
sesswitch open codex:THREAD_ID
sesswitch open --target terminal claude:SESSION_ID
sesswitch rename codex:THREAD_ID '排查通知投递问题'
sesswitch mark read codex:THREAD_ID
```

宿主激活失败时会停止自动打开。失效或后台位置可以回退到精确恢复；不会仅因为窗口激活失败就再启动一个 AI CLI。恢复会话需要已有的保存数据。

## JSON 集成

`list --json` 输出数组。`watch` 每行输出一个 JSON 对象：

```json
{"sessions": [], "catalog_at": "2026-01-01T00:00:00Z"}
```

快照可能包含 `warnings`。提供方查询失败时会保留缓存元数据并报告警告。历史数量上限不会排除 hooks 观察到的、尚未关闭的会话活动。每个会话包含键、提供方、标题、工作目录、更新时间、运行状态，以及可选的位置和任务数据。

`location.display_state` 的值为 `current`、`background` 或 `unknown`。`host` 保留最后验证过的终端类型，并不能证明旧位置仍然有效。
`resume: "unavailable"` 表示提供方确认没有保存数据。此类记录通常隐藏，除非仍有依附于原 CLI 的输入或审批请求。

0.x 系列的消费者必须忽略未知字段。启动器应通过此 CLI 边界集成，不应直接读取本地记录或提供方文件。
