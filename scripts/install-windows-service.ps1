[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$BinaryPath,

    [string]$ConfigPath = 'C:\ProgramData\Infineon\infineon.env',

    [string]$ServiceName = 'Infineon',

    [int]$Port = 8080
)

$ErrorActionPreference = 'Stop'

$resolvedBinary = (Resolve-Path -LiteralPath $BinaryPath -ErrorAction Stop).Path
if ([IO.Path]::GetExtension($resolvedBinary) -ne '.exe') {
    throw "BinaryPath must point to a Windows executable (.exe): $resolvedBinary"
}

$configDirectory = Split-Path -Parent $ConfigPath
New-Item -ItemType Directory -Force -Path $configDirectory | Out-Null
if (-not (Test-Path -LiteralPath $ConfigPath)) {
    @"
INFINEON_PORT=$Port
INFINEON_BIND_ADDRESS=127.0.0.1
# Set INFINEON_DATABASE_URL and INFINEON_JWT_SECRET before starting the service.
"@ | Set-Content -LiteralPath $ConfigPath -Encoding ascii -NoNewline
    Write-Warning "Created $ConfigPath. Set database and JWT values before starting the service."
}

Get-Content -LiteralPath $ConfigPath | ForEach-Object {
    $line = $_.Trim()
    if ($line -and -not $line.StartsWith('#') -and $line.Contains('=')) {
        $parts = $line.Split('=', 2)
        [Environment]::SetEnvironmentVariable($parts[0].Trim(), $parts[1].Trim(), 'Machine')
    }
}

$service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
$binPath = "`"$resolvedBinary`" serve --port $Port"

if ($null -eq $service) {
    & sc.exe create $ServiceName binPath= $binPath start= auto DisplayName= 'INFINEON Enterprise Platform'
    if ($LASTEXITCODE -ne 0) { throw "sc.exe create failed with exit code $LASTEXITCODE" }
} else {
    & sc.exe config $ServiceName binPath= $binPath start= auto
    if ($LASTEXITCODE -ne 0) { throw "sc.exe config failed with exit code $LASTEXITCODE" }
}

& sc.exe failure $ServiceName reset= 86400 actions= restart/5000/restart/15000/none/0
if ($LASTEXITCODE -ne 0) { throw "sc.exe failure configuration failed with exit code $LASTEXITCODE" }

Write-Host "Service '$ServiceName' is installed for $resolvedBinary."
Write-Host "Keep the application bound to 127.0.0.1 and manage configuration in $ConfigPath."
