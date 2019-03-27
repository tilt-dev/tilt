package model

import (
	"strings"
)

func TrySquash(runs []Cmd) []Cmd {
	newRuns := make([]Cmd, 0)
	for i := 0; i < len(runs); i++ {
		toSquash := []Cmd{}
		for j := i; j < len(runs); j++ {
			runJ := runs[j]
			if !runJ.IsShellStandardForm() {
				break
			}

			toSquash = append(toSquash, runJ)
		}

		if len(toSquash) < 2 {
			newRuns = append(newRuns, runs[i])
			continue
		}

		newRuns = append(newRuns, squashHelper(toSquash))
		i += len(toSquash) - 1
	}
	return newRuns
}

// Create a new shell script that combines the individual runs.
// We know all the scripts are in shell standard form.
func squashHelper(runs []Cmd) Cmd {
	scripts := make([]string, len(runs))
	for i, c := range runs {
		scripts[i] = c.ShellStandardScript()
	}

	return Cmd{
		// This could potentially break things (because it converts normal shell
		// scripts to scripts run with -ex). We're not too worried about it right
		// now.  In the future, we might need to do manual exit code checks for
		// correctness.
		Argv: []string{
			"sh",
			"-exc",
			strings.Join(scripts, ";\n"),
		},
	}
}
