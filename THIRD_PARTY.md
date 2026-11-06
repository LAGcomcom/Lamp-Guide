# 来源与第三方组件

Lamp Guide 是 zen-gate 的 Windows 桌面重构与扩展。本仓库保留上游版权声明，不将原网关或参考实现描述为从零独立实现。

| 来源 | 使用范围 | 许可与声明 |
| --- | --- | --- |
| [LAGcomcom/zen-gate](https://github.com/LAGcomcom/zen-gate) | 网关基础、Agent 接入、渠道与路由等 | 根目录 [LICENSE](LICENSE)，保留 LAGcomcom 版权声明 |
| [dsh-our-free-model](https://github.com/Ebony-Vinyl/dsh-our-free-model)（原 zouyuxuan122 项目） | 部分渠道协议及相关实现参考 | MIT，见 [独立声明](licenses/dsh-our-free-model.txt) |
| [iJetLi/dsh-providers-hub](https://github.com/iJetLi/dsh-providers-hub) | 渠道账号、额度、目录、每日领取与聚合实现参考；新增 AStudio / AutoClaw / MagiCrew 适配核对提交 `a5b3cd10dd` | MIT，见 [独立声明](licenses/dsh-providers-hub.txt) |
| [Morpheus799/qodercn-gateway](https://github.com/Morpheus799/qodercn-gateway) | 内嵌 QoderCN 网关 | 见 [原组件完整 LICENSE](internal/zen/qoderhub/LICENSE) |
| Go 标准库、go-webview2、go-winloader、go-qrcode、x/net、x/sys、yaml.v3 | 程序运行依赖 | 具体版本由 go.mod / go.sum 固定，许可证随构建生成 |
| @resvg/resvg-js | 构建图标，非应用运行依赖 | packaging/package-lock.json 固定版本 |
| Inno Setup 及简体中文语言文件 | 构建安装向导 | 遵守所用 Inno Setup 版本的许可；语言文件来自其官方源码 |
| Microsoft Edge WebView2 Runtime | 桌面页面运行环境 | 微软条款，安装包携带微软签名的官方安装程序 |

QoderCN 组件的许可证包含对 `coolxll/lingma-ipc-proxy` 来源及授权范围的额外说明；该说明保留在组件 LICENSE 中，根目录 MIT 许可不扩大这些第三方权利。

`tools/third-party-notices.cjs` 会将本项目、渠道协议参考、内嵌 QoderCN、Go 运行时及实际链接的 Go 模块许可汇总为 `THIRD-PARTY-NOTICES.txt`，随应用安装。

Lamp Guide 灯笼 SVG 为本项目编写的矢量图标，适用本项目许可证。第三方模型、渠道与产品名称和商标仍属于各自所有者。

## 源码沿革

本次发布从本地 zen-gate 桌面重构工作树整理。原仓库发布时的 master 参考提交为 `52e1e62ff7b6817f02bb8a30541c5009c91c0515`（2026-11-05 查询）；这用于定位原项目当时的状态，不表示本重构包含该提交之前的所有变更。开发会话、真实账号、调试抓包及本地缓存不属于发布源码。
