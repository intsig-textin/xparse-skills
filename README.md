# xParser CLI

Textin xParser 文档解析命令行工具，基于 [Textin xParser API](https://www.textin.com) 实现。

支持将 PDF、图片、Office 文档等 20+ 格式转换为 Markdown 及结构化数据。

## 安装

### 一键安装

**Linux / macOS**

```bash
source <(curl -fsSL https://dllf.intsig.net/download/2026/Solution/xparse-cli/install.sh)
```

**Windows (PowerShell)**

```powershell
irm https://dllf.intsig.net/download/2026/Solution/xparse-cli/install.ps1 | iex
```

### 从源码构建

> 要求 Go 1.23+

**单平台构建（当前系统）：**

```bash
cd cli
go build -o xparse-cli .
```

**带版本信息构建：**

```bash
VERSION=v0.0.1
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
PKG=github.com/intsig-textin/xparse-skills/cli/cmd

go build -ldflags "-s -w \
  -X ${PKG}.version=${VERSION} \
  -X ${PKG}.commit=${COMMIT} \
  -X ${PKG}.date=${DATE}" \
  -o xparse-cli .
```

**交叉编译全平台：**

```bash
VERSION=v0.0.1
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
PKG=github.com/intsig-textin/xparse-skills/cli/cmd
LDFLAGS="-s -w -X ${PKG}.version=${VERSION} -X ${PKG}.commit=${COMMIT} -X ${PKG}.date=${DATE}"

GOOS=linux   GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/xparse-cli-linux-amd64 .
GOOS=linux   GOARCH=arm64 go build -ldflags "$LDFLAGS" -o dist/xparse-cli-linux-arm64 .
GOOS=darwin  GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/xparse-cli-darwin-amd64 .
GOOS=darwin  GOARCH=arm64 go build -ldflags "$LDFLAGS" -o dist/xparse-cli-darwin-arm64 .
GOOS=windows GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/xparse-cli-windows-amd64.exe .
GOOS=windows GOARCH=arm64 go build -ldflags "$LDFLAGS" -o dist/xparse-cli-windows-arm64.exe .
```

产物位于 `cli/dist/` 目录，共 6 个二进制文件：

| 平台 | 文件 |
|------|------|
| Linux x86_64 | `xparse-cli-linux-amd64` |
| Linux ARM64 | `xparse-cli-linux-arm64` |
| macOS Intel | `xparse-cli-darwin-amd64` |
| macOS Apple Silicon | `xparse-cli-darwin-arm64` |
| Windows x86_64 | `xparse-cli-windows-amd64.exe` |
| Windows ARM64 | `xparse-cli-windows-arm64.exe` |

## 快速开始

### 1. 零配置解析（免费 API）

```bash
# 输出 Markdown 到终端
xparse-cli parse report.pdf

# JSON 视图
xparse-cli parse report.pdf --view json

# 保存到目录
xparse-cli parse report.pdf --output ./output/

# 指定页码范围
xparse-cli parse report.pdf --page-range "1-5"

# 加密 PDF
xparse-cli parse secret.pdf --password mypassword
```

### 2. 付费 API（可选）

前往 [Textin 控制台](https://www.textin.com/user/login?redirect=%252Fconsole%252Fdashboard%252Fsetting&from=xparse-parse-skill) 获取凭证（`x-ti-app-id` 和 `x-ti-secret-code`），然后运行：

```bash
xparse-cli auth
```

按提示输入 App ID 和 Secret Code，凭证将保存至 `~/.xparse-cli/config.yaml`。

也可通过环境变量配置（适合 CI/CD）：

```bash
export XPARSE_APP_ID=your_app_id
export XPARSE_SECRET_CODE=your_secret_code
```

```bash
# 显式使用付费 API
xparse-cli parse report.pdf --api paid
```

## 命令一览

| 命令 | 说明 |
|------|------|
| `xparse-cli parse` | 解析文档，输出 Markdown / JSON |
| `xparse-cli auth` | 配置 API 凭证（交互式） |
| `xparse-cli config` | 管理配置（show / set / reset / path） |
| `xparse-cli download` | 下载解析结果中 elements 的图片 |
| `xparse-cli update` | 自更新 CLI 到最新版本 |
| `xparse-cli version` | 显示版本信息 |

## parse 命令参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--view` | `markdown` | 输出视图：`markdown`、`json` |
| `--api` | _(auto)_ | API 模式：`free`、`paid` |
| `--page-range` | | 页码范围：`"1-5"` 或 `"1-2,5-10"` |
| `--password` | | 加密文档密码 |
| `--include-char-details` | `false` | 返回字符级坐标和置信度 |
| `--list` | | 从文件读取输入列表（需配合 `--output`） |
| `--output` | _(stdout)_ | 输出文件路径或目录（目录须已存在） |

**全局参数（所有命令均支持）：**

| 参数 | 说明 |
|------|------|
| `--app-id` | Textin App ID（覆盖环境变量和配置文件） |
| `--secret-code` | Textin Secret Code（覆盖环境变量和配置文件） |
| `--base-url` | API 地址（私有化部署时使用） |
| `--verbose` | 调试模式，打印 HTTP 请求详情 |

### API capabilities 默认值

CLI 默认开启以下能力，Agent 无需额外配置：

| 能力 | 默认 |
|------|------|
| 标题层级 | 开启 |
| 内嵌对象（图片） | 开启 |
| 图片数据 | 开启 |
| 表格结构（HTML） | 开启 |
| 分页结果 | 开启 |
| 目录树 | 开启 |
| 字符级详情 | **关闭**（`--include-char-details` 开启） |

## 使用示例

### 管道组合

```bash
# 解析并搜索
xparse-cli parse report.pdf | grep "revenue"

# 解析并喂给 LLM
xparse-cli parse paper.pdf | llm "summarize this paper"
```

### 批量处理

```bash
# 从文件列表读取
xparse-cli parse --list files.txt --output ./results/
```

### 下载图片

```bash
# 从解析结果 JSON 中提取 elements 图片并下载
xparse-cli download --from result.json --output ./images/

# 直接下载图片 URL
xparse-cli download https://web-api.textin.com/ocr_image/external/abc123.jpg --output ./images/
```

### 配置管理

```bash
xparse-cli config show
xparse-cli config set base_url https://your-server.com
xparse-cli config reset
xparse-cli config path
```

### 自更新

```bash
xparse-cli update
```

### 调试模式

```bash
xparse-cli parse report.pdf --verbose
```

## 凭证管理

| 优先级 | 方式 | 说明 |
|--------|------|------|
| 1 | 命令行参数 | `--app-id` 和 `--secret-code` |
| 2 | 环境变量 | `XPARSE_APP_ID` 和 `XPARSE_SECRET_CODE` |
| 3 | 配置文件 | `~/.xparse-cli/config.yaml` |

## 退出码与错误处理

| 码 | 含义 | stderr 格式 |
|----|------|-------------|
| 0 | 成功 | — |
| 1 | 一般错误 / 网络异常 | 纯文本 + `> [tag] suggestion` |
| 2 | 参数错误 | 纯文本 + `> [tag] suggestion` |
| 3 | API 返回错误 | `api_code：message` + `> [tag] suggestion` |

每条错误输出到 stderr，格式：

```
<错误信息>
> <建议操作>
  (request_id: xxx, contact Textin support if unresolved)   ← 部分 API 错误额外输出
```

第二行以 `>` 开头，包含 `[tag]` 标签指示处理方式：

| 标签 | 含义 |
|------|------|
| `[fix]` | 修正参数后重新执行 |
| `[retry]` | 自动重试（带退避） |
| `[fallback]` | 尝试替代方案 |
| `[ask human]` | 需要人工介入 |

示例：

```
invalid --view value, must be 'markdown' or 'json'
> [fix] use --view markdown or --view json
```

```
40306：服务暂时不可用
> [retry] wait 3s then retry, max 2 retries
  (request_id: 644e2efdb..., contact Textin support if unresolved)
```

> stdout 仅输出文档内容，stderr 仅输出错误信息，exit code 严格为 0/1/2/3。
> 完整的错误和建议枚举见 [suggestion.txt](suggestion.txt)。

## 支持的文件格式

| 类型 | 格式 |
|------|------|
| 文档 | PDF, DOC, DOCX, TXT, RTF, OFD |
| 图片 | PNG, JPG, JPEG, BMP, TIFF, WebP |
| 表格 | XLS, XLSX, CSV |
| 演示 | PPT, PPTX |
| 网页 | HTML, MHTML |

限制：

| 限制项 | 免费 API | 付费 API |
|--------|----------|----------|
| 文件大小 | 10MB | 500MB |
| PDF 页数 | — | 1000 页 |
| XLS/XLSX/CSV | — | 每 sheet ≤ 2000 行 × 100 列 |
| TXT | — | ≤ 100KB |
| 图片尺寸 | 20～20000 像素 | 20～20000 像素 |
