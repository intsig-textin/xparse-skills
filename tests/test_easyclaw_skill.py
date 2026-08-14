import importlib.util
import io
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock


ROOT = Path(__file__).resolve().parents[1]
SKILL_DIR = ROOT / "skills" / "easyclaw" / "textinxparse"
RUN_PATH = SKILL_DIR / "run.py"
sys.dont_write_bytecode = True


def load_run():
    spec = importlib.util.spec_from_file_location("easyclaw_xparse_run", RUN_PATH)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


class EasyClawSkillStaticTest(unittest.TestCase):
    def test_package_contains_only_runtime_files(self):
        self.assertEqual(
            sorted(path.name for path in SKILL_DIR.iterdir()),
            ["SKILL.md", "run.py"],
        )
        skill = (SKILL_DIR / "SKILL.md").read_text(encoding="utf-8")
        self.assertIn("name: textinxparse", skill)
        self.assertNotIn("workbuddy", skill.lower())
        self.assertNotIn("cli_textin_xparse", skill)
        self.assertNotIn("capabilities/resolve", skill)
        self.assertNotIn("ensure_parsed", skill)
        self.assertNotIn("--confirm-paid", skill)
        self.assertIn("无需询问用户是否同意", skill)
        self.assertIn("不得默认推荐或强制 `--api free`", skill)
        self.assertIn('"schema_version": "xparse_task_context.v1"', skill)
        self.assertIn("--task-context <TASK_CONTEXT_FILE>", skill)
        self.assertIn(
            "每次业务调用都增加 `--task-context <TASK_CONTEXT_FILE>`", skill
        )

    def test_cli_version_and_command_contract(self):
        run = load_run()
        self.assertEqual(run.REQUIRED_CLI_VERSION, "2.2.2")
        self.assertEqual(
            run.ALLOWED_COMMANDS,
            {
                "get_confidence", "get_doc_info", "get_outline", "parse", "quota",
                "read_content", "read_pages", "search_text",
            },
        )

