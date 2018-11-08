package dockerfile

import "testing"

func TestAllowEntrypoint(t *testing.T) {
	df := Dockerfile(`ENTRYPOINT cat`)
	err := df.ValidateBaseDockerfile()
	if err != nil {
		t.Errorf("Unexpected error %s", err)
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
