package main

import (
	"time"
)

type Resource struct {
	Name                    string
	DirectoryWatched        string
	LatestFileChanges       []string
	TimeSinceLastFileChange time.Duration
	Status                  ResourceStatus

	// e.g., "CrashLoopBackOff", "No Pod found", "Build error"
	StatusDesc string
}

type ResourceStatus int

const (
	// something is wrong and requires investigation, e.g. the build failed or the pod is crashlooping
	ResourceStatusBroken ResourceStatus = iota
	// tilt has observed changes since the last deploy, and is in the process of rebuilding and deploying
	ResourceStatusStale
	// the latest code is currently running
	ResourceStatusFresh
)

type View struct {
	Resources []Resource
}

func sampleView() View {
	return View{
		Resources: []Resource{
			Resource{
				"fe",
				"fe",
				[]string{"fe/main.go"},
				time.Second,
				ResourceStatusFresh,
				"1/1 pods up",
			},
			Resource{
				"be",
				"be",
				[]string{"/"},
				time.Second,
				ResourceStatusFresh,
				"1/1 pods up",
			},
			Resource{
				"graphql",
				"graphql",
				[]string{},
				time.Second,
				ResourceStatusFresh,
				"1/1 pods up",
			},
			Resource{
				"snacks",
				"snacks",
				[]string{"snacks/whoops.go"},
				time.Second,
				ResourceStatusFresh,
				"1/1 pods up",
			},
			Resource{
				"doggos",
				"doggos",
				[]string{"fe/main.go"},
				time.Second,
				ResourceStatusFresh,
				"1/1 pods up",
			},
			Resource{
				"elephants",
				"elephants",
				[]string{"fe/main.go"},
				time.Second,
				ResourceStatusFresh,
				"1/1 pods up",
			},
			Resource{
				"heffalumps",
				"heffalumps",
				[]string{"fe/main.go"},
				time.Second,
				ResourceStatusFresh,
				"1/1 pods up",
			},
			Resource{
				"aardvarks",
				"aardvarks",
				[]string{"fe/main.go"},
				time.Second,
				ResourceStatusFresh,
				"1/1 pods up",
			},
			Resource{
				"quarks",
				"quarks",
				[]string{"fe/main.go"},
				time.Second,
				ResourceStatusFresh,
				"1/1 pods up",
			},
			Resource{
				"boop",
				"boop",
				[]string{"fe/main.go"},
				time.Second,
				ResourceStatusFresh,
				"1/1 pods up",
			},
			Resource{
				"laurel",
				"laurel",
				[]string{"fe/main.go"},
				time.Second,
				ResourceStatusFresh,
				"1/1 pods up",
			},
			Resource{
				"hardy",
				"hardy",
				[]string{"fe/main.go"},
				time.Second,
				ResourceStatusFresh,
				"1/1 pods up",
			},
			Resource{
				"north",
				"north",
				[]string{"fe/main.go"},
				time.Second,
				ResourceStatusFresh,
				"1/1 pods up",
			},
			Resource{
				"west",
				"west",
				[]string{"fe/main.go"},
				time.Second,
				ResourceStatusFresh,
				"1/1 pods up",
			},
			Resource{
				"east",
				"east",
				[]string{"fe/main.go"},
				time.Second,
				ResourceStatusFresh,
				"1/1 pods up",
			},
		},
	}
}

const longText string = `Hello

Here is text

f

lots of it
It has looooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooong lines
And many of them
So many lines
a
a
a
a
a
fasdf
f
f
f
f
ff
f
f
f
f
f
f
ff
f
f
f
g
g
g
g
g
gasdfgasg
g
g
g
g
g
g
g
g
gasg
g
g
g
g
g
g
g
g
g
g
g
gasgsd
g
g
g
g
g
g
g
g
g
gqwer
g
g
g
g
g
g
g
g
g
g
g
g
g
g
g
g
gfff
g
g
g
g
g
g
g
g
g
g
g
g
g
g
g
g
g
g
g
g
g
g
gasf
g
g
g
g
g
g
g
g
g
g
g
g
g
g
g
g
g
g
g
g
g
asdf`
