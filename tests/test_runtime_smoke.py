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
    def test_simple_defined_and_archive_are_catalogued(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            target = install(root)
            backlog = target / "backlog_demo"
            (backlog / "archive" / "2026-09").mkdir(parents=True)
            (backlog / "0002.A-2.simple.todo.md").write_text(
                """# A-2 Simple\n\n## 작업 개요\n- 등록자: test\n- Agent: -\n- 변경범위: -\n- 선행: -\n- 연관: -\n\n## 작업 정의\n### 목표\nDo one thing.\n### 수용 기준\n- [ ] It works.\n\n## 실행 정보\n- RuntimeProvider: unknown\n- Dispatch상태: unknown\n- 실행근거: unknown\n- Fallback근거: -\n\n## 작업 노트\n- 대기: -\n- 대기유형: -\n- 재개조건: -\n- 대기근거: -\n- 메모: -\n\n## 결과\n-\n\n## 검증\n-\n""",
                encoding="utf-8",
            )
            (backlog / "archive" / "2026-09" / "0001.A-1.defined.done.md").write_text(
                """# A-1 Defined\n\n## 작업 개요\n- 등록자: test\n- Agent: -\n- 변경범위: -\n- 선행: -\n- 연관: -\n\n## 요건 정의서\n### 배경 및 문제\nProblem.\n### 목표\nGoal.\n### 요구사항\nRequirement.\n### 범위\n#### 포함\nIn.\n#### 제외\nOut.\n### 수용 기준\n- [x] Done.\n### 제약 및 보존 조건\nNone.\n\n## 실행 정보\n- RuntimeProvider: unknown\n- Dispatch상태: completed\n- 실행근거: test\n- Fallback근거: -\n\n## 작업 노트\n- 대기: -\n- 대기유형: -\n- 재개조건: -\n- 대기근거: -\n- 메모: -\n\n## 결과\nDone.\n\n## 검증\nPass.\n""",
                encoding="utf-8",
            )
            subprocess.run(["git", "init"], cwd=root, check=True, stdout=subprocess.DEVNULL)
            subprocess.run(["git", "config", "user.email", "ci@example.invalid"], cwd=root, check=True)
            subprocess.run(["git", "config", "user.name", "CI"], cwd=root, check=True)
            subprocess.run(["git", "add", "."], cwd=root, check=True)
            subprocess.run(["git", "commit", "-m", "demo"], cwd=root, check=True, stdout=subprocess.DEVNULL)

            sys.path.insert(0, str(target))
            try:
                spec = importlib.util.spec_from_file_location("collab_tools_test", target / "collab_tools.py")
                module = importlib.util.module_from_spec(spec)
                assert spec.loader is not None
                sys.modules[spec.name] = module
                try:
                    spec.loader.exec_module(module)
                finally:
                    sys.modules.pop(spec.name, None)
                rows = module.catalog(backlog)
            finally:
                sys.path.remove(str(target))
            self.assertEqual({row["id"] for row in rows}, {"A-1", "A-2"})
            schemas = {row["id"]: row["document"]["schema"] for row in rows}
            self.assertEqual(schemas["A-1"], "defined-v2")
            self.assertEqual(schemas["A-2"], "simple-v2")


if __name__ == "__main__":
    unittest.main()
