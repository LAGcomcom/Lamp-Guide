# 从 zen-gate 迁移到 Lamp Guide

Lamp Guide 是 zen-gate 的 Windows 桌面重构版本，使用独立仓库及版本号。目前支持 Windows 10 / 11 x64；原版其他平台发行包仍在 [zen-gate](https://github.com/LAGcomcom/zen-gate/releases)。

## 安装前

1. 正常退出 zen-gate，备份 `%APPDATA%\zen-gate`；若配置了 `ZEN_GATE_HOME`，备份实际目录。
2. 从 [Lamp Guide Releases](https://github.com/LAGcomcom/Lamp-Guide/releases/latest) 下载安装包，并核对同名 SHA-256 文件。
3. 按向导安装。程序安装目录独立，账号和配置继续从原数据目录读取。

## 首次启动

- 在“服务”核对端口和 Base URL；已有客户端沿用原地址和访问 Key。
- 在“渠道”核对登录状态；授权过期的账号可能需要重新登录。
- 在“聚合”核对模型与渠道参与状态，确认需要使用的渠道已开启。
- 需要调用积分模型时，展开模型查看各渠道返回的倍率和余额。
- 原版与重构版不能同时占用同一端口；两者共享数据，避免同时启动和修改配置。

## 更新与回退

Lamp Guide 从本项目 Releases 检查更新。继承的 zen-gate 默认更新地址在本程序中按 Lamp Guide 解释，读取时不改写共享配置；自定义更新地址继续保留。下载新安装包、正常退出程序、安装到原目录即可升级。

两个项目的版本号互相独立。旧仓库的重构版介绍发布不替换原版自动更新资产。

需要回到原版时，先退出 Lamp Guide，再启动原版。若新版配置与原版行为存在差异，退出全部实例后恢复安装前的数据备份。卸载 Lamp Guide 本身不会恢复配置，也不会删除用户数据。

## 数据与网络

网关数据默认在 `%APPDATA%\zen-gate`，WebView2 与主题数据默认在 `%LOCALAPPDATA%\ZenUI\webview`。卸载保留这些目录及共享 WebView2 环境。

渠道请求会连接相应上游；更新检查和公告会访问所配置的来源。配置统计服务地址后，网关会发送安装标识、版本及出口国家的心跳；未配置地址时不发送该心跳。不要公开上传账号配置、日志中的授权内容、Cookie 或订阅链接。
