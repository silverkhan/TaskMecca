# A-24: backlog-local search and folder selection

Search and backlog-folder selection now share the backlog list control surface with sort, page size and pagination. They are absent from Global Hub, workload, attention, issues, notification settings, manuals, release history, terminal and task detail. Empty lists still expose folder selection so users can recover without leaving the list.

The existing query/API, status and tag filters, sort and page-size storage, automatic row calculation, folder auto/manual choice, project context and history routes are unchanged. Each list render rebinds its new controls. Search text comes from application state; focused search retains its selection through refresh. Delayed search refresh runs only while the backlog list is active. `/` focuses the existing list search only outside editable controls; it does not switch other views or dereference missing inputs.

The existing visual language and tokens are retained. Search and folder controls are 44px high with visible keyboard focus; narrow screens stack them in reading order and truncate long selected folder names. Go and Python index/app/style assets are byte-identical, with parity regression covering all three files.

Verification uses `node tests/backlog_list_controls.test.cjs`, the supported Python full suite, Go full tests/vet, JavaScript syntax and diff checks. Opt-in `TestSeedA24ListControlsUIFixture` only seeds explicitly named `/private/tmp/a24-raichyu-*` synthetic fixtures; it does not connect to operational projects or create fake runtime events.

Operational installation, restart, canonical lifecycle and cleanup remain Controller responsibilities. A-23 runtime-observation history and the process-only Telegram transport gate are unchanged.
