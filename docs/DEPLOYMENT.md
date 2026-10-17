# Native Deployment

HotelMERP is intended to run on a customer-owned host with PostgreSQL installed
as an operating-system service. Remote access is optional and should be added
with Cloudflare Tunnel or a site VPN, not by exposing the application port
directly to the internet.

## Support Matrix

| Host | Status | Service manager | Notes |
| --- | --- | --- | --- |
| Ubuntu/Debian Linux | Target | systemd | PostgreSQL 16 and Go build tools are installed natively. |
| Windows Server 2022+ | Target | Windows Service | Use the checked-in PowerShell installer and a local PostgreSQL service. |
| Docker/Podman | Compatibility only | Container runtime | Existing files remain for compatibility; they are not the release path. |

The repository currently contains the Go modules and server packages expected by
the native design, but it does not contain `cmd/infineon`, the executable entry
point referenced by the existing Makefile, Dockerfile, and service template.
Restore or supply that entrypoint before producing a production binary. The
native assets in this document fail early rather than silently installing a
nonexistent executable.

## Prerequisites

- PostgreSQL 16, running as a native service.
- Go version declared by `go.mod` when building the backend.
- Node.js 20+ and npm for frontend development/build tasks.
- A generated `infineon` binary for service installation.
- A strong operator-managed `JWT_SECRET` and database credentials.

Do not commit `.env` files, database passwords, tunnel credentials, or JWT
secrets. Copy `.env.example` to a protected location and set values through the
service manager or an environment file with owner-only permissions.

## Linux

Build and validate from the repository root:

```bash
go build -trimpath -ldflags='-s -w' -o bin/infineon ./cmd/infineon
./scripts/validate-native-deployment.sh
```

Install the binary and service as an operator with access to `/usr/local/bin`
and `/etc/infineon`. Place the protected environment file at
`/etc/infineon/infineon.env`, then review `infrastructure/infineon.service` and
run:

```bash
sudo install -Dm755 bin/infineon /usr/local/bin/infineon
sudo install -Dm644 infrastructure/infineon.service /etc/systemd/system/infineon.service
sudo systemctl daemon-reload
sudo systemctl enable --now infineon
sudo systemctl status infineon
```

The application should bind to `127.0.0.1` by default. Only change the bind
address after reviewing the host firewall and trust boundary.

## Windows Server

Build the executable from a Windows development host or CI runner:

```powershell
go build -trimpath -ldflags="-s -w" -o bin\infineon.exe .\cmd\infineon
```

Run the installer from an elevated PowerShell prompt. It is idempotent: reruns
update the service definition without creating a second service. The generated
wrapper reads the protected env file and sets `DATABASE_URL`, `JWT_SECRET`, and
other settings only in the service process; secrets are not written to the
machine-wide environment.

```powershell
.\scripts\install-windows-service.ps1 `
  -BinaryPath (Resolve-Path .\bin\infineon.exe) `
  -ConfigPath 'C:\ProgramData\Infineon\infineon.env'
```

Check service state and the local readiness endpoint with:

```powershell
Get-Service Infineon
node .\scripts\healthcheck.mjs --url http://127.0.0.1:8080/api/v1/health
```

## Health Checks

Use `scripts/healthcheck.mjs` from either Windows or Linux. It performs a GET,
requires a 2xx response, and emits a useful failure message for connection,
HTTP-status, or malformed-response errors.

```bash
node scripts/healthcheck.mjs
node scripts/healthcheck.mjs --url http://127.0.0.1:8080/api/v1/health --timeout-ms 5000
```

Keep health checks local. A reverse proxy or tunnel can expose the application
without making PostgreSQL or the application listener publicly reachable.

## Optional Cloudflare Tunnel

The tunnel is operator-configured and is not required to run HotelMERP. Install
`cloudflared` using Cloudflare's platform instructions, authenticate outside
this repository, and place the generated credential JSON on the host with
owner-only permissions. Start from the checked-in template:

```bash
sudo install -Dm600 infrastructure/cloudflared/config.yml.example \
  /etc/cloudflared/config.yml
sudo systemctl enable --now cloudflared
```

Edit the hostname and tunnel UUID in `/etc/cloudflared/config.yml`. The
ingress service must target `http://127.0.0.1:8080`; never put database
credentials or the tunnel token in this repository.

## Existing Container Files

`Dockerfile` and `infrastructure/docker-compose.yml` are retained as legacy
compatibility artifacts. They are not used by the native CI job or the native
runbook, and they should not be treated as the supported production install
until the missing Go entrypoint and container-specific assumptions are
reconciled.
