package goassets

import "embed"

// Template is the project framework bundled into the standalone CLI.
//go:embed template/_task_mecca template/_task_mecca/.gitignore template/_task_mecca/framework/_template.md
var Template embed.FS
