package tiltfile

import (
	"fmt"
	"github.com/google/skylark"
	)

type Tiltfile struct {
	globals  skylark.StringDict
	filename string
	thread   *skylark.Thread
}

func Load(filename string) (*Tiltfile, error) {
	thread := &skylark.Thread{
		Print: func(_ *skylark.Thread, msg string) { fmt.Println(msg) },
	}

	predeclared := skylark.StringDict{}

	globals, err := skylark.ExecFile(thread, filename, nil, predeclared)
	if err != nil {
		return nil, err
	}

	return &Tiltfile{globals, filename, thread}, nil
}
