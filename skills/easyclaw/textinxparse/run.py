#!/usr/bin/env python3
"""EasyClaw TextIn xParse Skill 的安全 CLI 入口。"""

from __future__ import annotations

import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
from typing import Any


XPARSE_COMMAND = "xparse-cli"
EASYCLAWCLI = "easyclawcli"
PLATFORM_ID = "textinxparse"
PROFILE = "easyclaw"
REQUIRED_CLI_VERSION = "2.2.2"
VERSION_PATTERN = re.compile(r"(?<!\d)(\d+)\.(\d+)\.(\d+)(?!\d)")

ALLOWED_COMMANDS = frozenset({
    "get_confidence", "get_doc_info", "get_outline", "parse", "quota",
    "read_content", "read_pages", "search_text",
})
CONTEXT_COMMANDS = ALLOWED_COMMANDS - {"quota"}
SAFE_SCHEMA_COMMAND = re.compile(r"^[a-z][a-z_]*$")
FORBIDDEN_OPTIONS = frozenset({
    "--access-token", "--app-id", "--authorization", "--base-url",
    "--client-id", "--client-secret", "--cookie", "--error-format",
    "--header", "--profile", "--redirect-uri",
    "--refresh-token", "--scope", "--secret-code",
    "--token", "--verbose",
})
AUTH_MARKERS = (
    "auth required", "authentication required", "invalid_grant", "login required",
    "not authenticated", "not logged in", "oauth authentication failed",
    "oauth login required", "token expired",
    "unauthorized", "未登录", "未授权", "授权已过期", "登录已过期",
)
SENSITIVE_ENV = frozenset({
    "APP_ID", "APP_SECRET", "AUTHORIZATION", "XPARSE_APP_ID",
    "XPARSE_AUTH_METHOD", "XPARSE_BASE_URL", "XPARSE_CLIENT_FROM",
    "XPARSE_COMMAND", "XPARSE_CONFIG_DIR", "XPARSE_OAUTH_CLIENT_ID", "XPARSE_SECRET_CODE",
    "XPARSE_TOKEN",
})


class Result:
    def __init__(self, code: int, stdout: str = "", stderr: str = "") -> None:
        self.code = code
        self.stdout = stdout
        self.stderr = stderr


def resolve_command(command: str) -> str | None:
    resolved = shutil.which(command)
    if resolved:
        return resolved
    if os.path.isabs(command) and os.path.isfile(command):
        return command
    normalized = Path(command).name.lower()
    if normalized in {"xparse-cli", "xparse-cli.exe"}:
        names = ("xparse-cli.exe", "xparse-cli") if os.name == "nt" else ("xparse-cli",)
        state = Path(os.environ.get("EASYCLAW_STATE_DIR", Path.home() / ".easyclaw"))
        directories = [Path.home() / ".xparse-cli" / "bin", Path.home() / ".local" / "bin"]
        for directory in directories:
            for name in names:
                candidate = directory / name
                if candidate.is_file():
                    return str(candidate)
        tools = state / "ai" / "tool_cache" / "resources" / "tools"
        if tools.is_dir():
            for name in names:
                matches = sorted(tools.glob(f"*/{name}"))
                if matches:
                    return str(matches[-1])
        return None
    if normalized not in {"easyclawcli", "easyclawcli.cmd", "easyclawcli.exe"}:
        return None
    state = Path(os.environ.get("EASYCLAW_STATE_DIR", Path.home() / ".easyclaw"))
    names = ("easyclawcli.cmd", "easyclawcli.exe", "easyclawcli") if os.name == "nt" else ("easyclawcli",)
    for name in names:
        candidate = state / name
        if candidate.is_file():
            return str(candidate)
    return None


def safe_env() -> dict[str, str]:
    return {
        key: value for key, value in os.environ.items()
        if key.upper() not in SENSITIVE_ENV
        and not key.upper().endswith("_ACCESS_TOKEN")
        and not key.upper().endswith("_REFRESH_TOKEN")
    }


