import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SKILL_ROOT = ROOT / "skills" / "xparse-parse"


class XParseParseSkillContractTest(unittest.TestCase):
    def test_output_directory_is_created_when_missing(self):
        skill = (SKILL_ROOT / "SKILL.md").read_text(encoding="utf-8")
        guidance = (SKILL_ROOT / "references" / "cli-guidance.md").read_text(
            encoding="utf-8"
        )

        self.assertIn("creates the directory when it does not exist", skill)
        self.assertIn("creates the output directory when it does not exist", guidance)
        self.assertIn("OUTPUT_FAILED", guidance)

    def test_40422_keeps_service_error_and_original_message(self):
        error_handling = (
            SKILL_ROOT / "references" / "error-handling.md"
        ).read_text(encoding="utf-8")
        api_reference = (SKILL_ROOT / "references" / "api-reference.md").read_text(
            encoding="utf-8"
        )
        contract = error_handling + api_reference

        for value in ("40422", "SERVICE_ERROR", "message", "PROVIDE_FILE"):
            self.assertIn(value, contract)
        self.assertNotIn("INVALID_PDF", contract)
        self.assertNotIn("40422 | Password required", api_reference)

    def test_task_runtime_uses_current_output_and_password_contract(self):
        skill = (SKILL_ROOT / "SKILL.md").read_text(encoding="utf-8")
        task_runtime = (
            SKILL_ROOT / "references" / "task-runtime.md"
        ).read_text(encoding="utf-8")
        error_handling = (
            SKILL_ROOT / "references" / "error-handling.md"
        ).read_text(encoding="utf-8")
        contract = skill + task_runtime + error_handling

        for forbidden in ("--out-dir", "--passwords-stdin", "--password-map-stdin"):
            self.assertNotIn(forbidden, contract)

        for required in (
            "task export <TASK_ID> --run-id <RUN_ID> --output <DIR>",
            "task continue <TASK_ID> --password <PASSWORD>",
            "--password <SELECTOR>=<PASSWORD>",
        ):
            self.assertIn(required, contract)

    def test_task_runtime_documents_every_rerun_mode(self):
        task_runtime = (
            SKILL_ROOT / "references" / "task-runtime.md"
        ).read_text(encoding="utf-8")

        for required in (
            "task rerun <TASK_ID> --mode all",
            "task rerun <TASK_ID> --mode new-files",
            "task rerun <TASK_ID> --mode selected-files",
            "--resource-id <RESOURCE_ID>",
            "binds them to the existing Task",
        ):
            self.assertIn(required, task_runtime)

    def test_task_runtime_preserves_active_run_conflict_recovery(self):
        task_runtime = (
            SKILL_ROOT / "references" / "task-runtime.md"
        ).read_text(encoding="utf-8")
        error_handling = (
            SKILL_ROOT / "references" / "error-handling.md"
        ).read_text(encoding="utf-8")
        contract = task_runtime + error_handling

        for required in (
            "TASK_RUN_ALREADY_ACTIVE",
            "retryable=false",
            "next_action=POLL_STATUS",
            "task status <TASK_ID> --run-id <RUN_ID>",
            "do not create a replacement Task or Run",
        ):
            self.assertIn(required, contract)


if __name__ == "__main__":
    unittest.main()
