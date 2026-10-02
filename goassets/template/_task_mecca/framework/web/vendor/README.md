# Optional local browser runtimes

Task Mecca can use local standalone browser bundles before trying a CDN.

## Mermaid

Local path:

`_task_mecca/web/vendor/mermaid.min.js`

If this file is absent, the Web UI tries the classic standalone Mermaid bundle from cdnjs and then jsDelivr. Task Mecca does **not** use dynamic ES-module `import()` for Mermaid because that path can fail in Safari, restrictive proxies, or corporate browser environments.

## Web Terminal / xterm.js

Optional local paths:

`_task_mecca/web/vendor/xterm.js`

`_task_mecca/web/vendor/xterm.css`

When both are available, Web Terminal uses the local xterm.js runtime. Otherwise it tries cdnjs and jsDelivr. If no xterm.js runtime can be loaded, Terminal stays usable through its built-in basic command-input fallback; full ANSI rendering and interactive terminal-key behavior require xterm.js.
