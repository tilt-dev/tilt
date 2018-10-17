package demo

import "time"

const Pause = 3 * time.Second

type Step struct {
	Narration       string
	Command         string
	CreateManifests bool
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
		Narration:       "\t🎂  Building and deploying demo server",
		CreateManifests: true,
	},
	Step{
		Narration: "\t🎉  Deployment success!",
	},
}
