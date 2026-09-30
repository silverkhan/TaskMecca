#!/usr/bin/env python3
"""상위 작업-서브에이전트 협업용 Git backlog 도구.

이 파일은 표준 라이브러리만 사용하며 backlog를 읽고 검사한다. Codex/Claude의
spawn/message/wait 같은 협업 기능은 모델 런타임
기능이므로 이 로컬 Python 프로그램이 호출하거나 흉내 내지 않는다.

지원 명령:

* ``agent``: canonical logical task path를 검증한다.
* ``worker-name``: 새 worker에 사용할 포켓몬 별칭을 결정한다.
* ``ensure-backlog``: 기존 원장을 선택하거나 첫 등록용 canonical `data/backlog`를 생성한다.\n* ``preflight``: backlog 원장과 effective Full Access/dispatch 준비 상태를 확인한다.
* ``next-id``: archive를 포함한 다음 ID와 6자리 정렬키를 계산한다.
* ``search``: 전체 backlog에서 관련 후보를 좁힌다.
* ``inspect``: 항목 상태, 의존성, 담당 범위와 연속성 근거를 본다.
* ``ready``: 선행관계상 착수 가능한 todo를 계산한다.
* ``workload``: Agent별 doing과 ready 연속성 후보를 집계한다.
* ``coordinate``: controller가 조율에 쓰는 fresh Git backlog snapshot을 만든다.
* ``audit``: 상태별 필수 필드 누락을 찾는다.
* ``check``: 문서 링크와 dependency를 검사한다.
* ``doctor``: 중복 ID, 파일명, 의존성, Agent, 쓰기 범위 충돌을 함께 검사한다.
* ``status``: 현재 hot set과 선택적 완료 이력을 표시한다.
* 무인자 실행/``web``/``monitor``: 사람용 읽기 전용 local Web UI를 연다. agent를 깨우지 않는다.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import select
import shutil
import subprocess
import html
import mimetypes
import socket
import threading
import webbrowser
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, unquote, urlparse
import sys
import time
import unicodedata
from dataclasses import dataclass, field, replace
from datetime import datetime, timezone
from pathlib import Path
from typing import Iterable, Optional

if __package__:
    from . import runtime_metadata as runtime
else:  # Direct documented CLI invocation.
    import runtime_metadata as runtime

try:
    import termios
    import tty
except ImportError:  # pragma: no cover - Windows
    termios = None  # type: ignore[assignment]
    tty = None  # type: ignore[assignment]

try:
    import msvcrt
except ImportError:  # pragma: no cover - POSIX
    msvcrt = None  # type: ignore[assignment]


FRAMEWORK_ROOT = Path(__file__).resolve().parent
TASK_MECCA_ROOT = FRAMEWORK_ROOT.parent
PROJECT_ROOT = TASK_MECCA_ROOT.parent
DATA_ROOT = TASK_MECCA_ROOT / "data"
CANONICAL_BACKLOG_ROOT = DATA_ROOT / "backlog"
# Kept as the protocol/document root so existing internal references to roles/web/docs
# remain framework-local.
PROTOCOL_ROOT = FRAMEWORK_ROOT
ITEM_RE = re.compile(
    r"^(?P<sort_key>(?:\d{4}|\d{6}))\."
    r"(?P<id>[A-Za-z]+-\d+)\."
    r"(?P<slug>[a-z0-9-]+)\."
    r"(?P<state>todo|doing|hold|done)\.md$"
)
TASK_PATH_RE = re.compile(r"^/root(?:/[a-z0-9_]+)*$")
ID_RE = re.compile(r"[A-Za-z]+-\d+")

# One naming policy, used only for creation/validation/reservation (never display).
# Korean-pronunciation ASCII spellings; the first three are examples, not a cap.
# Values are the original 16 English aliases retained for read/reuse/reservation.
# Add reviewed names here; allocation never invents suffixes on exhaustion.
WORKER_ALIAS_POLICY = {
    "kkobugi": ("squirtle",),       # 꼬부기
    "pairi": ("charmander",),       # 파이리
    "isanghaessi": ("bulbasaur",),  # 이상해씨
    "pikachyu": ("pikachu",),      # 피카츄
    "raichyu": (),                 # 라이츄
    "naong": ("meowth",),          # 나옹
    "jammanbo": ("snorlax",),      # 잠만보
    "ibui": ("eevee",),            # 이브이
    "purin": (),                   # 푸린
    "metamong": (),                # 메타몽
    "mangnanyong": ("dragonite",), # 망나뇽
    "gorapadeok": ("psyduck",),     # 고라파덕
    "paenteom": ("gengar",),       # 팬텀
    "rukario": ("lucario",),       # 루카리오
    "sikseuteil": ("vulpix",),     # 식스테일
    "rapeuraseu": ("lapras",),     # 라프라스
    "rioreu": ("riolu",),          # 리오르
    "togepi": ("togepi",),         # 토게피: old/new spelling is identical
    "seurakeu": ("scyther",),      # 스라크
    "eonibugi": (),                # 어니부기
    "geobukwang": (),              # 거북왕
    "rijadeu": (),                 # 리자드
    "rijamong": (),                # 리자몽
    "isanghaepul": (),             # 이상해풀
    "isanghaekkot": (),            # 이상해꽃
    "ppippi": (),                  # 삐삐
    "myu": (),                     # 뮤
    "digeuda": (),                 # 디그다
    "kkoret": (),                  # 꼬렛
    "moraeduji": (),               # 모래두지
    "kkomadol": (),                # 꼬마돌
    "rongseuton": (),              # 롱스톤
}
POKEMON_WORKER_NAMES = tuple(WORKER_ALIAS_POLICY)
NEW_POKEMON_WORKER_SET = frozenset(POKEMON_WORKER_NAMES)
POKEMON_WORKER_SET = NEW_POKEMON_WORKER_SET | frozenset(
    alias for old_aliases in WORKER_ALIAS_POLICY.values() for alias in old_aliases
)
DEFAULT_IMPLEMENTATION_WORKER_CAP = 3
LINK_RE = re.compile(r"\]\((?!https?://|#)([^)]+)\)")

FIELD_NAMES = (
    "등록자",
    "Agent",
    "변경범위",
    "대기",
    # Optional hold evidence. Legacy prose stays readable; absence is not a schema error.
    "대기유형",
    "재개조건",
    "대기근거",
    "선행",
    "연관",
    "설명",
    "메모",
    "결과",
    "검증",
    "Tags",
    # 기존 archive 읽기 호환. 새 템플릿에서는 사용하지 않는다.
    "Branch",
    "실행기",
)
FIELD_NAMES += tuple(runtime.FIELDS)
FIELD_START_RE = re.compile(
    r"^-\s+(?P<name>" + "|".join(re.escape(name) for name in FIELD_NAMES) + r"):\s*(?P<value>.*)$"
)


def _configure_stdio() -> None:
    """Codex/PowerShell 파이프에서도 구조화된 한글 출력을 UTF-8로 유지한다."""
    for stream in (sys.stdout, sys.stderr):
        reconfigure = getattr(stream, "reconfigure", None)
        if callable(reconfigure):
            reconfigure(encoding="utf-8", errors="replace")


def _root(value: Optional[Path]) -> Path:
    return TASK_MECCA_ROOT if value is None else Path(value).resolve()


BACKLOG_SCAN_MAX_DEPTH = int(os.getenv("TASK_MECCA_BACKLOG_SCAN_DEPTH", "4"))
BACKLOG_SCAN_SKIP = {
    ".git", ".venv", "venv", "node_modules", "__pycache__", ".mypy_cache", ".pytest_cache",
    "framework", ".runtime", "backups",
}
ARCHIVE_MONTH_RE = re.compile(r"^\d{4}-\d{2}$")


def _history_paths(folder: Path) -> list[Path]:
    """Read the active ledger plus recursive archive/_complete history.

    ``archive/YYYY-MM/*.md`` is the preferred archive layout, but rglob keeps older
    archive layouts readable without migration.
    """
    paths = list(folder.glob("*.md"))
    for dirname in ("archive", "_complete"):
        directory = folder / dirname
        if directory.is_dir():
            paths.extend(directory.rglob("*.md"))
    return sorted(set(paths))


def _recognized_history_paths(folder: Path) -> list[Path]:
    return [path for path in _history_paths(folder) if ITEM_RE.fullmatch(path.name)]


def _looks_like_backlog_folder(folder: Path) -> bool:
    if not folder.is_dir():
        return False
    # Canonical name is "backlog". Compatibility remains open for backlog_b,
    # backlog-team, and similar legacy/custom ledgers without treating arbitrary
    # agent-created data folders as ledgers.
    return folder.name.lower().startswith("backlog")


def _backlog_scan_root(base: Path) -> Path:
    # Normal 0.2+ layout scans only _task_mecca so sibling legacy backups or
    # unrelated project folders cannot be selected accidentally.
    resolved = base.resolve()
    if resolved in {TASK_MECCA_ROOT.resolve(), FRAMEWORK_ROOT.resolve()}:
        return TASK_MECCA_ROOT.resolve()
    if _looks_like_backlog_folder(base):
        return base.parent.resolve()
    return resolved


def discover_backlog_folders(root: Optional[Path] = None) -> list[dict[str, object]]:
    """Discover candidate ledger folders and rank the most recently active one first.

    Folder names containing ``backlog`` have priority.  Within the same priority the
    latest recognized task Markdown modification across active/archive/_complete wins.
    """
    base = _root(root)
    scan_root = _backlog_scan_root(base)
    candidates: list[dict[str, object]] = []
    seen: set[Path] = set()
    if not scan_root.is_dir():
        return []
    for current, dirnames, _filenames in os.walk(scan_root):
        folder = Path(current)
        try:
            depth = len(folder.relative_to(scan_root).parts)
        except ValueError:
            continue
        dirnames[:] = [d for d in dirnames if d not in BACKLOG_SCAN_SKIP and not d.startswith(".")]
        if depth >= BACKLOG_SCAN_MAX_DEPTH:
            dirnames[:] = []
        relative_parts = folder.relative_to(scan_root).parts
        if any(part in {"archive", "_complete"} for part in relative_parts):
            # Archive trees belong to their ledger root and must not become candidates.
            dirnames[:] = []
            continue
        if not _looks_like_backlog_folder(folder):
            continue
        resolved = folder.resolve()
        if resolved in seen:
            continue
        seen.add(resolved)
        task_paths = _recognized_history_paths(folder)
        mtimes = [path.stat().st_mtime for path in task_paths if path.exists()]
        try:
            folder_mtime = folder.stat().st_mtime
        except OSError:
            folder_mtime = 0.0
        latest = max(mtimes, default=folder_mtime)
        try:
            resolved.relative_to(DATA_ROOT.resolve())
            under_data = True
        except ValueError:
            under_data = False
        candidates.append({
            "path": str(resolved),
            "name": folder.name,
            "canonical": resolved == CANONICAL_BACKLOG_ROOT.resolve(),
            "under_data": under_data,
            "backlog_named": folder.name.lower().startswith("backlog"),
            "record_count": len(task_paths),
            "latest_modified": datetime.fromtimestamp(latest, timezone.utc).astimezone().isoformat(timespec="seconds") if latest else None,
            "latest_modified_epoch": latest,
        })
    candidates.sort(
        key=lambda row: (
            int(int(row["record_count"]) > 0),
            int(bool(row["canonical"])),
            int(bool(row["under_data"])),
            int(bool(row["backlog_named"])),
            float(row["latest_modified_epoch"]),
            str(row["path"]),
        ),
        reverse=True,
    )
    return candidates


def ensure_backlog(root: Optional[Path] = None) -> dict[str, object]:
    """Return an existing ledger or create the canonical data/backlog ledger.

    The installer intentionally creates no project data. Registrar calls this when
    the first task is being registered.
    """
    base = _root(root)
    existing = _item_dirs(base)
    if existing:
        selected = existing[0]
        return {
            "ok": True,
            "created": False,
            "path": str(selected),
            "canonical": selected.resolve() == CANONICAL_BACKLOG_ROOT.resolve(),
        }

    target = (
        Path(root).resolve()
        if root is not None and Path(root).name.lower().startswith("backlog")
        else CANONICAL_BACKLOG_ROOT
    )
    target.mkdir(parents=True, exist_ok=True)
    return {
        "ok": True,
        "created": True,
        "path": str(target.resolve()),
        "canonical": target.resolve() == CANONICAL_BACKLOG_ROOT.resolve(),
    }


def _item_dirs(root: Path) -> list[Path]:
    # An explicit ledger root is authoritative. Otherwise auto-select exactly one
    # candidate so independent backlog families never get merged accidentally.
    if _looks_like_backlog_folder(root):
        return [root.resolve()]
    candidates = discover_backlog_folders(root)
    return [Path(str(candidates[0]["path"]))] if candidates else []


def _selected_ledger_root(base: Path) -> Path:
    folders = _item_dirs(base)
    if folders:
        return folders[0]
    if base.resolve() in {TASK_MECCA_ROOT.resolve(), FRAMEWORK_ROOT.resolve()}:
        return CANONICAL_BACKLOG_ROOT
    return base


def _task_timings_for(base: Path) -> dict[str, dict[str, object]]:
    ledger = _selected_ledger_root(base)
    return task_state_timings(_repo_root(ledger), ledger)


def _parse_fields(text: str) -> dict[str, str]:
    fields = {name: "" for name in FIELD_NAMES}
    current: Optional[str] = None
    chunks: list[str] = []

    def flush() -> None:
        nonlocal chunks
        if current is not None:
            value = "\n".join(chunks).strip()
            fields[current] = "" if value == "-" else value
        chunks = []

    for line in text.splitlines():
        match = FIELD_START_RE.match(line)
        if match:
            flush()
            current = match.group("name")
            chunks = [match.group("value")]
        elif current is not None:
            if line.startswith("#"):
                flush()
                current = None
            else:
                chunks.append(line)
    flush()
    return fields



SECTION_RE = re.compile(r"^(?P<level>#{2,4})\s+(?P<title>.+?)\s*$")
CHECKBOX_RE = re.compile(r"^-\s+\[(?P<checked>[ xX])\]\s+(?P<text>.+)$")


def _section_map(text: str) -> dict[str, str]:
    """Return Markdown ##/###/#### sections without making headings part of fields.

    The parser is intentionally permissive: legacy flat ledgers remain readable and
    unknown headings are preserved for the Web UI instead of becoming schema errors.
    """
    sections: dict[str, list[str]] = {}
    current = ""
    for line in text.splitlines():
        match = SECTION_RE.match(line)
        if match:
            current = match.group("title").strip()
            sections.setdefault(current, [])
            continue
        if current:
            sections[current].append(line)
    return {name: "\n".join(lines).strip() for name, lines in sections.items()}


def _subsections(text: str, parent_heading: str) -> dict[str, str]:
    """Extract ### descendants of a named ## section."""
    lines = text.splitlines()
    in_parent = False
    current = ""
    result: dict[str, list[str]] = {}
    for line in lines:
        if line.startswith("## "):
            in_parent = line[3:].strip() == parent_heading
            current = ""
            continue
        if not in_parent:
            continue
        if line.startswith("### "):
            current = line[4:].strip()
            result.setdefault(current, [])
            continue
        if current:
            result[current].append(line)
    return {name: "\n".join(lines).strip() for name, lines in result.items()}


def _acceptance_items(value: str) -> list[dict[str, object]]:
    items: list[dict[str, object]] = []
    for line in value.splitlines():
        match = CHECKBOX_RE.match(line.strip())
        if match:
            items.append({"text": match.group("text").strip(), "checked": match.group("checked").lower() == "x"})
        elif line.strip().startswith("- "):
            items.append({"text": line.strip()[2:].strip(), "checked": False})
    return items


def _summary_key(label: str) -> str:
    normalized = label.strip()
    if normalized in {"목적", "작업의 목적"}:
        return "purpose"
    if normalized in {"핵심 변경", "변경"}:
        return "change"
    if normalized in {"상태·결과", "현재 상태·결과", "상태/결과"}:
        return "status_result"
    if normalized in {"확인·후속", "확인·후속 사항", "확인/후속"}:
        return "follow_up"
    return ""


def _human_summary(value: str) -> dict[str, str]:
    result = {
        "purpose": "",
        "change": "",
        "status_result": "",
        "follow_up": "",
    }
    current = ""
    chunks: list[str] = []

    def flush() -> None:
        nonlocal chunks
        if not current:
            chunks = []
            return
        text = "\n".join(chunks).strip()
        result[current] = "" if text == "-" else text
        chunks = []

    for line in value.splitlines():
        stripped = line.strip()
        if stripped.startswith("- ") and ":" in stripped:
            label, raw = stripped[2:].split(":", 1)
            key = _summary_key(label)
            if key:
                flush()
                current = key
                chunks = [raw.strip()]
                continue
        if current:
            chunks.append(line)
    flush()
    return result


def _document_model(text: str, fields: dict[str, str]) -> dict[str, object]:
    sections = _section_map(text)
    defined = _subsections(text, "요건 정의서")
    simple = _subsections(text, "작업 정의")
    scope = _subsections(text, "범위")
    # Scope is normally nested under a Defined Task requirement document, so parse
    # explicit #### labels while keeping Simple Task documents intentionally small.
    includes: list[str] = []
    excludes: list[str] = []
    bucket: list[str] | None = None
    in_scope = False
    for line in text.splitlines():
        if line.startswith("### 범위"):
            in_scope = True
            continue
        if in_scope and line.startswith("### "):
            break
        if in_scope and line.startswith("#### 포함"):
            bucket = includes
            continue
        if in_scope and line.startswith("#### 제외"):
            bucket = excludes
            continue
        if bucket is not None:
            bucket.append(line)
    scope_in = "\n".join(includes).strip() or scope.get("포함", "")
    scope_out = "\n".join(excludes).strip() or scope.get("제외", "")

    if "요건 정의서" in sections:
        schema = "defined-v2"
        contract_kind = "defined"
        contract = defined
    elif "작업 정의" in sections:
        schema = "simple-v2"
        contract_kind = "simple"
        contract = simple
    else:
        schema = "legacy"
        contract_kind = "legacy"
        contract = {}

    acceptance = contract.get("수용 기준", "")
    summary = _human_summary(sections.get("핵심 요약", ""))
    return {
        "schema": schema,
        "contract_kind": contract_kind,
        "sections": sections,
        "summary": summary,
        "summary_present": bool(sections.get("핵심 요약", "").strip()),
        # Keep one normalized object for CLI/search/UI consumers. Simple Tasks only
        # populate goal + acceptance; Defined Tasks populate the full structure.
        "requirements": {
            "background": defined.get("배경 및 문제", ""),
            "goal": contract.get("목표", ""),
            "requirements": defined.get("요구사항", ""),
            "scope_in": scope_in if contract_kind == "defined" else "",
            "scope_out": scope_out if contract_kind == "defined" else "",
            "acceptance": acceptance,
            "acceptance_items": _acceptance_items(acceptance),
            "constraints": defined.get("제약 및 보존 조건", ""),
        },
        "task_definition": {
            "goal": simple.get("목표", ""),
            "acceptance": simple.get("수용 기준", ""),
            "acceptance_items": _acceptance_items(simple.get("수용 기준", "")),
        },
        "result": sections.get("결과", fields.get("결과", "")),
        "verification": sections.get("검증", fields.get("검증", "")),
        "notes": sections.get("작업 노트", ""),
    }

def _title(text: str) -> str:
    for line in text.splitlines():
        if line.startswith("# "):
            return line[2:].strip()
    return ""


def _location(path: Path, folder: Path) -> str:
    try:
        rel = path.relative_to(folder)
    except ValueError:
        return "unknown"
    return "active" if len(rel.parts) == 1 else ("archive" if rel.parts[0] == "archive" else "legacy")


def _record(path: Path, folder: Path) -> Optional[dict[str, object]]:
    match = ITEM_RE.fullmatch(path.name)
    if not match:
        return None
    text = path.read_text(encoding="utf-8-sig")
    fields = _parse_fields(text)
    document = _document_model(text, fields)
    if not fields.get("결과") and document.get("result"):
        fields["결과"] = str(document["result"])
    if not fields.get("검증") and document.get("verification"):
        fields["검증"] = str(document["verification"])
    try:
        rel_parts = path.relative_to(folder).parts
    except ValueError:
        rel_parts = ()
    archive_month = rel_parts[1] if len(rel_parts) >= 3 and rel_parts[0] == "archive" and ARCHIVE_MONTH_RE.fullmatch(rel_parts[1]) else ""
    return {
        "id": match.group("id").upper(),
        "sort_key": match.group("sort_key"),
        "slug": match.group("slug"),
        "state": match.group("state"),
        "path": str(path),
        "folder": folder.name,
        "location": _location(path, folder),
        "archive_month": archive_month,
        "title": _title(text),
        "fields": fields,
        "document": document,
        "raw_markdown": text,
        "runtime_metadata": runtime.from_fields(fields),
        "mtime": datetime.fromtimestamp(path.stat().st_mtime, timezone.utc).isoformat(),
        # POSIX ctime changes on rename/metadata updates and is a better fallback
        # for an uncommitted filename-state transition than mtime, which rename
        # preserves.  On Windows ctime is creation time, so callers prefer mtime.
        "ctime": datetime.fromtimestamp(path.stat().st_ctime, timezone.utc).isoformat(),
    }


def catalog(root: Optional[Path] = None) -> list[dict[str, object]]:
    base = _root(root)
    records: list[dict[str, object]] = []
    for folder in _item_dirs(base):
        for path in _history_paths(folder):
            if path.name.startswith("_"):
                continue
            record = _record(path, folder)
            if record is not None:
                records.append(record)
    return sorted(records, key=lambda row: (str(row["id"]), str(row["path"])))


def backlog_presence(
    root: Optional[Path] = None,
    records: Optional[list[dict[str, object]]] = None,
) -> dict[str, object]:
    base = _root(root)
    folders = _item_dirs(base)
    if not folders:
        return {
            "ok": True,
            "status": "uninitialized",
            "root": str(base),
            "folders": [],
            "record_count": 0,
            "active_count": 0,
            "message": "아직 등록된 작업이 없어 backlog 원장이 생성되지 않았다. 첫 등록 시 Registrar가 data/backlog를 생성한다.",
        }
    rows = catalog(base) if records is None else records
    status = "ok" if rows else "empty"
    return {
        "ok": True,
        "status": status,
        "root": str(base),
        "folders": [str(path) for path in folders],
        "record_count": len(rows),
        "active_count": sum(1 for row in rows if row["location"] == "active"),
        "message": (
            "backlog를 찾았고 항목을 읽을 수 있다."
            if status == "ok"
            else "backlog 후보 폴더는 있으나 인식 가능한 항목이 없다."
        ),
    }


def _refs(value: str) -> list[str]:
    seen: set[str] = set()
    result: list[str] = []
    for raw in ID_RE.findall(value or ""):
        item_id = raw.upper()
        if item_id not in seen:
            seen.add(item_id)
            result.append(item_id)
    return result


def _by_id(records: list[dict[str, object]]) -> dict[str, list[dict[str, object]]]:
    grouped: dict[str, list[dict[str, object]]] = {}
    for row in records:
        grouped.setdefault(str(row["id"]), []).append(row)
    return grouped


def dependency_report(records: list[dict[str, object]]) -> dict[str, object]:
    grouped = _by_id(records)
    missing: list[dict[str, str]] = []
    graph: dict[str, list[str]] = {}
    for row in records:
        fields = row["fields"]
        assert isinstance(fields, dict)
        item_id = str(row["id"])
        deps = _refs(str(fields.get("선행", "")))
        graph[item_id] = deps
        for dep in deps:
            if dep not in grouped:
                missing.append({"id": item_id, "depends_on": dep})

    cycles: list[list[str]] = []
    visited: set[str] = set()
    visiting: list[str] = []

    def walk(node: str) -> None:
        if node in visiting:
            start = visiting.index(node)
            cycle = visiting[start:] + [node]
            if cycle not in cycles:
                cycles.append(cycle)
            return
        if node in visited:
            return
        visiting.append(node)
        for dep in graph.get(node, []):
            if dep in graph:
                walk(dep)
        visiting.pop()
        visited.add(node)

    for node in graph:
        walk(node)
    return {"missing": missing, "cycles": cycles}


def _claim_blockers(fields: dict[str, str], grouped: dict[str, list[dict[str, object]]]) -> dict[str, object]:
    """B55: an explicit waiting note blocks todo allocation independently of dependencies."""
    required = _refs(str(fields.get("선행", "")))
    waiting = [dep for dep in required
               if dep not in grouped or not any(row["state"] == "done" for row in grouped[dep])]
    note = str(fields.get("대기", "")).strip()
    return {
        "depends_on": required, "waiting_for": waiting, "waiting_note": note,
        "blocked_by": (["waiting_for_dependencies"] if waiting else []) + (["waiting_note"] if note else []),
    }


def hold_review_report(root: Optional[Path] = None, records: Optional[list[dict[str, object]]] = None) -> dict[str, object]:
    """Read recorded hold evidence only. Review is neither readiness nor acceptance.

    No prose classification, state mutation, clock/ack ledger, or runtime dispatch.
    A documented external/user wait remains a wait even after all dependencies finish.
    """
    rows = catalog(root) if records is None else records
    grouped = _by_id(rows)
    candidates: list[dict[str, object]] = []
    waiting: list[dict[str, object]] = []
    for row in rows:
        if row["location"] != "active" or row["state"] != "hold":
            continue
        fields = row["fields"]
        assert isinstance(fields, dict)
        note = str(fields.get("대기", "")).strip()
        kind = str(fields.get("대기유형", "")).strip()
        resume = str(fields.get("재개조건", "")).strip()
        evidence = str(fields.get("대기근거", "")).strip()
        reasons: list[dict[str, str]] = []

        def warn(code: str, message: str) -> None:
            reasons.append({"code": code, "message": message})

        for value, code, message in (
            (note, "hold_wait_reason_missing", "직접 대기 사유가 기록되지 않았습니다."),
            (resume, "hold_resume_condition_missing", "재개 조건이 별도 기록되지 않았습니다."),
            (evidence, "hold_evidence_missing", "대기 판단 근거가 별도 기록되지 않았습니다."),
        ):
            if not value:
                warn(code, message)
        if not kind:
            warn("hold_wait_kind_unrecorded", "legacy/유형 미기록: 자유 서술에서 대기 유형을 추측하지 않습니다.")
        elif kind not in {"external", "user", "dependency", "internal"}:
            warn("hold_wait_kind_invalid", "대기유형은 external/user/dependency/internal 중 하나입니다.")
        dependencies = []
        for dependency in _refs(str(fields.get("선행", ""))):
            matches = grouped.get(dependency, [])
            status = ("missing" if not matches else "ambiguous" if len(matches) != 1
                      else "done" if matches[0]["state"] == "done" else "pending")
            dependencies.append({"id": dependency, "status": status,
                                 "states": [match["state"] for match in matches],
                                 "paths": [match["path"] for match in matches]})
        if kind == "internal":
            warn("hold_internal_work", "내부 실행 가능한 구현·통합·검증은 doing인지 검토하세요.")
        if kind == "dependency" and not dependencies:
            warn("hold_dependencies_missing", "dependency 대기인데 명시한 선행 ID가 없습니다.")
        # A completed dependency is not evidence that an external response arrived.
        if kind in {"dependency", ""} and dependencies and all(dep["status"] == "done" for dep in dependencies):
            warn("hold_dependencies_done", "모든 명시 선행이 done입니다. 현재 직접 대기 사유를 재검토하세요.")
        target = {
            "id": row["id"], "title": row["title"], "path": row["path"], "state": "hold",
            "wait_kind": kind or "unrecorded", "wait_note": note,
            "resume_condition": resume, "wait_evidence": evidence,
            "dependencies": dependencies, "reasons": reasons,
            "review_required": bool(reasons), "ready": False,
        }
        (candidates if reasons else waiting).append(target)
    return {
        "review_needed": bool(candidates), "candidates": candidates, "waiting": waiting,
        "meaning": "수동 검토 권고입니다. ready/수용 통과/자동 재개/spawn 또는 drain 지속 명령이 아닙니다. "
                   "현재 이벤트에서 검토 후 실제 외부·사용자 대기만 남으면 cycle을 끝낼 수 있습니다.",
    }


def ready_report(root: Optional[Path] = None, records: Optional[list[dict[str, object]]] = None) -> dict[str, object]:
    base = _root(root)
    rows = catalog(base) if records is None else records
    grouped = _by_id(rows)
    deps = dependency_report(rows)
    ready: list[dict[str, object]] = []
    blocked: list[dict[str, object]] = []
    for row in rows:
        if row["location"] != "active" or row["state"] != "todo":
            continue
        fields = row["fields"]
        assert isinstance(fields, dict)
        blockers = _claim_blockers(fields, grouped)
        target = blocked if blockers["blocked_by"] else ready
        target.append({
            "id": row["id"],
            "title": row["title"],
            "path": row["path"],
            **blockers,
        })
    return {
        "root": str(base),
        "ready": ready,
        "blocked": blocked,
        "problems": deps,
    }


def _normalize_words(value: str) -> list[str]:
    normalized = unicodedata.normalize("NFKC", value).lower()
    return [word for word in re.findall(r"[0-9a-z가-힣_]+", normalized) if len(word) > 1]


def search_report(query: str, root: Optional[Path] = None, limit: int = 10) -> dict[str, object]:
    words = _normalize_words(query)
    results: list[dict[str, object]] = []
    for row in catalog(root):
        fields = row["fields"]
        assert isinstance(fields, dict)
        title = str(row["title"])
        description = str(fields.get("설명", ""))
        document = row.get("document", {})
        searchable = json.dumps(document, ensure_ascii=False) if isinstance(document, dict) else str(document)
        haystack = unicodedata.normalize("NFKC", " ".join((str(row["id"]), title, description, searchable, str(fields)))).lower()
        score = sum((6 if word in title.lower() else 2) for word in words if word in haystack)
        if query.lower() in haystack:
            score += 8
        if score:
            results.append({
                "id": row["id"],
                "state": row["state"],
                "location": row["location"],
                "title": title,
                "description": description[:500],
                "path": row["path"],
                "score": score,
            })
    results.sort(key=lambda item: (-int(item["score"]), str(item["id"])))
    return {
        "query": query,
        "results": results[: max(1, limit)],
        "warning": "검색 점수는 읽을 후보를 좁힐 뿐 의미상 중복을 확정하지 않는다.",
    }


def _canonical_agent(value: str) -> bool:
    return bool(TASK_PATH_RE.fullmatch(value.strip()))


def _worker_alias(value: str) -> str:
    value = value.strip().rstrip("/")
    prefix = "/root/controller/"
    if value.startswith(prefix):
        return value[len(prefix):].split("/", 1)[0]
    return value.rsplit("/", 1)[-1]


def _pokemon_worker_path(alias: str) -> str:
    return f"/root/controller/{alias}"


def _is_pokemon_worker_path(value: str, *, new: bool = False) -> bool:
    value = value.strip()
    prefix = "/root/controller/"
    if not value.startswith(prefix):
        return False
    tail = value[len(prefix):]
    return "/" not in tail and tail in (NEW_POKEMON_WORKER_SET if new else POKEMON_WORKER_SET)


def _naming_alias(value: str) -> Optional[str]:
    """Accept exact worker paths or bare known aliases, without rewriting runtime IDs."""
    text = value.strip()
    alias = text.removeprefix("/root/controller/")
    return alias if "/" not in alias and alias in POKEMON_WORKER_SET else None


def worker_name_report(root: Optional[Path] = None, used: Optional[Iterable[str]] = None) -> dict[str, object]:
    """Return the next stable Pokemon worker alias.

    The local backlog cannot know which runtime agents are alive, so callers should pass
    live aliases/paths through ``used``. Active doing backlog Agents are included
    automatically as a second collision guard. Released holds and completed history
    do not reserve a nickname; a still-live worker must be supplied through ``used``.
    """
    base = _root(root)
    reservations: dict[str, list[dict[str, str]]] = {}
    ignored_used: list[str] = []

    def reserve(value: str, source: str) -> None:
        alias = _naming_alias(value)
        equivalent = next((new for new, old in WORKER_ALIAS_POLICY.items()
                           if alias == new or alias in old), None)
        if equivalent is None:
            if source == "live_used":
                ignored_used.append(value)
            return
        evidence = {"identity": _pokemon_worker_path(alias), "source": source}
        group = reservations.setdefault(equivalent, [])
        if evidence not in group:
            group.append(evidence)

    for value in used or ():
        reserve(str(value), "live_used")
    for row in catalog(base):
        if row.get("location") != "active" or row.get("state") != "doing":
            continue
        fields = row.get("fields", {})
        if not isinstance(fields, dict):
            continue
        reserve(str(fields.get("Agent", "")), f"active_doing:{row['id']}")
    selected = next((name for name in POKEMON_WORKER_NAMES if name not in reservations), None)
    conflicts = [
        {"new_alias": alias, "identities": sorted({entry["identity"] for entry in entries})}
        for alias, entries in sorted(reservations.items())
        if len({entry["identity"] for entry in entries}) > 1
    ]
    return {
        "ok": selected is not None,
        "alias": selected,
        "task_name": selected,
        "path": _pokemon_worker_path(selected) if selected else None,
        "reason": None if selected else "pool_exhausted",
        "unavailable": [name for name in POKEMON_WORKER_NAMES if name in reservations],
        "reservations": {alias: sorted(entries, key=lambda x: (x["identity"], x["source"]))
                         for alias, entries in sorted(reservations.items())},
        "equivalent_conflicts": conflicts,
        "ignored_used": sorted(set(ignored_used)),
        "pool": list(POKEMON_WORKER_NAMES),
        "equivalences": {new: list(old) for new, old in WORKER_ALIAS_POLICY.items()},
        "new_validation": "agent <path> --new --json",
        "rule": "new workers use confirmed Korean-pronunciation ASCII aliases; identity is stable and task-independent; no task/model/feature names or fallback",
        "live_state_note": "pass current runtime worker aliases/paths with --used; Git alone cannot know live agents",
    }


def _continuity(rows: list[dict[str, object]], row: dict[str, object]) -> tuple[list[dict[str, object]], list[str]]:
    fields = row["fields"]
    assert isinstance(fields, dict)
    evidence_ids = set(_refs(str(fields.get("선행", ""))) + _refs(str(fields.get("연관", ""))))
    scores: dict[str, list[str]] = {}
    legacy: set[str] = set()
    for candidate in rows:
        if candidate["id"] not in evidence_ids or candidate["state"] != "done":
            continue
        candidate_fields = candidate["fields"]
        assert isinstance(candidate_fields, dict)
        agent = str(candidate_fields.get("Agent", "")).strip()
        if not agent:
            continue
        if _canonical_agent(agent):
            scores.setdefault(agent, []).append(str(candidate["id"]))
        else:
            legacy.add(agent)
    continuity = [
        {"agent": agent, "evidence_ids": ids, "count": len(ids)}
        for agent, ids in scores.items()
    ]
    continuity.sort(key=lambda item: (-int(item["count"]), str(item["agent"])))
    return continuity, sorted(legacy)


def inspect_report(item_id: str, root: Optional[Path] = None) -> dict[str, object]:
    wanted = item_id.upper()
    base = _root(root)
    rows = catalog(base)
    matches = [row for row in rows if row["id"] == wanted]
    if not matches:
        return {"exists": False, "id": wanted, "duplicate_count": 0}
    row = matches[0]
    fields = row["fields"]
    assert isinstance(fields, dict)
    grouped = _by_id(rows)
    blockers = _claim_blockers(fields, grouped)
    reviews = hold_review_report(base, rows)
    continuity, legacy = _continuity(rows, row)
    lifecycle = _task_timings_for(base).get(wanted, {})
    assignment = _assignment_view(row)
    return {
        "exists": True,
        "id": wanted,
        "duplicate_count": len(matches),
        "duplicate_paths": [match["path"] for match in matches],
        "state": row["state"],
        "location": row["location"],
        "title": row["title"],
        "path": row["path"],
        "ready": row["location"] == "active" and row["state"] == "todo" and not blockers["blocked_by"],
        **blockers,
        "hold_review": next((review for review in reviews["candidates"] + reviews["waiting"]
                             if review["path"] == row["path"]), None),
        "related": _refs(str(fields.get("연관", ""))),
        "agent": assignment["agent"],
        "change_scope": assignment["change_scope"],
        "assignment_kind": assignment["assignment_kind"],
        "hold_audit": assignment["hold_audit"],
        "runtime_metadata": runtime.from_fields(fields),
        "registrant": fields.get("등록자", ""),
        "continuity_agents": continuity,
        "legacy_agent_history": legacy,
        "lifecycle": lifecycle,
        "document": row.get("document", {}),
        "raw_markdown": row.get("raw_markdown", ""),
        "fields": fields,
    }


def next_id_report(prefix: str, root: Optional[Path] = None, allow_empty: bool = False) -> dict[str, object]:
    prefix = prefix.upper().strip()
    if not re.fullmatch(r"[A-Z]+", prefix):
        raise ValueError("접두사는 영문자만 사용할 수 있다.")
    presence = backlog_presence(root)
    if not presence["ok"] and not allow_empty:
        raise LookupError(str(presence["message"]))
    numbers = [
        int(str(row["id"]).rsplit("-", 1)[1])
        for row in catalog(root)
        if str(row["id"]).startswith(prefix + "-")
    ]
    number = max(numbers, default=0) + 1
    if number > 999999:
        raise ValueError("6자리 정렬키 한계(999999)를 넘었다.")
    return {
        "prefix": prefix,
        "number": number,
        "id": f"{prefix}-{number}",
        "sort_key": f"{number:06d}",
        "reservation": False,
    }


def _scope_tokens(value: str) -> list[str]:
    value = value.strip()
    if not value or value in {"읽기 전용", "read-only", "readonly"}:
        return []
    tokens: list[str] = []
    for raw in re.split(r"[,;\n]+", value):
        token = raw.strip().strip("`").replace("\\", "/")
        if not token or token == "-":
            continue
        wildcard = min([pos for pos in (token.find("*"), token.find("?")) if pos >= 0], default=-1)
        if wildcard >= 0:
            token = token[:wildcard].rstrip("/")
        token = re.sub(r"^\./", "", token).rstrip("/")
        if token:
            tokens.append(token.lower())
    return sorted(set(tokens))


def _scope_overlap(left: str, right: str) -> bool:
    return left == right or left.startswith(right + "/") or right.startswith(left + "/")


def _working_tree_changes() -> tuple[list[str], Optional[str]]:
    try:
        done = subprocess.run(
            ["git", "status", "--porcelain=v1", "-z", "--untracked-files=all", "--", "."],
            cwd=PROJECT_ROOT,
            capture_output=True,
            text=True,
            timeout=10,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        return [], f"{type(exc).__name__}: {exc}"
    if done.returncode != 0:
        return [], (done.stderr or f"git status exited {done.returncode}").strip()

    parts = done.stdout.split("\x00")
    changes: set[str] = set()
    index = 0
    while index < len(parts):
        entry = parts[index]
        index += 1
        if len(entry) < 4:
            continue
        code = entry[:2]
        path = entry[3:].replace("\\", "/")
        if path:
            changes.add(path)
        if ("R" in code or "C" in code) and index < len(parts):
            other = parts[index].replace("\\", "/")
            index += 1
            if other:
                changes.add(other)
    return sorted(changes), None


def _scoped_working_tree_changes(changes: list[str], scope: str) -> list[str]:
    scopes = _scope_tokens(scope)
    if not scopes or not changes:
        return []
    matched: list[str] = []
    for change in changes:
        normalized = re.sub(r"^\./", "", change.replace("\\", "/")).lower()
        if any(_scope_overlap(token, normalized) for token in scopes):
            matched.append(change)
    return sorted(set(matched))


def _continuity_health_consumes_worker_slot(health: str) -> bool:
    return health not in {"awaiting_finalize", "worker_missing", "needs_user"}


def scope_conflicts(records: list[dict[str, object]]) -> list[dict[str, object]]:
    active: list[tuple[dict[str, object], list[str]]] = []
    for row in records:
        if row["location"] != "active" or row["state"] != "doing":
            continue
        fields = row["fields"]
        assert isinstance(fields, dict)
        tokens = _scope_tokens(str(fields.get("변경범위", "")))
        if tokens:
            active.append((row, tokens))
    conflicts: list[dict[str, object]] = []
    for index, (left_row, left_scopes) in enumerate(active):
        for right_row, right_scopes in active[index + 1 :]:
            overlaps = sorted({
                f"{left} <> {right}"
                for left in left_scopes
                for right in right_scopes
                if _scope_overlap(left, right)
            })
            if overlaps:
                conflicts.append({
                    "left": left_row["id"],
                    "right": right_row["id"],
                    "overlaps": overlaps,
                })
    return conflicts


def audit_report(root: Optional[Path] = None, records: Optional[list[dict[str, object]]] = None) -> list[dict[str, str]]:
    rows = catalog(root) if records is None else records
    findings: list[dict[str, str]] = []
    for row in rows:
        state = str(row["state"])
        fields = row["fields"]
        assert isinstance(fields, dict)
        required: tuple[str, ...]
        if row["location"] == "active" and state == "doing":
            required = ("Agent", "변경범위")
        elif row["location"] == "active" and state in {"todo", "hold"}:
            required = ()
            for name in ("Agent", "변경범위"):
                value = str(fields.get(name, "")).strip()
                if value and value != "-":
                    findings.append({
                        "file": Path(str(row["path"])).name,
                        "state": state,
                        "field": name,
                        "value": value,
                        "problem": "미배정 상태는 현재 Agent/변경범위를 가질 수 없다.",
                    })
        elif state == "done":
            required = ("Agent", "결과", "검증")
        else:
            required = ()
        for name in required:
            if not str(fields.get(name, "")).strip():
                findings.append({
                    "file": Path(str(row["path"])).name,
                    "state": state,
                    "field": name,
                    "value": "<missing>",
                    "problem": "상태에 필요한 필드가 비어 있다.",
                })
    return findings


def _contract_problems(records: list[dict[str, object]]) -> list[dict[str, str]]:
    """Validate only new dual-lane contracts; legacy ledgers remain readable."""
    problems: list[dict[str, str]] = []
    for row in records:
        document = row.get("document", {})
        if not isinstance(document, dict):
            continue
        sections = document.get("sections", {})
        if not isinstance(sections, dict):
            sections = {}
        has_simple = "작업 정의" in sections
        has_defined = "요건 정의서" in sections
        if has_simple and has_defined:
            problems.append({
                "file": Path(str(row["path"])).name,
                "problem": "Simple Task의 `작업 정의`와 Defined Task의 `요건 정의서`를 동시에 둘 수 없다.",
            })
            continue
        kind = str(document.get("contract_kind", "legacy"))
        if kind not in {"simple", "defined"}:
            continue
        requirements = document.get("requirements", {})
        if not isinstance(requirements, dict):
            requirements = {}
        for label, key in (("목표", "goal"), ("수용 기준", "acceptance")):
            value = str(requirements.get(key, "")).strip()
            if not value or value == "-":
                problems.append({
                    "file": Path(str(row["path"])).name,
                    "problem": f"{kind} contract에 `{label}`가 비어 있다.",
                })
    return problems


def _unrecognized_files(root: Optional[Path] = None) -> list[dict[str, str]]:
    base = _root(root)
    problems: list[dict[str, str]] = []
    for folder in _item_dirs(base):
        for path in _history_paths(folder):
            if path.name.startswith("_"):
                continue
            if not ITEM_RE.fullmatch(path.name):
                problems.append({"file": str(path), "problem": "unrecognized backlog filename"})
    return problems


def _filename_problems(records: list[dict[str, object]]) -> list[dict[str, str]]:
    problems: list[dict[str, str]] = []
    for row in records:
        try:
            number = int(str(row["id"]).rsplit("-", 1)[1])
            valid = int(str(row["sort_key"])) == number
        except (ValueError, IndexError):
            valid = False
        if not valid:
            problems.append({"file": str(row["path"]), "problem": "sort-key/id mismatch"})
    return problems


def _duplicate_ids(records: list[dict[str, object]]) -> list[dict[str, object]]:
    return [
        {"id": item_id, "files": [str(row["path"]) for row in rows]}
        for item_id, rows in _by_id(records).items()
        if len(rows) > 1
    ]


def _agent_problems(records: list[dict[str, object]]) -> list[dict[str, str]]:
    problems: list[dict[str, str]] = []
    for row in records:
        if row["location"] != "active" or row["state"] != "doing":
            continue
        fields = row["fields"]
        assert isinstance(fields, dict)
        agent = str(fields.get("Agent", "")).strip()
        if agent and not _canonical_agent(agent):
            problems.append({
                "id": str(row["id"]),
                "agent": agent,
                "problem": "새 active Agent는 /root 또는 /root/<task_name> canonical task path여야 한다.",
            })
    return problems


def _assignment_view(row: dict[str, object]) -> dict[str, object]:
    """Separate a current doing claim from preserved hold/archive audit data."""
    fields = row["fields"]
    assert isinstance(fields, dict)
    recorded_agent = str(fields.get("Agent", "")).strip()
    recorded_scope = str(fields.get("변경범위", "")).strip()
    state = str(row["state"])
    location = str(row["location"])
    if state == "done":
        return {
            "agent": recorded_agent,
            "change_scope": recorded_scope,
            "assignment_kind": "historical",
            "hold_audit": None,
        }
    if location == "active" and state == "doing":
        return {
            "agent": recorded_agent,
            "change_scope": recorded_scope,
            "assignment_kind": "current",
            "hold_audit": None,
        }
    if location == "active" and state == "hold":
        return {
            "agent": "",
            "change_scope": "",
            "assignment_kind": "released",
            "hold_audit": {
                "recorded_agent": recorded_agent,
                "recorded_change_scope": recorded_scope,
                "branch": str(fields.get("Branch", "")).strip(),
                "runner": str(fields.get("실행기", "")).strip(),
            },
        }
    if location == "active":
        return {
            "agent": "",
            "change_scope": "",
            "assignment_kind": "unassigned",
            "hold_audit": None,
        }
    return {
        "agent": recorded_agent,
        "change_scope": recorded_scope,
        "assignment_kind": "historical",
        "hold_audit": None,
    }


def dangling_links(protocol: Path) -> list[str]:
    missing: list[str] = []
    text = protocol.read_text(encoding="utf-8-sig")
    for target in LINK_RE.findall(text):
        relative = target.split("#", 1)[0].strip()
        if relative and not (protocol.parent / relative).resolve().exists():
            missing.append(target)
    return missing


def runtime_findings(rows: list[dict[str, object]]) -> list[dict[str, object]]:
    """Advisory only: missing legacy metadata must not break backlog readability."""
    findings = []
    agents: dict[str, dict[str, dict]] = {}
    for row in rows:
        metadata = runtime.from_fields(row["fields"])
        active = row["location"] == "active" and row["state"] == "doing"
        codes = list(metadata["findings"])
        if active and metadata["coverage"] == "legacy":
            codes.append("legacy_runtime_unknown")
        if active and metadata["runtime_schema"] == runtime.SCHEMA_VERSION and metadata["missing_fields"]:
            codes.append("runtime_fields_missing")
        if active and metadata["dispatch_status"] == "failed":
            codes.append("failed_dispatch_on_doing")
        findings.extend({"id": row["id"], "path": row["path"], "code": code,
                         "missing_fields": metadata["missing_fields"]} for code in codes)
        if active and row["fields"].get("Agent"):
            agents.setdefault(row["fields"]["Agent"], {})[str(row["id"])] = metadata
    for agent, tasks in agents.items():
        if runtime.aggregate(tasks)["conflict"]:
            findings.append({"agent": agent, "tasks": sorted(tasks), "code": "agent_runtime_conflict"})
    return findings


def doctor_report(
    root: Optional[Path] = None,
    protocol_checks: bool = True,
    records: Optional[list[dict[str, object]]] = None,
) -> dict[str, object]:
    base = _root(root)
    rows = catalog(base) if records is None else records
    deps = dependency_report(rows)
    presence = backlog_presence(base, rows)
    checks: dict[str, object] = {
        "backlog_presence": [] if presence["ok"] else [presence],
        "audit": audit_report(base, rows),
        "duplicate_ids": _duplicate_ids(rows),
        "missing_dependencies": deps["missing"],
        "dependency_cycles": deps["cycles"],
        "filenames": _filename_problems(rows) + _unrecognized_files(base),
        "agent_paths": _agent_problems(rows),
        "scope_conflicts": scope_conflicts(rows),
        "contracts": _contract_problems(rows),
    }
    if protocol_checks:
        protocol = PROTOCOL_ROOT / "collab.md"
        checks["dangling_links"] = dangling_links(protocol) if protocol.is_file() else [str(protocol)]
    return {
        "ok": not any(bool(value) for value in checks.values()),
        "root": str(base),
        "checks": checks,
        "warnings": {"hold_review": hold_review_report(base, rows)["candidates"],
                     "runtime_metadata": runtime_findings(rows)},
    }


def workload_report(
    root: Optional[Path] = None,
    records: Optional[list[dict[str, object]]] = None,
    timings: Optional[dict[str, dict[str, object]]] = None,
) -> dict[str, object]:
    """Agent별 Doing/Blocking/Ready 이력과 released hold 감사를 별도 집계한다.

    Git backlog는 live agent 생존/idle을 말해 주지 않는다. 이 보고서는 allocation 근거 중
    durable history만 제공하며 실제 live 상태는 런타임 agent 목록으로 확인한다.
    """
    base = _root(root)
    rows = catalog(base) if records is None else records
    ready = ready_report(base, rows)
    grouped: dict[str, dict[str, object]] = {}
    by_id = {str(row["id"]): row for row in rows}

    def bucket_for(agent: str) -> dict[str, object]:
        return grouped.setdefault(agent, {
            "agent": agent,
            "doing": [],
            "blocking": [],
            "ready_candidates": [],
            "hold_history": [],  # Legacy JSON key; released ownership lives in released_holds.
            "change_scopes": {},
        })

    unassigned: list[str] = []
    released_holds: list[dict[str, object]] = []
    for row in rows:
        if row["location"] != "active" or row["state"] not in {"doing", "hold"}:
            continue
        assignment = _assignment_view(row)
        if row["state"] == "hold":
            released_holds.append({
                "id": row["id"],
                "title": row["title"],
                "path": row["path"],
                "current_agent": "",
                "current_change_scope": "",
                "audit": assignment["hold_audit"],
                "runtime_metadata": runtime.from_fields(row["fields"]),
            })
            continue
        fields = row["fields"]
        assert isinstance(fields, dict)
        agent = str(assignment["agent"])
        if not agent:
            unassigned.append(str(row["id"]))
            continue
        bucket = bucket_for(agent)
        bucket["doing"].append(row["id"])  # type: ignore[union-attr]
        bucket["change_scopes"][str(row["id"])] = assignment["change_scope"]  # type: ignore[index]

    # blocked todo가 현재 doing 선행을 기다리면 그 doing Agent의 downstream blocking으로 센다.
    for blocked in ready["blocked"]:
        if not isinstance(blocked, dict):
            continue
        task_id = str(blocked.get("id", ""))
        for dep_id in blocked.get("waiting_for", []):
            dep = by_id.get(str(dep_id))
            if not dep or dep.get("state") != "doing":
                continue
            fields = dep.get("fields", {})
            if not isinstance(fields, dict):
                continue
            agent = str(fields.get("Agent", "")).strip()
            if not agent:
                continue
            bucket = bucket_for(agent)
            if task_id not in bucket["blocking"]:  # type: ignore[operator]
                bucket["blocking"].append(task_id)  # type: ignore[union-attr]

    # Ready 후보는 명시적 선행/연관 완료 Agent 이력에만 근거한다. 실제 배정은 아니다.
    for candidate in ready["ready"]:
        if not isinstance(candidate, dict):
            continue
        item_id = str(candidate.get("id", ""))
        source = by_id.get(item_id)
        if not source:
            continue
        continuity, _legacy = _continuity(rows, source)
        if not continuity:
            continue
        top = int(continuity[0].get("count", 0))
        for entry in continuity:
            if int(entry.get("count", 0)) != top:
                break
            agent = str(entry.get("agent", ""))
            if not agent:
                continue
            bucket_for(agent)["ready_candidates"].append({  # type: ignore[union-attr]
                "id": item_id,
                "evidence_ids": list(entry.get("evidence_ids", [])),
                "count": int(entry.get("count", 0)),
            })

    timings = _task_timings_for(base) if timings is None else timings
    agents: list[dict[str, object]] = []
    for bucket in grouped.values():
        bucket["doing_count"] = len(bucket["doing"])  # type: ignore[arg-type]
        bucket["blocking_count"] = len(bucket["blocking"])  # type: ignore[arg-type]
        bucket["ready_candidate_count"] = len(bucket["ready_candidates"])  # type: ignore[arg-type]
        bucket["hold_history_count"] = len(bucket["hold_history"])  # type: ignore[arg-type]
        bucket["doing_details"] = [
            {
                "id": item_id,
                "elapsed": timings.get(str(item_id), {}).get("elapsed", "-"),
                "elapsed_seconds": timings.get(str(item_id), {}).get("elapsed_seconds"),
                "lifecycle": timings.get(str(item_id), {}),
                "runtime_metadata": runtime.from_fields(by_id[str(item_id)]["fields"]),
            }
            for item_id in bucket["doing"]  # type: ignore[union-attr]
        ]
        for candidate in bucket["ready_candidates"]:  # type: ignore[union-attr]
            if isinstance(candidate, dict):
                candidate["lifecycle"] = timings.get(str(candidate.get("id", "")), {})
        bucket["runtime_metadata"] = runtime.aggregate({
            detail["id"]: detail["runtime_metadata"] for detail in bucket["doing_details"]
        })
        agents.append(bucket)
    agents.sort(key=lambda item: (-int(item["doing_count"]), -int(item["blocking_count"]), str(item["agent"])))
    return {
        "agents": agents,
        "execution_summary": {"model_effort_tracking": "removed", "basis": "RuntimeProvider/Dispatch상태 only"},
        "unassigned_doing": sorted(unassigned),
        "released_holds": sorted(released_holds, key=lambda item: str(item["id"])),
        "ready_candidate_basis": "explicit depends/related completed Agent history only; not live state or assignment",
    }

def coordinate_report(root: Optional[Path] = None, worker_cap: int = DEFAULT_IMPLEMENTATION_WORKER_CAP) -> dict[str, object]:
    base = _root(root)
    rows = catalog(base)
    ready = ready_report(base, rows)
    timings = _task_timings_for(base)
    activity = _runtime_activity(base, rows, timings)
    working_changes, working_tree_error = _working_tree_changes()
    continuity_gaps: list[dict[str, object]] = []
    for row in rows:
        if row["location"] != "active" or row["state"] != "doing":
            continue
        fields = row["fields"]
        assert isinstance(fields, dict)
        item_id = str(row["id"])
        signal = activity.get(item_id)
        if not signal:
            continue
        health = str(signal.get("health", ""))
        code = ""
        action = ""
        if health == "awaiting_finalize":
            code = "worker_completed_backlog_doing"
            action = "verify acceptance; finalize done if satisfied, otherwise dispatch a fresh worker turn"
        elif health == "worker_missing":
            code = "worker_missing_backlog_doing"
            action = "refresh live agent state; re-dispatch the same worker only with a confirmed fresh turn, otherwise reassign explicitly"
        elif health == "needs_user":
            code = "user_decision_required_backlog_doing"
            action = "release the worker claim, move the task to hold(user), record resume condition, and surface USER_DECISION_REQUIRED to Root"
        if not code:
            continue
        uncommitted = _scoped_working_tree_changes(
            working_changes, str(fields.get("변경범위", ""))
        )
        if uncommitted and health != "needs_user":
            action = "inspect and preserve uncommitted changes before recovery; " + action
        recovery: dict[str, object] = {
            "continuity_confirmed": False,
            "requires_live_agent_refresh": True,
            "requires_fresh_preflight": health != "needs_user",
            "allowed_outcomes": [
                "finalize_done", "confirmed_fresh_turn", "explicit_reassignment", "hold_user"
            ],
            "uncommitted_changes": uncommitted,
            "uncommitted_change_count": len(uncommitted),
            "working_tree_check": "ok" if working_tree_error is None else "unavailable",
        }
        if working_tree_error is not None:
            recovery["working_tree_error"] = working_tree_error
        continuity_gaps.append({
            "id": item_id,
            "title": row["title"],
            "agent": fields.get("Agent", ""),
            "change_scope": fields.get("변경범위", ""),
            "health": health,
            "runtime_state": signal.get("runtime_state"),
            "code": code,
            "action": action,
            "recovery": recovery,
            "last_activity_at": signal.get("last_activity_at"),
            "last_activity_source": signal.get("last_activity_source"),
        })
    doing = []
    active_worker_count = 0
    for row in rows:
        if row["location"] != "active" or row["state"] != "doing":
            continue
        fields = row["fields"]
        assert isinstance(fields, dict)
        item_id = str(row["id"])
        timing = timings.get(item_id, {})
        signal = activity.get(item_id)
        health = str(signal.get("health", "runtime_unknown")) if signal else "runtime_unknown"
        if _continuity_health_consumes_worker_slot(health):
            active_worker_count += 1
        doing.append({
            "id": item_id,
            "title": row["title"],
            "agent": fields.get("Agent", ""),
            "change_scope": fields.get("변경범위", ""),
            "runtime_health": health,
            "runtime_metadata": runtime.from_fields(fields),
            "elapsed": timing.get("elapsed", "-"),
            "elapsed_seconds": timing.get("elapsed_seconds"),
            "lifecycle": timing,
        })
    def with_lifecycle(values: object) -> list[dict[str, object]]:
        enriched: list[dict[str, object]] = []
        if not isinstance(values, list):
            return enriched
        for value in values:
            if not isinstance(value, dict):
                continue
            item = dict(value)
            item["lifecycle"] = timings.get(str(item.get("id", "")), {})
            enriched.append(item)
        return enriched

    ready_items = with_lifecycle(ready["ready"])
    blocked_items = with_lifecycle(ready["blocked"])
    worker_cap = max(1, int(worker_cap))
    backlog_doing_count = len(doing)
    candidate_slots = max(0, worker_cap - active_worker_count)
    recovery_to_review = min(len(continuity_gaps), candidate_slots)
    ready_slots = max(0, candidate_slots - recovery_to_review)
    ready_to_review = min(len(ready_items), ready_slots)
    hold_review = hold_review_report(base, rows)
    return {
        "snapshot_at": datetime.now(timezone.utc).astimezone().isoformat(),
        "source": "git_backlog",
        "root": str(base),
        "scheduling_needed": bool(ready_items) or bool(continuity_gaps),
        "controller_review_needed": bool(hold_review["review_needed"]) or bool(continuity_gaps),
        "continuity_gaps": continuity_gaps,
        "recovery_queue": continuity_gaps,
        "hold_review": hold_review,
        "ready": ready_items,
        "blocked": blocked_items,
        "doing": doing,
        "workload": workload_report(base, rows, timings),
        "scope_conflicts": scope_conflicts(rows),
        "parallel_fill": {
            "implementation_worker_cap": worker_cap,
            "active_doing": active_worker_count,
            "backlog_doing": backlog_doing_count,
            "recovery_count": len(continuity_gaps),
            "candidate_slots": candidate_slots,
            "recovery_to_review_this_pass": recovery_to_review,
            "ready_count": len(ready_items),
            "ready_to_review_this_pass": ready_to_review,
            "review_required": bool(recovery_to_review) or bool(ready_to_review),
            "meaning": "resolve continuity recovery first, then inspect/allocate ready tasks before waiting; final dispatch still requires fresh preflight, live-state and scope checks",
        },
        "worker_naming": {
            "prefix": "/root/controller/",
            "pool": list(POKEMON_WORKER_NAMES),
            "task_derived_names_forbidden": True,
            "allocator": "worker-name --used <live worker path/alias> ... --json",
            "new_validation": "agent <path> --new --json",
            "reuse_validation": "agent <existing-path> --json; preserve actual identity",
        },
        "contract": {
            "fresh_snapshot": True,
            "live_agent_state_source": "runtime list_agents; not inferred from Git",
            "worker_selection_is_controller_judgment": True,
            "adaptive_worker_allocation": True,
            "parallel_fill_invariant": "do not wait after the first assignment while another independent ready task and an implementation slot remain",
            "batch_before_wait": "build and dispatch the full safe allocation batch for this pass before wait_agent",
            "continuity_wait_exception": "waiting for a busy preferred worker requires strong continuity plus concrete near-term completion evidence; vague 'soon' is insufficient",
            "prefer_continuity_then_avoid_unnecessary_wait": True,
            "controller_drain_invariant": "after DONE/BLOCKED, refresh and refill freed slots until no actionable ready and no active workers remain",
            "spawn_proxy": "if controller cannot spawn in this runtime, /root executes controller's batch spawn request without rejudging",
            "dispatch_requires_recheck": "inspect <ID> --json immediately before doing/dispatch",
            "no_work_statement_requires": "this turn's coordinate + live agent state",
            "individual_completion": "settle original acceptance independently of queue drain; worker DONE alone is not proof",
            "continuity_gap_invariant": "a doing task must not remain ownerless after a worker turn ends; completed/missing/user-wait runtime signals require explicit finalize, fresh re-dispatch/reassignment, or hold(user)+Root escalation before the Controller can consider the pass settled",
            "self_delegation_forbidden": "a worker cannot create continuity by delegating follow-up work to itself; only a confirmed fresh runtime turn or explicit Controller reassignment counts as resumed execution",
            "orphaned_doing_does_not_consume_slot": "doing tasks whose worker is completed, missing, or waiting for user are recovery work, not active implementation workers",
            "recovery_precedes_parallel_fill": "continuity recovery is reviewed before new ready work; dispatch recovery requires fresh preflight and confirmed live-agent state",
            "uncommitted_recovery_evidence": "when a continuity gap overlaps declared change scope, uncommitted files are surfaced and must be preserved/inspected before finalize or reassignment",
            "hold_review_is_advisory": "review this event, not automatic readiness/spawn or a command to keep draining unchanged external waits",
        },
    }

def status_report(root: Optional[Path] = None, include_done: bool = False) -> dict[str, object]:
    base = _root(root)
    rows = catalog(base)
    ready = ready_report(base, rows)
    timings = _task_timings_for(base)
    ready_by_id = {str(item["id"]): item for item in ready["ready"]}
    blocked_by_id = {str(item["id"]): item for item in ready["blocked"]}
    ready_ids = set(ready_by_id)
    blocked_ids = set(blocked_by_id)
    active: list[dict[str, object]] = []
    done: list[dict[str, object]] = []
    for row in rows:
        fields = row["fields"]
        assert isinstance(fields, dict)
        assignment = _assignment_view(row)
        item_id = str(row["id"])
        file_state = str(row["state"])
        timing = timings.get(item_id, {})
        item = {
            "id": item_id,
            "title": row["title"],
            "state": ("ready" if item_id in ready_ids else "blocked" if item_id in blocked_ids else file_state),
            "file_state": file_state,
            "agent": assignment["agent"],
            "change_scope": assignment["change_scope"],
            "assignment_kind": assignment["assignment_kind"],
            "hold_audit": assignment["hold_audit"],
            "runtime_metadata": runtime.from_fields(fields),
            "wait_note": fields.get("대기", ""),
            "depends_on": _refs(str(fields.get("선행", ""))),
            "waiting_for": blocked_by_id.get(item_id, {}).get("waiting_for", []),
            "path": row["path"],
            "mtime": row["mtime"],
            "time": timing.get("elapsed", "-") if file_state == "doing" else timing.get("duration", "-") if file_state == "done" else "-",
            "created_at": timing.get("created_at"),
            "started_at": timing.get("started_at"),
            "claimed_at": timing.get("claimed_at"),
            "completed_at": timing.get("completed_at"),
            "queue_time": timing.get("queue", "-"),
            "work_time": timing.get("work", "-"),
            "lead_time": timing.get("lead", "-"),
            "lifecycle": timing,
        }
        if row["location"] == "active":
            active.append(item)
        elif include_done and file_state == "done":
            done.append(item)
    done.sort(key=lambda item: (str(item.get("completed_at") or ""), str(item["id"])), reverse=True)
    problems = ready["problems"]
    assert isinstance(problems, dict)
    health = {
        "audit": audit_report(base, rows),
        "duplicate_ids": _duplicate_ids(rows),
        "missing_dependencies": problems["missing"],
        "dependency_cycles": problems["cycles"],
        "agent_paths": _agent_problems(rows),
        "scope_conflicts": scope_conflicts(rows),
        "hold_review": hold_review_report(base, rows)["candidates"],
        "runtime_metadata": runtime_findings(rows),
    }
    return {
        "snapshot_at": datetime.now().astimezone().isoformat(timespec="seconds"),
        "root": str(base),
        "active": active,
        "done": done,
        "health": health,
        "hold_review": hold_review_report(base, rows),
        "execution_summary": workload_report(base, rows, timings)["execution_summary"],
        "counts": {
            "doing": sum(1 for item in active if item["file_state"] == "doing"),
            "ready": len(ready_ids),
            "blocked": len(blocked_ids),
            "hold": sum(1 for item in active if item["file_state"] == "hold"),
            "done_shown": len(done),
            "done_total": sum(1 for row in rows if row["state"] == "done"),
            "warnings": sum(len(value) for value in health.values()),
        },
    }

def _hold_review_lines(report: dict[str, object]) -> list[str]:
    candidates = report.get("candidates", [])
    waiting = report.get("waiting", [])
    if not candidates and not waiting:
        return []
    lines = [f"Hold review: {len(candidates)} · documented wait: {len(waiting)} (검토 권고, 자동 재개 아님)"]
    for row in [*candidates, *waiting]:
        codes = ", ".join(reason["code"] for reason in row["reasons"]) or "recorded_wait"
        lines.extend((
            f"  {row['id']} hold [{row['wait_kind']}] {codes}",
            f"    대기: {row['wait_note'] or '-'} / 재개조건: {row['resume_condition'] or '-'}",
            f"    근거: {row['wait_evidence'] or '-'} / 파일: {row['path']}",
        ))
        for dep in row["dependencies"]:
            lines.append(f"    선행 {dep['id']}: {dep['status']} (states={','.join(dep['states']) or '-'})")
    return lines


def render_status(report: dict[str, object]) -> str:
    counts = report["counts"]
    assert isinstance(counts, dict)
    lines = [
        "Task Mecca · root/registrar/controller/worker backlog",
        f"updated={report['snapshot_at']}",
        f"root={report['root']}",
        (
            f"doing={counts['doing']} ready={counts['ready']} blocked={counts['blocked']} "
            f"hold={counts['hold']} done={counts['done_total']} warnings={counts['warnings']}"
        ),
        "",
        "Active",
    ]
    active = report["active"]
    assert isinstance(active, list)
    if not active:
        lines.append("(active 항목 없음)")
    for item in active:
        assert isinstance(item, dict)
        agent = str(item.get("agent") or "-")
        lines.append(f"{str(item['id']):<8} {str(item['state']):<8} {agent:<24} {str(item.get('time') or '-'):<12} {item['title']}")
        if item.get("depends_on"):
            lines.append(f"         deps: {', '.join(str(value) for value in item['depends_on'])}")
        if item.get("waiting_for"):
            lines.append(f"         blocked by: {', '.join(str(value) for value in item['waiting_for'])}")
        if item.get("wait_note"):
            lines.append(f"         wait: {item['wait_note']}")
        if item.get("change_scope"):
            lines.append(f"         scope: {item['change_scope']}")
        if item.get("runtime_metadata"):
            lines.extend("         " + line for line in runtime.summary_lines(item["runtime_metadata"]))
    lines.extend(_hold_review_lines(report.get("hold_review", {})))
    done = report["done"]
    assert isinstance(done, list)
    if done:
        lines.extend(("", "Done"))
        for item in done:
            assert isinstance(item, dict)
            lines.append(f"{str(item['id']):<8} done     {str(item.get('agent') or '-'):<24} {str(item.get('time') or '-'):<12} {item['title']}")
    health = report["health"]
    assert isinstance(health, dict)
    warnings = [(name, value) for name, value in health.items() if value]
    if warnings:
        lines.extend(("", "Warnings"))
        for name, value in warnings:
            lines.append(f"- {name}: {len(value)}")
    lines.extend(("", "Live agents: use the current model runtime agent list; this local monitor does not infer liveness from Git."))
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Legacy terminal renderer retained only for compatibility tests; human UI is Web UI.
# ---------------------------------------------------------------------------

ANSI_RESET = "\x1b[0m"
ANSI_BOLD = "\x1b[1m"
ANSI_DIM = "\x1b[2m"
ANSI_CYAN = "\x1b[38;5;51m"
ANSI_GREEN = "\x1b[38;5;82m"
ANSI_YELLOW = "\x1b[38;5;226m"
ANSI_MAGENTA = "\x1b[38;5;213m"
ANSI_RED = "\x1b[38;5;203m"
ANSI_BLUE = "\x1b[38;5;75m"
ANSI_GRAY = "\x1b[38;5;245m"
ANSI_WHITE = "\x1b[38;5;255m"
ANSI_BG_SELECTED = "\x1b[48;5;24m"
ANSI_BG_TAB = "\x1b[48;5;236m"

DASHBOARD_TABS = ("All", "Ready", "Working", "Blocked", "Done", "Hold", "Issues", "Search")
DASHBOARD_HELP = (
    "w Execution  ↑↓ 이동  ←→ 트리 접기/펼치기  t 제목펼침  [/] 최근완료±5  "
    "Tab/1~8 뷰  Enter 상세  / 검색  r 새로고침  ? 도움말  q 종료"
)
DASHBOARD_DETAIL_HELP = (
    "↑↓ Task  ⇧↑⇧↓ 본문  PgUp/PgDn Page  Home/End  m Raw/Rendered  Enter/Esc 닫기  q 종료"
)
_DASHBOARD_2BEOL_SHORTCUTS = {
    "ㅂ": "q", "ㅃ": "q", "ㄱ": "r", "ㄲ": "r", "ㅅ": "t", "ㅆ": "t", "ㅡ": "m",
}


@dataclass
class DashboardState:
    tab: int = 0
    selected: int = 0
    detail: bool = False
    detail_scroll: int = 0
    detail_raw: bool = False
    search_mode: bool = False
    search_query: str = ""
    search_result_ids: list[str] = field(default_factory=list)
    help: bool = False
    workload: bool = False
    workload_scroll: int = 0
    stopped: bool = False
    force_refresh: bool = False
    collapsed: set[str] = field(default_factory=set)
    expanded: set[str] = field(default_factory=set)
    recent_done_limit: int = 5
    notice: str = ""
    notice_until: float = 0.0


def _ansi(text: str, *codes: str) -> str:
    return "".join(codes) + text + ANSI_RESET if codes else text


def _display_width(text: str) -> int:
    return sum(2 if unicodedata.east_asian_width(ch) in "WF" else 1 for ch in text)


def _clip_display(text: str, width: int) -> str:
    text = str(text).replace("\r", " ").replace("\n", " ")
    if width <= 0:
        return ""
    if _display_width(text) <= width:
        return text
    out: list[str] = []
    used = 0
    for char in text:
        step = 2 if unicodedata.east_asian_width(char) in "WF" else 1
        if used + step > max(0, width - 1):
            break
        out.append(char)
        used += step
    return "".join(out) + "…"


def _fit_display(text: str, width: int) -> str:
    clipped = _clip_display(text, width)
    return clipped + " " * max(0, width - _display_width(clipped))


def _wrap_display(text: str, width: int) -> list[str]:
    width = max(4, width)
    output: list[str] = []
    for paragraph in str(text).replace("\r", "").expandtabs(4).split("\n"):
        current = ""
        for char in paragraph:
            if _display_width(current + char) > width:
                output.append(current.rstrip())
                current = ""
            current += char
        output.append(current.rstrip())
    return output or [""]


def _age(iso_value: object) -> str:
    """mtime 기반 fallback age. Task Time의 정식 근거는 Git 상태 전환 이력이다."""
    try:
        changed = datetime.fromisoformat(str(iso_value))
        seconds = max(0, int((datetime.now(timezone.utc) - changed.astimezone(timezone.utc)).total_seconds()))
    except (TypeError, ValueError):
        return "-"
    return _format_duration(float(seconds))


def _git(args: list[str], repo: Path) -> str:
    """읽기 전용 git 호출. 실패하면 빈 문자열을 반환해 snapshot 생성이 죽지 않게 한다."""
    try:
        done = subprocess.run(
            ["git", *args], cwd=repo, capture_output=True, text=True, timeout=30
        )
    except (OSError, subprocess.TimeoutExpired):
        return ""
    return done.stdout if done.returncode == 0 else ""


def _repo_root(root: Path) -> Path:
    probe = root
    while not probe.exists() and probe != probe.parent:
        probe = probe.parent
    found = _git(["rev-parse", "--show-toplevel"], probe).strip()
    return Path(found) if found else probe.parent


ACCESS_CACHE_REL = Path(".runtime") / "access_preflight.json"
ACCESS_CACHE_MAX_AGE_SECONDS = int(os.getenv("TASK_MECCA_ACCESS_CACHE_SECONDS", "900"))


def _path_within(path: Path, parent: Path) -> bool:
    try:
        path.resolve().relative_to(parent.resolve())
        return True
    except ValueError:
        return False


def _probe_directory_write(directory: Path, label: str) -> dict[str, object]:
    """Write-and-delete a tiny exclusive probe file without mutating user content."""
    token = f".task_mecca_access_probe_{os.getpid()}_{time.time_ns()}"
    path = directory / token
    try:
        directory.mkdir(parents=True, exist_ok=True)
        with path.open("x", encoding="utf-8") as stream:
            stream.write("task-mecca-access-probe\n")
        path.unlink(missing_ok=True)
        return {"name": label, "ok": True, "status": "pass", "path": str(directory)}
    except OSError as exc:
        try:
            path.unlink(missing_ok=True)
        except OSError:
            pass
        return {
            "name": label,
            "ok": False,
            "status": "fail",
            "path": str(directory),
            "error": f"{type(exc).__name__}: {exc}",
        }


def _codex_config_hints() -> dict[str, object]:
    """Read config only as a hint; effective session policy always wins."""
    config = Path(os.getenv("CODEX_HOME", str(Path.home() / ".codex"))) / "config.toml"
    result: dict[str, object] = {"path": str(config), "readable": False}
    try:
        text = config.read_text(encoding="utf-8")
    except OSError:
        return result
    result["readable"] = True
    for key in ("sandbox_mode", "sandbox", "approval_policy", "approvals_reviewer"):
        match = re.search(rf"(?m)^\s*{re.escape(key)}\s*=\s*[\"']([^\"']+)[\"']\s*$", text)
        if match:
            result[key] = match.group(1)
    return result


def _access_cache_path(base: Path) -> Path:
    # Runtime metadata belongs to Task Mecca itself, not to the currently selected
    # backlog folder. This keeps liveness/access state stable when the UI switches ledgers.
    return TASK_MECCA_ROOT / ACCESS_CACHE_REL


def _write_access_cache(base: Path, report: dict[str, object]) -> None:
    path = _access_cache_path(base)
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        temporary = path.with_suffix(".tmp")
        temporary.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        temporary.replace(path)
    except OSError:
        # A read-only/restricted session can still return the preflight report.
        return


def _read_access_cache(base: Path) -> Optional[dict[str, object]]:
    path = _access_cache_path(base)
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return None
    return data if isinstance(data, dict) else None


def _sandbox_marker_state(raw: str) -> tuple[str, Optional[str]]:
    """Classify known Codex sandbox markers without treating every non-empty value as restricted.

    The environment marker is diagnostic, not the sole source of truth.  Unknown
    values remain ``unknown`` and the effective write probes decide readiness.
    """
    value = raw.strip().lower()
    if not value:
        return "absent", None
    restricted = {
        "1", "true", "yes", "on", "sandbox", "read-only", "workspace-write",
        "seatbelt", "landlock", "seccomp",
    }
    full = {"0", "false", "no", "off", "none", "full-access", "danger-full-access", "unrestricted"}
    if value in restricted:
        return "restricted", raw
    if value in full:
        return "full_hint", raw
    return "unknown", raw


def access_preflight(root: Optional[Path] = None, *, active_probe: bool = True) -> dict[str, object]:
    """Evaluate effective orchestration access without trusting config alone.

    Codex sets ``CODEX_SANDBOX`` on sandboxed child processes.  Full filesystem
    access is therefore confirmed only when no current sandbox marker contradicts
    it *and* harmless effective probes can write both Git metadata and a location
    outside the repository.  Network access is reported separately because Codex
    treats it independently from filesystem sandbox mode.
    """
    base = _root(root)
    now = datetime.now(timezone.utc).astimezone()
    sandbox_env = str(os.getenv("CODEX_SANDBOX", "")).strip()
    sandbox_marker, sandbox_marker_value = _sandbox_marker_state(sandbox_env)
    network_disabled_raw = str(os.getenv("CODEX_SANDBOX_NETWORK_DISABLED", "")).strip()
    network_disabled = network_disabled_raw.lower() not in {"", "0", "false", "no"}

    if not active_probe:
        # A recognized current restriction overrides a previous successful cache.
        if sandbox_marker == "restricted":
            return {
                "status": "restricted",
                "full_access_confirmed": False,
                "orchestration_ready": False,
                "checked_at": now.isoformat(timespec="seconds"),
                "source": "runtime_environment",
                "sandbox_env": sandbox_env,
                "sandbox_marker": sandbox_marker,
                "network": "disabled" if network_disabled else "unknown",
                "message": "현재 프로세스에 제한된 Codex sandbox 신호가 있습니다. subagent dispatch 전에 Full Access를 활성화하세요.",
                "dispatch_recheck_required": True,
                "restriction_current": True,
                "reasons": [f"CODEX_SANDBOX={sandbox_env}"],
            }
        cached = _read_access_cache(base)
        if cached:
            checked = _parse_iso(str(cached.get("checked_at", "")))
            age = (now - checked.astimezone(now.tzinfo)).total_seconds() if checked else None
            cached = dict(cached)
            cached["source"] = "cached_effective_probe"
            cached["cache_age_seconds"] = age
            cached["cache_stale"] = age is None or age > ACCESS_CACHE_MAX_AGE_SECONDS
            # Passive dashboard reads never authorize a future dispatch.  Keep the
            # last observed access state for operator context, but require a fresh
            # active probe immediately before every subagent spawn.  An old
            # successful probe therefore stays visually informative instead of
            # being downgraded to an alarming "unknown" state.
            cached["last_observed_status"] = cached.get("status", "unknown")
            cached["dispatch_recheck_required"] = True
            cached["orchestration_ready"] = False
            if cached.get("status") == "full":
                cached["message"] = (
                    "마지막 effective Full Access 검증은 성공했습니다. "
                    "대시보드에서는 관측 이력으로만 표시하며 subagent dispatch 직전에 자동으로 다시 검증합니다."
                )
            elif cached["cache_stale"]:
                cached["message"] = (
                    "마지막 권한 검증 결과가 오래되었습니다. 경고 상태는 아니며, "
                    "subagent dispatch 직전에 active preflight를 자동 재실행합니다."
                )
            cached["sandbox_env"] = sandbox_env or cached.get("sandbox_env")
            cached["sandbox_marker"] = sandbox_marker
            cached["restriction_current"] = sandbox_marker == "restricted"
            if network_disabled:
                cached["network"] = "disabled"
            return cached
        return {
            "status": "unknown",
            "full_access_confirmed": False,
            "orchestration_ready": False,
            "checked_at": None,
            "source": "no_effective_probe",
            "sandbox_env": sandbox_env or None,
            "sandbox_marker": sandbox_marker,
            "network": "disabled" if network_disabled else "unknown",
            "message": "아직 effective Full Access 검증 이력이 없습니다. subagent dispatch 직전에 active preflight를 자동 실행합니다.",
            "dispatch_recheck_required": True,
            "restriction_current": False,
            "reasons": [],
        }

    repo = _repo_root(base)
    probes: dict[str, dict[str, object]] = {}

    # Workspace write: use ignored ephemeral runtime storage and clean the probe.
    probes["workspace_write"] = _probe_directory_write(TASK_MECCA_ROOT / ".runtime", "workspace_write")

    # Git metadata write catches sandboxes that allow working-tree edits but deny .git.
    git_dir_text = _git(["rev-parse", "--git-dir"], repo).strip()
    if git_dir_text:
        git_dir = Path(git_dir_text)
        if not git_dir.is_absolute():
            git_dir = repo / git_dir
        probes["git_metadata_write"] = _probe_directory_write(git_dir, "git_metadata_write")
    else:
        probes["git_metadata_write"] = {
            "name": "git_metadata_write", "ok": False, "status": "fail", "error": "Git repository not detected"
        }

    try:
        done = subprocess.run(
            [sys.executable, "-c", "raise SystemExit(0)"],
            cwd=repo,
            capture_output=True,
            text=True,
            timeout=5,
        )
        probes["subprocess"] = {
            "name": "subprocess", "ok": done.returncode == 0,
            "status": "pass" if done.returncode == 0 else "fail", "returncode": done.returncode,
        }
    except (OSError, subprocess.TimeoutExpired) as exc:
        probes["subprocess"] = {
            "name": "subprocess", "ok": False, "status": "fail", "error": f"{type(exc).__name__}: {exc}"
        }

    # Prefer HOME: in the normal project layout it is outside the Git workspace and
    # workspace-write sandboxing should reject it, while Full Access can write there.
    home = Path.home().resolve()
    if _path_within(home, repo) or home == repo.resolve():
        probes["external_write"] = {
            "name": "external_write", "ok": False, "status": "unavailable",
            "path": str(home), "error": "HOME is inside the repository; no safe outside-workspace probe location"
        }
    else:
        probes["external_write"] = _probe_directory_write(home, "external_write")

    reasons: list[str] = []
    if sandbox_marker == "restricted":
        reasons.append(f"CODEX_SANDBOX={sandbox_env}")
    elif sandbox_marker == "unknown":
        reasons.append(f"CODEX_SANDBOX(unrecognized)={sandbox_env}")
    required_probe_names = ("workspace_write", "git_metadata_write", "subprocess", "external_write")
    for name in required_probe_names:
        probe = probes[name]
        if not probe.get("ok"):
            reasons.append(f"{name}={probe.get('status', 'fail')}")

    unavailable = probes["external_write"].get("status") == "unavailable"
    hard_failure = sandbox_marker == "restricted" or any(
        probes[name].get("status") == "fail" for name in required_probe_names
    )
    all_effective = all(bool(probes[name].get("ok")) for name in required_probe_names)
    if all_effective and sandbox_marker != "restricted":
        status = "full"
        message = "Effective Full Access가 확인되었습니다. subagent orchestration을 시작할 수 있습니다."
    elif hard_failure:
        status = "restricted"
        message = "Effective Full Access가 확인되지 않았습니다. subagent dispatch 전에 Codex에서 Full Access를 활성화하세요."
    elif unavailable:
        status = "unknown"
        message = "외부 workspace 쓰기 검증 위치를 확보하지 못해 Full Access를 확정할 수 없습니다."
    else:
        status = "unknown"
        message = "Effective Full Access를 확정할 수 없습니다. subagent dispatch 전에 권한을 확인하세요."

    report: dict[str, object] = {
        "status": status,
        "full_access_confirmed": status == "full",
        "orchestration_ready": status == "full",
        "dispatch_recheck_required": False,
        "restriction_current": sandbox_marker == "restricted",
        "checked_at": now.isoformat(timespec="seconds"),
        "source": "effective_probe",
        "sandbox_env": sandbox_env or None,
        "sandbox_marker": sandbox_marker,
        "permission_contract": "effective-full-filesystem-access",
        "permission_note": "Product UI toggle is not read directly; readiness is based on current runtime markers plus harmless effective probes.",
        "network": "disabled" if network_disabled else "not_explicitly_disabled",
        "network_note": "Network is independent from filesystem sandbox mode; check it separately when the task requires network.",
        "probes": probes,
        "config_hint": _codex_config_hints(),
        "reasons": reasons,
        "message": message,
    }
    _write_access_cache(base, report)
    return report


def _parse_iso(value: str) -> Optional[datetime]:
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00"))
    except (TypeError, ValueError):
        return None


def _format_duration(seconds: Optional[float]) -> str:
    if seconds is None or seconds < 0:
        return "-"
    total = int(seconds)
    minutes, sec = divmod(total, 60)
    if minutes < 60:
        return f"{minutes}m {sec:02d}s" if minutes else f"{sec}s"
    hours, minutes = divmod(minutes, 60)
    if hours < 24:
        return f"{hours}h {minutes:02d}m {sec:02d}s"
    days, hours = divmod(hours, 24)
    return f"{days}d {hours:02d}h {minutes:02d}m {sec:02d}s"




def _lifecycle_journal_path() -> Path:
    return TASK_MECCA_ROOT / ".runtime" / "lifecycle_observations.json"


def _load_lifecycle_journal() -> dict[str, object]:
    path = _lifecycle_journal_path()
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return {"version": 1, "ledgers": {}}
    if not isinstance(data, dict):
        return {"version": 1, "ledgers": {}}
    ledgers = data.get("ledgers")
    if not isinstance(ledgers, dict):
        data["ledgers"] = {}
    data["version"] = 1
    return data


def _save_lifecycle_journal(data: dict[str, object]) -> None:
    path = _lifecycle_journal_path()
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    tmp.replace(path)


def _journal_events_for(
    journal: dict[str, object], ledger: Path, item_id: str
) -> list[dict[str, str]]:
    ledgers = journal.setdefault("ledgers", {})
    if not isinstance(ledgers, dict):
        ledgers = {}
        journal["ledgers"] = ledgers
    ledger_key = str(ledger.resolve())
    ledger_row = ledgers.setdefault(ledger_key, {"items": {}})
    if not isinstance(ledger_row, dict):
        ledger_row = {"items": {}}
        ledgers[ledger_key] = ledger_row
    items = ledger_row.setdefault("items", {})
    if not isinstance(items, dict):
        items = {}
        ledger_row["items"] = items
    raw = items.setdefault(item_id, [])
    if not isinstance(raw, list):
        raw = []
        items[item_id] = raw
    return raw  # type: ignore[return-value]


def _set_journal_events_for(
    journal: dict[str, object], ledger: Path, item_id: str, events: list[dict[str, str]]
) -> None:
    ledgers = journal.setdefault("ledgers", {})
    assert isinstance(ledgers, dict)
    ledger_key = str(ledger.resolve())
    ledger_row = ledgers.setdefault(ledger_key, {"items": {}})
    assert isinstance(ledger_row, dict)
    items = ledger_row.setdefault("items", {})
    assert isinstance(items, dict)
    if events:
        items[item_id] = events
    else:
        items.pop(item_id, None)


def task_state_timings(repo: Path, root: Optional[Path] = None) -> dict[str, dict[str, object]]:
    """Restore lifecycle history and calculate cumulative durations.

    Git state transitions are the durable source of truth.  Because controllers can
    rename/update backlog files before the lifecycle commit lands, the dashboard also
    keeps an ephemeral observation journal in ``_task_mecca/.runtime``.  The journal
    stores the *first observed* time of an uncommitted state transition so ordinary
    backlog edits cannot reset Active/Wait timers on every refresh.

    Journal observations are provisional only.  Once Git records the transition, the
    durable Git event wins and the corresponding observation is discarded.  Historical
    tasks whose doing/hold states were never committed or observed are reported as
    unobserved rather than falsely showing zero seconds.
    """
    base = _selected_ledger_root(_root(None)) if root is None else Path(root)

    def history_pathspecs() -> list[str]:
        specs: list[str] = []
        repo_root = Path(repo).resolve()

        def add(path: Path) -> None:
            try:
                value = str(path.resolve().relative_to(repo_root))
            except ValueError:
                value = path.name
            if value not in specs:
                specs.append(value)

        add(base)
        # 0.2 moves durable project data under _task_mecca/data/. Keep the exact
        # pre-0.2 ledger basename as an additional Git pathspec so historical
        # lifecycle events survive e.g. backlog_b -> data/backlog_b migration.
        if base.resolve().parent == DATA_ROOT.resolve():
            add(TASK_MECCA_ROOT / base.name)
        return specs

    pathspecs = history_pathspecs()
    out = _git([
        "log", "--reverse", "-M", "--format=@@COLLAB@@%cI", "--name-status", "--", *pathspecs
    ], repo)
    git_events: dict[str, list[tuple[str, str, str]]] = {}
    last_state: dict[str, str] = {}
    stamp = ""
    for line in out.splitlines():
        if line.startswith("@@COLLAB@@"):
            stamp = line[len("@@COLLAB@@"):].strip()
            continue
        if not stamp or not line.strip():
            continue
        parts = line.split("\t")
        if not parts:
            continue
        status = parts[0]
        if status.startswith(("R", "C")) and len(parts) >= 3:
            path_text = parts[2]
        elif status[:1] in {"A", "M"} and len(parts) >= 2:
            path_text = parts[1]
        else:
            continue
        match = ITEM_RE.fullmatch(Path(path_text).name)
        if not match:
            continue
        item_id = match.group("id").upper()
        state = match.group("state")
        if last_state.get(item_id) == state:
            continue
        git_events.setdefault(item_id, []).append((state, stamp, "git"))
        last_state[item_id] = state

    current_rows: dict[str, dict[str, object]] = {}
    for row in catalog(base):
        item_id = str(row.get("id", "")).upper()
        existing = current_rows.get(item_id)
        if existing is None or (row.get("location") == "active" and existing.get("location") != "active"):
            current_rows[item_id] = row

    now = datetime.now(timezone.utc).astimezone()
    journal = _load_lifecycle_journal()
    journal_changed = False
    events: dict[str, list[tuple[str, str, str]]] = {item_id: list(seq) for item_id, seq in git_events.items()}

    for item_id, row in current_rows.items():
        current_state = str(row.get("state", ""))
        if not current_state:
            continue
        durable = git_events.get(item_id, [])
        durable_last_state = durable[-1][0] if durable else None
        durable_last_dt = _parse_iso(durable[-1][1]) if durable else None

        cached_raw = _journal_events_for(journal, base, item_id)
        cached: list[dict[str, str]] = []
        for entry in cached_raw:
            if not isinstance(entry, dict):
                continue
            state = str(entry.get("state", ""))
            at = str(entry.get("at", ""))
            dt = _parse_iso(at)
            if state in {"todo", "doing", "hold", "done"} and dt is not None:
                if durable_last_dt is None or dt >= durable_last_dt.astimezone(dt.tzinfo):
                    cached.append({"state": state, "at": at})
        if cached != cached_raw:
            _set_journal_events_for(journal, base, item_id, cached)
            journal_changed = True

        # If Git already reflects the current state there is no unresolved worktree
        # transition to observe.  Any older provisional journal is redundant.
        if durable_last_state == current_state:
            if cached:
                _set_journal_events_for(journal, base, item_id, [])
                journal_changed = True
            cached = []
        else:
            known_state = cached[-1]["state"] if cached else durable_last_state
            if known_state != current_state:
                # Capture the first observable bound once.  Filesystem ctime/mtime is
                # used only for this initial observation; subsequent backlog edits do
                # not move the timer because the journal retains the original value.
                observed_raw = str(row.get("ctime") or row.get("mtime") or "") if os.name != "nt" else str(row.get("mtime") or "")
                observed_dt = _parse_iso(observed_raw) or now
                observed_dt = observed_dt.astimezone(now.tzinfo)
                previous_dt = _parse_iso(cached[-1]["at"]) if cached else durable_last_dt
                if previous_dt is not None and observed_dt < previous_dt.astimezone(now.tzinfo):
                    observed_dt = now
                cached.append({"state": current_state, "at": observed_dt.isoformat(timespec="seconds")})
                _set_journal_events_for(journal, base, item_id, cached)
                journal_changed = True

        combined = list(durable)
        last_combined_state = combined[-1][0] if combined else None
        for entry in cached:
            state = entry["state"]
            if state == last_combined_state:
                continue
            combined.append((state, entry["at"], "runtime_observed"))
            last_combined_state = state
        events[item_id] = combined

    if journal_changed:
        try:
            _save_lifecycle_journal(journal)
        except OSError:
            # Timing remains usable in-memory even if an environment disallows the
            # optional ephemeral journal.  It will simply be re-observed next refresh.
            pass

    result: dict[str, dict[str, object]] = {}
    labels = {"todo": "Registered", "doing": "Started", "hold": "Hold", "done": "Completed"}
    for item_id, seq in events.items():
        parsed = [(state, at, _parse_iso(at), source) for state, at, source in seq]
        parsed = [(state, at, dt, source) for state, at, dt, source in parsed if dt is not None]
        if not parsed:
            continue
        parsed.sort(key=lambda x: x[2])
        created_at = parsed[0][1]
        has_doing = any(state == "doing" for state, _at, _dt, _source in parsed)
        has_hold = any(state == "hold" for state, _at, _dt, _source in parsed)
        first_doing = next((at for state, at, _dt, _source in parsed if state == "doing"), "")
        latest_doing = next((at for state, at, _dt, _source in reversed(parsed) if state == "doing"), "")
        completed_at = next((at for state, at, _dt, _source in reversed(parsed) if state == "done"), "")
        active_seconds = 0.0
        wait_seconds = 0.0
        event_rows: list[dict[str, object]] = []
        for idx, (state, at, dt, source) in enumerate(parsed):
            next_dt = parsed[idx + 1][2] if idx + 1 < len(parsed) else now
            interval = max(0.0, (next_dt.astimezone(now.tzinfo) - dt.astimezone(now.tzinfo)).total_seconds())
            if state == "done":
                interval = 0.0
            elif state == "doing":
                active_seconds += interval
            elif state == "hold":
                wait_seconds += interval
            event_rows.append({
                "state": state,
                "label": labels.get(state, state.title()),
                "at": at,
                "interval_seconds": interval,
                "interval": _format_duration(interval) if interval else "-",
                "source": source,
                "provisional": source != "git",
            })

        current_state, current_state_at, current_dt, current_source = parsed[-1]
        created = parsed[0][2]
        started = next((dt for state, _at, dt, _source in parsed if state == "doing"), None)
        completed = next((dt for state, _at, dt, _source in reversed(parsed) if state == "done"), None)

        # Queue is knowable while still waiting, or after an observed start.  A
        # completed task with no observed start must not pretend its whole lead time
        # was queue time.
        if started is not None:
            queue_seconds: Optional[float] = max(0.0, (started.astimezone(now.tzinfo) - created.astimezone(now.tzinfo)).total_seconds())
        elif completed is None:
            queue_seconds = max(0.0, (now - created.astimezone(now.tzinfo)).total_seconds())
        else:
            queue_seconds = None

        lead_end = completed if completed is not None else now
        lead_seconds = max(0.0, (lead_end.astimezone(now.tzinfo) - created.astimezone(now.tzinfo)).total_seconds())
        current_segment_seconds = 0.0 if current_state == "done" else max(0.0, (now - current_dt.astimezone(now.tzinfo)).total_seconds())
        elapsed_seconds = current_segment_seconds if current_state == "doing" else None
        inferred_events = [row for row in event_rows if row["provisional"]]

        active_known = has_doing
        wait_known = has_hold or has_doing or current_state in {"todo", "doing"}
        active_value: Optional[float] = active_seconds if active_known else None
        wait_value: Optional[float] = wait_seconds if wait_known else None
        timing_incomplete = completed is not None and not has_doing

        result[item_id] = {
            "current_state": current_state,
            "current_state_at": current_state_at,
            "current_state_source": current_source,
            "current_segment_seconds": current_segment_seconds,
            "created_at": created_at,
            "started_at": first_doing or None,
            "claimed_at": latest_doing or None,
            "completed_at": completed_at or None,
            "queue_seconds": queue_seconds,
            "active_seconds": active_value,
            "wait_seconds": wait_value,
            "work_seconds": active_value,
            "lead_seconds": lead_seconds,
            "elapsed_seconds": elapsed_seconds,
            "duration_seconds": active_value if current_state == "done" else None,
            "queue": _format_duration(queue_seconds),
            "active": _format_duration(active_value),
            "wait": _format_duration(wait_value),
            "work": _format_duration(active_value),
            "lead": _format_duration(lead_seconds),
            "elapsed": _format_duration(elapsed_seconds),
            "duration": _format_duration(active_value) if current_state == "done" else "-",
            "events": event_rows,
            "lifecycle_inferred": bool(inferred_events),
            "lifecycle_inference_note": (
                "Current state includes a provisional runtime observation that is retained across refreshes until Git records the transition."
                if inferred_events else ""
            ),
            "timing_incomplete": timing_incomplete,
            "timing_incomplete_note": (
                "No doing transition was committed or observed for this completed task, so Active/Queue timing is unknown rather than zero."
                if timing_incomplete else ""
            ),
        }
    return result


def _runtime_activity(base: Path, rows: list[dict[str, object]], timings: dict[str, dict[str, object]]) -> dict[str, dict[str, object]]:
    """Combine optional ephemeral heartbeat data with durable file/Git activity.

    ``.runtime/agents/*.json`` is intentionally outside the backlog ledger.  When the
    directory exists it is treated as a runtime registry; absence means liveness is
    unknown, not dead.  Each JSON record may contain ``agent``, ``task_id``,
    ``heartbeat_at`` and ``state``.
    """
    runtime_dir = TASK_MECCA_ROOT / ".runtime" / "agents"
    registry_available = runtime_dir.is_dir()
    heartbeats: list[dict[str, object]] = []
    if registry_available:
        for path in runtime_dir.glob("*.json"):
            try:
                data = json.loads(path.read_text(encoding="utf-8"))
                if isinstance(data, dict):
                    data["_path"] = str(path)
                    heartbeats.append(data)
            except (OSError, json.JSONDecodeError):
                continue
    by_agent = {str(x.get("agent", "")): x for x in heartbeats if str(x.get("agent", ""))}
    by_task = {str(x.get("task_id", "")).upper(): x for x in heartbeats if str(x.get("task_id", ""))}
    warn = int(os.getenv("TASK_MECCA_STALE_WARN_SECONDS", "1800"))
    critical = int(os.getenv("TASK_MECCA_STALE_CRITICAL_SECONDS", "3600"))
    now = datetime.now(timezone.utc).astimezone()
    output: dict[str, dict[str, object]] = {}
    for row in rows:
        if row.get("location") != "active" or row.get("state") != "doing":
            continue
        fields = row.get("fields", {})
        agent = str(fields.get("Agent", "")) if isinstance(fields, dict) else ""
        item_id = str(row.get("id", ""))
        hb = by_task.get(item_id) or by_agent.get(agent)
        source = "file"
        last_at = str(row.get("mtime", ""))
        runtime_state = "unknown"
        if hb:
            candidate = str(hb.get("heartbeat_at") or hb.get("updated_at") or "")
            if _parse_iso(candidate):
                last_at = candidate
                source = "runtime heartbeat"
            runtime_state = str(hb.get("state") or "running")
        elif timings.get(item_id, {}).get("claimed_at"):
            git_at = str(timings[item_id]["claimed_at"])
            file_dt = _parse_iso(last_at)
            git_dt = _parse_iso(git_at)
            if git_dt and (not file_dt or git_dt > file_dt):
                last_at = git_at
                source = "git lifecycle"
        last_dt = _parse_iso(last_at)
        inactivity = max(0.0, (now - last_dt.astimezone(now.tzinfo)).total_seconds()) if last_dt else None
        normalized_state = runtime_state.strip().lower()
        if normalized_state in {"done", "completed", "complete", "finished", "succeeded", "success"}:
            health = "awaiting_finalize"
        elif normalized_state in {
            "needs_user", "user_input", "user-action-required", "user_action_required",
            "blocked_user", "waiting_for_user",
        }:
            health = "needs_user"
        elif registry_available and agent and not hb:
            health = "worker_missing"
        elif inactivity is None:
            health = "runtime_unknown"
        elif inactivity >= critical:
            health = "stale"
        elif inactivity >= warn:
            health = "quiet"
        else:
            health = "healthy" if hb else "runtime_unknown"
        output[item_id] = {
            "health": health,
            "runtime_registry_available": registry_available,
            "runtime_state": runtime_state,
            "last_activity_at": last_at or None,
            "last_activity_source": source if last_at else "none",
            "inactivity_seconds": inactivity,
            "warn_after_seconds": warn,
            "critical_after_seconds": critical,
            "agent": agent,
        }
    return output

def _dashboard_item(
    row: dict[str, object],
    state_name: str,
    waiting: list[str],
    timing: Optional[dict[str, object]] = None,
    candidates: Optional[list[dict[str, object]]] = None,
) -> dict[str, object]:
    fields = row["fields"]
    assert isinstance(fields, dict)
    timing = timing or {}
    file_state = str(row["state"])
    assignment = _assignment_view(row)
    top_agents = [str(x.get("agent", "")) for x in (candidates or []) if str(x.get("agent", ""))]
    return {
        "id": str(row["id"]),
        "sort_key": str(row["sort_key"]),
        "title": str(row["title"]),
        "state": state_name,
        "file_state": file_state,
        "location": str(row["location"]),
        "archive_month": str(row.get("archive_month", "")),
        "path": str(row["path"]),
        "mtime": row["mtime"],
        "updated_at": max(
            (
                value
                for value in (
                    [str(row.get("mtime") or "")]
                    + [str(event.get("at") or "") for event in timing.get("events", []) if isinstance(event, dict)]
                )
                if value
            ),
            key=lambda value: _parse_iso(value) or datetime.min.replace(tzinfo=timezone.utc),
            default=str(row.get("mtime") or ""),
        ),
        "agent": str(assignment["agent"]),
        "assignment_kind": assignment["assignment_kind"],
        "hold_audit": assignment["hold_audit"],
        "candidate_agents": top_agents,
        "scope": str(assignment["change_scope"]),
        "runtime_metadata": runtime.from_fields(fields),
        "document": row.get("document", {}),
        "raw_markdown": row.get("raw_markdown", ""),
        "registrant": str(fields.get("등록자", "")),
        "wait_note": str(fields.get("대기", "")),
        "depends_on": _refs(str(fields.get("선행", ""))),
        "related": _refs(str(fields.get("연관", ""))),
        "waiting_for": waiting,
        "time": timing.get("elapsed", "-") if file_state == "doing" else timing.get("duration", "-") if file_state == "done" else "-",
        "created_at": timing.get("created_at"),
        "started_at": timing.get("started_at"),
        "claimed_at": timing.get("claimed_at"),
        "completed_at": timing.get("completed_at"),
        "queue_time": timing.get("queue", "-"),
        "work_time": timing.get("work", "-"),
        "lead_time": timing.get("lead", "-"),
        "active_seconds": timing.get("active_seconds"),
        "wait_seconds": timing.get("wait_seconds"),
        "queue_seconds": timing.get("queue_seconds"),
        "lead_seconds": timing.get("lead_seconds"),
        "lifecycle": timing,
        "fields": fields,
    }

def dashboard_snapshot(root: Optional[Path] = None, *, recent_done_limit: int = 5) -> dict[str, object]:
    """Web UI/CLI에 필요한 canonical backlog snapshot. local/Git read-only다."""
    base = _root(root)
    rows = catalog(base)
    presence = backlog_presence(base, rows)
    ready = ready_report(base, rows)
    hold_review = hold_review_report(base, rows)
    review_by_path = {review["path"]: review for review in hold_review["candidates"] + hold_review["waiting"]}
    timings = _task_timings_for(base)
    activity = _runtime_activity(base, rows, timings)
    ready_ids = {str(item["id"]) for item in ready["ready"]}
    blocked = {str(item["id"]): item for item in ready["blocked"]}
    by_id: dict[str, dict[str, object]] = {}
    for row in rows:
        by_id.setdefault(str(row["id"]), row)

    continuity_map: dict[str, list[dict[str, object]]] = {}
    for item_id in ready_ids:
        source = by_id.get(item_id)
        if not source:
            continue
        continuity, _legacy = _continuity(rows, source)
        if continuity:
            top = int(continuity[0].get("count", 0))
            continuity_map[item_id] = [x for x in continuity if int(x.get("count", 0)) == top]

    all_items: dict[str, dict[str, object]] = {}
    for item_id, row in by_id.items():
        file_state = str(row["state"])
        view_state = (
            "ready" if file_state == "todo" and item_id in ready_ids
            else "blocked" if file_state == "todo" and item_id in blocked
            else file_state
        )
        waiting = blocked.get(item_id, {}).get("waiting_for", [])
        all_items[item_id] = _dashboard_item(
            row,
            view_state,
            list(waiting) if isinstance(waiting, list) else [],
            timings.get(item_id, {}),
            continuity_map.get(item_id, []),
        )
        all_items[item_id]["hold_review"] = review_by_path.get(row["path"])
        all_items[item_id]["activity"] = activity.get(item_id, {"health": "n/a"})

    visible = {
        item_id for item_id, item in all_items.items()
        if item["location"] == "active" and item["file_state"] in {"todo", "doing", "hold"}
    }
    stack = list(visible)
    while stack:
        item = all_items.get(stack.pop())
        if not item:
            continue
        for dependency in item["depends_on"]:
            if dependency in all_items and dependency not in visible:
                visible.add(dependency)
                stack.append(dependency)

    completed = sorted(
        (item for item in all_items.values() if item["file_state"] == "done"),
        key=lambda item: (str(item.get("completed_at") or item.get("mtime") or ""), str(item["sort_key"])),
        reverse=True,
    )
    for item in completed:
        item["completion_sort_at"] = item.get("completed_at") or item.get("mtime")
    visible.update(str(item["id"]) for item in completed[: max(0, recent_done_limit)])

    parent: dict[str, str] = {}
    children: dict[str, list[str]] = {item_id: [] for item_id in visible}
    for item_id in visible:
        dependencies = all_items[item_id]["depends_on"]
        first = next((dep for dep in dependencies if dep in visible and dep != item_id), "")
        if first:
            parent[item_id] = first
            children.setdefault(first, []).append(item_id)
    for values in children.values():
        values.sort(key=lambda key: (all_items[key]["sort_key"], key))

    tree_rows: list[dict[str, object]] = []
    visited: set[str] = set()
    def append_tree(item_id: str, depth: int, ancestors: list[str]) -> None:
        if item_id in visited:
            return
        visited.add(item_id)
        item = dict(all_items[item_id])
        item.update({"depth": depth, "ancestors": list(ancestors), "has_children": bool(children.get(item_id))})
        tree_rows.append(item)
        for child in children.get(item_id, []):
            append_tree(child, depth + 1, ancestors + [item_id])

    roots = sorted((item_id for item_id in visible if item_id not in parent), key=lambda key: (all_items[key]["sort_key"], key))
    for item_id in roots:
        append_tree(item_id, 0, [])
    for item_id in sorted(visible - visited):
        append_tree(item_id, 0, [])

    health = doctor_report(base, records=rows)["checks"] if presence["ok"] else {"backlog_presence": [presence]}
    assert isinstance(health, dict)
    health["hold_review"] = hold_review["candidates"]
    health["runtime_metadata"] = runtime_findings(rows)
    workload = workload_report(base, rows, timings)
    ledger_root = _selected_ledger_root(base)
    repo = _repo_root(ledger_root)
    access = access_preflight(ledger_root, active_probe=False)
    return {
        "snapshot_at": datetime.now().astimezone().isoformat(timespec="seconds"),
        "root": str(ledger_root),
        "repo": repo.name,
        "backlog_presence": presence,
        "tree_rows": tree_rows,
        "all_items": all_items,
        "active_ids": sorted(
            item_id for item_id, item in all_items.items()
            if item["location"] == "active" and item["file_state"] in {"todo", "doing", "hold"}
        ),
        "unrecognized_files": _unrecognized_files(base),
        "done_items": completed,
        "health": health,
        "workload": workload,
        "task_timings": timings,
        "activity": activity,
        "hold_review": hold_review,
        "access": access,
        "attention": [
            {"id": item_id, **signal}
            for item_id, signal in activity.items()
            if signal.get("health") in {"quiet", "stale", "worker_missing"}
        ],
        "counts": {
            "working": sum(1 for item in all_items.values() if item["file_state"] == "doing"),
            "ready": len(ready_ids),
            "blocked": len(blocked),
            "hold": sum(1 for item in all_items.values() if item["file_state"] == "hold"),
            "hold_review": len(hold_review["candidates"]),
            "done": len(completed),
            "issues": sum(len(value) if isinstance(value, list) else int(bool(value)) for value in health.values()),
            "attention": sum(1 for signal in activity.values() if signal.get("health") in {"quiet", "stale", "worker_missing"}),
        },
    }

def _collapsed_rows(rows: list[dict[str, object]], collapsed: set[str]) -> list[dict[str, object]]:
    return [row for row in rows if not any(str(parent) in collapsed for parent in row.get("ancestors", []))]


def _dashboard_rows(snapshot: dict[str, object], state: DashboardState) -> list[dict[str, object]]:
    tab = DASHBOARD_TABS[state.tab]
    tree = [row for row in snapshot.get("tree_rows", []) if isinstance(row, dict)]
    all_items = snapshot.get("all_items", {})
    assert isinstance(all_items, dict)
    if tab == "All":
        return _collapsed_rows(tree, state.collapsed)
    if tab == "Done":
        return [item for item in snapshot.get("done_items", []) if isinstance(item, dict)]
    if tab == "Search":
        return [all_items[item_id] for item_id in state.search_result_ids if item_id in all_items]
    wanted = {"Ready": "ready", "Working": "doing", "Blocked": "blocked", "Hold": "hold"}.get(tab)
    return [row for row in tree if row.get("state") == wanted] if wanted else []


def _search_snapshot(snapshot: dict[str, object], query: str) -> list[str]:
    words = _normalize_words(query)
    all_items = snapshot.get("all_items", {})
    assert isinstance(all_items, dict)
    scored: list[tuple[int, str]] = []
    for item_id, item in all_items.items():
        if not isinstance(item, dict):
            continue
        fields = item.get("fields", {})
        text = " ".join((str(item_id), str(item.get("title", "")), json.dumps(fields, ensure_ascii=False)))
        normalized = unicodedata.normalize("NFKC", text).lower()
        score = sum(3 for word in words if word in normalized)
        if query.lower() in normalized:
            score += 8
        if score:
            scored.append((score, str(item_id)))
    scored.sort(key=lambda value: (-value[0], value[1]))
    return [item_id for _, item_id in scored[:50]]


def apply_dashboard_key(key: str, state: DashboardState, snapshot: dict[str, object]) -> DashboardState:
    new = replace(
        state,
        collapsed=set(state.collapsed),
        expanded=set(state.expanded),
        search_result_ids=list(state.search_result_ids),
        force_refresh=False,
    )
    if not new.search_mode:
        key = _DASHBOARD_2BEOL_SHORTCUTS.get(key, key)
    if new.search_mode:
        if key in {"\r", "\n"}:
            new.search_mode = False
            new.search_result_ids = _search_snapshot(snapshot, new.search_query.strip()) if new.search_query.strip() else []
            new.tab = DASHBOARD_TABS.index("Search")
            new.selected = 0
        elif key in {"\x1b", "\x03"}:
            new.search_mode = False
        elif key in {"\x7f", "\b"}:
            new.search_query = new.search_query[:-1]
        elif key and key.isprintable() and not key.startswith("\x1b"):
            new.search_query += key
        return new
    if new.help:
        if key in {"q", "\x03"}:
            new.stopped = True
        elif key in {"?", "\x1b", "\r", "\n"}:
            new.help = False
        return new
    if new.workload:
        if key in {"q", "\x03"}:
            new.stopped = True
        elif key in {"w", "\x1b", "\r", "\n"}:
            new.workload = False
        elif key in {"j", "\x1b[B"}:
            new.workload_scroll += 1
        elif key in {"k", "\x1b[A"}:
            new.workload_scroll = max(0, new.workload_scroll - 1)
        elif key == "\x1b[6~":
            new.workload_scroll += 10
        elif key == "\x1b[5~":
            new.workload_scroll = max(0, new.workload_scroll - 10)
        elif key in {"\x1b[H", "\x1b[1~"}:
            new.workload_scroll = 0
        elif key in {"\x1b[F", "\x1b[4~"}:
            new.workload_scroll = 10**9
        elif key == "r":
            new.force_refresh = True
        return new

    rows = _dashboard_rows(snapshot, new)
    new.selected = max(0, min(new.selected, max(0, len(rows) - 1)))
    selected = rows[new.selected] if rows else None
    if key in {"q", "\x03"}:
        new.stopped = True
    elif key == "?":
        new.help = True
    elif key == "w":
        new.workload, new.workload_scroll = True, 0
    elif key == "/":
        new.search_mode = True
        new.search_query = ""
    elif key == "\t":
        new.tab = (new.tab + 1) % len(DASHBOARD_TABS)
        new.selected, new.detail, new.detail_scroll = 0, False, 0
    elif key in {str(number) for number in range(1, 9)}:
        new.tab = int(key) - 1
        new.selected, new.detail, new.detail_scroll = 0, False, 0
    elif key == "[":
        new.recent_done_limit = max(0, new.recent_done_limit - 5)
        new.force_refresh = True
        new.notice = f"Recent Done: {new.recent_done_limit}"
        new.notice_until = time.monotonic() + 3.0
    elif key == "]":
        new.recent_done_limit += 5
        new.force_refresh = True
        new.notice = f"Recent Done: {new.recent_done_limit}"
        new.notice_until = time.monotonic() + 3.0
    elif new.detail and key in {"\x1b[1;2A", "\x1b[1;2a"}:
        new.detail_scroll = max(0, new.detail_scroll - 1)
    elif new.detail and key in {"\x1b[1;2B", "\x1b[1;2b"}:
        new.detail_scroll += 1
    elif new.detail and key in {"\x1b[5~", "\x1b[5;2~"}:
        new.detail_scroll = max(0, new.detail_scroll - 10)
    elif new.detail and key in {"\x1b[6~", "\x1b[6;2~"}:
        new.detail_scroll += 10
    elif new.detail and key in {"\x1b[H", "\x1b[1~", "\x1b[1;2H"}:
        new.detail_scroll = 0
    elif new.detail and key in {"\x1b[F", "\x1b[4~", "\x1b[1;2F"}:
        new.detail_scroll = 10**9
    elif new.detail and key == "m":
        new.detail_raw = not new.detail_raw
        new.detail_scroll = 0
    elif new.detail and key == "\x1b":
        new.detail, new.detail_scroll = False, 0
    elif key in {"j", "\x1b[B"}:
        new.selected = min(new.selected + 1, max(0, len(rows) - 1))
        new.detail_scroll = 0
    elif key in {"k", "\x1b[A"}:
        new.selected = max(0, new.selected - 1)
        new.detail_scroll = 0
    elif key == "\x1b[D" and selected and selected.get("has_children"):
        new.collapsed.add(str(selected["id"]))
    elif key == "\x1b[C" and selected:
        new.collapsed.discard(str(selected["id"]))
    elif key == "t" and selected:
        item_id = str(selected["id"])
        new.expanded.discard(item_id) if item_id in new.expanded else new.expanded.add(item_id)
    elif key in {"\r", "\n", " "} and selected:
        new.detail = not new.detail
        new.detail_scroll = 0
    elif key == "r":
        new.force_refresh = True
    return new


def _state_color(state_name: str) -> str:
    return {
        "doing": ANSI_YELLOW,
        "ready": ANSI_CYAN,
        "blocked": ANSI_RED,
        "hold": ANSI_MAGENTA,
        "done": ANSI_GREEN,
    }.get(state_name, ANSI_WHITE)


def _help_document(width: int) -> list[str]:
    rows = [
        "[조작]",
        "↑ / ↓ 또는 j / k   작업 선택 이동",
        "← / →               dependency 하위 트리 접기 / 펼치기",
        "t                   긴 제목 펼치기 / 접기",
        "Tab 또는 1~8        All/Ready/Working/Blocked/Done/Hold/Issues/Search 전환",
        "Enter               선택 작업 상세 보기 / 닫기",
        "Shift+↑ / Shift+↓   상세 본문 한 줄 스크롤",
        "PgUp / PgDn         상세 본문 페이지 스크롤",
        "Home / End          상세 본문 처음 / 끝",
        "m                   상세 Markdown Rendered / Raw 전환",
        "/                   전체 active/archive 백로그 검색",
        "[ / ]               All 뷰 Recent Done을 5개씩 조절",
        "r                   즉시 새로고침",
        "? 또는 Esc          도움말 닫기",
        "q                   종료",
        "",
        "[표시 의미]",
        "Time: doing=마지막 todo→doing 이후 경과, done=마지막 doing→done 구간. Git commit 시각 기준.",
        "Lifecycle: Created=최초 등록, Started=최초 doing, Completed=done, Q=생성→최초착수, W=현재/마지막 doing, L=생성→완료.",
        "Ready 이력 후보: 선행/연관 완료 Agent 이력 근거이며 live 상태나 실제 배정이 아님.",
        "Worker 이름: task ID가 아니라 pikachu/charmander 같은 안정적인 Pokémon alias를 사용.",
        "Agent Workload: Doing / downstream Blocking / Ready 이력 후보.",
        "배정 해제 감사: Hold 상세와 workload JSON의 released_holds에서 확인. 현재 worker 점유가 아님.",
        "w: 전체 Agent 실행 정보. ↑↓/j/k, PgUp/PgDn, Home/End, w/Esc 닫기.",
        "실행 정보는 RuntimeProvider/Dispatch상태와 근거만 표시한다.",
        "TUI는 local/Git read-only이며 agent를 만들거나 깨우지 않는다.",
    ]
    output: list[str] = []
    for row in rows:
        output.extend(_wrap_display(row, width))
    return output


def _markdown_inline_render(text: str) -> str:
    out = str(text)
    out = re.sub(r"!\[([^\]]*)\]\([^)]+\)", lambda m: f"[image: {m.group(1) or '-'}]", out)
    out = re.sub(r"\[([^\]]+)\]\(([^)]+)\)", lambda m: f"{m.group(1)} <{m.group(2)}>", out)
    out = re.sub(r"`([^`]+)`", lambda m: f"‹{m.group(1)}›", out)
    out = re.sub(r"\*\*([^*]+)\*\*", r"\1", out)
    out = re.sub(r"__([^_]+)__", r"\1", out)
    out = re.sub(r"(?<!\*)\*([^*]+)\*(?!\*)", r"\1", out)
    out = re.sub(r"(?<!_)_([^_]+)_(?!_)", r"\1", out)
    return out


def _markdown_lite_lines(text: str, width: int, *, raw: bool = False) -> list[str]:
    width = max(12, width)
    source = str(text or "").replace("\r\n", "\n").replace("\r", "\n")
    if raw:
        out: list[str] = []
        for line in source.split("\n"):
            out.extend(_wrap_display(line, width))
        return out or [""]
    out: list[str] = []
    in_code = False
    for original in source.split("\n"):
        stripped = original.strip()
        if stripped.startswith("```") or stripped.startswith("~~~"):
            in_code = not in_code
            if in_code:
                label = stripped[3:].strip()
                out.append(("CODE " + label).rstrip())
            continue
        if in_code:
            chunks = _wrap_display(original.expandtabs(4), max(4, width - 4))
            out.extend("    " + chunk for chunk in chunks)
            continue
        if not stripped:
            out.append("")
            continue
        if re.fullmatch(r"(?:-{3,}|\*{3,}|_{3,})", stripped):
            out.append("─" * min(width, 36))
            continue
        heading = re.match(r"^(#{1,6})\s+(.*)$", stripped)
        if heading:
            chunks = _wrap_display(_markdown_inline_render(heading.group(2)).strip(), width)
            out.extend(chunks)
            if chunks:
                out.append("─" * min(width, max(3, _display_width(chunks[-1]))))
            continue
        quote = re.match(r"^>\s?(.*)$", stripped)
        if quote:
            out.extend("│ " + x for x in _wrap_display(_markdown_inline_render(quote.group(1)), max(4, width - 2)))
            continue
        checkbox = re.match(r"^[-*+]\s+\[([ xX])\]\s+(.*)$", stripped)
        if checkbox:
            mark = "☑" if checkbox.group(1).lower() == "x" else "☐"
            chunks = _wrap_display(_markdown_inline_render(checkbox.group(2)), max(4, width - 2))
            out.append(f"{mark} {chunks[0]}")
            out.extend("  " + x for x in chunks[1:])
            continue
        bullet = re.match(r"^[-*+]\s+(.*)$", stripped)
        if bullet:
            chunks = _wrap_display(_markdown_inline_render(bullet.group(1)), max(4, width - 2))
            out.append("• " + chunks[0])
            out.extend("  " + x for x in chunks[1:])
            continue
        numbered = re.match(r"^(\d+[.)])\s+(.*)$", stripped)
        if numbered:
            prefix = numbered.group(1) + " "
            chunks = _wrap_display(_markdown_inline_render(numbered.group(2)), max(4, width - _display_width(prefix)))
            out.append(prefix + chunks[0])
            out.extend(" " * _display_width(prefix) + x for x in chunks[1:])
            continue
        out.extend(_wrap_display(_markdown_inline_render(original), width))
    return out or [""]

def _detail_document(item: dict[str, object], width: int, raw: bool) -> list[str]:
    if raw:
        try:
            return Path(str(item["path"])).read_text(encoding="utf-8-sig").splitlines()
        except OSError as exc:
            return [f"파일 읽기 실패: {exc}"]
    fields = item.get("fields", {})
    assert isinstance(fields, dict)
    candidate_text = ", ".join(item.get("candidate_agents", [])) or "-"
    rows = [
        f"{item['id']} · {item['title']}",
        f"상태: {item['state']} (file={item['file_state']}, location={item['location']})",
        f"Time: {item.get('time') or '-'}",
        f"Created: {item.get('created_at') or '-'}",
        f"Started: {item.get('started_at') or '-'}",
        f"Claimed(latest): {item.get('claimed_at') or '-'}",
        f"Completed: {item.get('completed_at') or '-'}",
        f"Queue / Work / Lead: {item.get('queue_time') or '-'} / {item.get('work_time') or '-'} / {item.get('lead_time') or '-'}",
        f"Agent: {item.get('agent') or '-'}",
        f"Ready 이력 후보: {candidate_text}",
        f"변경범위: {item.get('scope') or '-'}",
        f"등록자: {item.get('registrant') or '-'}",
        f"선행: {', '.join(item.get('depends_on', [])) or '-'}",
        f"대기 중 선행: {', '.join(item.get('waiting_for', [])) or '-'}",
        f"대기: {fields.get('대기') or '-'}",
        f"대기유형: {fields.get('대기유형') or '-'}",
        f"재개조건: {fields.get('재개조건') or '-'}",
        f"대기근거: {fields.get('대기근거') or '-'}",
        f"연관: {', '.join(item.get('related', [])) or '-'}",
        f"경로: {item['path']}",
        "",
    ]
    rows.extend(runtime.summary_lines(item.get("runtime_metadata", runtime.from_fields(fields)), detail=True))
    output: list[str] = []
    for row in rows:
        output.extend(_wrap_display(row, width))
    if item.get("hold_review"):
        review = item["hold_review"]
        for line in _hold_review_lines({"candidates" if review["review_required"] else "waiting": [review]}):
            output.extend(_wrap_display(line, width))
    for name in ("설명", "메모", "결과", "검증"):
        output.append(f"[{name}]")
        output.extend(_markdown_lite_lines(str(fields.get(name, "") or "-"), width, raw=False))
        output.append("")
    return output

def _issue_lines(snapshot: dict[str, object], width: int) -> list[str]:
    health = snapshot.get("health", {})
    assert isinstance(health, dict)
    output: list[str] = []
    for name, values in health.items():
        if not values:
            continue
        output.append(f"[{name}]")
        if isinstance(values, list):
            for value in values:
                output.extend(_wrap_display(json.dumps(value, ensure_ascii=False, default=str), width))
        else:
            output.extend(_wrap_display(str(values), width))
    return output or ["문제 없음"]


ANSI_PATTERN = re.compile(r"\x1b\[[0-9;]*m")


def _strip_ansi(text: str) -> str:
    return ANSI_PATTERN.sub("", text)


def _fit_ansi(text: str, width: int) -> str:
    plain = _strip_ansi(text)
    if _display_width(plain) <= width:
        return text + " " * max(0, width - _display_width(plain))
    return _fit_display(plain, width)


def _box_lines(
    title: str,
    rows: list[str],
    width: int,
    height: int,
    *,
    title_color: str = ANSI_CYAN,
) -> list[str]:
    width, height = max(12, width), max(3, height)
    inner = width - 2
    label = f" {title} "
    top = "┌" + _ansi(label, title_color, ANSI_BOLD) + "─" * max(0, inner - _display_width(label)) + "┐"
    output = [top]
    for row in rows[: height - 2]:
        output.append("│" + _fit_ansi(row, inner) + "│")
    while len(output) < height - 1:
        output.append("│" + " " * inner + "│")
    output.append("└" + "─" * inner + "┘")
    return output[:height]


def _side_by_side(
    left: list[str],
    right: list[str],
    left_width: int,
    right_width: int,
    height: int,
) -> list[str]:
    return [
        _fit_ansi(left[index] if index < len(left) else "", left_width)
        + " "
        + _fit_ansi(right[index] if index < len(right) else "", right_width)
        for index in range(height)
    ]


def _stack_boxes(boxes: list[list[str]], height: int, width: int) -> list[str]:
    rows = [row for box in boxes for row in box][:height]
    rows.extend(" " * width for _ in range(max(0, height - len(rows))))
    return rows


def _top_metrics(snapshot: dict[str, object], width: int) -> list[str]:
    counts = snapshot.get("counts", {})
    assert isinstance(counts, dict)
    repo = str(snapshot.get("repo") or Path(str(snapshot.get("root", "-"))).parent.name or "-")
    now = str(snapshot.get("snapshot_at", ""))[11:19] or "--:--:--"
    title = f" Task Mecca Collaboration TUI  ·  Repo: {repo} "
    right = f" {now} "
    gap = max(1, width - _display_width(title) - _display_width(right))
    line_one = _ansi(_fit_display(title + " " * gap + right, width), ANSI_BOLD, ANSI_CYAN)
    cells = (
        ("Total", sum(int(counts.get(name, 0)) for name in ("working", "ready", "blocked", "hold", "done")), ANSI_BLUE),
        ("Done", int(counts.get("done", 0)), ANSI_GREEN),
        ("Working", int(counts.get("working", 0)), ANSI_YELLOW),
        ("Ready", int(counts.get("ready", 0)), ANSI_CYAN),
        ("Blocked", int(counts.get("blocked", 0)), ANSI_RED),
        ("보류/배정해제", int(counts.get("hold", 0)), ANSI_MAGENTA),
        ("Issues", int(counts.get("issues", 0)), ANSI_RED),
    )
    per = max(11, width // len(cells))
    line_two = "".join(_ansi(_fit_display(f" {label} {value}", per), color, ANSI_BOLD) for label, value, color in cells)
    return [line_one, _fit_ansi(line_two, width)]

def _tab_line(state: DashboardState, width: int) -> str:
    names = ("전체", "Ready", "Working", "Blocked", "Done", "보류/배정해제", "Issues", "Search")
    output = "뷰: "
    for index, name in enumerate(names):
        chunk = f" [{index + 1}] {name} "
        output += (
            _ansi(chunk, ANSI_BG_SELECTED, ANSI_WHITE, ANSI_BOLD)
            if index == state.tab
            else _ansi(chunk, ANSI_BG_TAB, ANSI_GRAY)
        )
    return _fit_ansi(output, width)


def _visible_row_window(heights: list[int], selected: int, visible: int) -> tuple[int, int]:
    if not heights:
        return 0, 0
    visible = max(1, visible)
    selected = max(0, min(selected, len(heights) - 1))
    start, end = selected, selected + 1
    used = min(heights[selected], visible)
    while start > 0 and used + heights[start - 1] <= max(1, visible // 2):
        start -= 1
        used += heights[start]
    while end < len(heights) and used + heights[end] <= visible:
        used += heights[end]
        end += 1
    while start > 0 and used + heights[start - 1] <= visible:
        start -= 1
        used += heights[start]
    return start, end


def _display_agent_name(value: object) -> str:
    text = str(value or "-").strip()
    if text.startswith("/root/controller/"):
        alias = _worker_alias(text)
        return alias or text
    if text == "/root/registrar":
        return "registrar"
    if text == "/root/controller":
        return "controller"
    return text


def _display_title(row: dict[str, object]) -> str:
    title = str(row.get("title", "")).strip()
    item_id = str(row.get("id", "")).strip()
    return re.sub(r"^" + re.escape(item_id) + r"(?:\s+|[:：-]\s*)", "", title).strip() if item_id else title


def _task_table_lines(
    snapshot: dict[str, object],
    state: DashboardState,
    width: int,
    height: int,
) -> tuple[list[str], list[dict[str, object]]]:
    rows = _dashboard_rows(snapshot, state)
    state.selected = max(0, min(state.selected, max(0, len(rows) - 1)))
    if DASHBOARD_TABS[state.tab] == "Issues":
        issues = _issue_lines(snapshot, max(10, width - 2))
        output = [_ansi(_fit_display("Issue Type / Detail", width), ANSI_GRAY)]
        output.extend(_ansi(_fit_display(row, width), ANSI_RED) for row in issues[: max(1, height - 1)])
        output.extend(" " * width for _ in range(max(0, height - len(output))))
        return output[:height], []

    gap = 2
    w_id = max(7, min(24, max((_display_width(str(row.get("id", ""))) + 2 * int(row.get("depth", 0)) + 2 for row in rows), default=7)))
    w_state = 7
    w_time = 12 if width >= 100 else 9
    w_dep = 14 if width >= 110 else 10
    w_rel = 16 if width >= 150 else 0
    min_agent = 16
    min_title = 20 if width < 130 else 30
    columns = 7 if w_rel else 6
    fixed_without_title_agent = w_id + w_state + w_time + w_dep + w_rel + gap * (columns - 1)
    w_agent = max(min_agent, min(34, width - fixed_without_title_agent - min_title))
    w_title = max(12, width - fixed_without_title_agent - w_agent)
    headers = (("ID", w_id), ("State", w_state), ("Title", w_title), ("Agent", w_agent), ("Time", w_time), ("Depends On", w_dep)) + (("Related To", w_rel),) if w_rel else (("ID", w_id), ("State", w_state), ("Title", w_title), ("Agent", w_agent), ("Time", w_time), ("Depends On", w_dep))
    output = [_ansi("  ".join(_fit_display(label, size) for label, size in headers), ANSI_GRAY)]
    rendered_rows: list[list[str]] = []
    for row in rows:
        item_id = str(row.get("id", ""))
        depth = int(row.get("depth", 0)) if DASHBOARD_TABS[state.tab] != "Done" else 0
        marker = "▸ " if row.get("has_children") and item_id in state.collapsed else "▾ " if row.get("has_children") else ""
        id_text = "  " * depth + marker + item_id
        title = _display_title(row)
        title_chunks = _wrap_display(title, w_title) if item_id in state.expanded else [title]
        if row.get("state") == "ready" and row.get("candidate_agents"):
            agent_text = "→ " + "/".join(_display_agent_name(x) for x in row.get("candidate_agents", [])[:2]) + " (이력 후보)"
        elif row.get("state") == "hold" and row.get("agent"):
            agent_text = _display_agent_name(row.get("agent")) + " (이전)"
        else:
            agent_text = _display_agent_name(row.get("agent"))
        item_lines: list[str] = []
        for line_index, title_chunk in enumerate(title_chunks):
            cells = [
                _fit_display(id_text if line_index == 0 else "", w_id),
                _fit_display(str(row.get("state", "")) if line_index == 0 else "", w_state),
                _fit_display(title_chunk, w_title),
                _fit_display(agent_text if line_index == 0 else "", w_agent),
                _fit_display(str(row.get("time", "-")) if line_index == 0 else "", w_time),
                _fit_display((",".join(row.get("depends_on", [])) or "-") if line_index == 0 else "", w_dep),
            ]
            if w_rel:
                cells.append(_fit_display((",".join(row.get("related", [])) or "-") if line_index == 0 else "", w_rel))
            item_lines.append("  ".join(cells))
        rendered_rows.append(item_lines)
    start, end = _visible_row_window([len(value) for value in rendered_rows], state.selected, max(1, height - 1))
    for index in range(start, end):
        for raw in rendered_rows[index]:
            output.append(_ansi(_fit_display(raw, width), ANSI_BG_SELECTED, ANSI_WHITE) if index == state.selected else _ansi(_fit_display(raw, width), _state_color(str(rows[index].get("state", "")))))
    if not rows:
        message = "검색하려면 / 를 누르세요" if DASHBOARD_TABS[state.tab] == "Search" else "(표시할 항목이 없습니다)"
        output.append(_ansi(_fit_display(message, width), ANSI_DIM, ANSI_GRAY))
    output.extend(" " * width for _ in range(max(0, height - len(output))))
    return output[:height], rows

def _selected_panel_lines(item: Optional[dict[str, object]], width: int) -> list[str]:
    if not item:
        return ["-"]
    output: list[str] = []
    output.extend(_wrap_display(f"{item['id']}  {_display_title(item)}", width))
    output.append(f"State      {item.get('state', '-')}  (file: {item.get('file_state', '-')})")
    output.append(f"Time       {item.get('time') or '-'}")
    output.extend(_wrap_display(f"Lifecycle  Q {item.get('queue_time') or '-'} · W {item.get('work_time') or '-'} · L {item.get('lead_time') or '-'}", width))
    output.extend(_wrap_display(f"Agent      {item.get('agent') or '-'}", width))
    hold_audit = item.get("hold_audit")
    if isinstance(hold_audit, dict):
        recorded_agent = str(hold_audit.get("recorded_agent") or "-")
        recorded_scope = str(hold_audit.get("recorded_change_scope") or "-")
        output.extend(_wrap_display(f"이전 Agent={recorded_agent} · Scope={recorded_scope} (현재 배정 아님)", width))
    if item.get("candidate_agents"):
        output.extend(_wrap_display("이력 후보  " + ", ".join(item.get("candidate_agents", [])), width))
    output.extend(_wrap_display(f"등록자     {item.get('registrant') or '-'}", width))
    output.extend(_wrap_display(f"Scope      {item.get('scope') or '-'}", width))
    output.append("Depends    " + (", ".join(item.get("depends_on", [])) or "-"))
    output.append("Related    " + (", ".join(item.get("related", [])) or "-"))
    waiting = ", ".join(item.get("waiting_for", [])) or str(item.get("wait_note") or "-")
    output.extend(_wrap_display("Waiting    " + waiting, width))
    if item.get("hold_review"):
        review = item["hold_review"]
        output.extend(_wrap_display("Hold review " + (", ".join(x["code"] for x in review["reasons"]) or "recorded_wait"), width))
        output.extend(_wrap_display("재개조건   " + (review["resume_condition"] or "-"), width))
    return output

def _health_panel_lines(snapshot: dict[str, object]) -> list[str]:
    health = snapshot.get("health", {})
    assert isinstance(health, dict)
    def count(name: str) -> int:
        value = health.get(name, [])
        return len(value) if isinstance(value, list) else int(bool(value))
    return [
        f"Issues             {sum(count(name) for name in health)}",
        f"Dependency cycle   {count('dependency_cycles')}",
        f"Missing dependency {count('missing_dependencies')}",
        f"Duplicate ID       {count('duplicate_ids')}",
        f"Record/Audit       {count('audit')}",
        f"Filename           {count('filenames')}",
        f"Agent/Scope        {count('agent_paths') + count('scope_conflicts')}",
        f"Hold review        {count('hold_review')} (advisory)",
    ]


def _agent_runtime_lines(agent: dict[str, object]) -> list[str]:
    bundle = agent.get("runtime_metadata", {})
    if bundle.get("conflict"):
        lines = ["! metadata 충돌 (임의 선택 안 함)"]
        for task_id, data in bundle["tasks"].items():
            lines.extend(f"{task_id}: {line}" for line in runtime.summary_lines(data))
        return lines
    return runtime.summary_lines(bundle["metadata"]) if bundle.get("metadata") else []


def _wrap_runtime_line(line: str, width: int) -> list[str]:
    """Keep short model/effort transition tokens whole; long IDs still wrap losslessly."""
    output: list[str] = []
    current = ""
    for word in line.split(" "):
        if current and _display_width(current + " " + word) > width:
            output.append(current)
            current = ""
        if _display_width(word) > width:
            chunks = _wrap_display(word, width)
            output.extend(chunks[:-1])
            current = chunks[-1]
        else:
            current = current + " " + word if current else word
    return output + ([current] if current else [])


def _workload_panel_lines(snapshot: dict[str, object], width: int, *, full: bool = False) -> list[str]:
    report = snapshot.get("workload", {})
    if not isinstance(report, dict):
        return ["(Agent 이력 없음)"]
    agents = report.get("agents", [])
    if not isinstance(agents, list):
        agents = []
    output = []
    for row in agents:
        if not isinstance(row, dict):
            continue
        agent = _display_agent_name(row.get("agent", "-"))
        doing_details = row.get("doing_details", [])
        doing_text = "-"
        if isinstance(doing_details, list) and doing_details:
            doing_text = ",".join(f"{x.get('id')}({x.get('elapsed','-')})" for x in doing_details if isinstance(x, dict)) or "-"
        bundle = row.get("runtime_metadata", {})
        metadata = bundle.get("metadata")
        provenance = f" · {metadata['runtime_provider']} · source={metadata['requested_source']}" if metadata and not full else ""
        output.extend(_wrap_runtime_line(
            f"{agent}: D {doing_text} · B {','.join(row.get('blocking', [])) or '-'} · R {','.join(str(x.get('id')) for x in row.get('ready_candidates', []) if isinstance(x, dict)) or '-'}{provenance}",
            width,
        ))
        runtime_lines = _agent_runtime_lines(row)
        # Two logical rows in the compact panel: task/provenance then execution metadata.
        # Conflicts retain each task instead of choosing an arbitrary assignment.
        if metadata and not full:
            runtime_lines = runtime_lines[:1]
            if metadata.get("findings"):
                runtime_lines[0] += " · ! audit (w)"
        for line in runtime_lines:
            output.extend(_wrap_runtime_line(line, width))
        if full:
            for task_id, data in row.get("runtime_metadata", {}).get("tasks", {}).items():
                output.extend(_wrap_runtime_line(f"{task_id} source={data['requested_source']} · 근거={data['evidence_source']} · fallback={data['fallback_reason']}", width))
    for line in _released_hold_lines(report):
        output.extend(_wrap_display(line, width))
    return output


def _workload_box(snapshot: dict[str, object], width: int, height: int) -> list[str]:
    rows = _workload_panel_lines(snapshot, width - 4)
    if len(rows) > height - 2:
        rows = rows[:max(0, height - 3)] + ["… w: 전체 실행 정보"]
    return _box_lines("AGENT WORKLOAD", rows, width, height, title_color=ANSI_BLUE)


def _released_hold_lines(report: dict[str, object]) -> list[str]:
    released = report.get("released_holds", [])
    ids = [str(row["id"]) for row in released if isinstance(row, dict)] if isinstance(released, list) else []
    if not ids:
        return []
    return ["배정 해제 감사: " + ", ".join(ids) + " (현재 Agent 아님; Hold 상세 / released_holds)"]

def _detail_box(
    item: Optional[dict[str, object]],
    state: DashboardState,
    width: int,
    height: int,
) -> list[str]:
    if not item:
        return _box_lines("DETAIL -", ["-"], width, height)
    document = _detail_document(item, max(18, width - 4), state.detail_raw)
    visible = max(1, height - 2)
    maximum = max(0, len(document) - visible)
    state.detail_scroll = max(0, min(state.detail_scroll, maximum))
    view = document[state.detail_scroll : state.detail_scroll + visible]
    position = f"{state.detail_scroll + 1}-{min(len(document), state.detail_scroll + visible)} / {len(document)} lines"
    mode = "RAW" if state.detail_raw else "MD"
    return _box_lines(f"DETAIL {item['id']} · {mode} · {position}", view, width, height)


def _backlog_problem_screen(snapshot: dict[str, object], width: int, height: int) -> str:
    presence = snapshot.get("backlog_presence", {})
    assert isinstance(presence, dict)
    title = "BACKLOG NOT FOUND" if presence.get("status") == "missing" else "EMPTY BACKLOG"
    body = [str(presence.get("message", title)), "", f"검색 위치: {presence.get('root', snapshot.get('root', '-'))}", "", "이 화면은 읽기 전용 오류 상태를 유지합니다."]
    lines = [_ansi(_fit_display(f" Task Mecca Collaboration TUI · {title} ", width), ANSI_RED, ANSI_BOLD)]
    lines.extend(_box_lines(title, body, width, max(10, min(height - 3, len(body) + 4)), title_color=ANSI_RED))
    lines.extend(" " * width for _ in range(max(0, height - len(lines) - 2)))
    lines.extend((_ansi(_fit_display("r 새로고침   ? 도움말   q 종료", width), ANSI_GRAY), _ansi(_fit_display("READ-ONLY", width), ANSI_RED)))
    return "\n".join(lines[:height])


def render_dashboard(
    snapshot: dict[str, object],
    state: DashboardState,
    *,
    width: int = 140,
    height: int = 40,
) -> str:
    width, height = max(72, width), max(18, height)
    presence = snapshot.get("backlog_presence", {})
    if isinstance(presence, dict) and not presence.get("ok"):
        return _backlog_problem_screen(snapshot, width, height)
    top = _top_metrics(snapshot, width)
    body_height = max(8, height - len(top) - 2)
    tab_name = DASHBOARD_TABS[state.tab]
    rows = _dashboard_rows(snapshot, state)
    state.selected = max(0, min(state.selected, max(0, len(rows) - 1)))
    selected = rows[state.selected] if rows else None
    lines = list(top)

    if width >= 150 and body_height >= 13 and tab_name not in {"Issues", "Search"}:
        right_width = max(52, min(76, width // 3))
        left_width = width - right_width - 1
        table, _ = _task_table_lines(snapshot, state, left_width - 2, body_height - 2)
        left = _box_lines("TASK TREE", table, left_width, body_height, title_color=ANSI_BLUE)
        if state.detail:
            right = _detail_box(selected, state, right_width, body_height)
        else:
            selected_height = max(6, body_height // 3)
            health_height = max(5, body_height // 3)
            workload_height = max(5, body_height - selected_height - health_height)
            while selected_height + health_height + workload_height > body_height and selected_height > 5:
                selected_height -= 1
            while selected_height + health_height + workload_height > body_height and health_height > 5:
                health_height -= 1
            while selected_height + health_height + workload_height > body_height and workload_height > 5:
                workload_height -= 1
            right = _stack_boxes(
                [
                    _box_lines("SELECTED TASK", _selected_panel_lines(selected, right_width - 4), right_width, selected_height),
                    _box_lines("BACKLOG HEALTH", _health_panel_lines(snapshot), right_width, health_height, title_color=ANSI_GREEN),
                    _workload_box(snapshot, right_width, workload_height),
                ],
                body_height,
                right_width,
            )
        lines.extend(_side_by_side(left, right, left_width, right_width, body_height))
    elif width >= 112 and body_height >= 20 and tab_name not in {"Issues", "Search"}:
        panel_height = max(8, min(11, body_height // 3))
        table_height = max(7, body_height - panel_height)
        table, _ = _task_table_lines(snapshot, state, width - 2, table_height - 2)
        lines.extend(_box_lines("TASK TREE", table, width, table_height, title_color=ANSI_BLUE))
        if state.detail:
            lines.extend(_detail_box(selected, state, width, panel_height))
        else:
            health_width = max(24, width // 5)
            workload_width = max(46, width * 2 // 5)
            selected_width = width - health_width - workload_width - 2
            selected_box = _box_lines("SELECTED TASK", _selected_panel_lines(selected, selected_width - 4), selected_width, panel_height)
            health_box = _box_lines("BACKLOG HEALTH", _health_panel_lines(snapshot), health_width, panel_height, title_color=ANSI_GREEN)
            workload_box = _workload_box(snapshot, workload_width, panel_height)
            lines.extend(
                _side_by_side(
                    selected_box,
                    _side_by_side(health_box, workload_box, health_width, workload_width, panel_height),
                    selected_width,
                    health_width + workload_width + 1,
                    panel_height,
                )
            )
    else:
        if state.detail and selected and tab_name not in {"Issues", "Search"}:
            lines.extend(_detail_box(selected, state, width, body_height))
        elif tab_name not in {"Issues", "Search"}:
            workload_lines = _workload_panel_lines(snapshot, width - 4)
            panel_height = min(len(workload_lines) + 2, max(5, body_height - 5))
            table_height = body_height - panel_height
            table, _ = _task_table_lines(snapshot, state, width - 2, table_height - 2)
            lines.extend(_box_lines("TASK TREE", table, width, table_height, title_color=ANSI_BLUE))
            lines.extend(_workload_box(snapshot, width, panel_height))
        else:
            table, _ = _task_table_lines(snapshot, state, width - 2, body_height - 2)
            lines.extend(_box_lines("TASK TREE", table, width, body_height, title_color=ANSI_BLUE))

    if state.workload:
        document = _workload_panel_lines(snapshot, width - 4, full=True)
        visible = body_height - 2
        state.workload_scroll = min(state.workload_scroll, max(0, len(document) - visible))
        start = state.workload_scroll
        lines = list(top) + _box_lines(
            f"AGENT WORKLOAD {start + 1}-{min(len(document), start + visible)}/{len(document)}",
            document[start:start + visible], width, body_height,
        )
    if state.help:
        lines = list(top)
        lines.extend(_box_lines("HELP · ? / Esc로 닫기", _help_document(width - 4), width, body_height))

    lines.append(
        _ansi(_fit_display(f"Search: {state.search_query}_", width), ANSI_BG_SELECTED, ANSI_WHITE)
        if state.search_mode
        else _tab_line(state, width)
    )
    prompt = "w/Esc 닫기 · ↑↓/j/k 스크롤 · PgUp/PgDn · Home/End · q 종료" if state.workload else DASHBOARD_DETAIL_HELP if state.detail else DASHBOARD_HELP
    if state.notice and time.monotonic() < state.notice_until:
        prompt = state.notice + " · " + prompt
    lines.append(_ansi(_fit_display(prompt, width), ANSI_GRAY))
    return "\n".join(lines[:height])


def _windows_shift_pressed() -> bool:
    if os.name != "nt":
        return False
    try:
        import ctypes
        return bool(ctypes.windll.user32.GetKeyState(0x10) & 0x8000)
    except Exception:
        return False


def _windows_special_key(code: str, shift: bool = False) -> str:
    normal = {
        "H": "\x1b[A", "P": "\x1b[B", "K": "\x1b[D", "M": "\x1b[C",
        "I": "\x1b[5~", "Q": "\x1b[6~", "G": "\x1b[H", "O": "\x1b[F",
    }
    shifted = {
        "H": "\x1b[1;2A", "P": "\x1b[1;2B", "K": "\x1b[1;2D", "M": "\x1b[1;2C",
        "I": "\x1b[5;2~", "Q": "\x1b[6;2~", "G": "\x1b[1;2H", "O": "\x1b[1;2F",
    }
    return (shifted if shift else normal).get(code, "")


def _read_dashboard_key(stream, timeout: float) -> str:
    if os.name == "nt" and msvcrt is not None:
        deadline = time.monotonic() + max(0.0, timeout)
        while time.monotonic() < deadline:
            if msvcrt.kbhit():
                value = msvcrt.getwch()
                if value in {"\x00", "\xe0"}:
                    return _windows_special_key(msvcrt.getwch(), _windows_shift_pressed())
                return value
            time.sleep(0.02)
        return ""
    fd = stream.fileno()
    if not select.select([fd], [], [], timeout)[0]:
        return ""
    first = os.read(fd, 1)
    if not first:
        return ""
    chunks = [first]
    if first == b"\x1b":
        deadline = time.monotonic() + 0.06
        while time.monotonic() < deadline and select.select([fd], [], [], 0.01)[0]:
            part = os.read(fd, 1)
            chunks.append(part)
            if part.isalpha() or part == b"~":
                break
    else:
        needed = 1 if first[0] < 0x80 else 2 if first[0] < 0xE0 else 3 if first[0] < 0xF0 else 4
        while len(chunks) < needed and select.select([fd], [], [], 0.03)[0]:
            chunks.append(os.read(fd, 1))
    return b"".join(chunks).decode("utf-8", errors="ignore")


def interactive_is_available(stream=None, out=None) -> bool:
    stream = sys.stdin if stream is None else stream
    out = sys.stdout if out is None else out
    try:
        return bool(stream.isatty() and out.isatty())
    except (AttributeError, ValueError):
        return False



def _surface_dashboard_arrivals(
    state: DashboardState, old_snapshot: dict[str, object], new_snapshot: dict[str, object]
) -> DashboardState:
    old_ids = {str(x) for x in old_snapshot.get("active_ids", [])}
    new_ids = {str(x) for x in new_snapshot.get("active_ids", [])}
    arrivals = sorted(new_ids - old_ids)
    old_bad = {str(x.get("file", "")) for x in old_snapshot.get("unrecognized_files", []) if isinstance(x, dict)}
    new_bad = {str(x.get("file", "")) for x in new_snapshot.get("unrecognized_files", []) if isinstance(x, dict)}
    bad_arrivals = sorted(x for x in (new_bad - old_bad) if x)
    if arrivals:
        tree = [r for r in new_snapshot.get("tree_rows", []) if isinstance(r, dict)]
        by_id = {str(r.get("id", "")): r for r in tree}
        for item_id in arrivals:
            row = by_id.get(item_id)
            if not row:
                continue
            for ancestor in row.get("ancestors", []):
                state.collapsed.discard(str(ancestor))
        if DASHBOARD_TABS[state.tab] == "All":
            visible = _dashboard_rows(new_snapshot, state)
            indexes = [i for i, row in enumerate(visible) if str(row.get("id", "")) in arrivals]
            if indexes:
                state.selected = indexes[-1]
                state.detail = False
        suffix = "" if DASHBOARD_TABS[state.tab] == "All" else " · All 뷰에서 확인"
        state.notice = "NEW " + ", ".join(arrivals) + " 등록 감지" + suffix
        state.notice_until = time.monotonic() + 6.0
    if bad_arrivals:
        names = ", ".join(Path(x).name for x in bad_arrivals[:3])
        state.notice = "! backlog 파일명 인식 실패: " + names + " · Issues 확인"
        state.notice_until = time.monotonic() + 10.0
    return state

def run_dashboard(args: argparse.Namespace, stream=None, out=None) -> int:
    stream = sys.stdin if stream is None else stream
    out = sys.stdout if out is None else out
    if not interactive_is_available(stream, out):
        print("monitor TUI는 실제 대화형 터미널이 필요합니다. 자동화에는 monitor --once/--json을 사용하세요.", file=sys.stderr)
        return 2
    root = _root(getattr(args, "root", None))
    state = DashboardState(recent_done_limit=max(0, int(getattr(args, "recent_done", 5))))
    snapshot = dashboard_snapshot(root, recent_done_limit=state.recent_done_limit)
    last_refresh = time.monotonic()
    dirty = True
    saved = None
    try:
        if os.name != "nt" and termios is not None and tty is not None:
            saved = termios.tcgetattr(stream.fileno())
            tty.setcbreak(stream.fileno())
        out.write("\x1b[?25l")
        while not state.stopped:
            if dirty:
                size = shutil.get_terminal_size((140, 40))
                out.write("\x1b[2J\x1b[H" + render_dashboard(snapshot, state, width=size.columns, height=size.lines))
                out.flush()
                dirty = False
            key = _read_dashboard_key(stream, min(0.2, float(args.interval)))
            old_recent = state.recent_done_limit
            state = apply_dashboard_key(key, state, snapshot)
            dirty = bool(key)
            now = time.monotonic()
            if state.force_refresh or old_recent != state.recent_done_limit or now - last_refresh >= args.interval:
                next_snapshot = dashboard_snapshot(root, recent_done_limit=state.recent_done_limit)
                state = _surface_dashboard_arrivals(state, snapshot, next_snapshot)
                snapshot = next_snapshot
                last_refresh = now
                dirty = True
    except KeyboardInterrupt:
        pass
    finally:
        if saved is not None and termios is not None:
            termios.tcsetattr(stream.fileno(), termios.TCSADRAIN, saved)
        out.write("\x1b[?25h\x1b[2J\x1b[H")
        out.flush()
    return 0


def _json(data: object) -> None:
    print(json.dumps(data, ensure_ascii=False, indent=2, default=str))


def _presence_or_error(root: Optional[Path]) -> bool:
    presence = backlog_presence(root)
    if presence["ok"]:
        return True
    print(f"BACKLOG {str(presence['status']).upper()}: {presence['message']}", file=sys.stderr)
    return False


def cmd_agent(args: argparse.Namespace) -> int:
    value = args.path.strip()
    syntax_ok = _canonical_agent(value)
    is_worker = value.startswith("/root/controller/")
    pokemon_ok = _is_pokemon_worker_path(value) if is_worker else None
    existing_ok = syntax_ok and (not is_worker or bool(pokemon_ok))
    new_ok = syntax_ok and _is_pokemon_worker_path(value, new=True)
    new_mode = bool(getattr(args, "new", False))
    policy_ok = new_ok if new_mode else existing_ok
    report = {
        "ok": policy_ok,
        "path": value,
        "kind": "root" if value == "/root" else "worker" if is_worker and syntax_ok else "subagent" if syntax_ok else "invalid",
        "path_syntax_ok": syntax_ok,
        "pokemon_worker_name": pokemon_ok,
        "existing_identity_ok": existing_ok,
        "new_worker_ok": new_ok,
        "validation_mode": "new_worker" if new_mode else "existing_identity",
        "rule": "new worker: agent <path> --new --json; existing identity/reuse: agent <path> --json",
        "legacy_note": "old identities stay readable and reusable; new creation uses only the confirmed ASCII pool, never rename an existing worker",
    }
    _json(report) if args.json else print("유효" if report["ok"] else "유효하지 않음", value,
                                        "(신규 생성)" if new_mode else "(기존 identity/재사용)")
    return 0 if report["ok"] else 1


def cmd_worker_name(args: argparse.Namespace) -> int:
    report = worker_name_report(args.root, args.used)
    if args.json:
        _json(report)
    elif report["ok"]:
        print(report["path"])
        print("신규 검증: agent " + report["path"] + " --new --json")
        if report["equivalent_conflicts"]:
            print("동치 identity 충돌: " + json.dumps(report["equivalent_conflicts"], ensure_ascii=False))
    else:
        print("사용 가능한 포켓몬 worker 이름이 없습니다.", file=sys.stderr)
    return 0 if report["ok"] else 2


def cmd_ensure_backlog(args: argparse.Namespace) -> int:
    report = ensure_backlog(args.root)
    if args.json:
        _json(report)
    else:
        print(("created: " if report["created"] else "existing: ") + str(report["path"]))
    return 0


def cmd_preflight(args: argparse.Namespace) -> int:
    backlog = backlog_presence(args.root)
    access = access_preflight(args.root, active_probe=True)
    require_full = bool(getattr(args, "require_full_access", False))
    ok = bool(backlog["ok"]) and (not require_full or bool(access["orchestration_ready"]))
    report = dict(backlog)
    report.update({
        "ok": ok,
        "backlog": backlog,
        "access": access,
        "full_access_required": require_full,
        "orchestration_ready": bool(access["orchestration_ready"]),
    })
    if args.json:
        _json(report)
    else:
        print(f"BACKLOG {str(backlog['status']).upper()}: {backlog['message']}")
        print(f"ACCESS {str(access['status']).upper()}: {access['message']}")
        if access.get("network") == "disabled":
            print("NETWORK DISABLED: 네트워크가 필요한 작업은 별도 권한 확인이 필요합니다.")
    if not backlog["ok"]:
        return 2
    if require_full and not access["orchestration_ready"]:
        return 3
    return 0


def cmd_next_id(args: argparse.Namespace) -> int:
    try:
        report = next_id_report(args.prefix, args.root, args.allow_empty)
    except (LookupError, ValueError) as exc:
        print(str(exc), file=sys.stderr)
        return 2
    _json(report) if args.json else print(f"{report['sort_key']}.{report['id']}")
    return 0


def cmd_search(args: argparse.Namespace) -> int:
    if not _presence_or_error(args.root):
        return 2
    report = search_report(args.query, args.root, args.limit)
    if args.json:
        _json(report)
    else:
        for row in report["results"]:
            print(f"{row['id']:<8} {row['state']:<5} {row['title']} score={row['score']}")
        if not report["results"]:
            print("(후보 없음)")
    return 0


def cmd_inspect(args: argparse.Namespace) -> int:
    if not _presence_or_error(args.root):
        return 2
    report = inspect_report(args.id, args.root)
    if not report["exists"]:
        print(f"{args.id} 항목을 찾지 못했다.", file=sys.stderr)
        return 2
    if args.json:
        _json(report)
    else:
        print(f"{report['id']} {report['state']} {report['title']}")
        print(f"Agent: {report['agent'] or '-'}  범위: {report['change_scope'] or '-'}")
        print("\n".join(runtime.summary_lines(report["runtime_metadata"], detail=True)))
        lifecycle = report.get("lifecycle", {})
        if isinstance(lifecycle, dict):
            print(
                "Lifecycle: "
                f"created={lifecycle.get('created_at') or '-'}  "
                f"started={lifecycle.get('started_at') or '-'}  "
                f"completed={lifecycle.get('completed_at') or '-'}  "
                f"Q/W/L={lifecycle.get('queue', '-')}/{lifecycle.get('work', '-')}/{lifecycle.get('lead', '-')}"
            )
        print(f"ready: {report['ready']}  waiting: {', '.join(report['waiting_for']) or '-'}")
        if report.get("waiting_note"):
            print(f"대기: {report['waiting_note']}")
        if report.get("hold_review"):
            review = report["hold_review"]
            print("\n".join(_hold_review_lines({"candidates" if review["review_required"] else "waiting": [review]})))
    return 1 if int(report["duplicate_count"]) > 1 else 0


def cmd_ready(args: argparse.Namespace) -> int:
    if not _presence_or_error(args.root):
        return 2
    report = ready_report(args.root)
    if args.json:
        _json(report)
    else:
        print("Ready: " + (", ".join(str(row["id"]) for row in report["ready"]) or "-"))
        print("Blocked: " + (", ".join(str(row["id"]) for row in report["blocked"]) or "-"))
        for row in report["blocked"]:
            if row.get("waiting_note"):
                print(f"  {row['id']} 대기: {row['waiting_note']}")
    problems = report["problems"]
    assert isinstance(problems, dict)
    return 1 if problems["missing"] or problems["cycles"] else 0


def cmd_workload(args: argparse.Namespace) -> int:
    if not _presence_or_error(args.root):
        return 2
    report = workload_report(args.root)
    if args.json:
        _json(report)
    else:
        for row in report["agents"]:
            ready_ids = [str(x.get("id")) for x in row.get("ready_candidates", []) if isinstance(x, dict)]
            print(
                f"{row['agent']}: doing={','.join(row.get('doing', [])) or '-'} "
                f"blocking={','.join(row.get('blocking', [])) or '-'} "
                f"ready_history={','.join(ready_ids) or '-'}"
            )
            for line in _agent_runtime_lines(row):
                print("  " + line)
        print("execution: RuntimeProvider/Dispatch상태 only; model/effort tracking removed")
        if not report["agents"]:
            print("(Agent workload 없음)")
        for line in _released_hold_lines(report):
            print(line)
    return 1 if report["unassigned_doing"] else 0


def cmd_coordinate(args: argparse.Namespace) -> int:
    if not _presence_or_error(args.root):
        return 2
    report = coordinate_report(args.root, args.worker_cap)
    if args.json:
        _json(report)
    else:
        print(f"snapshot: {report['snapshot_at']}")
        print(f"scheduling_needed: {str(report['scheduling_needed']).lower()}")
        print(f"controller_review_needed: {str(report['controller_review_needed']).lower()} (advisory)")
        print("ready: " + (", ".join(str(row["id"]) for row in report["ready"]) or "-"))
        print("doing: " + (", ".join(f"{row['id']}@{row['agent'] or '-'}" for row in report["doing"]) or "-"))
        print("execution: RuntimeProvider/Dispatch상태 only; model/effort tracking removed")
        for row in report["doing"]:
            print(row["id"] + ": " + " | ".join(runtime.summary_lines(row["runtime_metadata"])))
        print("\n".join(_hold_review_lines(report["hold_review"])))
    return 1 if report["scope_conflicts"] else 0


def cmd_audit(args: argparse.Namespace) -> int:
    findings = audit_report(args.root)
    if not findings:
        print("통과: 상태별 필수 칸이 모두 채워져 있다.")
        return 0
    _json(findings)
    return 1


def cmd_check(args: argparse.Namespace) -> int:
    protocol = args.protocol or PROTOCOL_ROOT / "collab.md"
    if not protocol.is_file():
        print(f"{protocol} 이 없다.", file=sys.stderr)
        return 2
    problems: list[str] = []
    problems.extend(f"없는 경로: {target}" for target in dangling_links(protocol))
    deps = dependency_report(catalog(protocol.parent))
    problems.extend(f"없는 선행: {row['id']} -> {row['depends_on']}" for row in deps["missing"])
    problems.extend("순환 선행: " + " -> ".join(cycle) for cycle in deps["cycles"])
    if problems:
        print("\n".join(problems), file=sys.stderr)
        return 1
    print("통과: 문서 링크와 백로그 선행관계가 유효하다.")
    return 0


def cmd_doctor(args: argparse.Namespace) -> int:
    report = doctor_report(args.root, protocol_checks=not args.backlog_only)
    if args.json:
        _json(report)
    else:
        print("통과: 기계 검사가 모두 정상이다." if report["ok"] else "문제 발견:")
        if not report["ok"]:
            for name, value in report["checks"].items():
                if value:
                    print(f"  {name}: {json.dumps(value, ensure_ascii=False)}")
        print("\n".join(_hold_review_lines({"candidates": report["warnings"]["hold_review"]})))
        for finding in report["warnings"]["runtime_metadata"]:
            print("! runtime: " + json.dumps(finding, ensure_ascii=False))
    return 0 if report["ok"] else 1


def cmd_status(args: argparse.Namespace) -> int:
    if args.watch:
        if not args.json:
            return run_web_ui(args.root, 8765, open_browser=True)
        return _monitor(args.root, args.done, args.interval, args.json)
    if not _presence_or_error(args.root):
        return 2
    report = status_report(args.root, args.done)
    _json(report) if args.json else print(render_status(report))
    return 0


def _monitor(
    root: Optional[Path],
    include_done: bool,
    interval: float,
    as_json: bool = False,
    once: bool = False,
) -> int:
    interval = max(0.5, float(interval))
    try:
        while True:
            report = status_report(root, include_done)
            if as_json:
                print(json.dumps(report, ensure_ascii=False, default=str), flush=True)
            else:
                clear = "\x1b[2J\x1b[H" if sys.stdout.isatty() else ""
                sys.stdout.write(clear + render_status(report) + "\n")
                sys.stdout.flush()
            if once:
                return 0
            time.sleep(interval)
    except KeyboardInterrupt:
        return 0



def _find_web_port(host: str, preferred: int) -> int:
    for port in range(preferred, preferred + 30):
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
            try:
                sock.bind((host, port))
            except OSError:
                continue
            return port
    raise OSError(f"사용 가능한 local Web UI port를 찾지 못했다: {preferred}-{preferred + 29}")


def _web_backlog_context(root: Optional[Path]) -> dict[str, object]:
    base = _root(root)
    candidates = discover_backlog_folders(base)
    explicit: Optional[Path] = None
    if root is not None and _looks_like_backlog_folder(base):
        explicit = base.resolve()
    selected = explicit or (Path(str(candidates[0]["path"])) if candidates else CANONICAL_BACKLOG_ROOT)
    scan_root = _backlog_scan_root(base)
    return {
        "scan_root": scan_root,
        "selected": selected,
        "explicit": explicit is not None,
        "candidates": candidates,
    }


def _web_handler(root: Optional[Path]):
    web_root = PROTOCOL_ROOT / "web"
    context = _web_backlog_context(root)

    def resolve_backlog(query: str) -> tuple[Optional[Path], list[dict[str, object]]]:
        candidates = discover_backlog_folders(Path(str(context["scan_root"])))
        by_path = {str(Path(str(row["path"])).resolve()): row for row in candidates}
        requested = parse_qs(query).get("backlog", [""])[0]
        if requested:
            try:
                wanted = str(Path(requested).resolve())
            except OSError:
                wanted = ""
            if wanted in by_path:
                return Path(wanted), candidates
        initial = context.get("selected")
        if isinstance(initial, Path):
            if str(initial.resolve()) in by_path or not candidates:
                return initial.resolve(), candidates
        if candidates:
            return Path(str(candidates[0]["path"])), candidates
        return CANONICAL_BACKLOG_ROOT, candidates

    def with_selection(snapshot: dict[str, object], selected: Optional[Path], candidates: list[dict[str, object]]) -> dict[str, object]:
        clean_candidates = []
        for row in candidates:
            clean = dict(row)
            clean.pop("latest_modified_epoch", None)
            clean["selected"] = bool(selected and Path(str(row["path"])).resolve() == selected.resolve())
            clean_candidates.append(clean)
        snapshot["backlog_selection"] = {
            "scan_root": str(context["scan_root"]),
            "selected": str(selected) if selected else None,
            "auto_selected": not bool(context.get("explicit")),
            "candidates": clean_candidates,
        }
        return snapshot

    class Handler(BaseHTTPRequestHandler):
        server_version = "TaskMecca/2.10"

        def log_message(self, fmt: str, *args: object) -> None:
            if os.getenv("TASK_MECCA_WEB_LOG") == "1":
                super().log_message(fmt, *args)

        def _send_json(self, payload: object, status: int = 200) -> None:
            body = json.dumps(payload, ensure_ascii=False, default=str).encode("utf-8")
            self.send_response(status)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.send_header("Cache-Control", "no-store")
            self.send_header("X-Content-Type-Options", "nosniff")
            self.end_headers()
            self.wfile.write(body)

        def _send_file(self, path: Path) -> None:
            if not path.is_file():
                self.send_error(HTTPStatus.NOT_FOUND)
                return
            body = path.read_bytes()
            kind = mimetypes.guess_type(path.name)[0] or "application/octet-stream"
            self.send_response(200)
            self.send_header("Content-Type", f"{kind}; charset=utf-8" if kind.startswith(("text/", "application/javascript")) else kind)
            self.send_header("Content-Length", str(len(body)))
            self.send_header("Cache-Control", "no-cache")
            self.send_header("X-Content-Type-Options", "nosniff")
            self.send_header("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' https://cdnjs.cloudflare.com https://cdn.jsdelivr.net; img-src 'self' data:; connect-src 'self' https://cdnjs.cloudflare.com https://cdn.jsdelivr.net")
            self.end_headers()
            self.wfile.write(body)

        def do_GET(self) -> None:  # noqa: N802
            parsed = urlparse(self.path)
            path = unquote(parsed.path)
            selected, candidates = resolve_backlog(parsed.query)
            if path == "/api/backlog-folders":
                payload: dict[str, object] = {}
                self._send_json(with_selection(payload, selected, candidates).get("backlog_selection", {}))
                return
            if path == "/api/snapshot":
                try:
                    self._send_json(with_selection(dashboard_snapshot(selected), selected, candidates))
                except Exception as exc:  # Web UI must surface diagnostics rather than die silently.
                    self._send_json({"error": str(exc)}, 500)
                return
            if path == "/api/manual":
                params = parse_qs(parsed.query)
                lang = (params.get("lang", ["ko"])[0] or "ko").lower()
                suffix = f".{lang}.md" if lang != "ko" else ".md"
                readme_path = PROTOCOL_ROOT / (f"README{suffix}" if lang != "ko" else "README.md")
                session_path = PROTOCOL_ROOT / (f"SESSION_GUIDE{suffix}" if lang != "ko" else "SESSION_GUIDE.md")
                if not readme_path.is_file():
                    readme_path = PROTOCOL_ROOT / "README.md"
                if not session_path.is_file():
                    session_path = PROTOCOL_ROOT / "SESSION_GUIDE.md"
                readme = readme_path.read_text(encoding="utf-8") if readme_path.is_file() else ""
                session_guide = session_path.read_text(encoding="utf-8") if session_path.is_file() else ""
                self._send_json({"readme": readme, "session_guide": session_guide, "language": lang})
                return
            if path.startswith("/api/tasks/"):
                if selected is None:
                    self._send_json({"error": "backlog folder not found"}, 404)
                    return
                item_id = path.rsplit("/", 1)[-1].upper()
                snap = dashboard_snapshot(selected)
                item = snap.get("all_items", {}).get(item_id) if isinstance(snap.get("all_items"), dict) else None
                if item is None:
                    self._send_json({"error": "task not found", "id": item_id}, 404)
                else:
                    self._send_json(item)
                return
            if path in {"/style.css", "/app.js"}:
                self._send_file(web_root / path.lstrip("/"))
                return
            if path.startswith("/vendor/"):
                target = (web_root / path.lstrip("/")).resolve()
                vendor_root = (web_root / "vendor").resolve()
                if vendor_root == target or vendor_root not in target.parents:
                    self.send_error(HTTPStatus.NOT_FOUND)
                    return
                self._send_file(target)
                return
            # SPA fallback keeps /tasks/<ID> bookmarkable.
            self._send_file(web_root / "index.html")

    return Handler


def run_web_ui(root: Optional[Path], port: int = 8765, *, open_browser: bool = True) -> int:
    context = _web_backlog_context(root)
    selected = context.get("selected")
    # Opening the read-only dashboard must not itself become an access gate.
    # Before the first registration there may be no backlog yet; the UI can still
    # start and report the uninitialized state without creating project data.
    # Show the last observation passively; Root/Controller performs a fresh
    # active probe immediately before each subagent dispatch.
    access = access_preflight(selected if isinstance(selected, Path) else TASK_MECCA_ROOT, active_probe=False)
    host = "127.0.0.1"
    chosen = _find_web_port(host, max(1, int(port)))
    server = ThreadingHTTPServer((host, chosen), _web_handler(root))
    url = f"http://{host}:{chosen}/"
    print(f"Task Mecca Web UI: {url}")
    print(
        f"backlog: {selected} {'(auto)' if not context.get('explicit') else '(explicit)'}"
        if isinstance(selected, Path)
        else "backlog: not initialized (Registrar creates data/backlog on first registration)"
    )
    print("read-only · localhost only · Ctrl+C to stop")
    if access.get("restriction_current"):
        print("WARNING: current runtime restriction detected · subagent dispatch will remain blocked until a fresh preflight succeeds")
    elif access.get("checked_at"):
        print(f"access: last observed {str(access.get('status', 'unknown')).upper()} · fresh active preflight runs automatically before dispatch")
    else:
        print("access: not yet observed · fresh active preflight runs automatically before dispatch")
    if open_browser:
        threading.Timer(0.2, lambda: webbrowser.open(url)).start()
    try:
        server.serve_forever(poll_interval=0.5)
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
    return 0

def cmd_web(args: argparse.Namespace) -> int:
    return run_web_ui(args.root, args.port, open_browser=not args.no_open)

def cmd_monitor(args: argparse.Namespace) -> int:
    # monitor is retained as a compatibility alias. Human interactive use now opens
    # the local read-only Web UI; --json/--once keep automation-friendly snapshots.
    if not args.once and not args.json:
        return run_web_ui(args.root, getattr(args, "port", 8765), open_browser=not getattr(args, "no_open", False))
    if not _presence_or_error(args.root):
        return 2
    return _monitor(args.root, args.done, args.interval, args.json, args.once)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description=__doc__.splitlines()[0],
        epilog=(
            "Web UI: python _task_mecca/framework/collab_tools.py web\n"
            "uv project: uv run _task_mecca/framework/collab_tools.py web"
        ),
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    sub = parser.add_subparsers(dest="command")

    command = sub.add_parser("agent", help="canonical logical task path 검증")
    command.add_argument("path")
    command.add_argument("--json", action="store_true")
    command.add_argument("--new", action="store_true", help="신규 worker 생성 이름만 검사; 생략 시 기존 identity/재사용 호환")
    command.set_defaults(func=cmd_agent)

    command = sub.add_parser("worker-name", help="새 worker용 포켓몬 별칭 선택")
    command.add_argument("--root", type=Path)
    command.add_argument("--used", action="append", default=[], help="현재 live worker path 또는 alias; 반복 지정 가능")
    command.add_argument("--json", action="store_true")
    command.set_defaults(func=cmd_worker_name)

    command = sub.add_parser("ensure-backlog", help="기존 원장을 선택하거나 첫 등록용 data/backlog 생성")
    command.add_argument("--root", type=Path)
    command.add_argument("--json", action="store_true")
    command.set_defaults(func=cmd_ensure_backlog)

    command = sub.add_parser("preflight", help="backlog + effective Full Access/dispatch 준비 상태 확인")
    command.add_argument("root", nargs="?", type=Path)
    command.add_argument("--json", action="store_true")
    command.add_argument(
        "--require-full-access",
        action="store_true",
        help="Full Access가 effective probe로 확인되지 않으면 exit 3; subagent dispatch gate용",
    )
    command.set_defaults(func=cmd_preflight)

    command = sub.add_parser("next-id", help="archive 포함 다음 ID 계산")
    command.add_argument("prefix")
    command.add_argument("--root", type=Path)
    command.add_argument("--allow-empty", action="store_true")
    command.add_argument("--json", action="store_true")
    command.set_defaults(func=cmd_next_id)

    command = sub.add_parser("search", help="관련 backlog 후보 검색")
    command.add_argument("query")
    command.add_argument("--root", type=Path)
    command.add_argument("--limit", type=int, default=10)
    command.add_argument("--json", action="store_true")
    command.set_defaults(func=cmd_search)

    command = sub.add_parser("inspect", help="한 항목의 상태와 맥락 확인")
    command.add_argument("id")
    command.add_argument("--root", type=Path)
    command.add_argument("--json", action="store_true")
    command.set_defaults(func=cmd_inspect)

    for name, help_text, func in (
        ("ready", "착수 가능한 todo 계산", cmd_ready),
        ("workload", "Agent별 workload와 배정 해제 감사", cmd_workload),
    ):
        command = sub.add_parser(name, help=help_text)
        command.add_argument("root", nargs="?", type=Path)
        command.add_argument("--json", action="store_true")
        command.set_defaults(func=func)

    command = sub.add_parser("coordinate", help="controller용 fresh Git backlog scheduling snapshot")
    command.add_argument("root", nargs="?", type=Path)
    command.add_argument("--worker-cap", type=int, default=DEFAULT_IMPLEMENTATION_WORKER_CAP, help="동시 구현 worker 목표 상한 (기본 3)")
    command.add_argument("--json", action="store_true")
    command.set_defaults(func=cmd_coordinate)

    command = sub.add_parser("audit", help="상태별 필수 필드 검사")
    command.add_argument("root", nargs="?", type=Path)
    command.set_defaults(func=cmd_audit)

    command = sub.add_parser("check", help="문서 링크와 선행관계 검사")
    command.add_argument("protocol", nargs="?", type=Path)
    command.set_defaults(func=cmd_check)

    command = sub.add_parser("doctor", help="백로그 협업 불변조건 전체 검사")
    command.add_argument("root", nargs="?", type=Path)
    command.add_argument("--backlog-only", action="store_true")
    command.add_argument("--json", action="store_true")
    command.set_defaults(func=cmd_doctor)

    command = sub.add_parser("status", help="현재 hot set 현황")
    command.add_argument("root", nargs="?", type=Path)
    command.add_argument("--done", action="store_true")
    command.add_argument("--watch", action="store_true")
    command.add_argument("--interval", type=float, default=1.0)
    command.add_argument("--json", action="store_true")
    command.set_defaults(func=cmd_status)

    command = sub.add_parser("web", help="사람용 읽기 전용 local Web UI")
    command.add_argument("root", nargs="?", type=Path)
    command.add_argument("--port", type=int, default=8765)
    command.add_argument("--no-open", action="store_true", help="브라우저 자동 열기 비활성화")
    command.set_defaults(func=cmd_web)

    command = sub.add_parser("monitor", help="Web UI 호환 alias / JSON snapshot")
    command.add_argument("root", nargs="?", type=Path)
    command.add_argument("--done", action="store_true")
    command.add_argument("--interval", type=float, default=1.0)
    command.add_argument("--json", action="store_true", help="반복 snapshot을 NDJSON으로 출력")
    command.add_argument("--once", action="store_true", help="snapshot 하나를 출력하고 종료")
    command.add_argument("--port", type=int, default=8765, help="Web UI port (기본 8765)")
    command.add_argument("--no-open", action="store_true", help="Web UI 브라우저 자동 열기 비활성화")
    command.add_argument(
        "--recent-done",
        type=int,
        default=5,
        help="All 뷰에 표시할 최근 완료 개수(기본 5, TUI에서 [/]로 조절)",
    )
    command.set_defaults(func=cmd_monitor)
    return parser


def subcommands() -> tuple[str, ...]:
    for action in build_parser()._actions:
        if isinstance(action, argparse._SubParsersAction):
            return tuple(action.choices)
    return ()


def main(argv: Optional[list[str]] = None) -> int:
    _configure_stdio()
    raw_args = sys.argv[1:] if argv is None else argv
    if not raw_args:
        return run_web_ui(None, 8765, open_browser=True)
    parser = build_parser()
    args = parser.parse_args(raw_args)
    if not getattr(args, "command", None):
        parser.print_help()
        return 0
    return int(args.func(args))


if __name__ == "__main__":
    raise SystemExit(main())
