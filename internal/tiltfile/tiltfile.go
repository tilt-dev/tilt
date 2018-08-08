package tiltfile

import (
	"fmt"
	"github.com/google/skylark"
	"log"
)

type Tiltfile struct {
	globals  skylark.StringDict
	filename string
	thread   *skylark.Thread
}

func Load(filename string) Tiltfile {
	thread := &skylark.Thread{
		Print: func(_ *skylark.Thread, msg string) { fmt.Println(msg) },
	}

	predeclared := skylark.StringDict{}

	globals, err := skylark.ExecFile(thread, filename, nil, predeclared)
	if err != nil {
		if evalErr, ok := err.(*skylark.EvalError); ok {
			log.Fatal(evalErr.Backtrace())
		}
		log.Fatal(err)
	}

	return Tiltfile{globals, filename, thread}
}
