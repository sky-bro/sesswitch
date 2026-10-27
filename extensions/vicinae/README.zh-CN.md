# Sesswitch 的 Vicinae 扩展

[English](README.md) | [简体中文](README.zh-CN.md)

这是 Sesswitch 的原生启动器适配器，负责显示 CLI 快照，并调用同一个二进制程序完成打开、重命名和标记操作。

- [安装与使用](../../docs/zh-CN/user-guide.md)
- [快捷键与二进制路径配置](../../docs/zh-CN/configuration.md#vicinae-shortcuts)
- [故障排查](../../docs/zh-CN/troubleshooting.md)
- [开发与检查](../../docs/zh-CN/development.md)

在仓库根目录执行：

```sh
make install
make install-vicinae-extension
```

在本目录开发时执行：

```sh
npm ci
npm test
npm run typecheck
npm run dev
```

`npm run build -- --out /tmp/sesswitch-vicinae` 可以构建到独立目录，不替换已安装的扩展。`vicinae-env.d.ts` 根据 `package.json` 自动生成。

Codex 的明暗主题标识取自官方桌面应用的图标资源。Claude 和 tmux 标识来自 Simple Icons。这些名称和标识属于各自所有者，仅用于说明兼容性，不代表其认可本项目。
