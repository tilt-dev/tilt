package httptest

import (
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
)

type FakeClient struct {
	Requests []http.Request
	Response http.Response
	Err      error

	mu sync.Mutex
}

func (fc *FakeClient) Do(req *http.Request) (*http.Response, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.Requests = append(fc.Requests, *req)
	r := fc.Response

	return &r, fc.Err
}

func (fc *FakeClient) SetResponse(s string) {
	fc.Response = http.Response{
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(strings.NewReader(s)),
	}
}

func (fc *FakeClient) RequestURLs() []string {
	var ret []string
	for _, req := range fc.Requests {
		ret = append(ret, req.URL.String())
	}
	return ret
}

func NewFakeClient() *FakeClient {
	return &FakeClient{
		Response: http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       ioutil.NopCloser(strings.NewReader("FakeClient response uninitialized")),
		},
	}
}
