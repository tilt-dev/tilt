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
	baseUrl        *url.URL
	defaultVersion model.WebVersion
}

func NewProdServer(bucket AssetBucket, version model.WebVersion) (prodServer, error) {
	loc, err := url.Parse(bucket.String())
	if err != nil {
		return prodServer{}, errors.Wrap(err, "NewProdServer")
	}
	return prodServer{
		baseUrl:        loc,
		defaultVersion: version,
	}, nil
}

func (s prodServer) TearDown(ctx context.Context) {
}

// This doesn't actually do any setup right now.
func (s prodServer) Serve(ctx context.Context) error {
	logger.Get(ctx).Verbosef("Serving Tilt production web assets from %s with default version %s",
		s.baseUrl, s.defaultVersion)
	<-ctx.Done()
	return nil
}

// NOTE(nick): The reverse proxy in httputil makes the storage server 500 and I have no idea
// why. But this only needs a very limited GET interface without query params,
// so just make the request by hand.
func (s prodServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	outurl, version := s.urlAndVersionForReq(req)
	outreq, err := http.NewRequest("GET", outurl.String(), bytes.NewBuffer(nil))
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

	w.WriteHeader(outres.StatusCode)
	resBody, _ := ioutil.ReadAll(outres.Body)
	resBodyWithVersion := s.injectVersion(resBody, version)
	_, _ = w.Write(resBodyWithVersion)
}

func (s prodServer) urlAndVersionForReq(req *http.Request) (url.URL, string) {
	u := *s.baseUrl
	origPath := req.URL.Path

	if matches := versionRe.FindStringSubmatch(origPath); len(matches) > 1 {
		// If url contains a version prefix, don't attach another version
		u.Path = path.Join(u.Path, origPath)
		return u, matches[1]
	}
	if matches := shaRe.FindStringSubmatch(origPath); len(matches) > 1 {
		u.Path = path.Join(u.Path, origPath)
		return u, matches[1]
	}

	if !(strings.HasPrefix(origPath, "/static/")) {
		// redirect everything else to the main entry point.
		origPath = "index.html"
	}

	version := req.URL.Query().Get(WebVersionKey)
	if version == "" {
		version = string(s.defaultVersion)
	}

	u.Path = path.Join(u.Path, version, origPath)
	return u, version
}

// injectVersion updates all links to "/static/..." to instead point to "/vA.B.C/static/..."
// We do this b/c asset index.html's may contain links to "/static/..." that don't specify the
// version prefix, but leave it up to the asset server to resolve. Now that the asset server
// may serve multiple versions at once, we need to specify.
func (s prodServer) injectVersion(html []byte, version string) []byte {
	newPrefix := fmt.Sprintf("/%s/static/", version)
	return bytes.ReplaceAll(html, []byte("/static/"), []byte(newPrefix))
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
