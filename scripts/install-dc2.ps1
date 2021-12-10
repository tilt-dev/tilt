# Install docker-compose v2 on Windows CI

# https://docs.docker.com/compose/install/#install-compose-on-windows-server

[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$dc = Get-Command -Name docker-compose -ErrorAction Stop
Rename-Item $dc.Source -NewName "docker-compose-v1.exe"
docker-compose-v1 version

$dc_version = "v2.2.2"
Invoke-WebRequest "https://github.com/docker/compose/releases/download/$dc_version/docker-compose-windows-x86_64.exe" -UseBasicParsing -OutFile $Env:ProgramFiles\Docker\docker-compose.exe
docker-compose version
