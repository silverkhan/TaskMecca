# A-25: Removed-project runtime suppression

Hub pause/removal state is a durable background-write boundary. Updated Task Mecca processes consult `projects.json` before project-local runtime, notification settings/ledger, observation history, preflight cache, tag cache, and operation-journal writes. A missing project is denied before any project-local directory or lock can be created.

## Cross-process protocol

`internal/projectguard` owns the independent guard. Its shared/exclusive lock lives in the configured Task Mecca management home, not inside the project. Writers check permission before acquiring a nonblocking shared lease and again under that lease. Hub mutations take an exclusive lease and wait for in-flight writes to finish. New writers fail closed while a mutation is active. Nested producer leases do not wait behind a pending mutation, preventing monitor → delivery → ledger lock inversion. Network delivery is fenced as well as local persistence.

`TASK_MECCA_HOME` selects the registry and lock together. There is no alternate-home fallback on errors. Unreadable or malformed registry state fails closed; an absent registry still permits ordinary existing unmanaged projects. Different independently configured homes are distinct management namespaces; this is not an authorization mechanism for obsolete or adversarial processes.

Removal stores both lexical and resolved canonical paused boundaries. Requests are compared using resolved aliases and lexical boundaries. Persisted boundaries are not re-resolved through a newly substituted source/ancestor symlink: removal must not expand to an unrelated repository. macOS fixed root aliases are normalized, and same-file identity covers case-insensitive volume aliases without merging distinct case-sensitive directories. The identity fallback rejects symlinked historical boundaries. Legacy custom-alias records lacking a persisted canonical boundary cannot safely be guessed or expanded to a current target; review those records explicitly.

Deleting a history record does not silently resume monitoring. Explicit registration or monitoring resume is required and removes equivalent canonical paused entries. A Web open/restart does not re-register removed projects, including a removal racing its initial check.

## Historical outcome and current observation

`moved_to_trash` describes the native move that completed, not a permanent claim that the original path remains absent. History refresh projects current `presence` independently. If a successful historical move is followed by a present source, `source_state=present_after_trash` warns that it may have been recreated or restored. No automatic second delete, rollback into a new source, or reclassification of historical success is performed.

The UI displays the confirmed project subtree, containing holder (explicitly outside the deletion target), staging project path, and restore-holder boundary separately. A successful Trash action remains disabled for retry even if a source later exists. Refresh is read-only and does not inventory the user's Trash.

## Restore-holder lifecycle

Native recycle stages the confirmed project in a unique same-volume sibling `.task-mecca-recycle-*` directory. The successful empty holder is intentionally retained for OS Put Back/Restore; OS restoration can return the project to that staging path. Move it manually to the original confirmed path only after checking that destination; never overwrite a newly present source.

Failure/rollback cleanup can remove only the same owned, empty, nonsymlink staging directory via nonrecursive `os.Remove`. It never recursively removes a holder or its contents. Failed cleanup reports the retained path and error. A holder with files, a replaced inode, an invalid boundary, or an unavailable inspection remains preserved. Successful holders are not treated as orphan trash to purge.

## Compatibility boundary

These guards protect cooperating updated processes. Already-running obsolete binaries do not implement the protocol and can recreate runtime directories after a native move. Stop narrowly identified obsolete writers before deployment; do not infer permission to remove recreated files, a containing worktree holder, or restore staging. Real-source validation is a separate read-only operational responsibility, not a fixture result.

macOS service tests must also isolate the fixed LaunchAgent restart command explicitly. `TASK_MECCA_HOME` alone isolates files, not `/bin/launchctl kickstart -k gui/UID/com.taskmecca.web`. Production rendering retains that command; rollback execution fixtures inject a non-destructive local stub and assert its arguments/call count. Production-renderer checks remain string-only and must not execute the operational script.
