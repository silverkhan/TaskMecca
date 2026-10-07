from __future__ import annotations

import argparse
import json
import io
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from task_mecca.cli import cmd_init, cmd_update
from task_mecca.installer import MANIFEST_NAME, install


class CliInitTests(unittest.TestCase):
    def test_init_preserves_existing_agents_md_without_prompting(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            agents = root / "AGENTS.md"
            original = "# Existing project instructions\n"
            agents.write_text(original, encoding="utf-8")
            args = argparse.Namespace(project=str(root))
            with patch(
                "builtins.input",
                side_effect=AssertionError("init must not prompt for AGENTS.md"),
            ):
                rc = cmd_init(args)
            self.assertEqual(rc, 0)
            self.assertEqual(agents.read_text(encoding="utf-8"), original)
            self.assertTrue((root / "_task_mecca" / "ROOT_PROMPT.md").is_file())
            self.assertTrue(
                (root / "_task_mecca" / "framework" / "collab_tools.py").is_file()
            )
            self.assertFalse((root / "_task_mecca" / "data").exists())

    def test_init_output_uses_project_local_python_runtime(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            args = argparse.Namespace(project=str(root))
            stream = io.StringIO()
            with patch("sys.stdout", stream):
                rc = cmd_init(args)
            self.assertEqual(rc, 0)
            output = stream.getvalue()
            self.assertIn(
                "python _task_mecca/framework/collab_tools.py web",
                output,
            )
            self.assertNotIn("uvx", output)
            self.assertNotIn("uv run", output)

    def test_init_does_not_create_agents_md_or_backlog(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            args = argparse.Namespace(project=str(root))
            with patch(
                "builtins.input",
                side_effect=AssertionError("init must not prompt for AGENTS.md"),
            ):
                rc = cmd_init(args)
            self.assertEqual(rc, 0)
            self.assertFalse((root / "AGENTS.md").exists())
            self.assertFalse((root / "_task_mecca" / "data").exists())


class CliUpdateTests(unittest.TestCase):
    def _conflicting_project(self, root: Path) -> Path:
        target = install(root)
        manifest_path = target / MANIFEST_NAME
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        manifest["managed_files"]["framework/roles/root.md"]["baseline_sha256"] = "0" * 64
        manifest["task_mecca_version"] = "0.1.1"
        manifest_path.write_text(json.dumps(manifest, indent=2), encoding="utf-8")
        role = target / "framework" / "roles" / "root.md"
        role.write_text("LOCAL ROLE EDIT\n", encoding="utf-8")
        return role

    def test_interactive_update_backs_up_before_overwrite(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            role = self._conflicting_project(root)
            args = argparse.Namespace(project=str(root), choice=None, json=False, expect_plan=None)
            with patch("sys.stdin.isatty", return_value=True), patch("builtins.input", side_effect=["2"]):
                rc = cmd_update(args)
            self.assertEqual(rc, 0)
            self.assertNotEqual(role.read_text(encoding="utf-8"), "LOCAL ROLE EDIT\n")
            backups = sorted(
                (root / "_task_mecca" / "backups").glob(
                    "*/framework/roles/root.md"
                )
            )
            self.assertEqual(len(backups), 1)
            self.assertEqual(
                backups[0].read_text(encoding="utf-8"), "LOCAL ROLE EDIT\n"
            )

    def test_interactive_update_can_cancel_before_backup(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            role = self._conflicting_project(root)
            args = argparse.Namespace(project=str(root), choice=None, json=False, expect_plan=None)
            with patch("sys.stdin.isatty", return_value=True), patch("builtins.input", side_effect=["3"]):
                rc = cmd_update(args)
            self.assertEqual(rc, 0)
            self.assertEqual(role.read_text(encoding="utf-8"), "LOCAL ROLE EDIT\n")
            self.assertFalse((root / "_task_mecca" / "backups").exists())


if __name__ == "__main__":
    unittest.main()
