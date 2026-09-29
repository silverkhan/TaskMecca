package goassets

import "embed"

// Template is the project framework bundled into the standalone CLI.
//go:embed template/_task_mecca template/_task_mecca/.gitignore
var Template embed.FS
