package visitor

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

// A simplified version of cli-runtime/pkg/resource Visitor
// for objects that don't query the cluster.
type Interface interface {
	Name() string
	Open() (io.ReadCloser, error)
}

func Stdin(stdin io.Reader) stdinVisitor {
	return stdinVisitor{reader: stdin}
}

type noOpCloseReader struct {
	io.Reader
}

func (noOpCloseReader) Close() error { return nil }

type stdinVisitor struct {
	reader io.Reader
}

func (v stdinVisitor) Name() string {
	return "stdin"
}

func (v stdinVisitor) Open() (io.ReadCloser, error) {
	return noOpCloseReader{Reader: v.reader}, nil
}

var _ Interface = stdinVisitor{}

func File(path string) fileVisitor {
	return fileVisitor{path: path}
}

type fileVisitor struct {
	path string
}

func (v fileVisitor) Name() string {
	return v.path
}

func (v fileVisitor) Open() (io.ReadCloser, error) {
	return os.Open(v.path)
}

var _ Interface = urlVisitor{}

type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

func URL(client HTTPClient, url string) urlVisitor {
	return urlVisitor{client: client, url: url}
}

type urlVisitor struct {
	client HTTPClient
	url    string
}

func (v urlVisitor) Name() string {
	return v.url
}

func (v urlVisitor) Open() (io.ReadCloser, error) {
	resp, err := v.client.Get(v.url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch(%q) failed with status code %d", v.url, resp.StatusCode)
	}
	return resp.Body, nil
}

var _ Interface = urlVisitor{}
