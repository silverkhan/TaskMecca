from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path

from . import __version__
from .installer import TARGET_DIR, apply_update, create_backup, install, update_plan


def _yes_no(prompt: str, *, default: bool = False) -> bool:
    suffix = " [Y/n] " if default else " [y/N] "
    while True:
        try:
            value = input(prompt + suffix).strip().lower()
        except EOFError:
            return default
        if not value:
            return default
        if value in {"y", "yes"}:
            return True
        if value in {"n", "no"}:
            return False
        print("Please answer y or n.")


def _project_root(value: str | None) -> Path:
    return Path(value or os.getcwd()).expanduser().resolve()


def _runtime_script(project_root: Path) -> Path:
    return project_root / TARGET_DIR / "framework" / "collab_tools.py"


def cmd_init(args: argparse.Namespace) -> int:
    root = _project_root(args.project)
    print(f"Task Mecca {__version__} — initialize")
    print(f"Project: {root}\n")
    try:
        target = install(root)
    except FileExistsError as exc:
        print(f"Not installed: {exc}")
        print("If this is an existing Task Mecca project, use `task-mecca migrate`.")
        return 2

    print(f"Installed to {target}")
    print("\nTask Mecca installs framework files only.")
    print("Project data is created later by agents under _task_mecca/data/.")
    print("The first Registrar registration creates _task_mecca/data/backlog/.")
    print("\nTask Mecca does not modify AGENTS.md and does not make every project session Root.")
    print("To activate Root explicitly, open the user-facing session you want to use as Root and paste the prompt from:")
    print("  _task_mecca/ROOT_PROMPT.md")
    print("\nDashboard (project-local, no network or uv required):")
    print("  python _task_mecca/framework/collab_tools.py web")
    return 0


def cmd_update(args: argparse.Namespace) -> int:
    root = _project_root(args.project)
    try:
        result = apply_update(root, choice=args.choice, expected_plan=args.expect_plan)
        if result.get("status") == "choice_required":
            if args.json or not sys.stdin.isatty():
                print(json.dumps(result, ensure_ascii=False))
                return 3
            print("수정된 프레임워크 파일:")
            for rel in result["modified_files"]:
                print("  " + rel)
            try:
                answer = input("1 새 버전으로 덮어쓰기 / 2 기존 수정사항을 백업하고 진행 / 3 취소: ").strip()
            except EOFError:
                answer = "3"
            result = apply_update(root, choice={"1": "overwrite", "2": "backup"}.get(answer, "cancel"), expected_plan=result["plan_digest"])
        if result.get("status") == "choice_required":
            print(json.dumps(result, ensure_ascii=False))
            return 3
        if args.json:
            print(json.dumps(result, ensure_ascii=False))
        elif result["status"] == "cancelled":
            print("Migration cancelled. No files were changed.")
        else:
            print(f"Task Mecca {result['available_version']} migrated.")
        return 0
    except (RuntimeError, OSError, ValueError) as exc:
        if args.json:
            print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False))
        else:
            print(str(exc), file=sys.stderr)
        return 2


def _run_runtime(project_root: Path, argv: list[str]) -> int:
    script = _runtime_script(project_root)
    if not script.exists():
        print("Task Mecca framework runtime is not installed in this project.", file=sys.stderr)
        return 2
    return subprocess.call([sys.executable, str(script), *argv], cwd=project_root)


def cmd_doctor(args: argparse.Namespace) -> int:
    root = _project_root(args.project)
    return _run_runtime(root, ["doctor"])


def cmd_web(args: argparse.Namespace) -> int:
    root = _project_root(args.project)
    argv = ["web"]
    if args.port is not None:
        argv.extend(["--port", str(args.port)])
    if args.no_open:
        argv.append("--no-open")
    return _run_runtime(root, argv)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="task-mecca",
        description="Install, update, and launch project-contained Task Mecca runtimes.",
    )
    parser.add_argument("--version", action="version", version=f"task-mecca {__version__}")
    sub = parser.add_subparsers(dest="command", required=True)

    p = sub.add_parser("init", help="Install Task Mecca framework into the current project")
    p.add_argument("--project", help="Project root (defaults to current directory)")
    p.set_defaults(func=cmd_init)

    p = sub.add_parser("migrate", aliases=["update"], help="Safely migrate framework files without changing monitoring")
    p.add_argument("--project", help="Project root (defaults to current directory)")
    p.add_argument("--choice", choices=["overwrite", "backup", "cancel"])
    p.add_argument("--expect-plan", help="Require the plan_digest returned before the human choice")
    p.add_argument("--json", action="store_true")
    p.set_defaults(func=cmd_update)

    p = sub.add_parser("doctor", help="Run the installed project's Task Mecca doctor")
    p.add_argument("--project", help="Project root (defaults to current directory)")
    p.set_defaults(func=cmd_doctor)

    p = sub.add_parser("web", help="Launch the installed project's local Task Mecca dashboard")
    p.add_argument("--project", help="Project root (defaults to current directory)")
    p.add_argument("--port", type=int, default=8765)
    p.add_argument("--no-open", action="store_true")
    p.set_defaults(func=cmd_web)

    return parser


def main() -> int:
    args = build_parser().parse_args()
    return args.func(args)


if __name__ == "__main__":
    raise SystemExit(main())
