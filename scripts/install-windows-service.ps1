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

# Provision the Event Log source up front. The service host cannot create it
# from its non-interactive SCM context, so the installer does it once.
if (-not [System.Diagnostics.EventLog]::SourceExists($ServiceName)) {
    New-EventLog -LogName Application -Source $ServiceName
}

$wrapperPath = Join-Path $configDirectory "$ServiceName-start.ps1"
$wrapper = @'
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

$configPath = '__CONFIG_PATH__'
$binaryPath = '__BINARY_PATH__'
$port = __PORT__
$serviceName = '__SERVICE_NAME__'

# Load configuration into the service process environment only. Secrets never
# leave the per-process scope.
Get-Content -LiteralPath $configPath | ForEach-Object {
    $line = $_.Trim()
    if ($line -and -not $line.StartsWith('#') -and $line.Contains('=')) {
        $parts = $line.Split('=', 2)
        [Environment]::SetEnvironmentVariable($parts[0].Trim(), $parts[1].Trim(), 'Process')
    }
}

# The wrapper hosts the Go binary inside a System.ServiceProcess.ServiceBase
# class so that it actually owns the Windows Service Control Manager
# lifecycle: StartServiceCtrlDispatcher handshake, status reporting, and
# graceful control handling. Registering powershell.exe directly with sc.exe
# would not satisfy the SCM protocol and the service would fail with
# error 1053 on startup.
$source = @'
using System;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.ServiceProcess;

