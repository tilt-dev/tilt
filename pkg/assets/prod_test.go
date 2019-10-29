package assets

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/pkg/model"
)

const (
	testUrl        = "https://fake.tilt.dev"
	versionDefault = model.WebVersion("v1.2.3")
	version666     = model.WebVersion("v6.6.6")
)

func TestIndexRequest(t *testing.T) {
	var recvReq *http.Request
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		recvReq = req
		res.Write([]byte(indexHTML))
	}))
	defer testServer.Close()

	req := httptest.NewRequest("GET", "/", bytes.NewBuffer(nil))
	res := httptest.NewRecorder()
	server, err := NewProdServer(AssetBucket(testServer.URL), versionDefault)
	assert.NoError(t, err)

	server.ServeHTTP(res, req)
	if assert.NotNil(t, recvReq) {
		assert.Equal(t, recvReq.URL.Path, "/v1.2.3/index.html")
	}
	assert.Contains(t, res.Body.String(), `<script src="/v1.2.3/static/js/2.f1bd84e9.chunk.js">`)
}

func TestChunkRequest(t *testing.T) {
	var recvReq *http.Request
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		recvReq = req
		res.Write([]byte(indexHTML))
	}))
	defer testServer.Close()

	req := httptest.NewRequest("GET", "/v1.2.3/static/js/2.f1bd84e9.chunk.js", bytes.NewBuffer(nil))
	res := httptest.NewRecorder()
	server, err := NewProdServer(AssetBucket(testServer.URL), versionDefault)
	assert.NoError(t, err)

	server.ServeHTTP(res, req)
	if assert.NotNil(t, recvReq) {
		assert.Equal(t, recvReq.URL.Path, "/v1.2.3/static/js/2.f1bd84e9.chunk.js")
	}
}

func TestBuildUrlForReq(t *testing.T) {
	s := prodServerForTest(t)
	expected := "https://fake.tilt.dev/v1.2.3/index.html"
	req := reqForTest(t, "/", "")
	u, v := s.urlAndVersionForReq(req)
	assert.Equal(t, expected, u.String())
	assert.Equal(t, string(versionDefault), v)
}

func TestBuildUrlForReqRedirectsToIndex(t *testing.T) {
	s := prodServerForTest(t)
	expected := "https://fake.tilt.dev/v1.2.3/index.html"
	req := reqForTest(t, "/some/random/path", "")
	u, v := s.urlAndVersionForReq(req)
	assert.Equal(t, expected, u.String())
	assert.Equal(t, string(versionDefault), v)
}

func TestBuildUrlForReqRespectsStatic(t *testing.T) {
	s := prodServerForTest(t)
	expected := "https://fake.tilt.dev/v1.2.3/static/stuff.html"
	req := reqForTest(t, "/static/stuff.html", "")
	u, v := s.urlAndVersionForReq(req)
	assert.Equal(t, expected, u.String())
	assert.Equal(t, string(versionDefault), v)
}

func TestBuildUrlForReqRespectsVersion(t *testing.T) {
	s := prodServerForTest(t)
	expected := "https://fake.tilt.dev/v111.222.333/stuff.html"
	req := reqForTest(t, "/v111.222.333/stuff.html", "")
	u, v := s.urlAndVersionForReq(req)
	assert.Equal(t, expected, u.String())
	assert.Equal(t, "v111.222.333", v)
}

func TestBuildUrlForReqWithVersionParam(t *testing.T) {
	s := prodServerForTest(t)
	expected := "https://fake.tilt.dev/v6.6.6/index.html"
	req := reqForTest(t, "/", version666)
	u, v := s.urlAndVersionForReq(req)
	assert.Equal(t, expected, u.String())
	assert.Equal(t, string(version666), v)
}

func TestBuildUrlForReqWithVersionParamAndStaticPath(t *testing.T) {
	s := prodServerForTest(t)
	expected := "https://fake.tilt.dev/v6.6.6/static/stuff.html"
	req := reqForTest(t, "/static/stuff.html", version666)
	u, v := s.urlAndVersionForReq(req)
	assert.Equal(t, expected, u.String())
	assert.Equal(t, string(version666), v)
}

func TestBuildUrlForReqWithVersionParamAndVersionPrefix(t *testing.T) {
	s := prodServerForTest(t)
	expected := "https://fake.tilt.dev/v111.222.333/stuff.html"
	req := reqForTest(t, "/v111.222.333/stuff.html", version666)
	u, v := s.urlAndVersionForReq(req)
	assert.Equal(t, expected, u.String())
	assert.Equal(t, "v111.222.333", v)
}

