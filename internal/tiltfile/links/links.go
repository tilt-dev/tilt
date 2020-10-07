package links

import (
	"fmt"
	"net/url"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
)

type Link struct {
	model.Link
}

var _ starlark.Value = Link{}

func (l Link) String() string {
	return fmt.Sprintf("Link(url=%q, name=%q)", l.URL, l.Name)
}

func (l Link) Type() string {
	return "Link"
}

func (l Link) Freeze() {}

func (l Link) Truth() starlark.Bool {
	return l.Link != model.Link{}
}

func (l Link) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: port_forward")
}

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

// Implements functions for dealing with k8s secret settings.
type Extension struct{}

var _ starkit.Extension = Extension{}

func NewExtension() Extension {
	return Extension{}
}

func (e Extension) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("link", e.link)
}

func (e Extension) link(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var url, name string

	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"url", &url,
		"name?", &name); err != nil {
		return nil, err
	}

	withScheme, err := maybeAddScheme(url)
	if err != nil {
		return nil, errors.Wrapf(err, "validating URL %q", url)
	}

	return Link{
		Link: model.Link{
			URL:  withScheme,
			Name: name,
		},
	}, nil
}
