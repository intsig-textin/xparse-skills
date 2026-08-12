#!/usr/bin/env python3
"""Validate the domestic WorkBuddy Connector before packaging."""

from __future__ import annotations

import json
import sys
from pathlib import Path


EXPECTED_VISIBLE_IN = ["internal", "iOA", "cloudhosted", "selfhosted"]


def load_json(path: Path) -> dict:
    with path.open(encoding="utf-8") as source:
        return json.load(source)


def fail(message: str) -> None:
    raise SystemExit(message)


def main() -> None:
    if len(sys.argv) != 6:
        fail(
            "usage: validate-review-input.py "
            "<cli> <connector-meta> <marketplace-entry> <version> <skill-root>"
        )

    cli_path, meta_path, marketplace_path, version, skill_root = map(
        Path, sys.argv[1:]
    )
    version_text = str(version)
    version_number = version_text.removeprefix("v")
    cli = load_json(cli_path)
    meta = load_json(meta_path)
    marketplace = load_json(marketplace_path)

    if marketplace.get("id") != "textin-xparse":
        fail("marketplace-entry.json must use the domestic Connector ID")
    if marketplace.get("source") != "textin-xparse" or meta.get("source") != "textin-xparse":
        fail("Connector metadata must use the domestic source")
    if marketplace.get("visible_in") != EXPECTED_VISIBLE_IN:
        fail("marketplace-entry.json visible_in does not match the approved scope")
    if marketplace.get("minWorkbuddyVersion") != "5.0.0":
        fail("marketplace-entry.json minWorkbuddyVersion must be 5.0.0")
    if meta.get("minWorkbuddyVersion") != "5.0.0":
        fail("connector-meta.json minWorkbuddyVersion must be 5.0.0")
    for field in ("description", "description_zh", "description_en"):
        if not marketplace.get(field) or marketplace[field] != meta.get(field):
            fail(f"Connector metadata field {field} is empty or inconsistent")

    if cli.get("authUrlDomain") != "api.textin.com":
        fail("cli.json must use the domestic production authorization domain")
    if cli.get("env", {}).get("XPARSE_BASE_URL") != "https://api.textin.com":
        fail("cli.json must use the domestic production base URL")
    if cli.get("versionCheck", {}).get("minVersion") != version_number:
        fail("cli.json minVersion does not match the package version")
    for platform in ("darwin", "linux", "win32"):
        init_command = cli.get("init", {}).get(platform, "")
        if f"/{version_text}/install" not in init_command or "/latest/" in init_command:
            fail(f"cli.json {platform} init is not pinned to {version_text}")

    if not (skill_root / "SKILL.md").is_file():
        fail("xparse-parse SKILL.md is missing")
    agent_config = (skill_root / "agents" / "openai.yaml").read_text(encoding="utf-8")
    if "./assets/logo.png" not in agent_config:
        fail("Skill agent config does not reference assets/logo.png")
    if not (skill_root / "assets" / "logo.png").is_file():
        fail("Skill logo is missing")


if __name__ == "__main__":
    main()
