package tiltfile

import (
	"fmt"
	"net/url"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/pkg/model"
)

func convertLinks(val starlark.Value) ([]model.Link, error) {
	// TODO: validate URL? (e.g. do we require scheme?)
	if val == nil {
		return nil, nil
	}
	switch val := val.(type) {
	case starlark.String:
		li, err := strToLink(val)
		if err != nil {
			return nil, err
		}
		return []model.Link{li}, nil
	case link:
		return []model.Link{val.Link}, nil
	case starlark.Sequence:
		var result []model.Link
		it := val.Iterate()
		defer it.Done()
		var v starlark.Value
		for it.Next(&v) {
			switch v := v.(type) {
			case starlark.String:
				li, err := strToLink(v)
				if err != nil {
					return nil, err
				}
				result = append(result, li)
			case link:
				result = append(result, v.Link)
			default:
				return nil, fmt.Errorf("`link` arg %v includes element %v which must be a string or a link; is a %T", val, v, v)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("`link` value must be an string, a link, or a sequence of those; is a %T", val)
	}
}

func (s *tiltfileState) link(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var url, name string

	if err := s.unpackArgs(fn.Name(), args, kwargs,
		"url", &url,
		"name?", &name); err != nil {
		return nil, err
	}

	withScheme, err := maybeAddScheme(url)
	if err != nil {
		return nil, errors.Wrapf(err, "validating URL %q", url)
	}

	return link{
		model.Link{
			URL:  withScheme,
			Name: name,
		},
	}, nil
}

type link struct {
	model.Link
}

var _ starlark.Value = link{}

func (l link) String() string {
	return fmt.Sprintf("link(url=%q, name=%q)", l.URL, l.Name)
}

func (l link) Type() string {
	return "link"
}

func (l link) Freeze() {}

func (l link) Truth() starlark.Bool {
	return l.Link != model.Link{}
}

func (l link) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: port_forward")
}

func strToLink(s starlark.String) (model.Link, error) {
	withScheme, err := maybeAddScheme(string(s))
	if err != nil {
		return model.Link{}, errors.Wrapf(err, "validating URL %q", string(s))
	}
	return model.Link{URL: withScheme}, nil
}

func maybeAddScheme(uStr string) (string, error) {
	if uStr == "" {
		return "", fmt.Errorf("url empty")
	}
	u, err := url.Parse(uStr)
	if err != nil {
		// NOTE(maia): this unfortunately isn't a very robust check, `url.Parse`
		// returns an error in very few cases, but it's better than nothing.
		return "", err
	}

	if u.Scheme == "" {
		// If the given URL string doesn't have a scheme, assume it's http
		u.Scheme = "http"
	}

	return u.String(), nil
}
