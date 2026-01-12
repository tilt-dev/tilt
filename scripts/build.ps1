#Requires -Version 5.1
<#
.SYNOPSIS
    Tilt Build Script (Windows)

.DESCRIPTION
    Unified build script with options for different build scenarios.

.PARAMETER Quick
    Go binary only (skip frontend build)

.PARAMETER Full
    Full build with frontend (default)

.PARAMETER JsOnly
    Build frontend only

.PARAMETER NoInstall
    Build but don't install to GOPATH

.PARAMETER Clean
    Clean before building

.EXAMPLE
    .\scripts\build.ps1              # Full build (JS + Go binary)
    .\scripts\build.ps1 -Quick       # Go binary only (fastest)
    .\scripts\build.ps1 -JsOnly      # Frontend only
#>

param(
    [switch]$Quick,
    [switch]$Full,
    [switch]$JsOnly,
    [switch]$NoInstall,
    [switch]$Clean,
    [switch]$Help
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir

# Defaults
$BuildJS = $true
$BuildGo = $true
$DoInstall = $true
$DoClean = $false

function Show-Usage {
    Write-Host "Tilt Build Script" -ForegroundColor White
    Write-Host ""
    Write-Host "Usage: .\scripts\build.ps1 [OPTIONS]"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  -Quick       Go binary only (skip frontend build)"
    Write-Host "  -Full        Full build with frontend (default)"
    Write-Host "  -JsOnly      Build frontend only"
    Write-Host "  -NoInstall   Build but don't install to GOPATH"
    Write-Host "  -Clean       Clean before building"
    Write-Host "  -Help        Show this help message"
    Write-Host ""
    Write-Host "Examples:"
    Write-Host "  .\scripts\build.ps1              # Full build (JS + Go binary)"
    Write-Host "  .\scripts\build.ps1 -Quick       # Go binary only (fastest)"
    Write-Host "  .\scripts\build.ps1 -JsOnly      # Frontend only"
}

function Write-Step {
    param($Num, $Total, $Message)
    Write-Host ""
    Write-Host "[$Num/$Total] $Message" -ForegroundColor White
}

function Test-Prerequisites {
    $checkScript = Join-Path $ScriptDir "check-prereqs.ps1"
    $result = & $checkScript 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Prerequisites check failed." -ForegroundColor Red
        Write-Host "Run .\scripts\check-prereqs.ps1 for details."
        exit 1
    }
    Write-Host "      All prerequisites satisfied."
}

function Clear-Build {
    Write-Host "      Cleaning previous build artifacts..."
    $assetsPath = Join-Path $RepoRoot "pkg\assets\build"
    if (Test-Path $assetsPath) {
        Remove-Item -Recurse -Force $assetsPath
    }
    New-Item -ItemType Directory -Path $assetsPath -Force | Out-Null

    $binPath = Join-Path $RepoRoot "bin\tilt.exe"
    if (Test-Path $binPath) {
        Remove-Item -Force $binPath
    }
}

function Build-Frontend {
    Write-Host "      Installing dependencies..."
    Set-Location (Join-Path $RepoRoot "web")

    # Enable corepack if available - suppress download prompts
    $corepack = Get-Command corepack -ErrorAction SilentlyContinue
    if ($corepack) {
        $env:COREPACK_ENABLE_DOWNLOAD_PROMPT = "0"
        & corepack enable 2>$null
    }

    & yarn install
    if ($LASTEXITCODE -ne 0) {
        Write-Host "yarn install failed" -ForegroundColor Red
        exit 1
    }

    Write-Host "      Building React app..."
    & yarn build
    if ($LASTEXITCODE -ne 0) {
        Write-Host "yarn build failed" -ForegroundColor Red
        exit 1
    }

    Write-Host "      Copying assets to pkg\assets\build\..."
    $srcPath = Join-Path $RepoRoot "web\build\*"
    $destPath = Join-Path $RepoRoot "pkg\assets\build"
    New-Item -ItemType Directory -Path $destPath -Force | Out-Null
    Copy-Item -Recurse -Force $srcPath $destPath

    Set-Location $RepoRoot
    Write-Host "      Frontend build complete." -ForegroundColor Green
}

function Build-GoApp {
    $commitSha = "unknown"
    try {
        $commitSha = & git rev-parse HEAD 2>$null
    } catch {}

    $ldflags = "-X 'github.com/tilt-dev/tilt/internal/cli.commitSHA=$commitSha'"

    if ($DoInstall) {
        $gopath = & go env GOPATH
        Write-Host "      Installing to $gopath\bin\tilt.exe..."
        & go install -mod vendor -ldflags $ldflags ./cmd/tilt/...
    } else {
        Write-Host "      Building to .\bin\tilt.exe..."
        $binDir = Join-Path $RepoRoot "bin"
        New-Item -ItemType Directory -Path $binDir -Force | Out-Null
        $output = Join-Path $binDir "tilt.exe"
        & go build -mod vendor -ldflags $ldflags -o $output ./cmd/tilt/...
    }

    if ($LASTEXITCODE -ne 0) {
        Write-Host "Go build failed" -ForegroundColor Red
        exit 1
    }

    Write-Host "      Go build complete." -ForegroundColor Green
}

function Main {
    # Parse parameters
    if ($Help) {
        Show-Usage
        exit 0
    }

    if ($Quick) {
        $script:BuildJS = $false
    }

    if ($JsOnly) {
        $script:BuildJS = $true
        $script:BuildGo = $false
        $script:DoInstall = $false
    }

    if ($NoInstall) {
        $script:DoInstall = $false
    }

    if ($Clean) {
        $script:DoClean = $true
    }

    Set-Location $RepoRoot

    Write-Host "Tilt Build" -ForegroundColor White
    Write-Host "=========="

    # Calculate total steps
    $total = 1  # prereqs check
    if ($DoClean) { $total++ }
    if ($BuildJS) { $total++ }
    if ($BuildGo) { $total++ }

    $currentStep = 0

    # Step: Check prerequisites
    $currentStep++
    Write-Step $currentStep $total "Checking prerequisites"
    Test-Prerequisites

    # Step: Clean (if requested)
    if ($DoClean) {
        $currentStep++
        Write-Step $currentStep $total "Cleaning build artifacts"
        Clear-Build
    }

    # Step: Build JS
    if ($BuildJS) {
        $currentStep++
        Write-Step $currentStep $total "Building frontend (web/)"
        Build-Frontend
    }

    # Step: Build Go
    if ($BuildGo) {
        $currentStep++
        Write-Step $currentStep $total "Building Go binary"
        Build-GoApp
    }

    # Summary
    Write-Host ""
    Write-Host "Build complete!" -ForegroundColor Green

    if ($BuildGo) {
        if ($DoInstall) {
            $gopath = & go env GOPATH
            $binary = Join-Path $gopath "bin\tilt.exe"
        } else {
            $binary = Join-Path $RepoRoot "bin\tilt.exe"
        }

        if (Test-Path $binary) {
            Write-Host "  Binary: $binary"
            try {
                $version = & $binary version 2>$null
                Write-Host "  Version: $version"
            } catch {
                Write-Host "  Version: unknown"
            }
        }
    }

    if ($BuildJS -and (-not $BuildGo)) {
        Write-Host ""
        Write-Host "Frontend built. Run '.\scripts\build.ps1 -Quick' to build Go binary."
    }
}

Main
