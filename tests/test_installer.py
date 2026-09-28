from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from task_mecca.installer import (
    MANIFEST_NAME,
    apply_update,
    build_manifest,
    bundled_files,
    create_backup,
    install,
    load_manifest,
    update_plan,
)


class InstallerTests(unittest.TestCase):
    def test_install_creates_runtime_and_manifest(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            self.assertTrue((target / "collab_tools.py").is_file())
            self.assertTrue((target / "web" / "app.js").is_file())
            manifest = load_manifest(target)
            self.assertIsNotNone(manifest)
            self.assertEqual(manifest["task_mecca_version"], "0.1.0")
            self.assertEqual(manifest["managed_files"]["roles/root.md"]["policy"], "customizable")
            self.assertEqual(manifest["managed_files"]["web/app.js"]["policy"], "framework")

    def test_local_customization_is_preserved_when_upstream_unchanged(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            role = target / "roles" / "root.md"
            role.write_text(role.read_text(encoding="utf-8") + "\nLOCAL CUSTOMIZATION\n", encoding="utf-8")
            plan = update_plan(root)
            self.assertIn("roles/root.md", plan["preserve"])
            self.assertNotIn("roles/root.md", plan["conflicts"])
            apply_update(root)
            self.assertIn("LOCAL CUSTOMIZATION", role.read_text(encoding="utf-8"))

    def test_conflicting_customization_can_be_backed_up_then_overwritten(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            manifest_path = target / MANIFEST_NAME
            manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
            # Simulate an older upstream baseline so the current bundled role represents a newer upstream revision.
            manifest["managed_files"]["roles/root.md"]["baseline_sha256"] = "0" * 64
            manifest["task_mecca_version"] = "0.0.9"
            manifest_path.write_text(json.dumps(manifest, indent=2), encoding="utf-8")

            role = target / "roles" / "root.md"
            role.write_text("LOCAL ROLE EDIT\n", encoding="utf-8")
            plan = update_plan(root)
            self.assertIn("roles/root.md", plan["conflicts"])

            backup = create_backup(
                root,
                plan["conflicts"],
                from_version=plan["installed_version"],
                to_version=plan["available_version"],
            )
            self.assertEqual((backup / "roles" / "root.md").read_text(), "LOCAL ROLE EDIT\n")
            apply_update(root, allow_conflicts=True)
            self.assertNotEqual(role.read_text(), "LOCAL ROLE EDIT\n")
            self.assertEqual(load_manifest(target)["task_mecca_version"], "0.1.0")

    def test_project_owned_backlog_is_never_touched(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            backlog = target / "backlog_demo" / "A-1_todo.md"
            backlog.parent.mkdir()
            backlog.write_text("# A-1 project data\n", encoding="utf-8")
            apply_update(root)
            self.assertEqual(backlog.read_text(encoding="utf-8"), "# A-1 project data\n")

    def test_manifest_declares_project_owned_patterns(self):
        manifest = build_manifest(bundled_files())
        patterns = manifest["project_owned_patterns"]
        self.assertIn("backlog_*/**", patterns)
        self.assertIn(".runtime/**", patterns)
        self.assertIn("backups/**", patterns)


if __name__ == "__main__":
    unittest.main()
