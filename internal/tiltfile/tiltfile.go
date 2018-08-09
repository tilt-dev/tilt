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

func (tiltfile Tiltfile) GetServiceConfig(serviceName string) (string, error) {
	f, ok := tiltfile.globals[serviceName]

	if !ok {
		return "", fmt.Errorf("%v does not define a global named '%v'", tiltfile.filename, serviceName)
	}

	serviceFunction, ok := f.(*skylark.Function)

	if !ok {
		return "", fmt.Errorf("'%v' is a '%v', not a function. service definitions must be functions.", serviceName, f.Type())
	}

	if serviceFunction.NumParams() != 0 {
		return "", fmt.Errorf("'%v' is defined to take more than 0 arguments. service definitions must take 0 arguments.", serviceName)
	}

	val, err := serviceFunction.Call(tiltfile.thread, nil, nil)
	if err != nil {
		return "", fmt.Errorf("error running '%v': %v", serviceName, err.Error())
	}

	yaml, ok := skylark.AsString(val)

	if !ok {
		return "", fmt.Errorf("service definition function '%v' returned a %v. A string was expected.", serviceName, val.Type())
	}

	return yaml, nil
}
