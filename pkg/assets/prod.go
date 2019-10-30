package assets

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

// A URL where assets live.
type AssetBucket string

func (b AssetBucket) String() string { return string(b) }

const ProdAssetBucket = AssetBucket("https://storage.googleapis.com/tilt-static-assets/")
const WebVersionKey = "web_version"

var versionRe = regexp.MustCompile(`^/(v\d+\.\d+\.\d+)/.*`) // matches `/vA.B.C/...`
var shaRe = regexp.MustCompile(`^/([0-9a-f]{5,40})\/.*`)    // matches /8bf2ea29eacff3a407272eb5631edbd1a14a0936/...

type prodServer struct {
	http.Handler
	baseURL        *url.URL
	defaultVersion model.WebVersion
}

func NewProdServer(bucket AssetBucket, version model.WebVersion) (prodServer, error) {
	loc, err := url.Parse(bucket.String())
	if err != nil {
		return prodServer{}, errors.Wrap(err, "NewProdServer")
	}
	s := prodServer{
		baseURL:        loc,
		defaultVersion: version,
	}
	s.Handler = InferVersion(version, http.HandlerFunc(s.fetchFromAssetBucket))
	return s, nil
}

func (s prodServer) TearDown(ctx context.Context) {
}

// This doesn't actually do any setup right now.
func (s prodServer) Serve(ctx context.Context) error {
	logger.Get(ctx).Verbosef("Serving Tilt production web assets from %s with default version %s",
		s.baseURL, s.defaultVersion)
	<-ctx.Done()
	return nil
}

// NOTE(nick): The reverse proxy in httputil makes the storage server 500 and I have no idea
// why. But this only needs a very limited GET interface without query params,
// so just make the request by hand.
func (s prodServer) fetchFromAssetBucket(w http.ResponseWriter, req *http.Request) {
	u := *s.baseURL
	u.Path = path.Join(u.Path, req.URL.Path)
	outreq, err := http.NewRequest("GET", u.String(), bytes.NewBuffer(nil))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	copyHeader(outreq.Header, req.Header)
	outres, err := http.DefaultClient.Do(outreq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer func() {
		_ = outres.Body.Close()
	}()

	// In case we change the length of the response below, we don't want the browser to be mad
	outres.Header.Del("Content-Length")
	copyHeader(w.Header(), outres.Header)

	resBody, err := ioutil.ReadAll(outres.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(outres.StatusCode)
	_, _ = w.Write(RewriteContentURLs(req, resBody))
}

func RewriteContentURLs(req *http.Request, content []byte) []byte {
	path := req.URL.Path
	shouldRewrite := strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".css")
	if !shouldRewrite {
		return content
	}

	prefix := getPublicPathPrefix(req)
	return bytes.ReplaceAll(content, []byte("/static/"), []byte(fmt.Sprintf("%s/static/", prefix)))
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
