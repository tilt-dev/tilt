package main

import (
	"time"

	"github.com/windmilleng/tilt/internal/hud/view"
)

func sampleView() view.View {
	return view.View{
		Resources: []view.Resource{
			view.Resource{
				"fe",
				"fe",
				[]string{"fe/main.go"},
				time.Second,
				view.ResourceStatusFresh,
				"1/1 pods up",
			},
			view.Resource{
				"be",
				"be",
				[]string{"/"},
				time.Second,
				view.ResourceStatusFresh,
				"1/1 pods up",
			},
			view.Resource{
				"graphql",
				"graphql",
				[]string{},
				time.Second,
				view.ResourceStatusFresh,
				"1/1 pods up",
			},
			view.Resource{
				"snacks",
				"snacks",
				[]string{"snacks/whoops.go"},
				time.Second,
				view.ResourceStatusFresh,
				"1/1 pods up",
			},
			view.Resource{
				"doggos",
				"doggos",
				[]string{"fe/main.go"},
				time.Second,
				view.ResourceStatusFresh,
				"1/1 pods up",
			},
			view.Resource{
				"elephants",
				"elephants",
				[]string{"fe/main.go"},
				time.Second,
				view.ResourceStatusFresh,
				"1/1 pods up",
			},
			view.Resource{
				"heffalumps",
				"heffalumps",
				[]string{"fe/main.go"},
				time.Second,
				view.ResourceStatusFresh,
				"1/1 pods up",
			},
			view.Resource{
				"aardvarks",
				"aardvarks",
				[]string{"fe/main.go"},
				time.Second,
				view.ResourceStatusFresh,
				"1/1 pods up",
			},
			view.Resource{
				"quarks",
				"quarks",
				[]string{"fe/main.go"},
				time.Second,
				view.ResourceStatusFresh,
				"1/1 pods up",
			},
			view.Resource{
				"boop",
				"boop",
				[]string{"fe/main.go"},
				time.Second,
				view.ResourceStatusFresh,
				"1/1 pods up",
			},
			view.Resource{
				"laurel",
				"laurel",
				[]string{"fe/main.go"},
				time.Second,
				view.ResourceStatusFresh,
				"1/1 pods up",
			},
			view.Resource{
				"hardy",
				"hardy",
				[]string{"fe/main.go"},
				time.Second,
				view.ResourceStatusFresh,
				"1/1 pods up",
			},
			view.Resource{
				"north",
				"north",
				[]string{"fe/main.go"},
				time.Second,
				view.ResourceStatusFresh,
				"1/1 pods up",
			},
			view.Resource{
				"west",
				"west",
				[]string{"fe/main.go"},
				time.Second,
				view.ResourceStatusFresh,
				"1/1 pods up",
			},
			view.Resource{
				"east",
				"east",
				[]string{"fe/main.go"},
				time.Second,
				view.ResourceStatusFresh,
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
