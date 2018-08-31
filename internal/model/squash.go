package model

import (
	"strings"
)

func TrySquash(steps []Cmd) []Cmd {
	newSteps := make([]Cmd, 0)
	for i := 0; i < len(steps); i++ {
		toSquash := []Cmd{}
		for j := i; j < len(steps); j++ {
			stepJ := steps[j]
			if !stepJ.IsShellStandardForm() {
				break
			}

			toSquash = append(toSquash, stepJ)
		}

		if len(toSquash) < 2 {
			newSteps = append(newSteps, steps[i])
			continue
		}

		newSteps = append(newSteps, squashHelper(toSquash))
		i += len(toSquash) - 1
	}
	return newSteps
}

// Create a new shell script that combines the individual steps.
// We know all the scripts are in shell standard form.
func squashHelper(steps []Cmd) Cmd {
	scripts := make([]string, len(steps))
	for i, c := range steps {
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
