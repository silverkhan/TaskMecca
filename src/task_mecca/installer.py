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

SCHEMA_VERSION = 2
TARGET_DIR = "_task_mecca"
MANIFEST_NAME = "manifest.json"

FRAMEWORK_FILES = {
    ".gitignore",
}
FRAMEWORK_PREFIXES = ("framework/",)
CUSTOMIZABLE_FILES = {
    "ROOT_PROMPT.md",
    "framework/README.md",
    "framework/README.en.md",
    "framework/SESSION_GUIDE.md",
    "framework/SESSION_GUIDE.en.md",
    "framework/EXECUTION_PROTOCOL.md",
    "framework/EXECUTION_PROTOCOL.en.md",
    "framework/collab.md",
    "framework/_template.md",
}
CUSTOMIZABLE_PREFIXES = ("framework/roles/",)
UPDATER_FILES = {"VERSION", MANIFEST_NAME}

# Everything below data/ belongs to the project, not the Task Mecca distribution.
# Legacy backlog locations remain declared for compatibility with pre-0.2 projects.
PROJECT_OWNED_PATTERNS = (
    "data/**",
    "backlog/**",
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
    if path in CUSTOMIZABLE_FILES or path.startswith(CUSTOMIZABLE_PREFIXES):
        return "customizable"
    if path in UPDATER_FILES:
        return "updater"
    if path in FRAMEWORK_FILES or path.startswith(FRAMEWORK_PREFIXES):
        return "framework"
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
    _safe_target_path(target, MANIFEST_NAME)
    (target / MANIFEST_NAME).write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )


def _write_file(root: Path, rel: str, data: bytes) -> None:
    _safe_target_path(root, rel)
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
    # Deliberately do not create data/ or backlog/. Registrar creates the canonical
    # data/backlog ledger only when the first task is registered.
    return target


def file_hash(path: Path) -> str | None:
    if not path.is_file():
        return None
    return _sha256(path.read_bytes())


def _safe_target_path(target: Path, rel: str) -> None:
    path = target / rel
    if path.is_absolute() != target.is_absolute() or ".." in Path(rel).parts:
        raise RuntimeError("Path outside framework boundary")
    for current in [path, *path.parents]:
        if current.is_symlink():
            raise RuntimeError(f"Symbolic-link framework boundary: {current}")
        if current == target:
            break


def _managed_path(rel: str) -> bool:
    path = Path(rel)
    return (not path.is_absolute() and ".." not in path.parts and "\\" not in rel
            and (rel in {"ROOT_PROMPT.md", "VERSION", ".gitignore", "collab_tools.py", "AGENTS_TASK_MECCA_SNIPPET.md"} or rel.startswith("framework/")))


def update_plan(project_root: Path) -> dict:
    target = project_root / TARGET_DIR
    _safe_target_path(target, MANIFEST_NAME)
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
    snapshot: dict[str, str | None] = {}

    for rel in paths:
        if not _managed_path(rel):
            raise RuntimeError(f"Manifest contains non-framework path: {rel}")
        disk_path = target / rel
        _safe_target_path(target, rel)
        if disk_path.is_symlink() or (disk_path.exists() and not disk_path.is_file()):
            raise RuntimeError(f"Managed file is not a regular file: {rel}")
        previous = previous_files.get(rel)
        incoming = current_files.get(rel)
        disk_hash = file_hash(target / rel)
        snapshot[rel] = disk_hash
        baseline = previous.get("baseline_sha256") if previous else None
        local_modified = previous is not None and disk_hash is not None and disk_hash != baseline
        if previous is None and incoming and disk_hash is not None and disk_hash != incoming.sha256:
            local_modified = True
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
            (conflicts if local_modified else safe).append(rel)
            continue
        if not upstream_changed:
            if local_modified:
                conflicts.append(rel)
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
        "plan_digest": _sha256(json.dumps([installed, snapshot, PACKAGE_VERSION], sort_keys=True).encode()),
    }


def create_backup(project_root: Path, files: Iterable[str], *, from_version: str, to_version: str) -> Path:
    target = project_root / TARGET_DIR
    _safe_target_path(target, "backups")
    stamp = datetime.now().astimezone().strftime("%Y-%m-%d_%H%M%S")
    backup = target / "backups" / stamp
    backup.mkdir(parents=True, exist_ok=False)
    copied: list[str] = []
    for rel in sorted(set(files)):
        src = target / rel
        _safe_target_path(target, rel)
        if not _managed_path(rel) or src.is_symlink() or not src.is_file():
            raise RuntimeError(f"Cannot back up modified framework file: {rel}")
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


def _prune_empty_managed_dirs(target: Path, removed_files: Iterable[str]) -> None:
    """Prune only directories that became empty because managed files were retired.

    Never scan arbitrary project-owned directories: unknown agent-created data must be
    preserved even when an empty directory is not named in the manifest.
    """
    candidates: set[Path] = set()
    for rel in removed_files:
        parent = (target / rel).parent
        while parent != target:
            candidates.add(parent)
            parent = parent.parent
    for path in sorted(candidates, key=lambda p: len(p.parts), reverse=True):
        try:
            path.rmdir()
        except OSError:
            pass


def apply_update(project_root: Path, *, allow_conflicts: bool = False, choice: str | None = None, expected_plan: str | None = None) -> dict:
    target = project_root / TARGET_DIR
    if choice not in {None, "overwrite", "backup", "cancel"}:
        raise ValueError("Invalid migration choice")
    if choice == "cancel":
        return {"status": "cancelled"}
    plan = update_plan(project_root)
    if allow_conflicts and choice is None:
        choice = "overwrite"  # Existing programmatic explicit opt-in compatibility.
    if (plan["conflicts"] and choice is None) or (expected_plan and expected_plan != plan["plan_digest"]):
        return {**plan, "status": "choice_required", "modified_files": plan["conflicts"],
                "choices": ["overwrite", "backup", "cancel"]}
    if plan["conflicts"] and choice == "backup":
        plan["backup_path"] = str(create_backup(project_root, plan["conflicts"],
            from_version=plan["installed_version"], to_version=plan["available_version"]))
    if update_plan(project_root)["plan_digest"] != plan["plan_digest"]:
        raise RuntimeError("Framework changed during migration; no framework files updated.")

    current_files = bundled_files()
    previous = load_manifest(target) or {}
    previous_files = previous.get("managed_files", {})

    retired = plan["removals"] + [p for p in plan["conflicts"] if p not in current_files]
    for rel in retired:
        path = target / rel
        if path.is_file():
            path.unlink()

    for rel, item in current_files.items():
        prev = previous_files.get(rel)
        disk_hash = file_hash(target / rel)
        baseline = prev.get("baseline_sha256") if prev else None
        local_modified = prev is not None and disk_hash != baseline
        upstream_changed = prev is None or baseline != item.sha256
        if rel in plan["conflicts"] or not local_modified or (upstream_changed and rel in plan["safe"]):
            _write_file(target, rel, item.content)
        elif local_modified and not upstream_changed:
            pass

    _prune_empty_managed_dirs(target, retired)
    _write_manifest(target, build_manifest(current_files))
    return {**plan, "status": "migrated"}
