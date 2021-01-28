package probe

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

func TestProbeMetaOptions(t *testing.T) {
	f := starkit.NewFixture(t, NewExtension())
	defer f.TearDown()

	f.File("Tiltfile", `
p = probe(initial_delay_secs=123,
          timeout_secs=456,
          period_secs=789,
          success_threshold=987,
          failure_threshold=654)

print(p.initial_delay_secs)
print(p.timeout_secs)
print(p.period_secs)
print(p.success_threshold)
print(p.failure_threshold)
print("exec:", p.exec)
print("http_get:", p.http_get)
print("tcp_socket:", p.tcp_socket)
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	expectedOutput := strings.TrimSpace(`
123
456
789
987
654
exec: None
http_get: None
tcp_socket: None
`)

	require.Contains(t, f.PrintOutput(), expectedOutput)
}

func TestProbeExec(t *testing.T) {
	f := starkit.NewFixture(t, NewExtension())
	defer f.TearDown()

	f.File("Tiltfile", `
p = probe(exec=exec_action(command=["sleep", "60"]))

print(p.exec.command)
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	require.Contains(t, f.PrintOutput(), `["sleep", "60"]`)
}

func TestProbeHTTPGet(t *testing.T) {
	f := starkit.NewFixture(t, NewExtension())
	defer f.TearDown()

	f.File("Tiltfile", `
p = probe(http_get=http_get_action(host="example.com", port=8888, scheme='https', path='/status'))

print(p.http_get.host)
print(p.http_get.port)
print(p.http_get.scheme)
print(p.http_get.path)
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	expectedOutput := strings.TrimSpace(`
example.com
8888
https
/status
`)

	require.Contains(t, f.PrintOutput(), expectedOutput)
}

func TestProbeTCP(t *testing.T) {
	f := starkit.NewFixture(t, NewExtension())
	defer f.TearDown()

	f.File("Tiltfile", `
p = probe(tcp_socket=tcp_socket_action("localhost", 1234))

print(p.tcp_socket.host)
print(p.tcp_socket.port)
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	expectedOutput := strings.TrimSpace(`
localhost
1234
`)

	require.Contains(t, f.PrintOutput(), expectedOutput)
}
