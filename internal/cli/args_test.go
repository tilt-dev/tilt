package cli

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArgsClear(t *testing.T) {
	f := newArgsFixture()
	f.cmd.clear = true
	err := f.cmd.run(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, "null\n", f.fakeHttpPoster.lastRequestBody)
}

func TestArgsNewValue(t *testing.T) {
	f := newArgsFixture()
	err := f.cmd.run(context.Background(), []string{"--foo", "bar"})
	require.NoError(t, err)
	require.Equal(t, "[\"--foo\",\"bar\"]\n", f.fakeHttpPoster.lastRequestBody)
}

func TestArgsClearAndNewValue(t *testing.T) {
	f := newArgsFixture()
	f.cmd.clear = true
	err := f.cmd.run(context.Background(), []string{"--foo", "bar"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "--clear cannot be specified with other values")
}

func TestArgsEmptyNewValueNoClear(t *testing.T) {
	f := newArgsFixture()
	err := f.cmd.run(context.Background(), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no args specified.")
	require.Contains(t, err.Error(), "run `tilt args --clear`")
}

type fakeHttpPoster struct {
	lastRequestBody string
}

var _ httpPoster = (&fakeHttpPoster{}).Post

func (fp *fakeHttpPoster) Post(url string, contentType string, body io.Reader) (*http.Response, error) {
	b, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	fp.lastRequestBody = string(b)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(strings.NewReader("fake http response")),
	}, nil
}

type argsFixture struct {
	cmd            argsCmd
	fakeHttpPoster *fakeHttpPoster
}

func newArgsFixture() *argsFixture {
	fp := &fakeHttpPoster{}
	return &argsFixture{cmd: argsCmd{post: fp.Post}, fakeHttpPoster: fp}
}
