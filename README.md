# GoWave

A lightweight proxy management panel written in Go. Same API and web interface, but it needs about 20 times less memory and handles several times more requests per second.

**Documentation: [Russian](https://aaasazan.github.io/gowave-docs/) · [English](https://aaasazan.github.io/gowave-docs/en/)**

| | GoWave | Original backend |
| :--- | :--- | :--- |
| Memory under load | about 50 MB | about 1 GB |
| API ready after start | 0.2 s | 4.8 s |
| Processes | one | three, plus Valkey |
| Database | SQLite or PostgreSQL | PostgreSQL |

Measurements and methodology are on the [benchmarks](https://aaasazan.github.io/gowave-docs/en/benchmarks/) page.

## Installation

You need a Linux server (Debian, Ubuntu or the RHEL family, `amd64`/`arm64`) and a domain whose A record points to that server.

```bash
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/aaaSaZaN/gowave/main/install.sh)"
```

The installer first asks for the language (Russian or English), then the database (SQLite or PostgreSQL in Docker), the reverse proxy (Caddy, Nginx or your own), the domain, and the dashboard path (default `/`), and sets up HTTPS. It also offers to install GoWave Node on the same server after the panel is ready. If an upstream panel or PasarGuard is found on the server, it offers to migrate the data. Running the same command again opens a menu: update, change the domain or dashboard path, install/update the local node, or uninstall.

Whoever opens the panel first creates the admin, so open it right after installation.

Native services run as the dedicated `gowave` account with `ProtectSystem=strict`,
`ProtectHome`, private devices and temporary directories, `NoNewPrivileges`, and
`MemoryDenyWriteExecute`. `CAP_NET_BIND_SERVICE` allows explicitly configured low
ports. The code is owned by root: `gowave-server` and `frontend/` cannot be changed
by the service account, so a compromised panel cannot plant a binary that root
later runs or persist in the dashboard files. `/opt/gowave` is `root:gowave` with
the sticky bit, so the panel can create its SQLite files there but cannot replace
root's files; `.env` is readable by the service and writable only by root.

In-panel updates go through a root helper: the panel leaves a request in
`/run/gowave`, `gowave-update.path` starts `gowave-update.service`, which downloads
the latest release, checks it against `SHA256SUMS`, installs it root-owned and
restarts the panel. Installers reapply the policy and the helper on every update.
A `gowave.service` you run as root keeps running as root, with a warning in the
log, until you update with the installer; installs where the service account owns
the binary are moved to the root-owned layout the same way, and the installer
refuses to run such a binary as root. Run native database/rescue commands as `sudo -u gowave` from
`/opt/gowave` so new files keep the service ownership. Custom writable paths require
an administrator's systemd override. Running the server as root is allowed but logs a
warning. The Docker image runs as UID/GID 10001; the supplied
Compose configuration also drops capabilities and uses a read-only root filesystem.

Yandex authentication is unavailable, including with previously saved OAuth
settings. Its legacy API fields remain disabled for contract compatibility.

## Documentation

The documentation is available in [Russian](https://aaasazan.github.io/gowave-docs/) and [English](https://aaasazan.github.io/gowave-docs/en/). The links below go to the English version.

| Topic | Pages |
| :--- | :--- |
| Getting started | [Quick start](https://aaasazan.github.io/gowave-docs/en/overview/quick-start/), [requirements](https://aaasazan.github.io/gowave-docs/en/install/requirements/) |
| Installation | [installer](https://aaasazan.github.io/gowave-docs/en/install/installer/), [Docker and PostgreSQL](https://aaasazan.github.io/gowave-docs/en/install/docker/), [systemd and SQLite](https://aaasazan.github.io/gowave-docs/en/install/systemd/), [updating](https://aaasazan.github.io/gowave-docs/en/install/updating/) |
| Reverse proxy | [Caddy](https://aaasazan.github.io/gowave-docs/en/install/reverse-proxy/caddy/), [Nginx](https://aaasazan.github.io/gowave-docs/en/install/reverse-proxy/nginx/) |
| Nodes | [Go node](https://aaasazan.github.io/gowave-docs/en/install/node/install/) ([gowave-node](https://github.com/aaaSaZaN/gowave-node)), [moving from the Docker node](https://aaasazan.github.io/gowave-docs/en/migration/docker-node/) |
| Subscription page | [Installation](https://aaasazan.github.io/gowave-docs/en/install/subscription-page/) |
| Migration | [from PasarGuard](https://aaasazan.github.io/gowave-docs/en/migration/pasarguard/), [from SQLite to Docker](https://aaasazan.github.io/gowave-docs/en/migration/sqlite-to-docker/), [moving to PostgreSQL 18](https://aaasazan.github.io/gowave-docs/en/migration/postgres-18/) |
| Subscriptions | [Response Rules (SRR)](https://aaasazan.github.io/gowave-docs/en/features/subscription-rules/), [templates](https://aaasazan.github.io/gowave-docs/en/features/templates/) |
| Reference | [environment variables](https://aaasazan.github.io/gowave-docs/en/reference/environment/), [CLI](https://aaasazan.github.io/gowave-docs/en/reference/cli/), [compatibility](https://aaasazan.github.io/gowave-docs/en/reference/compatibility/), [API](https://aaasazan.github.io/gowave-docs/api/) |

## Compatibility

GoWave implements all 218 commands of `@remnawave/backend-contract` 3.4.15. Tests check the routes against the contract and validate responses against its JSON schemas on SQLite and PostgreSQL, and subscription output is compared with the reference output of the original backend. Compatible web interfaces and nodes, the [Go node](https://github.com/aaaSaZaN/gowave-node) and the official subscription page all work.

The main difference: regular expressions in rules and templates use RE2 (Go) syntax, not JavaScript. See the [backend comparison](https://aaasazan.github.io/gowave-docs/en/overview/comparison/) for details.

## Development

Use Bun 1.4.2 (pinned in `frontend/package.json`) for frontend dependencies and scripts.

```bash
git clone https://github.com/aaaSaZaN/gowave.git && cd gowave
(cd frontend && bun install --frozen-lockfile && bun run build)
cd backend-go && go test ./... && CGO_ENABLED=0 go build -o gowave ./cmd/server
```

Contract checks, PostgreSQL tests and building the Docker image are covered on the [Building from source](https://aaasazan.github.io/gowave-docs/en/development/) page and in [AGENTS.md](AGENTS.md).

## Author

- GoWave: [@aaaSaZaN](https://github.com/aaaSaZaN)
- Original project: [Remnawave](https://github.com/remnawave)

## License

[AGPL-3.0-only](LICENSE), inherited from the original codebase.

## Dashboard path and upgrades

Set `PANEL_BASE_PATH=/private/panel/` in the panel's `.env` to place the dashboard
at `https://panel.example.com/private/panel/`. Nested paths are supported; only
letters, digits, underscores, and hyphens are accepted in each segment. The
default remains `/`. Restart the panel after changing this setting. Published
frontend bundles support this at runtime and do not need rebuilding.

API, health, WebSocket, metrics, and public subscription URLs keep their current
paths. A dashboard prefix changes the UI address; it does not restrict API access.
Caddy and Nginx configurations generated by the installer already forward the
original path. An external proxy must forward it unchanged, including the prefix.
The dashboard is served only beneath the configured path; `/` returns 404 when a
custom prefix is configured. Separately hosted frontends must supply the same
HTML base URL and serve assets beneath it.

The installer's **Change dashboard path** action backs up `.env`, restarts the
panel, checks dashboard delivery and API health, and restores the previous
configuration if verification fails. Update the panel first if an old release
does not support this setting. Ordinary updates and SQLite-to-Docker migration
preserve the setting, credentials, and subscription URLs.

Local node installation is optional and does not create panel configuration
profiles or register the node automatically. For a new node, select a free port
(default `2222`), then add its displayed address and port in the panel. In Docker,
use a server address reachable from the panel container. Existing nodes retain
their credentials and port; the node installer handles service migration and
rollback. A failed node installation leaves the panel running and can be retried
from the installer menu. Panel removal does not remove the independent node.

Project-owned branding uses GoWave. Required copyright notices, dependency
identifiers and runtime resources, and legacy protocol/database/migration
identifiers are retained for compatibility. Reviewed exceptions are recorded in
`scripts/branding-allowlist.json`; run `bun scripts/check-branding.cjs` to check
for unreviewed references.
