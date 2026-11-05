# 参与开发

Lamp Guide 是 zen-gate 的 Windows 桌面重构版本。桌面界面、安装包、聚合与渠道问题请提交到本仓库；原版其他平台的问题请提交到 zen-gate。

提交问题时请包含 Windows 版本、Lamp Guide 版本、复现步骤、期望结果及实际结果。涉及渠道或模型时，注明渠道名与模型 ID。截图和日志需遮盖访问 Key、账号、Cookie、订阅地址及对话内容。

开发环境和安装包构建见 [README](README.md) 与 [packaging/README.md](packaging/README.md)。提交前运行：

```powershell
go test ./...
go vet ./...
git diff --check
```

修改 JavaScript 后对每个改动文件执行 `node --check <文件>`。修改模型路由、账号同步或配置持久化时，应添加覆盖实际故障的回归测试；修改界面时，说明日夜主题与窄窗口下的验证结果。

请让每次 PR 围绕一个具体问题，写清触发条件、变更后的行为及验证方式。不要提交用户配置、真实凭据、抓包、运行日志、开发会话或本地缓存。引用上游代码时保留来源和许可证。
