package visitor

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

// Convert a list of filenames into YAML inputs.
func FromStrings(filenames []string, stdin io.Reader) ([]Interface, error) {
	result := []Interface{}
	for _, f := range filenames {

		switch {
		case f == "-":
			result = append(result, Stdin(stdin))

		case strings.Index(f, "http://") == 0 || strings.Index(f, "https://") == 0:
			url, err := url.Parse(f)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid URL %s", url)
			}
			result = append(result, URL(http.DefaultClient, f))

		default:
			result = append(result, File(f))

		}
	}
	return result, nil
}
