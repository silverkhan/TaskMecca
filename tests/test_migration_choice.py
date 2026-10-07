import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

from task_mecca.installer import apply_update, install, update_plan


class MigrationChoiceTests(unittest.TestCase):
    def test_stale_human_choice_returns_new_plan_without_writes_or_backup(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            file = target / "framework/web/app.js"
            file.write_text("first customization", encoding="utf-8")
            plan = apply_update(root)
            file.write_text("later customization", encoding="utf-8")
            result = apply_update(root, choice="backup", expected_plan=plan["plan_digest"])
            self.assertEqual(result["status"], "choice_required")
            self.assertNotEqual(result["plan_digest"], plan["plan_digest"])
            self.assertEqual(file.read_text(), "later customization")
            self.assertFalse((target / "backups").exists())

    def test_subprocess_non_tty_json_never_waits_or_changes_files(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            file = target / "framework/web/app.js"
            file.write_text("user modification", encoding="utf-8")
            manifest = (target / "manifest.json").read_bytes()
            env = {**os.environ, "PYTHONPATH": str(Path(__file__).resolve().parents[1] / "src")}
            result = subprocess.run([sys.executable, "-m", "task_mecca", "migrate", "--project", str(root), "--json"],
                                    input="", text=True, capture_output=True, env=env, timeout=10)
            self.assertEqual(result.returncode, 3, result.stderr)
            payload = json.loads(result.stdout)
            self.assertEqual(payload["status"], "choice_required")
            self.assertEqual(payload["modified_files"], ["framework/web/app.js"])
            self.assertEqual(payload["choices"], ["overwrite", "backup", "cancel"])
            self.assertEqual(file.read_text(), "user modification")
            self.assertEqual((target / "manifest.json").read_bytes(), manifest)
            self.assertFalse((target / "backups").exists())

    def test_choices_preserve_user_data_and_cancel(self):
        for choice in ("cancel", "overwrite", "backup"):
            with self.subTest(choice=choice), tempfile.TemporaryDirectory() as td:
                root = Path(td)
                target = install(root)
                file = target / "framework/web/app.js"
                file.write_text("custom", encoding="utf-8")
                for rel in ("data/backlog/private.md", "config.toml", ".runtime/notifications/telegram.json", "credentials.json"):
                    path = target / rel
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_bytes(b"private")
                result = apply_update(root, choice=choice)
                self.assertEqual(result["status"], "cancelled" if choice == "cancel" else "migrated")
                if choice == "cancel":
                    self.assertEqual(file.read_text(), "custom")
                    self.assertFalse((target / "backups").exists())
                if choice == "backup":
                    self.assertEqual((Path(result["backup_path"]) / "framework/web/app.js").read_text(), "custom")
                for rel in ("data/backlog/private.md", "config.toml", ".runtime/notifications/telegram.json", "credentials.json"):
                    self.assertEqual((target / rel).read_bytes(), b"private")

    def test_backup_failure_and_symlink_ancestor_fail_closed(self):
        with tempfile.TemporaryDirectory() as td, tempfile.TemporaryDirectory() as outside:
            root = Path(td)
            target = install(root)
            file = target / "framework/web/app.js"
            file.write_text("custom", encoding="utf-8")
            with patch("task_mecca.installer.create_backup", side_effect=OSError("backup failed")):
                with self.assertRaises(OSError):
                    apply_update(root, choice="backup")
            self.assertEqual(file.read_text(), "custom")
            web = target / "framework/web"
            web.rename(target / "framework/web-preserved")
            (Path(outside) / "app.js").write_text("private", encoding="utf-8")
            web.symlink_to(outside, target_is_directory=True)
            with self.assertRaises(RuntimeError):
                apply_update(root, choice="overwrite")
            self.assertEqual((Path(outside) / "app.js").read_text(), "private")

    def test_user_owned_manifest_path_and_read_errors_are_not_missing(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            with patch("task_mecca.installer.file_hash", side_effect=PermissionError("unreadable")):
                with self.assertRaises(PermissionError):
                    update_plan(root)
            path = target / "manifest.json"
            manifest = json.loads(path.read_text())
            manifest["managed_files"][".runtime/credentials.json"] = {"baseline_sha256": "bad"}
            path.write_text(json.dumps(manifest))
            with self.assertRaises(RuntimeError):
                apply_update(root, choice="overwrite")


if __name__ == "__main__":
    unittest.main()
