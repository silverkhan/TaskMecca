from __future__ import annotations

import argparse
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from task_mecca.cli import cmd_update
from task_mecca.installer import MANIFEST_NAME, install


class CliUpdateTests(unittest.TestCase):
    def _conflicting_project(self, root: Path) -> Path:
        target = install(root)
        manifest_path = target / MANIFEST_NAME
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        manifest["managed_files"]["roles/root.md"]["baseline_sha256"] = "0" * 64
        manifest["task_mecca_version"] = "0.0.9"
        manifest_path.write_text(json.dumps(manifest, indent=2), encoding="utf-8")
        role = target / "roles" / "root.md"
        role.write_text("LOCAL ROLE EDIT\n", encoding="utf-8")
        return role

    def test_interactive_update_backs_up_before_overwrite(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            role = self._conflicting_project(root)
            args = argparse.Namespace(project=str(root))
            with patch("builtins.input", side_effect=["y", "y"]):
                rc = cmd_update(args)
            self.assertEqual(rc, 0)
            self.assertNotEqual(role.read_text(encoding="utf-8"), "LOCAL ROLE EDIT\n")
            backups = sorted((root / "_task_mecca" / "backups").glob("*/roles/root.md"))
            self.assertEqual(len(backups), 1)
            self.assertEqual(backups[0].read_text(encoding="utf-8"), "LOCAL ROLE EDIT\n")

    def test_interactive_update_can_cancel_before_backup(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            role = self._conflicting_project(root)
            args = argparse.Namespace(project=str(root))
            with patch("builtins.input", side_effect=["n"]):
                rc = cmd_update(args)
            self.assertEqual(rc, 1)
            self.assertEqual(role.read_text(encoding="utf-8"), "LOCAL ROLE EDIT\n")
            self.assertFalse((root / "_task_mecca" / "backups").exists())


if __name__ == "__main__":
    unittest.main()
