from __future__ import annotations

import hashlib
import json
import shutil
from dataclasses import dataclass
from datetime import datetime
from importlib import resources
from pathlib import Path
from typing import Iterable

from . import __version__ as PACKAGE_VERSION
SCHEMA_VERSION = 1
TARGET_DIR = "_task_mecca"
MANIFEST_NAME = "manifest.json"

FRAMEWORK_FILES = {
    "collab_tools.py",
    "runtime_metadata.py",
    ".gitignore",
}
FRAMEWORK_PREFIXES = ("web/",)
CUSTOMIZABLE_FILES = {
    "README.md",
    "README.en.md",
    "SESSION_GUIDE.md",
    "SESSION_GUIDE.en.md",
    "collab.md",
    "_template.md",
    "AGENTS_TASK_MECCA_SNIPPET.md",
}
CUSTOMIZABLE_PREFIXES = ("roles/",)
UPDATER_FILES = {"VERSION", MANIFEST_NAME}
PROJECT_OWNED_PATTERNS = (
    "backlog_*/**",
    ".runtime/**",
    "backups/**",
    "config.toml",
)


@dataclass(frozen=True)
class BundledFile:
    path: str
    content: bytes
    sha256: str
    policy: str


def _sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def _policy_for(path: str) -> str:
    if path in FRAMEWORK_FILES or path.startswith(FRAMEWORK_PREFIXES):
        return "framework"
    if path in CUSTOMIZABLE_FILES or path.startswith(CUSTOMIZABLE_PREFIXES):
        return "customizable"
    if path in UPDATER_FILES:
        return "updater"
    return "framework"


def bundled_files() -> dict[str, BundledFile]:
    root = resources.files("task_mecca").joinpath("template", TARGET_DIR)
    out: dict[str, BundledFile] = {}

    def walk(node, prefix: str = "") -> None:
        for child in node.iterdir():
            rel = f"{prefix}{child.name}"
            if child.is_dir():
                walk(child, rel + "/")
            elif rel != MANIFEST_NAME:
                data = child.read_bytes()
                out[rel] = BundledFile(rel, data, _sha256(data), _policy_for(rel))

    walk(root)
    return out


def build_manifest(files: dict[str, BundledFile] | None = None) -> dict:
    files = files or bundled_files()
    return {
        "task_mecca_version": PACKAGE_VERSION,
        "schema_version": SCHEMA_VERSION,
        "managed_files": {
            path: {"policy": item.policy, "baseline_sha256": item.sha256}
            for path, item in sorted(files.items())
        },
        "project_owned_patterns": list(PROJECT_OWNED_PATTERNS),
    }


def load_manifest(target: Path) -> dict | None:
    path = target / MANIFEST_NAME
    if not path.exists():
        return None
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return None


def _write_manifest(target: Path, manifest: dict) -> None:
    (target / MANIFEST_NAME).write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )


def _write_file(root: Path, rel: str, data: bytes) -> None:
    path = root / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(data)


def install(project_root: Path, *, overwrite: bool = False) -> Path:
    target = project_root / TARGET_DIR
    if target.exists() and any(target.iterdir()) and not overwrite:
        raise FileExistsError(f"{target} already exists")
    target.mkdir(parents=True, exist_ok=True)
    files = bundled_files()
    for rel, item in files.items():
        _write_file(target, rel, item.content)
    _write_manifest(target, build_manifest(files))
    return target


def file_hash(path: Path) -> str | None:
    if not path.is_file():
        return None
    return _sha256(path.read_bytes())


def update_plan(project_root: Path) -> dict:
    target = project_root / TARGET_DIR
    installed = load_manifest(target)
    if not installed:
        raise RuntimeError("Task Mecca manifest not found. Run `task-mecca init` first.")

    current_files = bundled_files()
    previous_files = installed.get("managed_files", {})
    paths = sorted(set(previous_files) | set(current_files))
    safe: list[str] = []
    conflicts: list[str] = []
    preserve: list[str] = []
    removals: list[str] = []

    for rel in paths:
        previous = previous_files.get(rel)
        incoming = current_files.get(rel)
        disk_hash = file_hash(target / rel)
        baseline = previous.get("baseline_sha256") if previous else None
        local_modified = previous is not None and disk_hash != baseline
        upstream_changed = (
            previous is None
            or incoming is None
            or baseline != incoming.sha256
        )

        if previous is not None and incoming is None:
            if local_modified:
                conflicts.append(rel)
            else:
                removals.append(rel)
            continue

        if incoming is None:
            continue
        if previous is None:
            safe.append(rel)
            continue
        if not upstream_changed:
            if local_modified:
                preserve.append(rel)
            continue
        if local_modified:
            conflicts.append(rel)
        else:
            safe.append(rel)

    return {
        "installed_version": installed.get("task_mecca_version", "unknown"),
        "available_version": PACKAGE_VERSION,
        "safe": safe,
        "conflicts": conflicts,
        "preserve": preserve,
        "removals": removals,
    }


def create_backup(project_root: Path, files: Iterable[str], *, from_version: str, to_version: str) -> Path:
    target = project_root / TARGET_DIR
    stamp = datetime.now().astimezone().strftime("%Y-%m-%d_%H%M%S")
    backup = target / "backups" / stamp
    backup.mkdir(parents=True, exist_ok=False)
    copied: list[str] = []
    for rel in sorted(set(files)):
        src = target / rel
        if src.is_file():
            dst = backup / rel
            dst.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(src, dst)
            copied.append(rel)
    metadata = {
        "created_at": datetime.now().astimezone().isoformat(timespec="seconds"),
        "from_version": from_version,
        "to_version": to_version,
        "files": copied,
    }
    (backup / "backup.json").write_text(
        json.dumps(metadata, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    return backup


def apply_update(project_root: Path, *, allow_conflicts: bool = False) -> dict:
    target = project_root / TARGET_DIR
    plan = update_plan(project_root)
    if plan["conflicts"] and not allow_conflicts:
        raise RuntimeError("Local modifications require explicit confirmation before update.")

    current_files = bundled_files()
    previous = load_manifest(target) or {}
    previous_files = previous.get("managed_files", {})

    # Remove files retired by the new package. Conflicted removals have already been backed up.
    for rel in plan["removals"] + [p for p in plan["conflicts"] if p not in current_files]:
        path = target / rel
        if path.is_file():
            path.unlink()

    for rel, item in current_files.items():
        prev = previous_files.get(rel)
        disk_hash = file_hash(target / rel)
        baseline = prev.get("baseline_sha256") if prev else None
        local_modified = prev is not None and disk_hash != baseline
        upstream_changed = prev is None or baseline != item.sha256
        if rel in plan["conflicts"] or not local_modified or upstream_changed and rel in plan["safe"]:
            _write_file(target, rel, item.content)
        elif local_modified and not upstream_changed:
            # Keep local customization when upstream did not change this file.
            pass

    _write_manifest(target, build_manifest(current_files))
    return plan
