# 桌面 OAuth 客户端参数

部分兼容渠道使用公开桌面客户端的 OAuth client ID / client secret。`internal/zen/chprov/auth.go` 中的 Gemini 客户端参数与参考项目 [iJetLi/dsh-providers-hub 的 src/gemini-oauth.ts](https://github.com/iJetLi/dsh-providers-hub/blob/master/src/gemini-oauth.ts) 一致；发布前已核对，参考文件 Git blob 为 `5d9f6e84b32f2b5cccb2b8a3c1aa483c63a32d7b`。

这些参数标识公开桌面客户端，不是本项目维护者或用户的个人 API Key。桌面安装应用无法将客户端内置参数作为保密凭据；用户仍须完成上游授权，实际 access token / refresh token 保存在本地账号数据中，不随源码或安装包分发。

GitHub 推送保护可能将公开客户端参数识别为密钥。本次发布仅对经核对的公开参数标记误报，其他推送保护继续保留。新增或修改客户端参数时，应重新核对其来源、用途及授权条件，不应为未知凭据跳过检查。

另有一项微信登录 app ID 位于 `internal/zen/chprov/registry.go`，与参考插件的 [src/loomy-wechat.ts](https://github.com/iJetLi/dsh-providers-hub/blob/master/src/loomy-wechat.ts) 一致。它是授权页面公开使用的应用标识，不是用户凭据或微信 app secret。该项扫描告警同样按公开应用标识处理。