def execute(command: str, args: list[str]) -> Result:
    resolved = resolve_command(command)
    if not resolved:
        return Result(127, stderr=f"command_not_found: {command}")
    try:
        completed = subprocess.run(
            [resolved, *args], capture_output=True, text=True, encoding="utf-8",
            errors="replace", env=safe_env(), shell=False,
        )
    except FileNotFoundError:
        return Result(127, stderr=f"command_not_found: {command}")
    return Result(completed.returncode, completed.stdout or "", completed.stderr or "")


def emit(payload: Any) -> None:
    print(json.dumps(payload, ensure_ascii=False, indent=2))


def combined(result: Result) -> str:
    return "\n".join(part for part in (result.stdout.strip(), result.stderr.strip()) if part)


def parse_object(value: str) -> dict[str, Any] | None:
    try:
        parsed = json.loads(value)
    except (TypeError, json.JSONDecodeError):
        return None
    return parsed if isinstance(parsed, dict) else None


def authenticated(result: Result) -> bool:
    payload = parse_object(result.stdout)
    return result.code == 0 and payload is not None and payload.get("logged_in") is True and payload.get("method") == "oauth"


def auth_error(result: Result) -> bool:
    text = combined(result).lower()
    return result.code != 0 and any(marker in text for marker in AUTH_MARKERS)


def invalid(message: str, output_json: bool) -> int:
    if output_json:
        emit({"ok": False, "error": "TEXTIN_XPARSE_INVALID_ARGUMENTS", "data": {"message": message}})
    else:
        print(f"错误：{message}", file=sys.stderr)
    return 2


def auth_payload() -> dict[str, Any]:
    return {"ok": False, "error": "TEXTIN_XPARSE_AUTH_REQUIRED", "data": {
        "need_connect": True, "platform": PLATFORM_ID,
        "next_step": f"{EASYCLAWCLI} mcp auth login -s {PLATFORM_ID}",
    }}


def open_connection(output_json: bool) -> int:
    result = execute(EASYCLAWCLI, ["mcp", "auth", "login", "-s", PLATFORM_ID])
    if output_json:
        payload = auth_payload()
        if result.code != 0:
            payload["data"]["launcher_exit_code"] = result.code
        emit(payload)
    elif result.code == 0:
        print("已打开 EasyClaw TextIn xParse 连接窗口，请完成授权后重试。")
    else:
        print("无法打开 EasyClaw 连接窗口，请确认客户端正在运行。", file=sys.stderr)
    return 3


def write_result(result: Result, output_json: bool) -> None:
    if output_json:
        payload: dict[str, Any] = {"ok": result.code == 0, "data": {
            "exit_code": result.code, "stdout": result.stdout.rstrip(), "stderr": result.stderr.rstrip(),
        }}
        if result.code != 0:
            payload["error"] = "TEXTIN_XPARSE_CLI_ERROR"
        emit(payload)
        return
    if result.stdout:
        print(result.stdout.rstrip())
    if result.stderr:
        print(result.stderr.rstrip(), file=sys.stderr)


def cli_args(args: list[str]) -> list[str]:
    return ["--profile", PROFILE, *args]


def parsed_version(value: str) -> tuple[int, int, int] | None:
    match = VERSION_PATTERN.search(value)
    if not match:
        return None
    return tuple(int(part) for part in match.groups())


def version_supported(result: Result) -> bool:
    installed = parsed_version(combined(result))
    required = parsed_version(REQUIRED_CLI_VERSION)
    return result.code == 0 and installed is not None and required is not None and installed >= required


def version_error(version: Result, output_json: bool) -> int:
    installed = parsed_version(combined(version))
    installed_text = ".".join(str(part) for part in installed) if installed else combined(version)
    data = {
        "installed_version": installed_text,
        "required_version": REQUIRED_CLI_VERSION,
        "need_update": True,
    }
    if output_json:
        emit({"ok": False, "error": "TEXTIN_XPARSE_CLI_UPDATE_REQUIRED", "data": data})
    else:
        print(
            f"xparse-cli 版本不足或无法识别，需要 {REQUIRED_CLI_VERSION} 或更高版本。",
            file=sys.stderr,
        )
    return 4