func TestSHARootUrlForReq(t *testing.T) {
	// get a new version here
	sha := "8bf2ea29eacff3a407272eb5631edbd1a14a0936"
	s, err := NewProdServer(testUrl, model.WebVersion(sha))
	if err != nil {
		t.Fatal(err)
	}
	expected := fmt.Sprintf("https://fake.tilt.dev/%s/index.html", sha)
	req := reqForTest(t, "/", "")
	u, v := s.urlAndVersionForReq(req)
	assert.Equal(t, expected, u.String())
	assert.Equal(t, sha, v)
}

func TestSHAStaticUrlForReq(t *testing.T) {
	// get a new version here
	sha := "8bf2ea29eacff3a407272eb5631edbd1a14a0936"
	s, err := NewProdServer(testUrl, model.WebVersion(sha))
	if err != nil {
		t.Fatal(err)
	}
	expected := fmt.Sprintf("https://fake.tilt.dev/%s/static/stuff.html", sha)
	req := reqForTest(t, fmt.Sprintf("/%s/static/stuff.html", sha), "")
	u, v := s.urlAndVersionForReq(req)
	assert.Equal(t, expected, u.String())
	assert.Equal(t, sha, v)
}

func prodServerForTest(t *testing.T) prodServer {
	s, err := NewProdServer(testUrl, versionDefault)
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
		q.Set(WebVersionKey, string(version))
		u.RawQuery = q.Encode()
	}

	return &http.Request{URL: u}
}

// Copied from
// view-source:https://storage.googleapis.com/tilt-static-assets/v0.10.14/index.html
const indexHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><link rel="shortcut icon" href="https://tilt.build/favicon.ico"><link href="https://fonts.googleapis.com/css?family=Inconsolata:400,700|Montserrat:600" rel="stylesheet"><meta name="viewport" content="width=device-width,initial-scale=1,shrink-to-fit=no"><meta name="theme-color" content="#000000"><title>Tilt</title><link href="/static/css/main.a164f855.chunk.css" rel="stylesheet"></head><body><noscript>You need to enable JavaScript to run this app.</noscript><div id="root"></div><script>!function(f){function e(e){for(var t,r,n=e[0],o=e[1],u=e[2],i=0,l=[];i<n.length;i++)r=n[i],Object.prototype.hasOwnProperty.call(p,r)&&p[r]&&l.push(p[r][0]),p[r]=0;for(t in o)Object.prototype.hasOwnProperty.call(o,t)&&(f[t]=o[t]);for(s&&s(e);l.length;)l.shift()();return c.push.apply(c,u||[]),a()}function a(){for(var e,t=0;t<c.length;t++){for(var r=c[t],n=!0,o=1;o<r.length;o++){var u=r[o];0!==p[u]&&(n=!1)}n&&(c.splice(t--,1),e=i(i.s=r[0]))}return e}var r={},p={1:0},c=[];function i(e){if(r[e])return r[e].exports;var t=r[e]={i:e,l:!1,exports:{}};return f[e].call(t.exports,t,t.exports,i),t.l=!0,t.exports}i.m=f,i.c=r,i.d=function(e,t,r){i.o(e,t)||Object.defineProperty(e,t,{enumerable:!0,get:r})},i.r=function(e){"undefined"!=typeof Symbol&&Symbol.toStringTag&&Object.defineProperty(e,Symbol.toStringTag,{value:"Module"}),Object.defineProperty(e,"__esModule",{value:!0})},i.t=function(t,e){if(1&e&&(t=i(t)),8&e)return t;if(4&e&&"object"==typeof t&&t&&t.__esModule)return t;var r=Object.create(null);if(i.r(r),Object.defineProperty(r,"default",{enumerable:!0,value:t}),2&e&&"string"!=typeof t)for(var n in t)i.d(r,n,function(e){return t[e]}.bind(null,n));return r},i.n=function(e){var t=e&&e.__esModule?function(){return e.default}:function(){return e};return i.d(t,"a",t),t},i.o=function(e,t){return Object.prototype.hasOwnProperty.call(e,t)},i.p="/";var t=this["webpackJsonptilt-ui"]=this["webpackJsonptilt-ui"]||[],n=t.push.bind(t);t.push=e,t=t.slice();for(var o=0;o<t.length;o++)e(t[o]);var s=n;a()}([])</script><script src="/static/js/2.f1bd84e9.chunk.js"></script><script src="/static/js/main.99897104.chunk.js"></script></body></html>`
