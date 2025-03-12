# Tilt installer
#
# Usage:
#   iex ((new-object net.webclient).DownloadString('https://raw.githubusercontent.com/tilt-dev/tilt/master/scripts/install.ps1')

$version = "0.34.0"
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
Move-Item -Force -Path "$extractDir\tilt.exe" -Destination "$dest"

Remove-Item -Force -Path "$zip"
Remove-Item -Force -Recurse -Path "$extractDir"

iex "$dest version"
iex "$dest verify-install"

Write-Output "Tilt installed!"
Write-Output "Run '$dest up' to start."
Write-Output "Or add $binDir to your PATH"

