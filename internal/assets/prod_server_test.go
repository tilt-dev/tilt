package assets

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
)

const (
	testUrl     = "https://fake.tilt.dev"
	testVersion = model.WebVersion("v1.2.3")
)

func TestBuildUrlForReq(t *testing.T) {
	s := prodServerForTest(t)
	expected := "https://fake.tilt.dev/v1.2.3/index.html"
	req := reqForTest(t, "/", "")
	actual := s.buildUrlForReq(req)
	assert.Equal(t, expected, actual.String())
}

func TestBuildUrlForReqRedirectsToIndex(t *testing.T) {
	s := prodServerForTest(t)
	expected := "https://fake.tilt.dev/v1.2.3/index.html"
	req := reqForTest(t, "/some/random/path", "")
	actual := s.buildUrlForReq(req)
	assert.Equal(t, expected, actual.String())
}

func TestBuildUrlForReqRespectsStatic(t *testing.T) {
	s := prodServerForTest(t)
	expected := "https://fake.tilt.dev/v1.2.3/static/stuff.html"
	req := reqForTest(t, "/static/stuff.html", "")
	actual := s.buildUrlForReq(req)
	assert.Equal(t, expected, actual.String())
}

func TestBuildUrlForReqWithVersionParam(t *testing.T) {}

func TestBuildUrlForReqWithVersionParamAndStaticPath(t *testing.T) {}

func prodServerForTest(t *testing.T) prodServer {
	s, err := newProdServer(testUrl, testVersion)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func reqForTest(t *testing.T, path string, version model.WebVersion) *http.Request {
	u, err := url.Parse(path)
	if err != nil {
		t.Fatal(err)
	}

	if version != "" {
		q := u.Query()
		q.Set(webVersionKey, string(version))
		u.RawQuery = q.Encode()
	}

	return &http.Request{URL: u}
}
