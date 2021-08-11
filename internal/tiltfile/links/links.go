package links

import (
	"fmt"
	"net/url"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
)

type Link struct {
	*starlarkstruct.Struct
	model.Link
}

var _ starlark.Value = Link{}

// Parse resource links (string or `link`) into model.Link
// Not to be confused with a LinkED List :P
type LinkList struct {
	Links []model.Link
}

func (ll *LinkList) Unpack(v starlark.Value) error {
	seq := value.ValueOrSequenceToSlice(v)
	for _, val := range seq {
		switch val := val.(type) {
		case starlark.String:
			li, err := strToLink(val)
			if err != nil {
				return err
			}
			ll.Links = append(ll.Links, li)
		case Link:
			ll.Links = append(ll.Links, val.Link)
		default:
			return fmt.Errorf("`Want a string, a link, or a sequence of these; found %v (type: %T)", val, val)
		}
	}
	return nil
}

func strToLink(s starlark.String) (model.Link, error) {
	withScheme, err := parseAndMaybeAddScheme(string(s))
	if err != nil {
		return model.Link{}, errors.Wrapf(err, "validating URL %q", string(s))
	}
	return model.Link{URL: withScheme}, nil
}

func parseAndMaybeAddScheme(uStr string) (*url.URL, error) {
	if uStr == "" {
		return nil, fmt.Errorf("url empty")
	}
	u, err := url.Parse(uStr)
	if err != nil {
		// NOTE(maia): this unfortunately isn't a very robust check, `url.Parse`
		// returns an error in very few cases, but it's better than nothing.
		return nil, err
	}

	if u.Scheme == "" {
		// If the given URL string doesn't have a scheme, assume it's http
		u.Scheme = "http"
	}

	return u, nil
}

// Implements functions for dealing with k8s secret settings.
type Plugin struct{}

var _ starkit.Plugin = Plugin{}

func NewPlugin() Plugin {
	return Plugin{}
}

func (e Plugin) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("link", e.link)
}

func (e Plugin) link(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var url, name string

	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"url", &url,
		"name?", &name); err != nil {
		return nil, err
	}

	withScheme, err := parseAndMaybeAddScheme(url)
	if err != nil {
		return nil, errors.Wrapf(err, "validating URL %q", url)
	}

	return Link{
		Struct: starlarkstruct.FromStringDict(starlark.String("link"), starlark.StringDict{
			"url":  starlark.String(withScheme.String()),
			"name": starlark.String(name),
		}),
		Link: model.Link{
			URL:  withScheme,
			Name: name,
		},
	}, nil
}
