# Contributing

Thanks for contributing to Task Mecca.

## Principles

- Keep the installed runtime project-contained and Git-friendly.
- Do not move project-owned backlog data into a central database.
- Preserve backward compatibility for legacy backlog formats when practical.
- Keep the Web UI localhost-only and read-only unless a future design explicitly changes that boundary.
- Prefer standard-library Python for the installed runtime.

## Development setup

```bash
python -m pip install -e .
python -m unittest discover -s tests -v
```

## Changing managed template files

Files under `src/task_mecca/template/_task_mecca/` become the runtime installed into user projects. The updater records baseline hashes for these files. When adding a new file, decide whether it is framework-managed, customizable-managed, or project-owned and update `src/task_mecca/installer.py` when necessary.

## Pull requests

Please include:

- the problem being solved,
- compatibility impact,
- tests for parser/lifecycle/update behavior when relevant,
- user-facing documentation updates for CLI/UI changes.
