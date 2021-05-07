// `tilt-restart-wrapper` wraps `entr` (http://eradman.com/entrproject/) to easily
// rerun a user-specified command when a given file changes.
//
// This is Tilt's recommended way of restarting a process as part of a live_update:
// if your container invokes your app via the restart wrapper (e.g. `tilt-restart-wrapper /bin/my-app`),
// you can trigger re-execution of your app with a live_update `run` step that makes
// a trivial change to the file watched by `entr` (e.g. `run('date > /.restart-proc')`)
//
// This script exists (i.e. we're wrapping `entr` in a binary instead of invoking
// it directly) because in its canonical invocation, `entr` requires that the
// file(s) to watch be piped via stdin, i.e. it is invoked like:
//     echo "/.restart-proc" | entr -rz /bin/my-app
//
// When specified as a `command` in Kubernetes or Docker Compose YAML (this is how
// Tilt overrides entrypoints), the above would therefore need to be executed as shell:
//     /bin/sh -c 'echo "/.restart-proc" | entr -rz /bin/my-app'

// Any args specified in Kubernetes or Docker Compose YAML are attached to the end
// of this call, and therefore in this case apply TO THE `/bin/sh -c` CALL, rather
// than to the actual command run by `entr`; that is, any `args` specified by the
// user would be effectively ignored.
//
// In order to make `entr` executable as ARGV rather than as shell, we have wrapped it
// in a binary that can be called directly and takes care of the piping under the hood.
//
// Note: ideally `entr` could accept files-to-watch via flag instead of stdin,
// but (for a number of good reasons) this feature isn't likely to be added any
// time soon (see https://github.com/eradman/entr/issues/33).

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

var watchFile = flag.String("watch_file", "/.restart-proc", "File that entr will watch for changes; changes to this file trigger `entr` to rerun the command(s) passed")
var entrPath = flag.String("entr_path", "/entr", "Path to `entr` executable")

func main() {
	flag.Parse()

	cmd := exec.Command(*entrPath, "-rz")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("%s\n", *watchFile))
	cmd.Args = append(cmd.Args, flag.Args()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				if len(flag.Args()) == 0 {
					log.Println("`tilt-restart-wrapper` requires at least one positional arg " +
						"(a command or set of args to be  executed / rerun whenever `watch_file` changes)")
				}
				os.Exit(status.ExitStatus())
			}
		} else {
			log.Fatalf("error running command: %v", err)
		}
	}

	if len(flag.Args()) == 0 {
		log.Fatal("`tilt-restart-wrapper` requires at least one positional arg " +
			"(will be passed to `entr` and executed / rerun whenever `watch_file` changes)")
	}
}
