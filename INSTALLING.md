# Installing Tilt from Source

This guide covers building Tilt from source for development purposes.

## Prerequisites

### Required
- **Go 1.24+** ([download](https://golang.org/dl/))
- **Node.js 20+** ([download](https://nodejs.org/))
- **make**
- **C/C++ toolchain** (for CGO dependencies)

### Optional
- **Docker** - for running the full test suite
- **golangci-lint** - for linting

### Check Your Environment

Run the prerequisites checker to verify your setup:

```bash
# Linux/macOS
./scripts/check-prereqs.sh

# Windows (PowerShell)
.\scripts\check-prereqs.ps1
```

This will show which tools are installed and provide installation hints for any missing dependencies.

## Building

### Quick Start

```bash
# Full build (frontend + Go binary)
./scripts/build.sh

# Quick build (Go binary only - fastest)
./scripts/build.sh --quick
```

On Windows:
```powershell
.\scripts\build.ps1
.\scripts\build.ps1 -Quick
```

### Build Options

| Option | Description |
|--------|-------------|
| `--quick`, `-q` | Go binary only (skip frontend) |
| `--full`, `-f` | Full build with frontend (default) |
| `--js-only` | Build frontend only |
| `--clean` | Clean before building |
| `--no-install` | Build to `./bin/` instead of `$GOPATH/bin/` |
| `--help`, `-h` | Show help |

### Using Make (Alternative)

You can also build directly with make:

```bash
make build-js   # Build frontend (optional but recommended)
make install    # Build and install Go binary
```

## Verifying the Build

After building, verify with:

```bash
"$(go env GOPATH)/bin/tilt" version
```

The build date should match the current date.

## Running Tilt

Run `tilt up` in any project with a `Tiltfile`. Sample projects are available in the [integration](https://github.com/tilt-dev/tilt/tree/master/integration) directory.

## Troubleshooting

### Frontend Assets

If you skip the frontend build (`--quick`), Tilt will serve assets from a remote server (requires internet) or use the Webpack dev server in development mode.

For offline use, always do a full build to embed assets in the binary.

### Multiple Tilt Binaries

If you have Tilt installed via Homebrew or another method, ensure you're running the correct binary:

```bash
# Run the one you just built
"$(go env GOPATH)/bin/tilt" up

# Check which tilt is in your PATH
which tilt
```
