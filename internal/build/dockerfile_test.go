package build

import "testing"

func TestForbidEntrypoint(t *testing.T) {
	df := Dockerfile(`ENTRYPOINT cat`)
	err := df.ValidateBaseDockerfile()
	if err != ErrEntrypointInDockerfile {
		t.Errorf("Expected error %s, actual: %v", ErrEntrypointInDockerfile, err)
	}
}

func TestForbidAdd(t *testing.T) {
	df := Dockerfile(`RUN echo 'hi'
ADD . /go/src`)
	err := df.ValidateBaseDockerfile()
	if err != ErrAddInDockerfile {
		t.Errorf("Expected error %s, actual: %v", ErrAddInDockerfile, err)
	}
}

func TestForbidAddWithSpaces(t *testing.T) {
	df := Dockerfile(`RUN echo 'hi'
 add . /go/src`)
	err := df.ValidateBaseDockerfile()
	if err != ErrAddInDockerfile {
		t.Errorf("Expected error %s, actual: %v", ErrAddInDockerfile, err)
	}
}

func TestPermitAddInCmd(t *testing.T) {
	df := Dockerfile(`RUN echo ADD`)
	err := df.ValidateBaseDockerfile()
	if err != nil {
		t.Fatal(err)
	}
}

func TestPermitAddInCmd2(t *testing.T) {
	df := Dockerfile(`RUN echo \
ADD`)
	err := df.ValidateBaseDockerfile()
	if err != nil {
		t.Fatal(err)
	}
}
