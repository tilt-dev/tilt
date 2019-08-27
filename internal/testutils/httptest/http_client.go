package httptest

import (
	"net/http"
	"sync"
)

type FakeClient struct {
	requests []http.Request
	response http.Response
	err      error

	mu sync.Mutex
}

func (fc *FakeClient) Do(req *http.Request) (*http.Response, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.requests = append(fc.requests, *req)
	r := fc.response

	return &r, fc.err
}

func NewFakeClient() *FakeClient {
	return &FakeClient{}
}
