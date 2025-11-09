// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23

package command

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const (
	otelContextFieldName     string = "otel"
	otelExporterOTLPEndpoint string = "OTEL_EXPORTER_OTLP_ENDPOINT"
	debugEnvVarPrefix        string = "DOCKER_CLI_"
)

// dockerExporterOTLPEndpoint retrieves the OTLP endpoint used for the docker reporter
// from the current context.
func dockerExporterOTLPEndpoint(cli Cli) (endpoint string, secure bool) {
	meta, err := cli.ContextStore().GetMetadata(cli.CurrentContext())
	if err != nil {
		otel.Handle(err)
		return "", false
	}

	var otelCfg any
	switch m := meta.Metadata.(type) {
	case DockerContext:
		otelCfg = m.AdditionalFields[otelContextFieldName]
	case map[string]any:
		otelCfg = m[otelContextFieldName]
	}

	if otelCfg != nil {
		otelMap, ok := otelCfg.(map[string]any)
		if !ok {
			otel.Handle(errors.Errorf(
				"unexpected type for field %q: %T (expected: %T)",
				otelContextFieldName,
				otelCfg,
				otelMap,
			))
		}
		// keys from https://opentelemetry.io/docs/concepts/sdk-configuration/otlp-exporter-configuration/
		endpoint, _ = otelMap[otelExporterOTLPEndpoint].(string)
	}

	// Override with env var value if it exists AND IS SET
	// (ignore otel defaults for this override when the key exists but is empty)
	if override := os.Getenv(debugEnvVarPrefix + otelExporterOTLPEndpoint); override != "" {
		endpoint = override
	}

	if endpoint == "" {
		return "", false
	}

	// Parse the endpoint. The docker config expects the endpoint to be
	// in the form of a URL to match the environment variable, but this
	// option doesn't correspond directly to WithEndpoint.
	//
	// We pretend we're the same as the environment reader.
	u, err := url.Parse(endpoint)
	if err != nil {
		otel.Handle(errors.Errorf("docker otel endpoint is invalid: %s", err))
		return "", false
	}

	switch u.Scheme {
	case "unix":
		endpoint = unixSocketEndpoint(u)
	case "https":
		secure = true
		fallthrough
	case "http":
		endpoint = path.Join(u.Host, u.Path)
	}
	return endpoint, secure
}

func dockerSpanExporter(ctx context.Context, cli Cli) []sdktrace.TracerProviderOption {
	endpoint, secure := dockerExporterOTLPEndpoint(cli)
	if endpoint == "" {
		return nil
	}

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
	}
	if !secure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exp, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		otel.Handle(err)
		return nil
	}
	return []sdktrace.TracerProviderOption{sdktrace.WithBatcher(exp, sdktrace.WithExportTimeout(exportTimeout))}
}

func dockerMetricExporter(ctx context.Context, cli Cli) []sdkmetric.Option {
	endpoint, secure := dockerExporterOTLPEndpoint(cli)
	if endpoint == "" {
		return nil
	}

	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(endpoint),
	}
	if !secure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}

	exp, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		otel.Handle(err)
		return nil
	}
	return []sdkmetric.Option{sdkmetric.WithReader(newCLIReader(exp))}
}

// unixSocketEndpoint converts the unix scheme from URL to
// an OTEL endpoint that can be used with the OTLP exporter.
//
// The OTLP exporter handles unix sockets in a strange way.
// It seems to imply they can be used as an environment variable
// and are handled properly, but they don't seem to be as the behavior
// of the environment variable is to strip the scheme from the endpoint
// while the underlying implementation needs the scheme to use the
// correct resolver.
func unixSocketEndpoint(u *url.URL) string {
	// GRPC does not allow host to be used.
	socketPath := u.Path

	// If we are on windows and we have an absolute path
	// that references a letter drive, check to see if the
	// WSL equivalent path exists and we should use that instead.
	if isWsl() {
		if p := wslSocketPath(socketPath, os.DirFS("/")); p != "" {
			socketPath = p
		}
	}
	// Enforce that we are using forward slashes.
	return "unix://" + filepath.ToSlash(socketPath)
}

// wslSocketPath will convert the referenced URL to a WSL-compatible
// path and check if that path exists. If the path exists, it will
// be returned.
func wslSocketPath(s string, f fs.FS) string {
	if p := toWslPath(s); p != "" {
		if _, err := stat(p, f); err == nil {
			return "/" + p
		}
	}
	return ""
}

// toWslPath converts the referenced URL to a WSL-compatible
// path if this looks like a Windows absolute path.
//
// If no drive is in the URL, defaults to the C drive.
func toWslPath(s string) string {
	drive, p, ok := parseUNCPath(s)
	if !ok {
		return ""
	}
	return fmt.Sprintf("mnt/%s%s", strings.ToLower(drive), p)
}

func parseUNCPath(s string) (drive, p string, ok bool) {
	// UNC paths use backslashes but we're using forward slashes
	// so also enforce that here.
	//
	// In reality, this should have been enforced much earlier
	// than here since backslashes aren't allowed in URLs, but
	// we're going to code defensively here.
	s = filepath.ToSlash(s)

	const uncPrefix = "//./"
	if !strings.HasPrefix(s, uncPrefix) {
		// Not a UNC path.
		return "", "", false
	}
	s = s[len(uncPrefix):]

	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		// Not enough components.
		return "", "", false
	}

	drive, ok = splitWindowsDrive(parts[0])
	if !ok {
		// Not a windows drive.
		return "", "", false
	}
	return drive, "/" + parts[1], true
}

// splitWindowsDrive checks if the string references a windows
// drive (such as c:) and returns the drive letter if it is.
func splitWindowsDrive(s string) (string, bool) {
	if b := []rune(s); len(b) == 2 && unicode.IsLetter(b[0]) && b[1] == ':' {
		return string(b[0]), true
	}
	return "", false
}

func stat(p string, f fs.FS) (fs.FileInfo, error) {
	if f, ok := f.(fs.StatFS); ok {
		return f.Stat(p)
	}

	file, err := f.Open(p)
	if err != nil {
		return nil, err
	}

	defer file.Close()
	return file.Stat()
}

func isWsl() bool {
	return os.Getenv("WSL_DISTRO_NAME") != ""
}
