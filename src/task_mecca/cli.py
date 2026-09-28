from __future__ import annotations

import argparse
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
        print("If this is an existing Task Mecca project, use `task-mecca update`.")
        return 2

    print(f"Installed to {target}")
    print("\nTask Mecca installs framework files only.")
    print("Project data is created later by agents under _task_mecca/data/.")
    print("The first Registrar registration creates _task_mecca/data/backlog/.")
    print("\nTask Mecca does not modify AGENTS.md and does not make every project session Root.")
    print("To activate Root explicitly, open the user-facing session you want to use as Root and paste the prompt from:")
    print("  _task_mecca/ROOT_PROMPT.md")
    print("\nDashboard:")
    print("  uvx task-mecca web")
    print("  # before PyPI publication:")
    print("  uvx --from git+https://github.com/silverkhan/TaskMecca.git task-mecca web")
    print("\nDirect project-local runtime:")
    print("  uv run _task_mecca/framework/collab_tools.py web")
    return 0


def cmd_update(args: argparse.Namespace) -> int:
    root = _project_root(args.project)
    print("Task Mecca Update\n")
    try:
        plan = update_plan(root)
    except RuntimeError as exc:
        print(str(exc))
        return 2

    print(f"Installed   {plan['installed_version']}")
    print(f"Available   {plan['available_version']}\n")

    if plan["installed_version"] == plan["available_version"] and not plan["safe"] and not plan["conflicts"] and not plan["removals"]:
        print("Task Mecca is already up to date.")
        return 0

    if plan["preserve"]:
        print("Local customizations preserved because upstream did not change these files:")
        for rel in plan["preserve"]:
            print(f"  • {rel}")
        print()

    if plan["conflicts"]:
        print("Local modifications were detected in files that this update would replace or retire:")
        for rel in plan["conflicts"]:
            print(f"  • {rel}")
        print("\nA backup is strongly recommended before updating.")
        if not _yes_no("Create a backup of the modified files before updating?", default=True):
            print("Update cancelled. No files were changed.")
            return 1
        backup = create_backup(
            root,
            plan["conflicts"],
            from_version=plan["installed_version"],
            to_version=plan["available_version"],
        )
        print(f"\nBackup created: {backup}")
        print("\nThe modified project versions listed above will now be overwritten or retired by the official Task Mecca layout.")
        print("The backup will remain available even if you cancel here.")
        if not _yes_no("Continue with the update?", default=False):
            print("Update cancelled. The backup was kept; no managed files were overwritten.")
            return 1
        result = apply_update(root, allow_conflicts=True)
    else:
        print("No conflicting local modifications detected. Updating...")
        result = apply_update(root)

    print(f"Task Mecca {result['available_version']} installed successfully.")
    return 0


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

    p = sub.add_parser("update", help="Safely update an installed Task Mecca framework")
    p.add_argument("--project", help="Project root (defaults to current directory)")
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
