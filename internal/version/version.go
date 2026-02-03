package version

import (
	"fmt"
	"runtime"
)

var (
	Version    string // will be populated by linker during build
	BuildDate  string // will be populated by linker during build
	Commit     string // will be populated by linker during build
	DocsCommit string // git submodule commit hash for embedded prometheus/docs, populated by commit hash in embedded file
)

// Print outputs human readable build about the binary to stdout
// Models return on: github.com/prometheus/common/version.Print().
func Print(programName string) string {
	return fmt.Sprintf("%s build info:\n\tversion: %s\n\tbuild date: %s\n\tcommit: %s\n\tdocs commit: %s\n\tgo version: %s\n",
		programName,
		Version,
		BuildDate,
		Commit,
		DocsCommit,
		runtime.Version(),
	)
}

// Info print build info in a more condensed, single line format.
// Models return on: github.com/prometheus/common/version.Info().
func Info() string {
	return fmt.Sprintf("(version=%s, build_date=%s, commit=%s, docs_commit=%s)", Version, BuildDate, Commit, DocsCommit)
}