public class InfineonService : ServiceBase
{
    [StructLayout(LayoutKind.Sequential)]
    private struct JOBOBJECT_BASIC_LIMIT_INFORMATION
    {
        public long PerProcessUserTimeLimit;
        public long PerJobUserTimeLimit;
        public uint LimitFlags;
        public UIntPtr MinimumWorkingSetSize;
        public UIntPtr MaximumWorkingSetSize;
        public uint ActiveProcessLimit;
        public UIntPtr Affinity;
        public uint PriorityClass;
        public uint SchedulingClass;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct IO_COUNTERS
    {
        public ulong ReadOperationCount;
        public ulong WriteOperationCount;
        public ulong OtherOperationCount;
        public ulong ReadTransferCount;
        public ulong WriteTransferCount;
        public ulong OtherTransferCount;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct JOBOBJECT_EXTENDED_LIMIT_INFORMATION
    {
        public JOBOBJECT_BASIC_LIMIT_INFORMATION BasicLimitInformation;
        public IO_COUNTERS IoInfo;
        public UIntPtr ProcessMemoryLimit;
        public UIntPtr JobMemoryLimit;
        public UIntPtr PeakProcessMemoryUsed;
        public UIntPtr PeakJobMemoryUsed;
    }

    private const int JobObjectExtendedLimitInformation = 9;
    private const uint JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE = 0x2000;

    [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
    private static extern IntPtr CreateJobObject(IntPtr lpJobAttributes, string lpName);

    [DllImport("kernel32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool SetInformationJobObject(IntPtr hJob, int JobObjectInfoClass, IntPtr lpJobObjectInfo, uint cbJobObjectInfoLength);

    [DllImport("kernel32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool AssignProcessToJobObject(IntPtr hJob, IntPtr hProcess);

    [DllImport("kernel32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool CloseHandle(IntPtr hObject);

    private readonly string binaryPath;
    private readonly string binaryDirectory;
    private readonly string port;
    private readonly string logDirectory;
    private readonly object syncRoot = new object();
    private Process child;
    private IntPtr jobHandle = IntPtr.Zero;

    public InfineonService(string serviceName, string binaryPath, string binaryDirectory, string port, string logDirectory)
    {
        this.binaryPath = binaryPath;
        this.binaryDirectory = binaryDirectory;
        this.port = port;
        this.logDirectory = logDirectory;
        this.ServiceName = serviceName;
        this.AutoLog = true;
        this.CanShutdown = true;
        this.CanStop = true;
        this.CanPauseAndContinue = false;
    }

    protected override void OnStart(string[] args)
    {
        try
        {
            Directory.CreateDirectory(logDirectory);

            jobHandle = CreateJobObject(IntPtr.Zero, null);
            if (jobHandle == IntPtr.Zero)
                throw new Win32Exception(Marshal.GetLastWin32Error());

            var info = new JOBOBJECT_BASIC_LIMIT_INFORMATION { LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE };
            var extended = new JOBOBJECT_EXTENDED_LIMIT_INFORMATION { BasicLimitInformation = info };
            int length = Marshal.SizeOf(typeof(JOBOBJECT_EXTENDED_LIMIT_INFORMATION));
            IntPtr ptr = Marshal.AllocHGlobal(length);
            try
            {
                Marshal.StructureToPtr(extended, ptr, false);
                if (!SetInformationJobObject(jobHandle, JobObjectExtendedLimitInformation, ptr, (uint)length))
                    throw new Win32Exception(Marshal.GetLastWin32Error());
            }
            finally
            {
                Marshal.FreeHGlobal(ptr);
            }

            var psi = new ProcessStartInfo
            {
                FileName = binaryPath,
                Arguments = "serve --port " + port,
                UseShellExecute = false,
                CreateNoWindow = true,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                WorkingDirectory = binaryDirectory
            };

            child = new Process { StartInfo = psi, EnableRaisingEvents = true };
            child.OutputDataReceived += (s, e) => { if (e.Data != null) TryAppend(Path.Combine(logDirectory, "infineon.out.log"), e.Data); };
            child.ErrorDataReceived += (s, e) => { if (e.Data != null) TryAppend(Path.Combine(logDirectory, "infineon.err.log"), e.Data); };
            child.Exited += OnChildExited;
            if (!child.Start())
                throw new InvalidOperationException("Process.Start returned false.");
            if (!AssignProcessToJobObject(jobHandle, child.Handle))
                throw new Win32Exception(Marshal.GetLastWin32Error());
            child.BeginOutputReadLine();
            child.BeginErrorReadLine();
        }
        catch (Exception ex)
        {
            EventLog.WriteEntry(this.ServiceName, "Failed to start infineon: " + ex, EventLogEntryType.Error);
            throw;
        }
    }

    private void OnChildExited(object sender, EventArgs e)
    {
        int code = -1;
        bool exited = false;
        try
        {
            if (child != null)
            {
                code = child.ExitCode;
                exited = true;
            }
        }
        catch { }
        EventLog.WriteEntry(
            this.ServiceName,
            "Infineon child exited" + (exited ? (" with code " + code) : "") + ".",
            (exited && code == 0) ? EventLogEntryType.Information : EventLogEntryType.Error);
        try { Stop(); } catch { }
    }

    protected override void OnStop()
    {
        StopChild();
    }

    protected override void OnShutdown()
    {
        StopChild();
    }

    private void StopChild()
    {
        Process local;
        IntPtr localJob;
        lock (syncRoot)
        {
            local = child;
            localJob = jobHandle;
            child = null;
        }
        if (local != null)
        {
            try
            {
                if (!local.HasExited)
                {
                    local.CloseMainWindow();
                    if (!local.WaitForExit(15000))
                    {
                        local.Kill();
                        local.WaitForExit(5000);
                    }
                }
            }
            catch (Exception ex)
            {
                EventLog.WriteEntry(this.ServiceName, "Error stopping infineon child: " + ex.Message, EventLogEntryType.Warning);
            }
            finally
            {
                local.Dispose();
            }
        }
        if (localJob != IntPtr.Zero)
        {
            CloseHandle(localJob);
        }
    }

    private static void TryAppend(string path, string line)
    {
        try { File.AppendAllText(path, line + Environment.NewLine); }
        catch { /* logging is best-effort */ }
    }
}
'@

if (-not ('InfineonService' -as [type])) {
    Add-Type -TypeDefinition $source -Language CSharp
}

$binaryDirectory = [IO.Path]::GetDirectoryName($binaryPath)
$logDirectory = Join-Path $binaryDirectory 'logs'

$service = New-Object InfineonService -ArgumentList @($serviceName, $binaryPath, $binaryDirectory, $port, $logDirectory)
[System.ServiceProcess.ServiceBase]::Run($service)
'@
$wrapper = $wrapper.Replace('__CONFIG_PATH__', ($ConfigPath -replace "'", "''"))
$wrapper = $wrapper.Replace('__BINARY_PATH__', ($resolvedBinary -replace "'", "''"))
$wrapper = $wrapper.Replace('__SERVICE_NAME__', ($ServiceName -replace "'", "''"))
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