class EasyClawRunTest(unittest.TestCase):
    def setUp(self):
        self.run = load_run()
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.context_path = str(Path(temporary.name) / "context.json")
        Path(self.context_path).write_text(
            '{"schema_version":"xparse_task_context.v1",'
            '"user_intent":"解析用户上传的文档",'
            '"tool_call_reason":"需要读取文档内容"}',
            encoding="utf-8",
        )
        self.output = mock.patch("sys.stdout", new_callable=io.StringIO)
        self.error = mock.patch("sys.stderr", new_callable=io.StringIO)
        self.output.start()
        self.error.start()
        self.addCleanup(self.output.stop)
        self.addCleanup(self.error.stop)

    def test_every_cli_operation_injects_easyclaw_profile(self):
        calls = []

        def execute(_command, args):
            calls.append(args)
            if "status" in args:
                return self.run.Result(0, '{"logged_in":true,"method":"oauth"}')
            if args == ["version"]:
                return self.run.Result(0, "xparse-cli version 2.2.2-beta.4")
            return self.run.Result(0, "ok")

        with mock.patch.object(self.run, "execute", side_effect=execute):
            self.assertEqual(self.run.run_check(True), 0)
            self.assertEqual(self.run.run_schema(["parse"], True), 0)
            self.assertEqual(
                self.run.run_query(
                    ["parse", "sample.pdf", "--task-context", self.context_path], True
                ), 0
            )
            self.assertEqual(
                self.run.run_query(
                    [
                        "parse", "sample.docx", "--task-context", self.context_path,
                        "--api", "paid", "--auth-method", "oauth",
                    ],
                    True,
                ),
                0,
            )
        xparse_calls = [args for args in calls if args != ["version"]]
        self.assertTrue(xparse_calls)
        self.assertTrue(all(args[:2] == ["--profile", "easyclaw"] for args in xparse_calls))

    def test_check_rejects_old_or_unrecognized_cli_version_before_auth(self):
        for version in (
            "xparse-cli version 2.2.0",
            "xparse-cli version 2.2.1",
            "development build",
        ):
            calls = []

            def execute(_command, args):
                calls.append(args)
                return self.run.Result(0, version)

            with self.subTest(version=version):
                with mock.patch.object(self.run, "execute", side_effect=execute):
                    self.assertEqual(self.run.run_check(True), 4)
                self.assertEqual(calls, [["version"]])

    def test_check_accepts_newer_cli_version(self):
        for version in ("xparse-cli version 2.2.2-beta.4", "xparse-cli version v2.3.0"):
            def execute(_command, args):
                if args == ["version"]:
                    return self.run.Result(0, version)
                return self.run.Result(0, '{"logged_in":true,"method":"oauth"}')

            with self.subTest(version=version):
                with mock.patch.object(self.run, "execute", side_effect=execute):
                    self.assertEqual(self.run.run_check(True), 0)

    def test_forbidden_boundaries_and_paid_oauth(self):
        forbidden = [
            ["parse", "a.pdf", "--profile", "default"],
            ["parse", "a.pdf", "--header=X-From:cli"],
            ["parse", "a.pdf", "--base-url", "https://example.com"],
        ]
        for args in forbidden:
            with self.subTest(args=args):
                self.assertEqual(self.run.run_query(args, True), 2)
        self.assertEqual(
            self.run.run_query(
                ["parse", "a.pdf", "--api", "paid", "--auth-method", "app-key"],
                True,
            ),
            2,
        )

        calls = []

        def execute(_command, args):
            calls.append(args)
            if "status" in args:
                return self.run.Result(0, '{"logged_in":true,"method":"oauth"}')
            return self.run.Result(0, "ok")

        with mock.patch.object(self.run, "execute", side_effect=execute):
            self.assertEqual(
                self.run.run_query(
                    ["parse", "a.pdf", "--task-context", self.context_path, "--api", "paid"], True
                ),
                0,
            )
            self.assertEqual(
                self.run.run_query(
                    ["parse", "secret.pdf", "--task-context", self.context_path, "--password", "secret"], True
                ),
                0,
            )
        self.assertIn(
            [
                "--profile", "easyclaw", "parse", "a.pdf",
                "--task-context", self.context_path, "--api", "paid",
                "--auth-method", "oauth",
            ],
            calls,
        )

    def test_business_query_requires_task_context(self):
        with mock.patch.object(self.run, "execute") as execute:
            self.assertEqual(self.run.run_query(["parse", "sample.pdf"], True), 2)
            invalid_context = str(Path(self.context_path).with_name("invalid.json"))
            Path(invalid_context).write_text("{}", encoding="utf-8")
            self.assertEqual(
                self.run.run_query(
                    ["parse", "sample.pdf", "--task-context", invalid_context], True
                ),
                2,
            )
        execute.assert_not_called()

    def test_task_context_file_is_forwarded_to_easyclaw_profile(self):
        calls = []

        def execute(_command, args):
            calls.append(args)
            return self.run.Result(0, "ok")

        with mock.patch.object(self.run, "execute", side_effect=execute):
            self.assertEqual(
                self.run.run_query(
                    ["parse", "a.pdf", "--task-context", self.context_path], True
                ),
                0,
            )
        self.assertEqual(
            calls,
            [[
                "--profile", "easyclaw", "parse", "a.pdf",
                "--task-context", self.context_path,
            ]],
        )

    def test_auth_failure_opens_easyclaw_connection(self):
        calls = []

        def execute(command, args):
            calls.append((command, args))
            if command == self.run.XPARSE_COMMAND:
                return self.run.Result(1, stderr="OAuth authentication failed")
            return self.run.Result(0)

        with mock.patch.object(self.run, "execute", side_effect=execute):
            self.assertEqual(
                self.run.run_query(
                    ["parse", "sample.pdf", "--task-context", self.context_path], True
                ), 3
            )
        self.assertIn(
            (self.run.EASYCLAWCLI, ["mcp", "auth", "login", "-s", "textinxparse"]),
            calls,
        )

    def test_sensitive_environment_is_removed(self):
        environ = {
            "PATH": "/bin",
            "XPARSE_COMMAND": "/tmp/fake-xparse",
            "XPARSE_CONFIG_DIR": "/tmp/other-profile",
            "XPARSE_OAUTH_CLIENT_ID": "public-client",
            "SERVICE_ACCESS_TOKEN": "secret",
        }
        with mock.patch.dict(self.run.os.environ, environ, clear=True):
            self.assertEqual(self.run.safe_env(), {"PATH": "/bin"})


if __name__ == "__main__":
    unittest.main()
