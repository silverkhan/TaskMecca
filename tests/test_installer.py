from __future__ import annotations

import hashlib
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
    def test_installed_execution_protocol_and_roles_are_synchronized(self):
        with tempfile.TemporaryDirectory() as td:
            target = install(Path(td))
            source_root = Path(__file__).resolve().parents[1]
            for name in (
                "EXECUTION_PROTOCOL.md", "EXECUTION_PROTOCOL.en.md",
                "SESSION_GUIDE.md", "SESSION_GUIDE.en.md", "collab.md",
                "roles/root.md", "roles/registrar.md",
                "roles/controller.md", "roles/worker.md",
            ):
                with self.subTest(name=name):
                    installed = (target / "framework" / name).read_bytes()
                    go_template = source_root / "goassets/template/_task_mecca/framework" / name
                    self.assertEqual(installed, go_template.read_bytes())
                    if name.startswith("EXECUTION_PROTOCOL"):
                        manifest = load_manifest(target)
                        self.assertEqual(manifest["managed_files"]["framework/" + name]["policy"], "customizable")
            worker = (target / "framework/roles/worker.md").read_text()
            self.assertIn("collaboration.followup_task", worker)
            self.assertNotIn("task-mecca handoff prepare", worker)
            self.assertNotIn("task-mecca lifecycle record started", worker)

    def test_install_creates_framework_and_manifest_but_not_data(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            self.assertTrue((target / "framework" / "collab_tools.py").is_file())
            self.assertTrue((target / "framework" / "web" / "app.js").is_file())
            self.assertFalse((target / "data").exists())

            manifest = load_manifest(target)
            self.assertIsNotNone(manifest)
            self.assertEqual(manifest["task_mecca_version"], "0.2.1")
            self.assertEqual(manifest["schema_version"], 2)
            self.assertEqual(
                manifest["managed_files"]["framework/roles/root.md"]["policy"],
                "customizable",
            )
            self.assertEqual(
                manifest["managed_files"]["framework/web/app.js"]["policy"],
                "framework",
            )
            self.assertEqual(
                manifest["managed_files"]["ROOT_PROMPT.md"]["policy"],
                "customizable",
            )

    def test_local_customization_is_preserved_when_upstream_unchanged(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            role = target / "framework" / "roles" / "root.md"
            role.write_text(
                role.read_text(encoding="utf-8") + "\nLOCAL CUSTOMIZATION\n",
                encoding="utf-8",
            )
            plan = update_plan(root)
            self.assertIn("framework/roles/root.md", plan["preserve"])
            self.assertNotIn("framework/roles/root.md", plan["conflicts"])
            apply_update(root)
            self.assertIn("LOCAL CUSTOMIZATION", role.read_text(encoding="utf-8"))

    def test_conflicting_customization_can_be_backed_up_then_overwritten(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            manifest_path = target / MANIFEST_NAME
            manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
            manifest["managed_files"]["framework/roles/root.md"]["baseline_sha256"] = "0" * 64
            manifest["task_mecca_version"] = "0.1.1"
            manifest_path.write_text(json.dumps(manifest, indent=2), encoding="utf-8")

            role = target / "framework" / "roles" / "root.md"
            role.write_text("LOCAL ROLE EDIT\n", encoding="utf-8")
            plan = update_plan(root)
            self.assertIn("framework/roles/root.md", plan["conflicts"])

            backup = create_backup(
                root,
                plan["conflicts"],
                from_version=plan["installed_version"],
                to_version=plan["available_version"],
            )
            self.assertEqual(
                (backup / "framework" / "roles" / "root.md").read_text(),
                "LOCAL ROLE EDIT\n",
            )
            apply_update(root, allow_conflicts=True)
            self.assertNotEqual(role.read_text(), "LOCAL ROLE EDIT\n")
            self.assertEqual(load_manifest(target)["task_mecca_version"], "0.2.1")

    def test_update_migrates_unmodified_legacy_framework_layout(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            current = (target / "framework" / "collab_tools.py").read_bytes()
            legacy = target / "collab_tools.py"
            legacy.write_bytes(current)

            manifest_path = target / MANIFEST_NAME
            manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
            manifest["task_mecca_version"] = "0.1.1"
            manifest["schema_version"] = 1
            manifest["managed_files"].pop("framework/collab_tools.py")
            manifest["managed_files"]["collab_tools.py"] = {
                "policy": "framework",
                "baseline_sha256": hashlib.sha256(current).hexdigest(),
            }
            manifest_path.write_text(json.dumps(manifest, indent=2), encoding="utf-8")

            plan = update_plan(root)
            self.assertIn("collab_tools.py", plan["removals"])
            self.assertIn("framework/collab_tools.py", plan["safe"])
            apply_update(root)

            self.assertFalse(legacy.exists())
            self.assertTrue((target / "framework" / "collab_tools.py").is_file())
            self.assertEqual(load_manifest(target)["schema_version"], 2)

    def test_retired_unmodified_agents_snippet_is_removed_on_update(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            legacy = target / "AGENTS_TASK_MECCA_SNIPPET.md"
            legacy_bytes = b"legacy project-wide integration snippet\n"
            legacy.write_bytes(legacy_bytes)

            manifest_path = target / MANIFEST_NAME
            manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
            manifest["task_mecca_version"] = "0.1.0"
            manifest["managed_files"]["AGENTS_TASK_MECCA_SNIPPET.md"] = {
                "policy": "customizable",
                "baseline_sha256": hashlib.sha256(legacy_bytes).hexdigest(),
            }
            manifest_path.write_text(json.dumps(manifest, indent=2), encoding="utf-8")

            plan = update_plan(root)
            self.assertIn("AGENTS_TASK_MECCA_SNIPPET.md", plan["removals"])
            apply_update(root)
            self.assertFalse(legacy.exists())
            self.assertTrue((target / "ROOT_PROMPT.md").is_file())
            self.assertEqual(load_manifest(target)["task_mecca_version"], "0.2.1")

    def test_project_owned_data_is_never_touched(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            artifact = target / "data" / "measurements" / "A-1.json"
            artifact.parent.mkdir(parents=True)
            artifact.write_text('{"ok": true}\n', encoding="utf-8")
            unknown_project_dir = target / "audits"
            unknown_project_dir.mkdir()
            apply_update(root)
            self.assertEqual(artifact.read_text(encoding="utf-8"), '{"ok": true}\n')
            self.assertTrue(unknown_project_dir.is_dir())

    def test_installed_runtime_docs_are_python_first(self):
        files = bundled_files()
        for rel in (
            "framework/README.md",
            "framework/README.en.md",
            "framework/SESSION_GUIDE.md",
            "framework/SESSION_GUIDE.en.md",
            "framework/roles/root.md",
        ):
            text = files[rel].content.decode("utf-8")
            self.assertNotIn("uv run _task_mecca/framework/collab_tools.py", text)
        self.assertIn(
            "python _task_mecca/framework/collab_tools.py web",
            files["framework/README.md"].content.decode("utf-8"),
        )

    def test_manifest_declares_project_owned_patterns(self):
        manifest = build_manifest(bundled_files())
        patterns = manifest["project_owned_patterns"]
        self.assertIn("data/**", patterns)
        self.assertIn("backlog/**", patterns)
        self.assertIn("backlog_*/**", patterns)
        self.assertIn(".runtime/**", patterns)
        self.assertIn("backups/**", patterns)


if __name__ == "__main__":
    unittest.main()
