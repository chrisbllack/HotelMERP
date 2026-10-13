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
DATABASE_URL=postgresql://infineon:REPLACE_ME@127.0.0.1:5432/infineon
JWT_SECRET=REPLACE_ME_WITH_A_LONG_RANDOM_SECRET
"@ | Set-Content -LiteralPath $ConfigPath -Encoding ascii -NoNewline
    Write-Warning "Created $ConfigPath. Set DATABASE_URL and JWT_SECRET before starting the service."
}

# Keep credentials in the protected file. The wrapper sets them only in the
# service process instead of persisting them in the machine environment.
& icacls.exe $ConfigPath /inheritance:r /grant:r '*S-1-5-18:(R)' '*S-1-5-32-544:(R)' | Out-Null
if ($LASTEXITCODE -ne 0) { throw "icacls failed with exit code $LASTEXITCODE" }

$wrapperPath = Join-Path $configDirectory "$ServiceName-start.ps1"
$wrapper = @'
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$configPath = '__CONFIG_PATH__'
$binaryPath = '__BINARY_PATH__'

Get-Content -LiteralPath $configPath | ForEach-Object {
    $line = $_.Trim()
    if ($line -and -not $line.StartsWith('#') -and $line.Contains('=')) {
        $parts = $line.Split('=', 2)
        [Environment]::SetEnvironmentVariable($parts[0].Trim(), $parts[1].Trim(), 'Process')
    }
}

& $binaryPath serve --port __PORT__
exit $LASTEXITCODE
'@
$wrapper = $wrapper.Replace('__CONFIG_PATH__', ($ConfigPath -replace "'", "''"))
$wrapper = $wrapper.Replace('__BINARY_PATH__', ($resolvedBinary -replace "'", "''"))
$wrapper = $wrapper.Replace('__PORT__', $Port.ToString())
$wrapper | Set-Content -LiteralPath $wrapperPath -Encoding utf8 -NoNewline

$service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
$powershell = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
$binPath = "`"$powershell`" -NoProfile -NonInteractive -ExecutionPolicy Bypass -File `"$wrapperPath`""

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
Write-Host "Configuration is read from the protected service file $ConfigPath."
