#Requires -Version 5.1
<#
.SYNOPSIS
    Tilt Development Prerequisites Checker (Windows)

.DESCRIPTION
    Checks for required development tools and reports their status.
    Exit code 0 = all required tools present, 1 = missing required tools.

.EXAMPLE
    .\scripts\check-prereqs.ps1
#>

$ErrorActionPreference = "Stop"

# Version requirements
$REQUIRED_GO_VERSION = [Version]"1.24"
$REQUIRED_NODE_VERSION = 20

# Track missing/outdated tools
$script:Missing = @()
$script:Outdated = @()

function Write-Ok {
    param($Name, $Version, $Required)
    Write-Host "  " -NoNewline
    Write-Host "[OK]" -ForegroundColor Green -NoNewline
    Write-Host " $Name $Version (required: $Required)"
}

function Write-Fail {
    param($Name, $Version, $Required)
    Write-Host "  " -NoNewline
    Write-Host "[!!]" -ForegroundColor Red -NoNewline
    Write-Host " $Name $Version (required: $Required)"
}

function Write-Warn {
    param($Name, $Note)
    Write-Host "  " -NoNewline
    Write-Host "[--]" -ForegroundColor Yellow -NoNewline
    Write-Host " $Name ($Note)"
}

function Get-OSInfo {
    $os = [System.Environment]::OSVersion
    Write-Host "Operating System: Windows $($os.Version)"
}

function Test-Go {
    $goCmd = Get-Command go -ErrorAction SilentlyContinue
    if (-not $goCmd) {
        Write-Fail "Go" "not found" "$REQUIRED_GO_VERSION+"
        $script:Missing += "go"
        return
    }

    $versionOutput = & go version 2>&1
    if ($versionOutput -match 'go(\d+\.\d+)') {
        $version = [Version]$Matches[1]
        if ($version -ge $REQUIRED_GO_VERSION) {
            Write-Ok "Go" $version "$REQUIRED_GO_VERSION+"
        } else {
            Write-Fail "Go" $version "$REQUIRED_GO_VERSION+"
            $script:Outdated += "go"
        }
    } else {
        Write-Fail "Go" "unknown version" "$REQUIRED_GO_VERSION+"
        $script:Outdated += "go"
    }
}

function Test-Node {
    $nodeCmd = Get-Command node -ErrorAction SilentlyContinue
    if (-not $nodeCmd) {
        Write-Fail "Node.js" "not found" "$REQUIRED_NODE_VERSION+"
        $script:Missing += "node"
        return
    }

    $versionOutput = & node --version 2>&1
    $fullVersion = $versionOutput -replace '^v', ''
    if ($versionOutput -match 'v(\d+)') {
        $majorVersion = [int]$Matches[1]
        if ($majorVersion -ge $REQUIRED_NODE_VERSION) {
            Write-Ok "Node.js" "v$fullVersion" "$REQUIRED_NODE_VERSION+"
        } else {
            Write-Fail "Node.js" "v$fullVersion" "$REQUIRED_NODE_VERSION+"
            $script:Outdated += "node"
        }
    } else {
        Write-Fail "Node.js" "unknown version" "$REQUIRED_NODE_VERSION+"
        $script:Outdated += "node"
    }
}

function Test-Yarn {
    $yarnCmd = Get-Command yarn -ErrorAction SilentlyContinue
    if ($yarnCmd) {
        $version = & yarn --version 2>&1
        Write-Ok "Yarn" $version "bundled in project"
    } else {
        $corepackCmd = Get-Command corepack -ErrorAction SilentlyContinue
        if ($corepackCmd) {
            Write-Ok "Yarn" "via corepack" "bundled in project"
        } else {
            Write-Warn "Yarn" "not found, will use corepack"
        }
    }
}

function Test-Make {
    # On Windows, make is optional - Go build works without it
    $makeCmd = Get-Command make -ErrorAction SilentlyContinue
    if ($makeCmd) {
        Write-Ok "make" "" "optional on Windows"
    } else {
        Write-Warn "make" "optional on Windows, use build.ps1 instead"
    }
}

function Test-CC {
    # Check for C compiler (MSVC or MinGW)
    $hasCompiler = $false

    # Check for cl.exe (MSVC)
    $clCmd = Get-Command cl -ErrorAction SilentlyContinue
    if ($clCmd) {
        Write-Ok "C compiler" "(MSVC cl.exe)" "any"
        $hasCompiler = $true
    }

    # Check for gcc (MinGW)
    if (-not $hasCompiler) {
        $gccCmd = Get-Command gcc -ErrorAction SilentlyContinue
        if ($gccCmd) {
            Write-Ok "C compiler" "(gcc/MinGW)" "any"
            $hasCompiler = $true
        }
    }

    if (-not $hasCompiler) {
        Write-Fail "C compiler" "not found" "MSVC or MinGW"
        $script:Missing += "cc"
    }
}

function Test-Docker {
    $dockerCmd = Get-Command docker -ErrorAction SilentlyContinue
    if (-not $dockerCmd) {
        Write-Warn "Docker" "optional, for full test suite"
        return
    }

    $versionOutput = & docker --version 2>&1
    if ($versionOutput -match '(\d+\.\d+\.\d+)') {
        Write-Ok "Docker" $Matches[1] "optional"
    } else {
        Write-Ok "Docker" "unknown version" "optional"
    }
}

function Test-GolangciLint {
    $lintCmd = Get-Command golangci-lint -ErrorAction SilentlyContinue
    if (-not $lintCmd) {
        Write-Warn "golangci-lint" "optional, for linting"
        return
    }

    $versionOutput = & golangci-lint --version 2>&1
    if ($versionOutput -match '(\d+\.\d+\.\d+)') {
        Write-Ok "golangci-lint" $Matches[1] "optional"
    } else {
        Write-Ok "golangci-lint" "unknown version" "optional"
    }
}

function Show-InstallHints {
    Write-Host ""
    Write-Host "Installation Help:" -ForegroundColor White

    foreach ($tool in ($script:Missing + $script:Outdated)) {
        switch ($tool) {
            "go" {
                Write-Host "  Go: https://golang.org/dl/ or:"
                Write-Host "       winget install GoLang.Go"
                Write-Host "       scoop install go"
            }
            "node" {
                Write-Host "  Node.js: https://nodejs.org/ or:"
                Write-Host "       winget install OpenJS.NodeJS.LTS"
                Write-Host "       scoop install nodejs-lts"
            }
            "cc" {
                Write-Host "  C compiler options:"
                Write-Host "       Install Visual Studio Build Tools: https://visualstudio.microsoft.com/downloads/"
                Write-Host "       Or install MinGW: scoop install mingw"
            }
        }
    }
}

function Main {
    Write-Host "Tilt Development Environment Check" -ForegroundColor White
    Write-Host "==================================="
    Write-Host ""

    Get-OSInfo
    Write-Host ""

    Write-Host "Required Tools:" -ForegroundColor White
    Test-Go
    Test-Node
    Test-Yarn
    Test-Make
    Test-CC

    Write-Host ""
    Write-Host "Optional Tools:" -ForegroundColor White
    Test-Docker
    Test-GolangciLint

    if (($script:Missing.Count -gt 0) -or ($script:Outdated.Count -gt 0)) {
        Show-InstallHints
        Write-Host ""
        Write-Host "Some required tools are missing or outdated." -ForegroundColor Red
        exit 1
    }

    Write-Host ""
    Write-Host "All required tools are installed and meet version requirements." -ForegroundColor Green
    exit 0
}

Main
