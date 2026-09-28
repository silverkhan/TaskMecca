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


def _integrate_agents(project_root: Path) -> None:
    agents = project_root / "AGENTS.md"
    marker = "## Task Mecca"
    snippet = (
        "\n\n## Task Mecca\n\n"
        "When the user requests executable work, read `_task_mecca/SESSION_GUIDE.md` first and apply the Task Mecca workflow. "
        "Run the effective Full Access preflight before creating subagents. Do not force orchestration for simple Q&A or design-only discussion.\n"
    )
    if agents.exists():
        text = agents.read_text(encoding="utf-8")
        if marker in text:
            print("AGENTS.md already contains Task Mecca integration.")
            return
        print("\nProject AGENTS.md exists and will be preserved.")
        if _yes_no("Append the Task Mecca bootstrap block to AGENTS.md?", default=True):
            agents.write_text(text.rstrip() + snippet + "\n", encoding="utf-8")
            print("Updated AGENTS.md")
        else:
            print("Skipped AGENTS.md. See _task_mecca/AGENTS_TASK_MECCA_SNIPPET.md for manual integration.")
    else:
        if _yes_no("Create AGENTS.md with the Task Mecca bootstrap block?", default=True):
            agents.write_text(snippet.lstrip(), encoding="utf-8")
            print("Created AGENTS.md")
        else:
            print("Skipped AGENTS.md. See _task_mecca/AGENTS_TASK_MECCA_SNIPPET.md for manual integration.")


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
    _integrate_agents(root)
    print("\nNext:")
    print("  uv run _task_mecca/collab_tools.py web")
    print("Then give Root a task in natural language.")
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
        print("Local modifications were detected in files that this update would replace:")
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
        print("\nThe modified project versions listed above will now be overwritten by the official Task Mecca versions.")
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


def cmd_doctor(args: argparse.Namespace) -> int:
    root = _project_root(args.project)
    target = root / TARGET_DIR
    script = target / "collab_tools.py"
    if not script.exists():
        print("Task Mecca is not installed in this project.")
        return 2
    return subprocess.call([sys.executable, str(script), "doctor"], cwd=root)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="task-mecca", description="Install and update project-contained Task Mecca runtimes.")
    parser.add_argument("--version", action="version", version=f"task-mecca {__version__}")
    sub = parser.add_subparsers(dest="command", required=True)
    for name, handler, help_text in [
        ("init", cmd_init, "Install Task Mecca into the current project"),
        ("update", cmd_update, "Safely update an installed Task Mecca runtime"),
        ("doctor", cmd_doctor, "Run the installed project's Task Mecca doctor"),
    ]:
        p = sub.add_parser(name, help=help_text)
        p.add_argument("--project", help="Project root (defaults to current directory)")
        p.set_defaults(func=handler)
    return parser


def main() -> int:
    args = build_parser().parse_args()
    return args.func(args)


if __name__ == "__main__":
    raise SystemExit(main())
