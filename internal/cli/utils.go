package cli

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/token"
	"github.com/tilt-dev/wmclient/pkg/dirs"
)

func apiHost() string {
	return fmt.Sprintf("%s:%d", provideWebHost(), provideWebPort())
}

func apiURL(path string) string {
	path = strings.TrimLeft(path, "/")
	return fmt.Sprintf("http://%s:%d/api/%s", provideWebHost(), provideWebPort(), path)
}

func loadToken() string {
	dir, err := dirs.UseTiltDevDir()
	if err != nil {
		return ""
	}
	t, err := token.GetOrCreateToken(dir)
	if err != nil {
		return ""
	}
	return t.String()
}

func apiGet(path string) (body io.ReadCloser) {
	url := apiURL(path)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		cmdFail(fmt.Errorf("Could not build request for %s: %v", url, err))
	}
	req.Header.Set(server.TiltTokenHeaderName, loadToken())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		cmdFail(fmt.Errorf("Could not connect to Tilt at %s: %v", url, err))
	}

	if res.StatusCode != http.StatusOK {
		failWithNonOKResponse(url, res)
	}
	return res.Body
}

func apiPostJson(path string, payload []byte) (body io.ReadCloser, status int) {
	url := apiURL(path)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		cmdFail(fmt.Errorf("Could not build request for %s: %v", url, err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(server.TiltTokenHeaderName, loadToken())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		cmdFail(fmt.Errorf("Could not connect to Tilt at %s: %v", url, err))
	}

	return res.Body, res.StatusCode
}

func cmdFail(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
	os.Exit(1)
}

func failWithNonOKResponse(url string, res *http.Response) {
	body := "<no response body>"
	b, err := io.ReadAll(res.Body)
	if err != nil {
		cmdFail(fmt.Errorf("Error reading response body from %s: %v", url, err))
	}
	if string(b) != "" {
		body = string(b)
	}
	_ = res.Body.Close()
	cmdFail(fmt.Errorf("Request to %s failed with status %q: %s", url, res.Status, body))
}
