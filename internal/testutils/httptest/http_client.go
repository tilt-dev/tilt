package httptest

import (
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
)

type FakeClient struct {
	requests     []http.Request
	responseCode int
	responseBody string
	Err          error

	mu sync.Mutex
}

func (fc *FakeClient) Do(req *http.Request) (*http.Response, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.requests = append(fc.requests, *req)
	r := http.Response{
		StatusCode: fc.responseCode,
		Body:       ioutil.NopCloser(strings.NewReader(fc.responseBody)),
	}

	return &r, fc.Err
}

func (fc *FakeClient) SetResponse(s string) {
	fc.responseCode = http.StatusOK
	fc.responseBody = s
}

func (fc *FakeClient) ClearRequests() {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.requests = nil
}

func (fc *FakeClient) Requests() []http.Request {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	ret := append([]http.Request{}, fc.requests...)
	return ret
}

func NewFakeClient() *FakeClient {
	return &FakeClient{
		responseCode: http.StatusInternalServerError,
		responseBody: "FakeClient response uninitialized",
	}
}

func NewFakeClientEmptyJSON() *FakeClient {
	return &FakeClient{
		responseCode: http.StatusOK,
		responseBody: "{}",
	}
}
