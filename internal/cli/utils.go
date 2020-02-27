package cli

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

func apiURL(webPort int, path string) string {
	path = strings.TrimLeft(path, "/")
	return fmt.Sprintf("http://localhost:%d/api/%s", webPort, path)
}

func apiGet(webPort int, path string) (body io.ReadCloser) {
	url := apiURL(webPort, path)
	res, err := http.Get(url)
	if err != nil {
		cmdFail(fmt.Errorf("Could not connect to Tilt at %s: %v", url, err))
	}

	if res.StatusCode != http.StatusOK {
		failWithNonOKResponse(url, res)
	}
	return res.Body
}

func apiPostJson(webPort int, path string, payload []byte) (body io.ReadCloser) {
	url := apiURL(webPort, path)
	res, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		cmdFail(fmt.Errorf("Could not connect to Tilt at %s: %v", url, err))
	}

	if res.StatusCode != http.StatusOK {
		failWithNonOKResponse(url, res)
	}
	return res.Body
}

func cmdFail(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
	os.Exit(1)
}

func failWithNonOKResponse(url string, res *http.Response) {
	body := "<no response body>"
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		cmdFail(fmt.Errorf("Error reading response body from %s: %v", url, err))
	}
	if string(b) != "" {
		body = string(b)
	}
	_ = res.Body.Close()
	cmdFail(fmt.Errorf("Request to %s failed with status %q: %s", url, res.Status, body))
}
