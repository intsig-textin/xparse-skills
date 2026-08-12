# xParse Client 迁移说明

## 基线

- 源仓库：`https://github.com/intsig-textin/xparse-skills.git`
- 源分支：`origin/main`
- 源提交：`406a6a58663f974087fafe7203a005d9ec18d9a9`
- 目标仓库：`https://gitlab.intsig.net/xparse/xparse-client.git`

## 历史迁移方式

本仓库的初始历史由源仓库 `origin/main` 的 `cli/` 子树通过
`git subtree split` 生成。迁移只使用 `origin/main`，未导入 `test`、
`origin/test`、海外 Connector 分支或旧 tag。

迁移后的初始根提交链顶端为 `17a69fe61c363e7f26ac5024a016ed9884b6a146`。
Go module、import 和构建 ldflags 在后续迁移提交中统一改为：

```text
gitlab.intsig.net/xparse/xparse-client
```

## 兼容与发布约束

- 既有 v2.2.0 CDN 目录和安装资产不覆盖、不删除。
- 旧 GitHub tag 不批量推送到新仓；如需映射，必须逐个记录旧 tag、
  源提交、新仓提交和产物校验值。
- 国内 Connector 从 `origin/main` 基线重新迁移；`test` 分支中的既有
  实践只作为行为和测试证据，不 merge、cherry-pick 或整块套用。
- `.dev-flow` 过程记录不进入提交。
