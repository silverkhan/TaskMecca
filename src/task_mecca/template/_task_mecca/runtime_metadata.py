"""Task Mecca execution metadata compatibility layer.

Schema v2 intentionally drops model/effort capture.  These helpers keep older
collab_tools call-sites and legacy backlog readable while exposing only the
small set of execution fields that can be observed reliably.
"""
from __future__ import annotations

SCHEMA_VERSION = "2"
FIELDS = ("RuntimeProvider", "Dispatch상태", "실행상태", "실행근거", "Fallback근거")


def from_fields(fields):
    provider = str(fields.get("RuntimeProvider", "") or "").strip() or "unknown"
    status = str(fields.get("Dispatch상태", "") or fields.get("실행상태", "") or "").strip() or "unknown"
    evidence = str(fields.get("실행근거", "") or "").strip() or "unknown"
    fallback = str(fields.get("Fallback근거", "") or "").strip()
    coverage = "v2" if any(str(fields.get(name, "") or "").strip() for name in FIELDS) else "legacy"
    return {
        "runtime_schema": SCHEMA_VERSION if coverage == "v2" else "legacy",
        "coverage": coverage,
        "runtime_provider": provider,
        "dispatch_status": status,
        "execution_evidence": evidence,
        "fallback_evidence": fallback,
        "missing_fields": [],
        "findings": [],
    }


def aggregate(tasks):
    values = list(tasks.values()) if isinstance(tasks, dict) else list(tasks or [])
    providers = sorted({str(v.get("runtime_provider", "unknown")) for v in values if v})
    statuses = sorted({str(v.get("dispatch_status", "unknown")) for v in values if v})
    return {"providers": providers, "statuses": statuses, "conflict": False}


def model_mix(_agents):
    # Kept as a legacy JSON key so existing integrations do not crash.
    return {"deprecated": True, "message": "model/effort tracking removed in schema v2"}


def summary_lines(metadata):
    return [
        f"RuntimeProvider={metadata.get('runtime_provider', 'unknown')}",
        f"Dispatch={metadata.get('dispatch_status', 'unknown')}",
    ]


def mix_lines(_mix):
    return ["runtime: model/effort tracking removed (schema v2)"]
