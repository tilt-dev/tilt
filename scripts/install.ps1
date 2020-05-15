# Tilt installer
#
# Usage:
#   iex ((new-object net.webclient).DownloadString('https://raw.githubusercontent.com/tilt-dev/tilt/master/scripts/install.ps1')

$version = "0.13.6"
$url = "https://github.com/tilt-dev/tilt/releases/download/v" + $version + "/tilt." + $version + ".windows.x86_64.zip"
$zip = "tilt-" + $version + ".zip"
$extractDir = "tilt-" + $version
$binDir = "$HOME\bin"
$dest = "$binDir\tilt.exe"

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

Write-Output "Tilt installed!"
Write-Output "Run '$dest up' to start."
Write-Output "Or add $binDir to your PATH"

