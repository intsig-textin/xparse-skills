# 国内 WorkBuddy Connector 审核包

审核包必须从 `xparse-client` 当前提交和 `release-lock.json` 锁定的
`xparse-skills` 提交生成，不从历史目录人工复制。

```bash
./connector/package-review.sh v2.2.1 ~/Downloads
```

输出文件为 `textin-xparse-v2.2.1-cn.zip`，包内根目录为
`textin-xparse-v2.2.1/`。

生成门禁包括：

- 仅使用国内 Connector ID、域名和 OAuth Client ID；
- `visible_in` 精确匹配平台批准范围；
- CLI 安装目录和最低版本锁定到当前版本，禁止 `latest`；
- 只包含 `xparse-parse`，不包含 `xparse-doc-tools`；
- 排除 `REVIEW.md`、`.DS_Store`、`*.bak` 和 `.dev-flow`；
- 根 `icon.png` 与 Skill `assets/logo.png` 分别保留；
- 为每个交付文件重新生成 `SHA256SUMS`。

`REVIEW.md` 属于审查过程材料，不是正式 Connector 运行时文件。
两个图标当前内容相同，但分别服务 Connector 市场和 Skill Agent 配置，
不能因哈希相同删除其中一个。
