https://aaasazan.github.io/gowave-docs/

Repo with Ansible playbooks: https://github.com/aaaSaZaN/gowave-ansible.

## Development

The Go module is in `backend/`; the React application is in `frontend/`.
Use the Go version declared in `backend/go.mod` and Bun 1.4.2.

Copy `.env.sample` to `.env` at the repository root and set `APP_SECRET` to a
private value of at least 32 characters (`openssl rand -hex 32` generates one).
Keep it stable across restarts. The sample uses SQLite; the backend migrates and
seeds the configured database at startup, so use a separate development database.

```sh
cd frontend
bun install --frozen-lockfile
bun run dev
```

Open `http://localhost:3333`. `dev` starts the Go API on port 3000 and Vite on
3333; Ctrl+C stops both. Frontend edits reload automatically. Restart after Go
changes. `dev:frontend` and `dev:backend` run either component separately.

The optional `frontend/.env` should start from `frontend/.env.sample`. Defaults
use Vite's `/api` proxy to `127.0.0.1:3000`. Remove old port-3003 overrides and
`NODE_ENV` entries from existing frontend environment files. For a remote API,
set a full `DOMAIN_BACKEND` URL and `DOMAIN_OVERRIDE=1`; that API must allow the
frontend origin through CORS. Rebuild to apply these values to production output.

`bun run build` creates `frontend/dist/`; the Go server serves it alongside the
API. `bun run preview` serves an existing build on port 3333 for local inspection
and proxies `/api` to the local backend; start `dev:backend` separately.

## Verification and maintenance

Run frontend checks from `frontend/`, matching CI:

```sh
bun run check
bun run typecheck
bun run test
bun run build
```

`bun run fix` applies safe Biome fixes. `bun run test:browser` runs UI smoke
checks with Chromium (`CHROMIUM_BIN` overrides its path).

Run backend checks from `backend/`:

```sh
go vet ./...
go test -race -count=1 ./...
CGO_ENABLED=0 go build -o /tmp/gowave ./cmd/server
gofmt -l .
```

Frontend maintenance commands run from `frontend/`:

- `bun run css:types` regenerates exact CSS module declarations after class-name
  changes. Commit them with the CSS; `typecheck` detects stale declarations.
- `bun run i18n:types` regenerates types from local English translation resources.
- `bun run editor:download` fetches the hash-pinned Xray validator and schemas;
  network access is required. Run it before testing the Xray editor or packaging.
- `bun run editor:compress` compresses an existing `dist/` for release packaging.

Dependencies and strict checks are configured in each component. Update
`frontend/bun.lock` with intentional frontend dependency changes. Release tags
use `v<version>`; publishing is handled by the root GitHub workflows.

## Branding

Set `TELEGRAM_CHAT_URL` in `frontend/.env` or the build environment to configure
the header's Telegram button (default `https://telegram.org`). GitHub dev/release
builds use the Actions variable of the same name. Docker accepts it as a build
argument; the Compose build override reads it from the shell or root `.env`.
Use a full HTTP(S) URL and rebuild after changing it.

The root [LICENSE](LICENSE) covers the backend and frontend, including upstream
attribution. The GoWave logo and its PNG favicon reproductions are licensed
separately under [CC-BY-ND 4.0](frontend/public/favicons/CC-BY-ND-4.0.txt), with
[attribution](frontend/public/favicons/NOTICE.md).
