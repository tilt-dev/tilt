package httptest

import (
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
)

type FakeClient struct {
	requests []http.Request
	Response http.Response
	Err      error

	mu sync.Mutex
}

func (fc *FakeClient) Do(req *http.Request) (*http.Response, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.requests = append(fc.requests, *req)
	r := fc.Response

	return &r, fc.Err
}

func (fc *FakeClient) SetResponse(s string) {
	fc.Response = http.Response{
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(strings.NewReader(s)),
	}
}

func (fc *FakeClient) Requests() []http.Request {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	var ret []http.Request
	for _, req := range fc.requests {
		ret = append(ret, req)
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
