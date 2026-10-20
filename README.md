# GoWave

The [Remnawave](https://github.com/remnawave) backend rewritten in Go. Same API and web interface, but it needs about 20 times less memory and handles several times more requests per second.

**Documentation: [Russian](https://aaasazan.github.io/gowave-docs/) · [English](https://aaasazan.github.io/gowave-docs/en/)**

| | GoWave | Remnawave |
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

The installer first asks for the language (Russian or English), then the database (SQLite or PostgreSQL in Docker), the reverse proxy (Caddy, Nginx or your own) and the domain, and sets up HTTPS. If Remnawave or PasarGuard is found on the server, it offers to migrate the data. Running the same command again opens a menu: update, change the domain, uninstall.

Whoever opens the panel first creates the admin, so open it right after installation.

Yandex authentication is unavailable, including with previously saved OAuth
settings. Its legacy API fields remain disabled for contract compatibility.

## Documentation

The documentation is available in [Russian](https://aaasazan.github.io/gowave-docs/) and [English](https://aaasazan.github.io/gowave-docs/en/). The links below go to the English version.

| Topic | Pages |
| :--- | :--- |
| Getting started | [Quick start](https://aaasazan.github.io/gowave-docs/en/overview/quick-start/), [requirements](https://aaasazan.github.io/gowave-docs/en/install/requirements/) |
| Installation | [installer](https://aaasazan.github.io/gowave-docs/en/install/installer/), [Docker and PostgreSQL](https://aaasazan.github.io/gowave-docs/en/install/docker/), [systemd and SQLite](https://aaasazan.github.io/gowave-docs/en/install/systemd/), [updating](https://aaasazan.github.io/gowave-docs/en/install/updating/) |
| Reverse proxy | [Caddy](https://aaasazan.github.io/gowave-docs/en/install/reverse-proxy/caddy/), [Nginx](https://aaasazan.github.io/gowave-docs/en/install/reverse-proxy/nginx/) |
| Nodes | [Go node](https://aaasazan.github.io/gowave-docs/en/install/node/install/) ([remnanode-go](https://github.com/aaaSaZaN/remnanode-go)), [moving from the Docker node](https://aaasazan.github.io/gowave-docs/en/migration/docker-node/) |
| Subscription page | [Installation](https://aaasazan.github.io/gowave-docs/en/install/subscription-page/) |
| Migration | [from Remnawave](https://aaasazan.github.io/gowave-docs/en/migration/remnawave/), [from PasarGuard](https://aaasazan.github.io/gowave-docs/en/migration/pasarguard/), [from SQLite to Docker](https://aaasazan.github.io/gowave-docs/en/migration/sqlite-to-docker/), [moving to PostgreSQL 18](https://aaasazan.github.io/gowave-docs/en/migration/postgres-18/) |
| Subscriptions | [Response Rules (SRR)](https://aaasazan.github.io/gowave-docs/en/features/subscription-rules/), [templates](https://aaasazan.github.io/gowave-docs/en/features/templates/) |
| Reference | [environment variables](https://aaasazan.github.io/gowave-docs/en/reference/environment/), [CLI](https://aaasazan.github.io/gowave-docs/en/reference/cli/), [compatibility](https://aaasazan.github.io/gowave-docs/en/reference/compatibility/), [API](https://aaasazan.github.io/gowave-docs/api/) |

## Compatibility

GoWave implements all 218 commands of `@remnawave/backend-contract` 3.4.15. Tests check the routes against the contract and validate responses against its JSON schemas on SQLite and PostgreSQL, and subscription output is compared with the reference output of the original backend. The Remnawave web interface, `remnawave/node` nodes, the [Go node](https://github.com/aaaSaZaN/remnanode-go) and the official subscription page all work.

The main difference: regular expressions in rules and templates use RE2 (Go) syntax, not JavaScript. See the [comparison with Remnawave](https://aaasazan.github.io/gowave-docs/en/overview/comparison/) for details.

## Development

```bash
git clone https://github.com/aaaSaZaN/gowave.git && cd gowave
pnpm --dir frontend install --frozen-lockfile && pnpm --dir frontend build
cd backend-go && go test ./... && CGO_ENABLED=0 go build -o gowave ./cmd/server
```

Contract checks, PostgreSQL tests and building the Docker image are covered on the [Building from source](https://aaasazan.github.io/gowave-docs/en/development/) page and in [AGENTS.md](AGENTS.md).

## Author

- GoWave: [@aaaSaZaN](https://github.com/aaaSaZaN)
- Original project: [Remnawave](https://github.com/remnawave)

## License

[AGPL-3.0-only](LICENSE), same as the Remnawave codebase.
