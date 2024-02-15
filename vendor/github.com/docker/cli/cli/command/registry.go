package command

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	configtypes "github.com/docker/cli/cli/config/types"
	"github.com/docker/cli/cli/hints"
	"github.com/docker/cli/cli/streams"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/registry"
	"github.com/moby/term"
	"github.com/pkg/errors"
)

const patSuggest = "You can log in with your password or a Personal Access " +
	"Token (PAT). Using a limited-scope PAT grants better security and is required " +
	"for organizations using SSO. Learn more at https://docs.docker.com/go/access-tokens/"

// EncodeAuthToBase64 serializes the auth configuration as JSON base64 payload.
//
// Deprecated: use [registrytypes.EncodeAuthConfig] instead.
func EncodeAuthToBase64(authConfig registrytypes.AuthConfig) (string, error) {
	return registrytypes.EncodeAuthConfig(authConfig)
}

// RegistryAuthenticationPrivilegedFunc returns a RequestPrivilegeFunc from the specified registry index info
// for the given command.
func RegistryAuthenticationPrivilegedFunc(cli Cli, index *registrytypes.IndexInfo, cmdName string) types.RequestPrivilegeFunc {
	return func() (string, error) {
		fmt.Fprintf(cli.Out(), "\nPlease login prior to %s:\n", cmdName)
		indexServer := registry.GetAuthConfigKey(index)
		isDefaultRegistry := indexServer == registry.IndexServer
		authConfig, err := GetDefaultAuthConfig(cli, true, indexServer, isDefaultRegistry)
		if err != nil {
			fmt.Fprintf(cli.Err(), "Unable to retrieve stored credentials for %s, error: %s.\n", indexServer, err)
		}
		err = ConfigureAuth(cli, "", "", &authConfig, isDefaultRegistry)
		if err != nil {
			return "", err
		}
		return registrytypes.EncodeAuthConfig(authConfig)
	}
}

// ResolveAuthConfig returns auth-config for the given registry from the
// credential-store. It returns an empty AuthConfig if no credentials were
// found.
//
// It is similar to [registry.ResolveAuthConfig], but uses the credentials-
// store, instead of looking up credentials from a map.
func ResolveAuthConfig(_ context.Context, cli Cli, index *registrytypes.IndexInfo) registrytypes.AuthConfig {
	configKey := index.Name
	if index.Official {
		configKey = registry.IndexServer
	}

	a, _ := cli.ConfigFile().GetAuthConfig(configKey)
	return registrytypes.AuthConfig(a)
}

// GetDefaultAuthConfig gets the default auth config given a serverAddress
// If credentials for given serverAddress exists in the credential store, the configuration will be populated with values in it
func GetDefaultAuthConfig(cli Cli, checkCredStore bool, serverAddress string, isDefaultRegistry bool) (registrytypes.AuthConfig, error) {
	if !isDefaultRegistry {
		serverAddress = registry.ConvertToHostname(serverAddress)
	}
	authconfig := configtypes.AuthConfig{}
	var err error
	if checkCredStore {
		authconfig, err = cli.ConfigFile().GetAuthConfig(serverAddress)
		if err != nil {
			return registrytypes.AuthConfig{
				ServerAddress: serverAddress,
			}, err
		}
	}
	authconfig.ServerAddress = serverAddress
	authconfig.IdentityToken = ""
	res := registrytypes.AuthConfig(authconfig)
	return res, nil
}

