package demo

import "time"

const Pause = 3 * time.Second

type Step struct {
	Narration         string
	Command           string
	ChangeBranch      bool
	CreateManifests   bool
	WaitForHealthy    bool
	WaitForBuildError bool
	WaitForPodRestart bool
}

var steps = []Step{
	Step{
		Narration: "\t🚀  Launching demo... ",
	},
	Step{
		Narration: "\t📚  git clone https://github.com/windmilleng/tiltdemo",
		Command:   "git clone https://github.com/windmilleng/tiltdemo $(pwd)",
	},
	Step{
		Narration:    "\t🌲  Changing branch",
		ChangeBranch: true,
	},
	Step{
		Narration:       "\t🎂  Building and deploying demo server",
		CreateManifests: true,
		WaitForHealthy:  true,
	},
	Step{
		Narration: "\t🎉  Deployment success!",
	},
	Step{
		Narration:         "\t⚒️  Introducing build error on demoserver1",
		Command:           "sed -i -e 's!// \\(.*// tilt:BUILD_ERROR\\)!\\1!' cmd/demoserver1/main.go",
		WaitForBuildError: true,
	},
	Step{
		Narration: "\t😱  Oh no!",
	},
	Step{
		Narration:      "\t🚑  Fixing build error",
		Command:        "sed -i -e 's!^\\(.*// tilt:BUILD_ERROR\\)!// \\1!' cmd/demoserver1/main.go",
		WaitForHealthy: true,
	},
	Step{
		Narration: "\t😌  Whew! Back to normal...",
	},
	Step{
		Narration:         "\t⚒️  Introducing panic on demoserver1",
		Command:           "sed -i -e 's!// \\(.*// tilt:STARTUP_PANIC\\)!\\1!' cmd/demoserver1/main.go",
		WaitForPodRestart: true,
	},
	Step{
		Narration: "\t😱  Oh no!",
	},
	Step{
		Narration:      "\t🚒  Fixing panic",
		Command:        "sed -i -e 's!^\\(.*// tilt:STARTUP_PANIC\\)!// \\1!' cmd/demoserver1/main.go",
		WaitForHealthy: true,
	},
	Step{
		Narration: "\t😌  Whew! Back to normal...",
	},
	Step{
		Narration: "\t😎  Demo Complete!",
	},
}