def run_check(output_json: bool) -> int:
    version = execute(XPARSE_COMMAND, ["version"])
    if version.code == 127:
        return open_connection(output_json)
    if not version_supported(version):
        return version_error(version, output_json)
    status = execute(XPARSE_COMMAND, cli_args(["auth", "status", "--output=json"]))
    if not authenticated(status):
        return open_connection(output_json)
    if output_json:
        emit({"ok": True, "data": {
            "version": combined(version), "required_version": REQUIRED_CLI_VERSION,
            "authenticated": True, "profile": PROFILE,
        }})
    else:
        print("TextIn xParse 已安装并授权。")
    return 0


def forbidden(args: list[str]) -> bool:
    lowered = [arg.lower() for arg in args]
    return any(arg in FORBIDDEN_OPTIONS or any(arg.startswith(option + "=") for option in FORBIDDEN_OPTIONS) for arg in lowered)


def paid(args: list[str]) -> bool:
    lowered = [arg.lower() for arg in args]
    return "--api=paid" in lowered or any(lowered[i:i + 2] == ["--api", "paid"] for i in range(len(lowered) - 1))


def raw_option_value(args: list[str], option: str) -> str | None:
    prefix = option + "="
    for index, arg in enumerate(args):
        lowered = arg.lower()
        if lowered.startswith(prefix):
            return arg[len(prefix):]
        if lowered == option and index + 1 < len(args):
            return args[index + 1]
    return None


def option_value(args: list[str], option: str) -> str | None:
    value = raw_option_value(args, option)
    return value.lower() if value is not None else None


def valid_task_context(args: list[str]) -> bool:
    context_path = raw_option_value(args, "--task-context")
    if not context_path:
        return False
    try:
        if Path(context_path).stat().st_size > 32 * 1024:
            return False
        payload = json.loads(Path(context_path).read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError):
        return False
    return (
        isinstance(payload, dict)
        and set(payload) == {"schema_version", "user_intent", "tool_call_reason"}
        and payload.get("schema_version") == "xparse_task_context.v1"
        and isinstance(payload.get("user_intent"), str)
        and bool(payload["user_intent"].strip())
        and isinstance(payload.get("tool_call_reason"), str)
        and bool(payload["tool_call_reason"].strip())
    )


def run_schema(args: list[str], output_json: bool) -> int:
    if len(args) > 1 or (args and (not SAFE_SCHEMA_COMMAND.fullmatch(args[0]) or args[0] not in ALLOWED_COMMANDS)):
        return invalid("schema 只接受至多一个受支持的业务命令", output_json)
    result = execute(XPARSE_COMMAND, cli_args([*args, "--help"]))
    if result.code == 127:
        return open_connection(output_json)
    write_result(result, output_json)
    return result.code


def run_query(args: list[str], output_json: bool) -> int:
    if not args or args[0].lower() not in ALLOWED_COMMANDS:
        return invalid("query 只允许受支持的解析和定向读取命令", output_json)
    if forbidden(args):
        return invalid("禁止传递凭据、请求头、Profile、服务地址或调试参数", output_json)
    if args[0].lower() in CONTEXT_COMMANDS and not valid_task_context(args):
        return invalid("业务调用必须通过 --task-context 传递有效且非空的当前任务上下文文件", output_json)
    if paid(args):
        auth_method = option_value(args, "--auth-method")
        if auth_method not in {None, "oauth"}:
            return invalid("EasyClaw 付费解析只允许 OAuth", output_json)
        if auth_method is None:
            args = [*args, "--auth-method", "oauth"]
        status = execute(XPARSE_COMMAND, cli_args(["auth", "status", "--output=json"]))
        if not authenticated(status):
            return open_connection(output_json)
    result = execute(XPARSE_COMMAND, cli_args(args))
    if result.code == 127 or auth_error(result):
        return open_connection(output_json)
    write_result(result, output_json)
    return result.code


def main() -> int:
    argv = sys.argv[1:]
    output_json = "--json" in argv
    argv = [arg for arg in argv if arg != "--json"]
    if not argv or argv[0] in {"help", "-h", "--help"}:
        print(__doc__)
        return 0
    operation, args = argv[0], argv[1:]
    if operation == "check":
        return run_check(output_json)
    if operation == "schema":
        return run_schema(args, output_json)
    if operation == "query":
        return run_query(args, output_json)
    return invalid(f"不支持的操作 '{operation}'", output_json)


if __name__ == "__main__":
    raise SystemExit(main())
