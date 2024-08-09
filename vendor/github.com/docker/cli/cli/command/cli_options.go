package command

import (
	"context"
	"encoding/csv"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/docker/cli/cli/streams"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/moby/term"
	"github.com/pkg/errors"
)

// CLIOption is a functional argument to apply options to a [DockerCli]. These
// options can be passed to [NewDockerCli] to initialize a new CLI, or
// applied with [DockerCli.Initialize] or [DockerCli.Apply].
type CLIOption func(cli *DockerCli) error

// WithStandardStreams sets a cli in, out and err streams with the standard streams.
func WithStandardStreams() CLIOption {
	return func(cli *DockerCli) error {
		// Set terminal emulation based on platform as required.
		stdin, stdout, stderr := term.StdStreams()
		cli.in = streams.NewIn(stdin)
		cli.out = streams.NewOut(stdout)
		cli.err = streams.NewOut(stderr)
		return nil
	}
}

// WithBaseContext sets the base context of a cli. It is used to propagate
// the context from the command line to the client.
func WithBaseContext(ctx context.Context) CLIOption {
	return func(cli *DockerCli) error {
		cli.baseCtx = ctx
		return nil
	}
}

// WithCombinedStreams uses the same stream for the output and error streams.
func WithCombinedStreams(combined io.Writer) CLIOption {
	return func(cli *DockerCli) error {
		s := streams.NewOut(combined)
		cli.out = s
		cli.err = s
		return nil
	}
}

// WithInputStream sets a cli input stream.
func WithInputStream(in io.ReadCloser) CLIOption {
	return func(cli *DockerCli) error {
		cli.in = streams.NewIn(in)
		return nil
	}
}

// WithOutputStream sets a cli output stream.
func WithOutputStream(out io.Writer) CLIOption {
	return func(cli *DockerCli) error {
		cli.out = streams.NewOut(out)
		return nil
	}
}

// WithErrorStream sets a cli error stream.
func WithErrorStream(err io.Writer) CLIOption {
	return func(cli *DockerCli) error {
		cli.err = streams.NewOut(err)
		return nil
	}
}

// WithContentTrustFromEnv enables content trust on a cli from environment variable DOCKER_CONTENT_TRUST value.
func WithContentTrustFromEnv() CLIOption {
	return func(cli *DockerCli) error {
		cli.contentTrust = false
		if e := os.Getenv("DOCKER_CONTENT_TRUST"); e != "" {
			if t, err := strconv.ParseBool(e); t || err != nil {
				// treat any other value as true
				cli.contentTrust = true
			}
		}
		return nil
	}
}

// WithContentTrust enables content trust on a cli.
func WithContentTrust(enabled bool) CLIOption {
	return func(cli *DockerCli) error {
		cli.contentTrust = enabled
		return nil
	}
}

// WithDefaultContextStoreConfig configures the cli to use the default context store configuration.
func WithDefaultContextStoreConfig() CLIOption {
	return func(cli *DockerCli) error {
		cli.contextStoreConfig = DefaultContextStoreConfig()
		return nil
	}
}

// WithAPIClient configures the cli to use the given API client.
func WithAPIClient(c client.APIClient) CLIOption {
	return func(cli *DockerCli) error {
		cli.client = c
		return nil
	}
}

// envOverrideHTTPHeaders is the name of the environment-variable that can be
// used to set custom HTTP headers to be sent by the client. This environment
// variable is the equivalent to the HttpHeaders field in the configuration
// file.
//
// WARNING: If both config and environment-variable are set, the environment
// variable currently overrides all headers set in the configuration file.
// This behavior may change in a future update, as we are considering the
// environment variable to be appending to existing headers (and to only
// override headers with the same name).
//
// While this env-var allows for custom headers to be set, it does not allow
// for built-in headers (such as "User-Agent", if set) to be overridden.
// Also see [client.WithHTTPHeaders] and [client.WithUserAgent].
//
// This environment variable can be used in situations where headers must be
// set for a specific invocation of the CLI, but should not be set by default,
// and therefore cannot be set in the config-file.
//
// envOverrideHTTPHeaders accepts a comma-separated (CSV) list of key=value pairs,
// where key must be a non-empty, valid MIME header format. Whitespaces surrounding
// the key are trimmed, and the key is normalised. Whitespaces in values are
// preserved, but "key=value" pairs with an empty value (e.g. "key=") are ignored.
// Tuples without a "=" produce an error.
//
// It follows CSV rules for escaping, allowing "key=value" pairs to be quoted
// if they must contain commas, which allows for multiple values for a single
// header to be set. If a key is repeated in the list, later values override
// prior values.
//
// For example, the following value:
//
//	one=one-value,"two=two,value","three= a value with whitespace  ",four=,five=five=one,five=five-two
//
// Produces four headers (four is omitted as it has an empty value set):
//
// - one (value is "one-value")
// - two (value is "two,value")
// - three (value is " a value with whitespace  ")
// - five (value is "five-two", the later value has overridden the prior value)
const envOverrideHTTPHeaders = "DOCKER_CUSTOM_HEADERS"

// withCustomHeadersFromEnv overriding custom HTTP headers to be sent by the
// client through the [envOverrideHTTPHeaders] environment-variable. This
// environment variable is the equivalent to the HttpHeaders field in the
// configuration file.
//
// WARNING: If both config and environment-variable are set, the environment-
// variable currently overrides all headers set in the configuration file.
// This behavior may change in a future update, as we are considering the
// environment-variable to be appending to existing headers (and to only
// override headers with the same name).
//
// TODO(thaJeztah): this is a client Option, and should be moved to the client. It is non-exported for that reason.
func withCustomHeadersFromEnv() client.Opt {
	return func(apiClient *client.Client) error {
		value := os.Getenv(envOverrideHTTPHeaders)
		if value == "" {
			return nil
		}
		csvReader := csv.NewReader(strings.NewReader(value))
		fields, err := csvReader.Read()
		if err != nil {
			return errdefs.InvalidParameter(errors.Errorf("failed to parse custom headers from %s environment variable: value must be formatted as comma-separated key=value pairs", envOverrideHTTPHeaders))
		}
		if len(fields) == 0 {
			return nil
		}

		env := map[string]string{}
		for _, kv := range fields {
			k, v, hasValue := strings.Cut(kv, "=")

			// Only strip whitespace in keys; preserve whitespace in values.
			k = strings.TrimSpace(k)

			if k == "" {
				return errdefs.InvalidParameter(errors.Errorf(`failed to set custom headers from %s environment variable: value contains a key=value pair with an empty key: '%s'`, envOverrideHTTPHeaders, kv))
			}

			// We don't currently allow empty key=value pairs, and produce an error.
			// This is something we could allow in future (e.g. to read value
			// from an environment variable with the same name). In the meantime,
			// produce an error to prevent users from depending on this.
			if !hasValue {
				return errdefs.InvalidParameter(errors.Errorf(`failed to set custom headers from %s environment variable: missing "=" in key=value pair: '%s'`, envOverrideHTTPHeaders, kv))
			}

			env[http.CanonicalHeaderKey(k)] = v
		}

		if len(env) == 0 {
			// We should probably not hit this case, as we don't skip values
			// (only return errors), but we don't want to discard existing
			// headers with an empty set.
			return nil
		}

		// TODO(thaJeztah): add a client.WithExtraHTTPHeaders() function to allow these headers to be _added_ to existing ones, instead of _replacing_
		//  see https://github.com/docker/cli/pull/5098#issuecomment-2147403871  (when updating, also update the WARNING in the function and env-var GoDoc)
		return client.WithHTTPHeaders(env)(apiClient)
	}
}
