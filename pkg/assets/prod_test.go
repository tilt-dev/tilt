package assets

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
	f := newProdServerFixture(t)
	defer f.TearDown()

	req := httptest.NewRequest("GET", "/", bytes.NewBuffer(nil))
	res := httptest.NewRecorder()
	f.server.ServeHTTP(res, req)
	if assert.NotNil(t, f.recvReq) {
		assert.Equal(t, f.recvReq.URL.Path, "/v1.2.3/index.html")
	}
	assert.Contains(t, res.Body.String(), `<script src="/v1.2.3/static/js/2.f1bd84e9.chunk.js">`)
}

func TestChunkRequest(t *testing.T) {
	f := newProdServerFixture(t)
	defer f.TearDown()

	req := httptest.NewRequest("GET", "/v1.2.3/static/js/2.f1bd84e9.chunk.js", bytes.NewBuffer(nil))
	res := httptest.NewRecorder()

	f.server.ServeHTTP(res, req)
	if assert.NotNil(t, f.recvReq) {
		assert.Equal(t, f.recvReq.URL.Path, "/v1.2.3/static/js/2.f1bd84e9.chunk.js")
	}
}

func TestBuildUrlForReqRedirectsToIndex(t *testing.T) {
	f := newProdServerFixture(t)
	defer f.TearDown()

	req := httptest.NewRequest("GET", "/some/random/path", bytes.NewBuffer(nil))
	res := httptest.NewRecorder()
	f.server.ServeHTTP(res, req)
	if assert.NotNil(t, f.recvReq) {
		assert.Equal(t, f.recvReq.URL.Path, "/v1.2.3/index.html")
	}
	assert.Contains(t, res.Body.String(), `<script src="/v1.2.3/static/js/2.f1bd84e9.chunk.js">`)
}

func TestBuildUrlForReqRespectsStatic(t *testing.T) {
	f := newProdServerFixture(t)
	defer f.TearDown()

	req := httptest.NewRequest("GET", "/v1.2.3/static/stuff.html", bytes.NewBuffer(nil))
	res := httptest.NewRecorder()
	f.server.ServeHTTP(res, req)
	if assert.NotNil(t, f.recvReq) {
		assert.Equal(t, f.recvReq.URL.Path, "/v1.2.3/static/stuff.html")
	}
	assert.Contains(t, res.Body.String(), `some-content`)
}

func TestBuildUrlForReqRespectsVersion(t *testing.T) {
	f := newProdServerFixture(t)
	defer f.TearDown()

	req := httptest.NewRequest("GET", "/v111.222.333/stuff.html", bytes.NewBuffer(nil))
	res := httptest.NewRecorder()
	f.server.ServeHTTP(res, req)
	if assert.NotNil(t, f.recvReq) {
		assert.Equal(t, f.recvReq.URL.Path, "/v111.222.333/stuff.html")
	}
	assert.Contains(t, res.Body.String(), `some-content`)
}

func TestBuildUrlForReqWithVersionParam(t *testing.T) {
	f := newProdServerFixture(t)
	defer f.TearDown()

	req := httptest.NewRequest("GET", "/", bytes.NewBuffer(nil))
	attachQueryVersion(req, string(version666))

	res := httptest.NewRecorder()
	f.server.ServeHTTP(res, req)
	if assert.NotNil(t, f.recvReq) {
		assert.Equal(t, f.recvReq.URL.Path, "/v6.6.6/index.html")
	}
	assert.Contains(t, res.Body.String(), `<script src="/v6.6.6/static/js/2.f1bd84e9.chunk.js">`)
}

func TestBuildUrlForReqWithVersionParamAndStaticPath(t *testing.T) {
	f := newProdServerFixture(t)
	defer f.TearDown()

	req := httptest.NewRequest("GET", "/static/stuff.html", bytes.NewBuffer(nil))
	attachQueryVersion(req, string(version666))

	res := httptest.NewRecorder()
	f.server.ServeHTTP(res, req)
	if assert.NotNil(t, f.recvReq) {
		assert.Equal(t, f.recvReq.URL.Path, "/v6.6.6/static/stuff.html")
	}
	assert.Contains(t, res.Body.String(), `some-content`)
}

func TestBuildUrlForReqWithVersionParamAndVersionPrefix(t *testing.T) {
	f := newProdServerFixture(t)
	defer f.TearDown()

	req := httptest.NewRequest("GET", "/v111.222.333/stuff.html", bytes.NewBuffer(nil))
	attachQueryVersion(req, string(version666))

	res := httptest.NewRecorder()
	f.server.ServeHTTP(res, req)
	if assert.NotNil(t, f.recvReq) {
		assert.Equal(t, f.recvReq.URL.Path, "/v111.222.333/stuff.html")
	}
	assert.Contains(t, res.Body.String(), `some-content`)
}

func TestSHARootUrlForReq(t *testing.T) {
	f := newProdServerFixture(t)
	defer f.TearDown()

	sha := "8bf2ea29eacff3a407272eb5631edbd1a14a0936"
	f.SetupServerWithVersion(model.WebVersion(sha))
	req := httptest.NewRequest("GET", "/", bytes.NewBuffer(nil))
	res := httptest.NewRecorder()
	f.server.ServeHTTP(res, req)
	if assert.NotNil(t, f.recvReq) {
		assert.Equal(t, f.recvReq.URL.Path, fmt.Sprintf("/%s/index.html", sha))
	}
	assert.Contains(t, res.Body.String(), fmt.Sprintf(`<script src="/%s/static/js/2.f1bd84e9.chunk.js">`, sha))
}

