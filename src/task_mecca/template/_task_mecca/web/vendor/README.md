# Optional local Mermaid runtime

Task Mecca loads a standalone Mermaid browser bundle from:

`_task_mecca/web/vendor/mermaid.min.js`

If this file is absent, the Web UI loads the classic standalone Mermaid bundle from cdnjs.
Task Mecca does **not** use dynamic ES-module `import()` for Mermaid, because that path can fail in Safari, restrictive proxies, or corporate browser environments.

To make Mermaid fully offline, place a compatible standalone `mermaid.min.js` at the path above.
