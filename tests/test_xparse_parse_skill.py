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


if __name__ == "__main__":
    unittest.main()
