# TextIn xParse WorkBuddy Connector 修复包 v2.1.0-1

本目录是提交 WorkBuddy 连接器审核的正式材料。

## 审核入口

- `cli.json`：CLI 安装、版本检查、Device OAuth、状态检查和解绑命令。
- `connector-meta.json`：连接器名称、说明、类型和唯一试用示例。
- `marketplace-entry.json`：WorkBuddy 连接器市场条目。
- `icon.png`：TextIn xParse 正式图标。
- `skills/xparse-parse/`：唯一随连接器分发的 Skill。
- `SHA256SUMS`：包内所有审核文件的 SHA-256。

## 正式环境约束

- WorkBuddy 修复包位于独立的 `v2.1.0-1` 路径，CLI 继续复用固定的
  `v2.1.0`，不引用 `latest`。
- OAuth 域名固定为 `api.textin.com`。
- OAuth client 固定为 `cli_textin_xparse_workbuddy`。
- Connector 强制设置 `XPARSE_BASE_URL=https://api.textin.com`，安装时也会把
  `workbuddy` profile 中遗留的 pre 地址切换为正式地址，不删除登录凭证。
- CLI 默认使用免费解析，只有显式指定付费模式时才使用账号额度。
- WorkBuddy 请求携带来源标识，登录与注册链路保留 `launch_from=workbuddy`。
- 包内不包含 pre/test 配置、测试 marker 或 `xparse-doc-tools`。

## 验收建议

1. 在 WorkBuddy 连接器页面展示正式名称、说明和图标。
2. 点击添加后自动安装 CLI 与 `xparse-parse` Skill。
3. 点击连接后自动打开 Device OAuth 页面。
4. 新注册用户记录 WorkBuddy 注册来源。
5. 免费和显式付费解析请求均记录 WorkBuddy 请求来源。
6. 点击“试一试”后使用包内唯一 PDF 示例完成解析与总结。
