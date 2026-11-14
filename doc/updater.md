# Self-update

Etherpad-Go can check GitHub releases and — for writable single-binary installs —
download, verify and atomically swap in a new release binary, then exit so a process
supervisor restarts into the new version.

It is configured under the `updates` key in `settings.json` (defaults live in
`lib/settings/configRegistry.go`). All defaults are safe: out of the box the updater only
**checks and notifies**, it never replaces anything.

## Tiers (`updates.tier`)

| Tier         | Behaviour                                                                       |
|--------------|---------------------------------------------------------------------------------|
| `off`        | Do nothing.                                                                     |
| `notify`     | Check for releases and surface availability (log + admin UI). **Default.**      |
| `manual`     | As `notify`, plus an admin can trigger an apply from the admin UI.              |
| `auto`       | Scheduler auto-applies after `preApplyGraceMinutes`.                            |
| `autonomous` | Scheduler auto-applies, but only inside the configured `maintenanceWindow`.     |

`autonomous` **requires** a valid `maintenanceWindow`; without one it will not auto-apply
(it behaves like `manual`).

## Settings

| Key                                   | Default                  | Meaning                                                            |
|---------------------------------------|--------------------------|--------------------------------------------------------------------|
| `tier`                                | `notify`                 | See above.                                                         |
| `githubRepo`                          | `ether/etherpad-go`      | `owner/repo` to poll for releases.                                 |
| `checkIntervalHours`                  | `6`                      | Hours between release checks.                                      |
| `installMethod`                       | `auto`                   | `auto` \| `binary` \| `docker` \| `managed` (see below).           |
| `preApplyGraceMinutes`                | `0`                      | Delay between scheduling and applying an auto update.              |
| `drainSeconds`                        | `60`                     | Seconds to refuse new connections and warn clients before swap.    |
| `rollbackHealthCheckSeconds`          | `60`                     | Time to wait for a healthy boot before rolling back.               |
| `requireSignature`                    | `false`                  | Require an ed25519 signature over the checksums file.              |
| `trustedPublicKey`                    | `""`                     | Base64 ed25519 public key used when `requireSignature` is true.    |
| `stateFile`                           | `var/update-state.json`  | Persisted state machine (see persistence note).                    |
| `maintenanceWindow.{start,end,tz}`    | `"" / "" / local`        | `HH:MM` window for the `autonomous` tier; `tz` is `local` or `utc`.|

## Install-method detection

`installMethod: auto` resolves to:

- `docker` inside a container — `/.dockerenv` (Docker), `/run/.containerenv` (Podman, CRI-O)
  or `KUBERNETES_SERVICE_HOST` in the environment (every Kubernetes pod, also with containerd,
  which writes no marker file) — **never self-applies**; the container orchestrator owns the
  image. The updater only notifies.
- `binary` if the running executable's directory is writable — **self-applies**.
- `managed` otherwise (read-only/package-managed) — notifies only.

## Requirement: the supervisor must restart on exit code 75

A successful apply (and a rollback) **exits the process with code 75** and relies on a
supervisor to restart it. Without that, self-update simply stops Etherpad. Configure one of:

- **systemd**: `Restart=always` plus `RestartForceExitStatus=75` (and `SuccessExitStatus=75`).
- **Docker / Compose**: `restart: unless-stopped` (or `always`).
- **Kubernetes**: the default container restart policy already restarts on non-zero exit.

## Requirement: persistent state file

`updates.stateFile` (default `var/update-state.json`) **must survive the restart**. After a
swap the process exits 75 in `pending-verification`; on the next boot it reads the state file
to either confirm the update healthy or roll back. If `var/` is ephemeral the verify/rollback
handshake is lost. (For `docker` installs this is moot — they never self-apply.)

## Release artifacts

The updater downloads, per release tag:

- A **binary asset** whose name contains the running OS and arch tokens, e.g.
  `etherpad-go-linux-amd64`, `etherpad-go-windows-amd64.exe`. It is swapped in as the running
  executable (no archive — a raw binary).
- A **`checksums.txt`** asset (sha256, one line per binary). **Mandatory** — without it every
  apply fails. The release workflow (`.github/workflows/build.yml`) generates it via
  `sha256sum etherpad-go-* > checksums.txt`.
- Optionally a **`checksums.txt.sig`** (raw ed25519 over the checksums file) when
  `requireSignature` is enabled.

## Apply pipeline

Preflight refuses an apply without enough free disk space next to the executable (twice the
binary plus 50 MB).

`lock → preflight → drain (warn clients, refuse new /socket.io upgrades) → download + verify
→ atomic swap → exit 75`. On the next boot the update is held `pending-verification` until the
server serves a healthy boot (then `verified`); a crash loop or health-check timeout triggers
an automatic rollback to the previous binary. A failed rollback is terminal
(`rollback-failed`) and blocks further auto-applies until an admin **acknowledges** it from
the admin UI.

A release whose automatic apply failed, was rolled back or was cancelled is not applied
automatically again (that would repeat on every check); a newer release is, an admin can apply
it by hand, or acknowledge the failure to let it be scheduled again. A schedule survives a
restart, and an apply the process died in is recorded as failed instead of blocking the
updater. If, at boot, a version other than the update's target is running (installed by hand),
the pending verification is left to the admin instead of rolling back over it.

## Admin UI

The **Updates** page (`/admin/updates`) shows the running and latest version, what the
updater is doing and its log (`update.log` next to the state file, rotated at 1 MB, three
backups). There an admin can check for updates, apply one, **cancel** a scheduled update or
the drain before the restart, and **acknowledge** a failed preflight, a rollback, a failed
rollback or a verification nobody can finish.

## Outdated notice on pads

When the running version is a minor release or more behind, the first author of a pad sees a
short notice on opening it (`/api/version-status`, cached for a minute per pad and author). With
`tier: off` the endpoint does not exist.

## E-mail notices

With `adminEmail` set, the updater mails that address when the instance is a minor release or
more behind (at most every 30 days), when it schedules an update, and when an apply fails, is
rolled back or the rollback fails (once per release and outcome; a failed rollback always).
`mail` is the SMTP server; without `mail.host` the mails are only written to the log. The
Updates page can send a test mail.

```json
"adminEmail": "ops@example.org",
"mail": {
  "host": "smtp.example.org",
  "port": 587,
  "secure": false,
  "from": "etherpad@example.org",
  "auth": { "user": "etherpad", "pass": "${SMTP_PASSWORD}" }
}
```

`port` defaults to 587; there STARTTLS is used when the server offers it, and a password is
only sent over TLS (or to `localhost`). `secure: true` connects with TLS from the start (465).
The password is redacted on the admin settings page.

## Not covered

- **Session-cookie signing**: the Fiber session id is an opaque, server-side-stored random
  token (tampering yields an invalid session), so it is not signed/rotated. Only the OIDC
  signing secret is rotated (see `lib/security/secretrotator.go`).
- **Vulnerability notices** (etherpad-lite's `vulnerableBelow` list): etherpad-go publishes no
  such list.
