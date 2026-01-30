# Tilt installer
#
# Usage:
#   iex ((new-object net.webclient).DownloadString('https://raw.githubusercontent.com/tilt-dev/tilt/master/scripts/install.ps1')

& { #enclose in scope to avoid leaking variables when ran through iex
$ErrorActionPreference = 'Stop'
try {        
$version = "0.36.3"
$url = "https://github.com/tilt-dev/tilt/releases/download/v" + $version + "/tilt." + $version + ".windows.x86_64.zip"
$zip = "tilt-" + $version + ".zip"
$extractDir = "tilt-" + $version
$binDir = "$HOME\bin"
$dest = "$binDir\tilt.exe"

$useScoop = ""
if (Get-Command "scoop" -ErrorAction Ignore) {
   $useScoop = "true"
}

if ("true" -eq $useScoop) {
    Write-Host "Scoop detected! (https://scoop.sh)"
    scoop bucket add tilt-dev https://github.com/tilt-dev/scoop-bucket
    scoop install tilt
    scoop update tilt
    tilt version
    tilt verify-install
    Write-Output "Tilt installed with Scoop! Run 'tilt up' to start."
    return
}

Write-Output "Downloading $url"
if (Test-Path "$zip") {
    Remove-Item -Force "$zip"
}
Invoke-WebRequest $url -OutFile $zip

Write-Output "Extracting $zip"
if (Test-Path "$extractDir") {
    Remove-Item -Force -Recurse "$extractDir"
}
Expand-Archive $zip -DestinationPath $extractDir

Write-Output "Installing Tilt as $dest"
New-Item -ItemType Directory -Force -Path $binDir >$null
try {
    Move-Item -Force -Path "$extractDir\tilt.exe" -Destination "$dest"    
}
catch {
    Write-Host "Unable to install tilt.exe. Is tilt in use?" -ForegroundColor Red
    throw
}

iex "$dest version"
iex "$dest verify-install"

Write-Output "Tilt installed!"
Write-Output "Run '$dest up' to start."
Write-Output "Or add $binDir to your PATH"
}
catch {
    Write-Output $_
    Write-Host "Tilt installation failed" -ForegroundColor Red
}
finally {
    Remove-Item -Force -Path "$zip" -ErrorAction Ignore
    Remove-Item -Force -Recurse -Path "$extractDir" -ErrorAction Ignore
}
}