func TestSHAStaticUrlForReq(t *testing.T) {
	f := newProdServerFixture(t)
	defer f.TearDown()

	sha := "8bf2ea29eacff3a407272eb5631edbd1a14a0936"
	f.SetupServerWithVersion(model.WebVersion(sha))
	req := httptest.NewRequest("GET", fmt.Sprintf("/%s/static/stuff.html", sha), bytes.NewBuffer(nil))
	res := httptest.NewRecorder()
	f.server.ServeHTTP(res, req)
	if assert.NotNil(t, f.recvReq) {
		assert.Equal(t, f.recvReq.URL.Path, fmt.Sprintf("/%s/static/stuff.html", sha))
	}
	assert.Contains(t, res.Body.String(), `some-content`)
}

func TestStripPrefixIndexRequest(t *testing.T) {
	f := newProdServerFixture(t)
	defer f.TearDown()

	req := httptest.NewRequest("GET", "/tilt-assets", bytes.NewBuffer(nil))
	res := httptest.NewRecorder()
	handler := StripPrefix("/tilt-assets", f.server)
	handler.ServeHTTP(res, req)
	if assert.NotNil(t, f.recvReq) {
		assert.Equal(t, f.recvReq.URL.Path, "/v1.2.3/index.html")
	}
	assert.Contains(t, res.Body.String(), `<script src="/tilt-assets/v1.2.3/static/js/2.f1bd84e9.chunk.js">`)
}

type fixture struct {
	t          *testing.T
	testServer *httptest.Server
	server     prodServer
	recvReq    *http.Request
}

func newProdServerFixture(t *testing.T) *fixture {
	f := &fixture{t: t}

	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		f.recvReq = req
		if strings.HasSuffix(req.URL.Path, "index.html") {
			_, err := res.Write([]byte(indexHTML))
			if err != nil {
				fmt.Println(err)
			}
		} else {
			_, err := res.Write([]byte("some-content"))
			if err != nil {
				fmt.Println(err)
			}
		}
	}))
	f.testServer = testServer
	f.SetupServerWithVersion(versionDefault)
	return f
}

func (f *fixture) SetupServerWithVersion(v model.WebVersion) {
	server, err := NewProdServer(AssetBucket(f.testServer.URL), v)
	assert.NoError(f.t, err)
	f.server = server
}

func (f *fixture) TearDown() {
	f.testServer.Close()
}

func attachQueryVersion(req *http.Request, v string) {
	q := req.URL.Query()
	q.Set(WebVersionKey, v)
	req.URL.RawQuery = q.Encode()
}

// Copied from
// view-source:https://storage.googleapis.com/tilt-static-assets/v0.10.14/index.html
const indexHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><link rel="shortcut icon" href="https://tilt.build/favicon.ico"><link href="https://fonts.googleapis.com/css?family=Inconsolata:400,700|Montserrat:600" rel="stylesheet"><meta name="viewport" content="width=device-width,initial-scale=1,shrink-to-fit=no"><meta name="theme-color" content="#000000"><title>Tilt</title><link href="/static/css/main.a164f855.chunk.css" rel="stylesheet"></head><body><noscript>You need to enable JavaScript to run this app.</noscript><div id="root"></div><script>!function(f){function e(e){for(var t,r,n=e[0],o=e[1],u=e[2],i=0,l=[];i<n.length;i++)r=n[i],Object.prototype.hasOwnProperty.call(p,r)&&p[r]&&l.push(p[r][0]),p[r]=0;for(t in o)Object.prototype.hasOwnProperty.call(o,t)&&(f[t]=o[t]);for(s&&s(e);l.length;)l.shift()();return c.push.apply(c,u||[]),a()}function a(){for(var e,t=0;t<c.length;t++){for(var r=c[t],n=!0,o=1;o<r.length;o++){var u=r[o];0!==p[u]&&(n=!1)}n&&(c.splice(t--,1),e=i(i.s=r[0]))}return e}var r={},p={1:0},c=[];function i(e){if(r[e])return r[e].exports;var t=r[e]={i:e,l:!1,exports:{}};return f[e].call(t.exports,t,t.exports,i),t.l=!0,t.exports}i.m=f,i.c=r,i.d=function(e,t,r){i.o(e,t)||Object.defineProperty(e,t,{enumerable:!0,get:r})},i.r=function(e){"undefined"!=typeof Symbol&&Symbol.toStringTag&&Object.defineProperty(e,Symbol.toStringTag,{value:"Module"}),Object.defineProperty(e,"__esModule",{value:!0})},i.t=function(t,e){if(1&e&&(t=i(t)),8&e)return t;if(4&e&&"object"==typeof t&&t&&t.__esModule)return t;var r=Object.create(null);if(i.r(r),Object.defineProperty(r,"default",{enumerable:!0,value:t}),2&e&&"string"!=typeof t)for(var n in t)i.d(r,n,function(e){return t[e]}.bind(null,n));return r},i.n=function(e){var t=e&&e.__esModule?function(){return e.default}:function(){return e};return i.d(t,"a",t),t},i.o=function(e,t){return Object.prototype.hasOwnProperty.call(e,t)},i.p="/";var t=this["webpackJsonptilt-ui"]=this["webpackJsonptilt-ui"]||[],n=t.push.bind(t);t.push=e,t=t.slice();for(var o=0;o<t.length;o++)e(t[o]);var s=n;a()}([])</script><script src="/static/js/2.f1bd84e9.chunk.js"></script><script src="/static/js/main.99897104.chunk.js"></script></body></html>`
