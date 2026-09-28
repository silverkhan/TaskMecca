# Security

Task Mecca runs local project automation and may coordinate subagents with broad filesystem/tool access.

## Security boundaries

- The dashboard binds to localhost and is read-only by design.
- Full Access preflight is intentionally checked before subagent dispatch; do not treat an old dashboard observation as a current authorization signal.
- `_task_mecca/.runtime/` may contain ephemeral operational state and is excluded from Git by default.
- `_task_mecca/backups/` may contain previous customized role/manual files and is excluded from Git by default.

## Reporting a vulnerability

Please open a GitHub security advisory/private vulnerability report when available. Avoid posting secrets, credentials, private repository content, or exploit details in a public issue.