// ConfigureAuth handles prompting of user's username and password if needed
func ConfigureAuth(cli Cli, flUser, flPassword string, authconfig *registrytypes.AuthConfig, isDefaultRegistry bool) error {
	// On Windows, force the use of the regular OS stdin stream.
	//
	// See:
	// - https://github.com/moby/moby/issues/14336
	// - https://github.com/moby/moby/issues/14210
	// - https://github.com/moby/moby/pull/17738
	//
	// TODO(thaJeztah): we need to confirm if this special handling is still needed, as we may not be doing this in other places.
	if runtime.GOOS == "windows" {
		cli.SetIn(streams.NewIn(os.Stdin))
	}

	// Some links documenting this:
	// - https://code.google.com/archive/p/mintty/issues/56
	// - https://github.com/docker/docker/issues/15272
	// - https://mintty.github.io/ (compatibility)
	// Linux will hit this if you attempt `cat | docker login`, and Windows
	// will hit this if you attempt docker login from mintty where stdin
	// is a pipe, not a character based console.
	if flPassword == "" && !cli.In().IsTerminal() {
		return errors.Errorf("Error: Cannot perform an interactive login from a non TTY device")
	}

	authconfig.Username = strings.TrimSpace(authconfig.Username)

	if flUser = strings.TrimSpace(flUser); flUser == "" {
		if isDefaultRegistry {
			// if this is a default registry (docker hub), then display the following message.
			fmt.Fprintln(cli.Out(), "Log in with your Docker ID or email address to push and pull images from Docker Hub. If you don't have a Docker ID, head over to https://hub.docker.com/ to create one.")
			if hints.Enabled() {
				fmt.Fprintln(cli.Out(), patSuggest)
				fmt.Fprintln(cli.Out())
			}
		}
		promptWithDefault(cli.Out(), "Username", authconfig.Username)
		var err error
		flUser, err = readInput(cli.In())
		if err != nil {
			return err
		}
		if flUser == "" {
			flUser = authconfig.Username
		}
	}
	if flUser == "" {
		return errors.Errorf("Error: Non-null Username Required")
	}
	if flPassword == "" {
		oldState, err := term.SaveState(cli.In().FD())
		if err != nil {
			return err
		}
		fmt.Fprintf(cli.Out(), "Password: ")
		_ = term.DisableEcho(cli.In().FD(), oldState)
		defer func() {
			_ = term.RestoreTerminal(cli.In().FD(), oldState)
		}()
		flPassword, err = readInput(cli.In())
		if err != nil {
			return err
		}
		fmt.Fprint(cli.Out(), "\n")
		if flPassword == "" {
			return errors.Errorf("Error: Password Required")
		}
	}

	authconfig.Username = flUser
	authconfig.Password = flPassword

	return nil
}

// readInput reads, and returns user input from in. It tries to return a
// single line, not including the end-of-line bytes, and trims leading
// and trailing whitespace.
func readInput(in io.Reader) (string, error) {
	line, _, err := bufio.NewReader(in).ReadLine()
	if err != nil {
		return "", errors.Wrap(err, "error while reading input")
	}
	return strings.TrimSpace(string(line)), nil
}

func promptWithDefault(out io.Writer, prompt string, configDefault string) {
	if configDefault == "" {
		fmt.Fprintf(out, "%s: ", prompt)
	} else {
		fmt.Fprintf(out, "%s (%s): ", prompt, configDefault)
	}
}

// RetrieveAuthTokenFromImage retrieves an encoded auth token given a complete
// image. The auth configuration is serialized as a base64url encoded RFC4648,
// section 5) JSON string for sending through the X-Registry-Auth header.
//
// For details on base64url encoding, see:
// - RFC4648, section 5:   https://tools.ietf.org/html/rfc4648#section-5
func RetrieveAuthTokenFromImage(ctx context.Context, cli Cli, image string) (string, error) {
	// Retrieve encoded auth token from the image reference
	authConfig, err := resolveAuthConfigFromImage(ctx, cli, image)
	if err != nil {
		return "", err
	}
	encodedAuth, err := registrytypes.EncodeAuthConfig(authConfig)
	if err != nil {
		return "", err
	}
	return encodedAuth, nil
}

// resolveAuthConfigFromImage retrieves that AuthConfig using the image string
func resolveAuthConfigFromImage(ctx context.Context, cli Cli, image string) (registrytypes.AuthConfig, error) {
	registryRef, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return registrytypes.AuthConfig{}, err
	}
	repoInfo, err := registry.ParseRepositoryInfo(registryRef)
	if err != nil {
		return registrytypes.AuthConfig{}, err
	}
	return ResolveAuthConfig(ctx, cli, repoInfo.Index), nil
}
