package dockercompose

// Three hacky states just for now to get something into the hud.
const (
	StateDown   = "down"
	StateInProg = "in progress"
	StateUp     = "up"
)

type Info struct {
	State string
}

func (i Info) Log() string {
	return `What rolls down stairs
Alone or in pairs,
And over your neighbor's dog?
What's great for a snack,
And fits on your back?
It's log, log, log!

It's log, it's log,
It's big, it's heavy, it's wood.
It's log, it's log, it's better than bad, it's good!

Everyone wants a log
You're gonna love it, log
Come on and get your log
Everyone needs a log
Log log log!`
}
