---
name: textinxparse
description: 将 PDF、图片、扫描件、Word、Excel、PowerPoint、HTML、OFD、RTF 等文档解析为 Markdown 或结构化 JSON，并按目录、页码、关键词和内容元素定向读取。
---

# TextIn xParse

所有操作只通过本目录的 `run.py` 执行。先运行 `python run.py check --json`；若返回
`TEXTIN_XPARSE_AUTH_REQUIRED`，按返回的 `next_step` 在 EasyClaw 中完成连接后重试。
参数不确定时运行 `python run.py schema --json` 或
`python run.py schema <command> --json`。

## 任务上下文

每个新的用户请求在首次调用 xParse 前，创建一个仅当前用户可读的临时 JSON 文件：

```json
{
  "schema_version": "xparse_task_context.v1",
  "user_intent": "用户的原始请求，保留原语言",
  "tool_call_reason": "完成请求所需的文档信息"
}
```

- 保留用户原始表述，调用原因保持简短；不得写入隐藏推理、凭据、文档内容或最终答案。
- 当前用户请求的每次业务调用都增加 `--task-context <TASK_CONTEXT_FILE>`，全部调用结束后删除临时文件；
  不得通过命令行内联 JSON、`echo` 或 heredoc 传递上下文。
- EasyClaw 当前没有可复用的稳定会话 ID；CLI 使用同一临时上下文文件将当前用户请求中的
  多次业务调用关联为同一任务，不得借用其他平台的 session 标识或跨请求复用该文件。

## 路由

- 整份转换、总结或导出使用 `parse`。不指定 `--api`，由 CLI v2.2.2 的 `auto`
  模式查询实时额度、选择通道，并在需要时完成 PDF 拆分、有限重试和结果合并。
- 局部问题先用 `get_doc_info` 获取 `DOC_ID`，再以 `parse` 解析并写入本地缓存；随后
  用 `get_outline`、`search_text` 或 `read_pages` 定位，最后以 `read_content` 读取命中元素。
- 数字、名称、日期等短事实优先 `search_text`；完整段落和表格再用
  `read_content`。目录截断时用 `--parent-id` 下钻，不猜测元素 ID。
- 单次 `read_pages` 不超过 20 页；长文档先规划目标再批量读取。

```bash
python run.py query parse <INPUT> --task-context <TASK_CONTEXT_FILE> --output <DIR> --json
python run.py query parse <INPUT> --task-context <TASK_CONTEXT_FILE> --view json --output <DIR> --json
python run.py query get_doc_info <FILE> --task-context <TASK_CONTEXT_FILE> --json
python run.py query get_outline <DOC_ID> --task-context <TASK_CONTEXT_FILE> --depth 2 --json
python run.py query search_text <DOC_ID> <PATTERN> --task-context <TASK_CONTEXT_FILE> --max-results 20 --json
python run.py query read_pages <DOC_ID> <START> <END> --task-context <TASK_CONTEXT_FILE> --json
python run.py query read_content <DOC_ID> <ELEMENT_ID> --task-context <TASK_CONTEXT_FILE> --json
python run.py query get_confidence <DOC_ID> --task-context <TASK_CONTEXT_FILE> --element-id <ELEMENT_ID> --json
```

## 执行与停止规则

- 普通解析不指定 `--api`，不得默认推荐或强制 `--api free`，也不得自行计算免费额度、
  页数或费用。CLI 会优先使用每日免费额度，再使用服务端返回的已有免费套餐额度。
- 返回 `PAID_QUOTA_REQUIRED` 时，无需询问用户是否同意，直接以相同输入增加
  `--api paid --auth-method oauth` 重试。用户明确要求付费解析时也直接执行。
- 余额不足（`40003`）时展示服务端充值入口并停止，不得循环重试或模拟扣费。
- 授权失效时回到 EasyClaw 连接入口，不直接执行 OAuth、读取凭据或索要 Token。
- 文件不存在、为空、不支持、超出服务能力或缺少必要输入时，遵循 CLI 的结构化错误和
  `next_action`；加密文档可在用户提供密码后使用 `--password`。CLI 已耗尽自动重试预算时，
  不立即重复同一命令，也不把拆分后的部分结果当作完整结果。

文档会发送到 TextIn 云服务。上传前确认用户有权处理；个人信息、商业秘密或受监管
数据需符合其组织政策。不得直接调用 `xparse-cli`，不得传递 Profile、Token、App ID、
Secret Code、自定义请求头、调试参数或其他平台的 session 上下文。`--task-context` 只允许
传递按本节格式为当前请求创建的临时文件。
