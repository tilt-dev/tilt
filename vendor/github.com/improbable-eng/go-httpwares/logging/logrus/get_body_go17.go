// +build go1.7,!go1.8

package http_logrus

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
)

func getBody(r *http.Request) func() (io.ReadCloser, error) {
	if r.Body != nil {
		body, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			return func() (io.ReadCloser, error) {
				return nil, err
			}
		}

		b := bytes.NewBuffer(body)
		r.Body = ioutil.NopCloser(b)

		return func() (io.ReadCloser, error) {
			b := bytes.NewBuffer(body)
			return ioutil.NopCloser(b), nil
		}
	}
	return nil
}
