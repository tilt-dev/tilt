// Package ssh provides the connection helper for ssh:// URL.
// Requires Docker 18.09 or later on the remote host.
package ssh

import (
	"net/url"

	"github.com/pkg/errors"
)

// New returns cmd and its args
func New(daemonURL string) (string, []string, error) {
	sp, err := parseSSHURL(daemonURL)
	if err != nil {
		return "", nil, err
	}
	return "ssh", append(sp.Args(), []string{"--", "docker", "system", "dial-stdio"}...), nil
}

func parseSSHURL(daemonURL string) (*sshSpec, error) {
	u, err := url.Parse(daemonURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "ssh" {
		return nil, errors.Errorf("expected scheme ssh, got %s", u.Scheme)
	}

	var sp sshSpec

	if u.User != nil {
		sp.user = u.User.Username()
		if _, ok := u.User.Password(); ok {
			return nil, errors.New("ssh helper does not accept plain-text password")
		}
	}
	sp.host = u.Hostname()
	if sp.host == "" {
		return nil, errors.Errorf("host is not specified")
	}
	sp.port = u.Port()
	if u.Path != "" {
		return nil, errors.Errorf("extra path: %s", u.Path)
	}
	if u.RawQuery != "" {
		return nil, errors.Errorf("extra query: %s", u.RawQuery)
	}
	if u.Fragment != "" {
		return nil, errors.Errorf("extra fragment: %s", u.Fragment)
	}
	return &sp, err
}

type sshSpec struct {
	user string
	host string
	port string
}

func (sp *sshSpec) Args() []string {
	var args []string
	if sp.user != "" {
		args = append(args, "-l", sp.user)
	}
	if sp.port != "" {
		args = append(args, "-p", sp.port)
	}
	args = append(args, sp.host)
	return args
}
