from __future__ import annotations

import importlib.util
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from task_mecca.installer import install


class RuntimeSmokeTests(unittest.TestCase):
    def _load_runtime(self, target: Path):
        framework = target / "framework"
        sys.path.insert(0, str(framework))
        spec = importlib.util.spec_from_file_location(
            "collab_tools_test", framework / "collab_tools.py"
        )
        module = importlib.util.module_from_spec(spec)
        assert spec.loader is not None
        sys.modules[spec.name] = module
        try:
            spec.loader.exec_module(module)
        finally:
            sys.modules.pop(spec.name, None)
        return module, framework

    def _unload_runtime(self, framework: Path) -> None:
        if str(framework) in sys.path:
            sys.path.remove(str(framework))

    def _git(self, root: Path, *args: str) -> None:
        subprocess.run(
            ["git", *args],
            cwd=root,
            check=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )

    def _init_git(self, root: Path) -> None:
        self._git(root, "init")
        self._git(root, "config", "user.email", "ci@example.invalid")
        self._git(root, "config", "user.name", "CI")

    def test_first_registration_creates_canonical_backlog_and_catalogues_archive(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            self.assertFalse((target / "data").exists())

            module, framework = self._load_runtime(target)
            try:
                presence = module.backlog_presence()
                self.assertTrue(presence["ok"])
                self.assertEqual(presence["status"], "uninitialized")

                created = module.ensure_backlog()
                self.assertTrue(created["created"])
                backlog = Path(created["path"])
                self.assertEqual(backlog.resolve(), (target / "data" / "backlog").resolve())

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
                self._unload_runtime(framework)

    def test_legacy_backlog_basename_is_discovered_after_move_under_data(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            legacy = target / "backlog_b"
            legacy.mkdir()
            (legacy / "0001.A-1.legacy.todo.md").write_text("# A-1 legacy\n", encoding="utf-8")

            module, framework = self._load_runtime(target)
            try:
                before = module.discover_backlog_folders()
                self.assertEqual(Path(before[0]["path"]).resolve(), legacy.resolve())

                data = target / "data"
                data.mkdir()
                migrated = data / "backlog_b"
                shutil.move(str(legacy), str(migrated))

                after = module.discover_backlog_folders()
                self.assertEqual(Path(after[0]["path"]).resolve(), migrated.resolve())
            finally:
                self._unload_runtime(framework)

    def test_coordinate_surfaces_completed_worker_continuity_gap(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            backlog = target / "data" / "backlog"
            backlog.mkdir(parents=True)
            (backlog / "000023.A-23.incomplete.doing.md").write_text(
                "# A-23 Incomplete\n- Agent: /root/controller/kkobugi\n- 변경범위: internal/backlog\n",
                encoding="utf-8",
            )
            runtime_dir = target / ".runtime" / "agents"
            runtime_dir.mkdir(parents=True)
            (runtime_dir / "kkobugi.json").write_text(
                '{"agent":"/root/controller/kkobugi","task_id":"A-23","state":"completed","heartbeat_at":"2026-09-30T10:00:00+09:00"}',
                encoding="utf-8",
            )

            module, framework = self._load_runtime(target)
            try:
                report = module.coordinate_report(backlog, worker_cap=3)
                gaps = report["continuity_gaps"]
                self.assertEqual(len(gaps), 1)
                self.assertEqual(gaps[0]["code"], "worker_completed_backlog_doing")
                self.assertTrue(report["controller_review_needed"])
            finally:
                self._unload_runtime(framework)

    def test_lifecycle_history_survives_legacy_to_data_move(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            self._init_git(root)

            legacy = target / "backlog_b"
            legacy.mkdir()
            todo = legacy / "000001.A-1.lifecycle.todo.md"
            todo.write_text("# A-1 lifecycle\n", encoding="utf-8")
            self._git(root, "add", ".")
            self._git(root, "commit", "-m", "register")

            doing = legacy / "000001.A-1.lifecycle.doing.md"
            todo.rename(doing)
            self._git(root, "add", "-A")
            self._git(root, "commit", "-m", "start")

            data = target / "data"
            data.mkdir()
            migrated = data / "backlog_b"
            shutil.move(str(legacy), str(migrated))
            doing = migrated / "000001.A-1.lifecycle.doing.md"
            done = migrated / "000001.A-1.lifecycle.done.md"
            doing.rename(done)
            self._git(root, "add", "-A")
            self._git(root, "commit", "-m", "migrate and complete")

            module, framework = self._load_runtime(target)
            try:
                timings = module.task_state_timings(root, migrated)
                states = [event["state"] for event in timings["A-1"]["events"]]
                self.assertEqual(states, ["todo", "doing", "done"])
                self.assertEqual(timings["A-1"]["current_state"], "done")
            finally:
                self._unload_runtime(framework)


if __name__ == "__main__":
    unittest.main()
