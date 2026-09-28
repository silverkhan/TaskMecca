from __future__ import annotations

import importlib.util
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from task_mecca.installer import install


def load_runtime(target: Path):
    framework = target / "framework"
    sys.path.insert(0, str(framework))
    spec = importlib.util.spec_from_file_location("collab_tools_test", framework / "collab_tools.py")
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module, framework


def unload_runtime(module, framework: Path) -> None:
    sys.modules.pop(module.__name__, None)
    if str(framework) in sys.path:
        sys.path.remove(str(framework))


def init_git(root: Path) -> None:
    subprocess.run(["git", "init"], cwd=root, check=True, stdout=subprocess.DEVNULL)
    subprocess.run(["git", "config", "user.email", "ci@example.invalid"], cwd=root, check=True)
    subprocess.run(["git", "config", "user.name", "CI"], cwd=root, check=True)


def commit_all(root: Path, message: str) -> None:
    subprocess.run(["git", "add", "-A"], cwd=root, check=True)
    subprocess.run(["git", "commit", "-m", message], cwd=root, check=True, stdout=subprocess.DEVNULL)


class RuntimeSmokeTests(unittest.TestCase):
    def test_registrar_backlog_creation_is_lazy_and_canonical(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            self.assertFalse((target / "data").exists())

            module, framework = load_runtime(target)
            try:
                result = module.ensure_backlog()
                self.assertTrue(result["created"])
                self.assertTrue(result["canonical"])
                self.assertEqual(Path(result["path"]), target / "data" / "backlog")
                self.assertTrue((target / "data" / "backlog").is_dir())
            finally:
                unload_runtime(module, framework)

    def test_existing_legacy_backlog_is_reused_instead_of_creating_canonical(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            legacy = target / "backlog_b"
            legacy.mkdir()
            module, framework = load_runtime(target)
            try:
                result = module.ensure_backlog()
                self.assertFalse(result["created"])
                self.assertEqual(Path(result["path"]), legacy)
                self.assertFalse((target / "data" / "backlog").exists())
            finally:
                unload_runtime(module, framework)

    def test_simple_defined_and_archive_are_catalogued(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            module, framework = load_runtime(target)
            try:
                backlog = Path(module.ensure_backlog()["path"])
                (backlog / "archive" / "2026-09").mkdir(parents=True)
                (backlog / "0002.A-2.simple.todo.md").write_text(
                    """# A-2 Simple

## 작업 개요
- 등록자: test
- Agent: -
- 변경범위: -
- 선행: -
- 연관: -

## 작업 정의
### 목표
Do one thing.
### 수용 기준
- [ ] It works.

## 실행 정보
- RuntimeProvider: unknown
- Dispatch상태: unknown
- 실행근거: unknown
- Fallback근거: -

## 작업 노트
- 대기: -
- 대기유형: -
- 재개조건: -
- 대기근거: -
- 메모: -

## 결과
-

## 검증
-
""",
                    encoding="utf-8",
                )
                (backlog / "archive" / "2026-09" / "0001.A-1.defined.done.md").write_text(
                    """# A-1 Defined

## 작업 개요
- 등록자: test
- Agent: -
- 변경범위: -
- 선행: -
- 연관: -

## 요건 정의서
### 배경 및 문제
Problem.
### 목표
Goal.
### 요구사항
Requirement.
### 범위
#### 포함
In.
#### 제외
Out.
### 수용 기준
- [x] Done.
### 제약 및 보존 조건
None.

## 실행 정보
- RuntimeProvider: unknown
- Dispatch상태: completed
- 실행근거: test
- Fallback근거: -

## 작업 노트
- 대기: -
- 대기유형: -
- 재개조건: -
- 대기근거: -
- 메모: -

## 결과
Done.

## 검증
Pass.
""",
                    encoding="utf-8",
                )

                rows = module.catalog(backlog)
                self.assertEqual({row["id"] for row in rows}, {"A-1", "A-2"})
                schemas = {row["id"]: row["document"]["schema"] for row in rows}
                self.assertEqual(schemas["A-1"], "defined-v2")
                self.assertEqual(schemas["A-2"], "simple-v2")
            finally:
                unload_runtime(module, framework)

    def test_canonical_backlog_has_priority_over_legacy_candidates(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            legacy = target / "backlog_b"
            legacy.mkdir()
            canonical = target / "data" / "backlog"
            canonical.mkdir(parents=True)

            module, framework = load_runtime(target)
            try:
                candidates = module.discover_backlog_folders()
                self.assertGreaterEqual(len(candidates), 2)
                self.assertEqual(Path(candidates[0]["path"]), canonical)
                self.assertTrue(candidates[0]["canonical"])
            finally:
                unload_runtime(module, framework)

    def test_lifecycle_history_survives_pre_02_to_data_move(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            legacy = target / "backlog_b"
            legacy.mkdir()

            init_git(root)
            todo = legacy / "000001.A-1.demo.todo.md"
            todo.write_text("# A-1 demo\n", encoding="utf-8")
            commit_all(root, "register A-1")

            doing = legacy / "000001.A-1.demo.doing.md"
            todo.rename(doing)
            commit_all(root, "start A-1")

            migrated = target / "data" / "backlog_b"
            migrated.parent.mkdir(parents=True)
            shutil.move(str(legacy), str(migrated))
            commit_all(root, "move Task Mecca project data under data")

            module, framework = load_runtime(target)
            try:
                timings = module.task_state_timings(root, migrated)
                self.assertIn("A-1", timings)
                states = [row["state"] for row in timings["A-1"]["events"]]
                self.assertIn("todo", states)
                self.assertIn("doing", states)
            finally:
                unload_runtime(module, framework)


if __name__ == "__main__":
    unittest.main()
