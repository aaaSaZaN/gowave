# Working in Gowave

## Repository context

Gowave combines a Go replacement for the Remnawave backend with a React frontend. Compatibility with `@remnawave/backend-contract` is a core constraint, including endpoint coverage, response envelopes, status codes, and subscription output. The frontend currently pins contract version `3.4.15`; `internal/server/parity_test.go` also checks that version explicitly.

- `backend-go/` is the Go module (`gowave`); run Go commands there, not at the repository root. Its `go.mod` declares Go `1.27.1`.
- `frontend/` is a separate React/TypeScript/Vite application. CI uses Node 22 and pnpm 11 with `pnpm-lock.yaml`. A `package-lock.json` also exists, but CI installs with pnpm.
- `scripts/` contains contract export/validation, subscription differential fixtures, and a node migration installer.
- Root `README.md` documents installation, reverse proxy configuration, subscription-page deployment, and rescue/import commands, mostly in Russian. For defaults, consult executable configuration (`backend-go/config/config.go`); the default SQLite filename is `gowave.db`.

## Commands and verification

### Backend (working directory: `backend-go`)

```sh
go run ./cmd/server
CGO_ENABLED=0 go build -o /tmp/gowave ./cmd/server
go vet ./...
go test -race -count=1 ./...
gofmt -l .
```

`main.go` and `cmd/server/main.go` both call `server.Run()`; CI and release builds use `./cmd/server`. CI requires no files reported by `gofmt -l .`.

Tests default to in-memory SQLite. To run the PostgreSQL path against a test database:

```sh
GOWAVE_TEST_POSTGRES='postgres://gowave:gowave@localhost:5432/gowave_test?sslmode=disable' go test -count=1 ./...
```

CI provides PostgreSQL 18. `internal/testdb.Open` creates a separate schema for each PostgreSQL test and drops it on cleanup; its database role must be able to create/drop schemas. SQLite tests use one connection and run migrations/default seeding through the same database setup.

### Frontend (working directory: `frontend`)

```sh
pnpm install --frozen-lockfile
pnpm run start:dev
pnpm run typecheck
pnpm run build
pnpm run check
```

- `build` runs only `vite build`; it does **not** typecheck. CI runs `typecheck` separately. `start:build` runs TypeScript followed by the production build.
- `check` runs oxfmt and oxlint. `format:fix` and `lint:fix` apply their respective fixes. CI currently runs typecheck/build/contract checks, not this lint/format command.
- No frontend test script is defined. Backend tests and schema probes cover API compatibility.
- Vite listens on `0.0.0.0:3333` with `strictPort`; `/api` proxies to `127.0.0.1:3000`. `start:preview` also uses port 3333.
- `make download-monaco-deps` fetches the editor WASM/runtime/schema files into `public/`. See the asset-serving caveat below before changing their paths.

### API contract checks

After frontend dependencies are installed, run from the repository root:

```sh
node scripts/export-contract.cjs > /tmp/commands.json
diff -u backend-go/internal/contracts/commands.json /tmp/commands.json
node scripts/export-contract.cjs --openapi > /tmp/openapi.json
cmp backend-go/internal/contracts/openapi.json /tmp/openapi.json
```

To capture and validate live handler responses:

```sh
# From backend-go:
GOWAVE_PROBE_OUT=/tmp/probe.json go test ./internal/server -run Probe -count=1
# From the repository root:
node scripts/validate-contract.cjs /tmp/probe.json
node scripts/validate-contract.cjs /tmp/probe-mutations.json
```

CI performs these checks for SQLite and PostgreSQL. The exporter and validator resolve the contract from `frontend/node_modules`; they do not require a root package install. When updating the pinned contract, regenerate the committed `commands.json` and `openapi.json` using the exporter and inspect parity tests and scope registries as well.

## Backend architecture and data flow

`internal/server/server.go` is the composition root: `Run` loads configuration, opens/migrates the database, handles CLI modes, validates configuration, constructs `NewApp`, and calls `App.Start`. `NewApp` wires services and the Chi router; worker startup is deferred to `Start`, allowing HTTP tests to use `app.Router` without starting loops.

- Domain packages such as `users`, `nodes`, and `configprofiles` use handlers/services backed by GORM. Other domains register handlers directly against the database; follow the surrounding package rather than imposing a uniform layer structure.
- `users.Service` persists changes and enqueues node synchronization in the same transaction. `jobs` implements a persistent transactional outbox with leased workers. Node startup/stop/retire and other commands are registered in `NewApp`. Preserve the transaction boundary when changing mutations so state and downstream work stay consistent.
- `nodes` owns node RPC clients, health/usage collection, and applying queued work. `keygen` supplies certificates/keys. Config-profile and user services share the node service for synchronization.
- `cron` runs lifecycle work; `notify` handles queued webhook/Telegram delivery; `prometheus` runs collection and the separate metrics HTTP listener.
- `subscription` handles public `/api/sub/...` requests. `subtpl` renders client formats and templates, including custom JavaScript/object-path compatibility helpers in `subtpl/js` and `subtpl/ojson`. `subtemplates` manages stored templates/page configurations; `configrender` is a separate configuration-rendering package.
- `stubs` contains substantial real feature implementations (including integrations and authentication-related functionality), despite its name. Check `stubs.RegisterRoutes` and its individual files before assuming a route is unimplemented.
- `cli` contains rescue, import, and migration workflows. Both ordinary startup and most CLI paths initialize/migrate the database; inspect `Run` before invoking administrative commands.

### HTTP compatibility

Public routes and the protected router group are registered in `server.go`; the protected group uses `middleware.Auth`. SSH WebSocket and system tools also have registrations outside that group, so preserve their own authentication paths.

Use `internal/httpx` for upstream-compatible JSON: success is commonly `{"response": ...}`, while errors contain `timestamp`, `path`, `message`, and `errorCode`. Unsupported capabilities use HTTP 501 and `GW501`. `middleware.ContractStatus` aligns successful status codes with the pinned contract; assess the final router response, not just the status written inside a handler.

`internal/contracts` contains embedded endpoint/scope data and scope lookup rules. `AdminOnly` restricts specific API prefixes. Changes to routes or authorization need corresponding parity/security coverage, not just handler tests.

## Database constraints

- `database.AutoMigrate` is a project wrapper: it applies numbered, embedded SQL migrations and seeds defaults. It does **not** invoke GORM's destructive type reconciliation. Changing a model alone does not migrate the schema.
- Add schema changes under `internal/database/migrations/sqlite/` and `postgres/`. The runner sorts filenames, records them in `gowave_migrations`, and executes statements **one non-comment line at a time**. Preserve that format. Its special `ADD COLUMN table column definition` syntax checks whether the column already exists.
- Imported upstream database types are deliberately preserved. PostgreSQL migration handling includes matching imported foreign-key types; examine migration and schema-export tests before changing existing types or relations.
- SQLite enables foreign keys, WAL, and a busy timeout, and limits the pool to one connection. Use the transaction handle inside transaction callbacks; separate database access can contend for that single connection.
- Timestamps are UTC: `Run` sets `time.Local = time.UTC`, GORM uses UTC `NowFunc`, and `testdb.Open` mirrors production. This matters for SQLite text ordering and upstream ISO timestamp parity.

## Frontend patterns

`src/app.tsx` composes Mantine, authentication, React Query, i18n, and routing. `src/app/router/` owns routing and `app/layouts/` owns shells. UI code is organized into `pages`, `widgets`, `features`, `entities`, and `shared`.

- Page `connectors` fetch data and handle loading before passing props to page `components`; `config-profiles` is a representative example. Filenames use role suffixes such as `.page.connector.tsx`, `.page.component.tsx`, `.widget.tsx`, and `.feature.tsx`.
- API hooks live in `shared/api/hooks/<domain>`. They use command definitions from `@remnawave/backend-contract` and reusable factories in `shared/api/tsq-helpers`, including Zod schemas, route/query parameters, and query keys. Extend these patterns for related endpoints.
- `shared/api/axios.ts` owns backend-domain selection, authorization headers, browser-client headers, and logout on 401. Production defaults to `window.location.origin`; `DOMAIN_OVERRIDE=1` forces the compiled backend domain. Vite injects `DOMAIN_BACKEND`, `DOMAIN_OVERRIDE`, and `NODE_ENV` at build time.
- The sample frontend environment points at port **3003** with override enabled, while the Go default and Vite proxy target are **3000**. Adjust environment values deliberately when setting up development.
- TypeScript aliases for `@shared`, `@entities`, `@features`, `@widgets`, `@pages`, and `@public` are declared in `tsconfig.json`; Vite resolves those paths. Do not assume every formatter import group has a matching TypeScript alias.
- Styling uses Mantine and colocated CSS modules. Formatting is defined by `.oxfmtrc.json`: four spaces for code, single quotes, no semicolons or trailing commas, and import sorting. Oxlint also enforces React hook/compiler rules and rejects explicit `any`.
- Translations live in `public/locales`; `pnpm run toc` regenerates `@types/resources.ts` from English resources. Existing Crowdin scripts target the `dev` branch; the download script explicitly selects Afrikaans.

## Tests and fixtures

Backend tests live beside their packages. `internal/server` includes route parity, response probes, mutation probes, and review/security behavior tests. Use `testdb.Open(t)` for migrated isolated databases and `server.NewApp`/HTTP test helpers for router behavior.

Subscription parity tests compare output against committed upstream fixtures under `internal/subtpl/testdata` and `internal/subscription`. The scripts in `scripts/subscription-diff` explain how to generate matrix, golden, crypto, YAML, and JavaScript vectors. Ordinary Go tests use committed fixtures without requiring upstream Node code.

Fixture regeneration has a separate runtime requirement: `setup-upstream.sh <work-dir>` extracts a pinned upstream commit from `UPSTREAM_REPO` (default `~/remnawave-backend`) and downloads Node **24.21.0**. Its comment notes Node 26 breaks the upstream x25519 helper. Rendering pins time and randomness; preserve `TZ=UTC` and the documented generator invocation when refreshing golden outputs.

## Runtime and deployment gotchas

- Go configuration loads `.env` and `../.env` relative to the working directory. `APP_SECRET` must be private and at least 32 characters; startup rejects known placeholder values. Optional notification settings also have validation gates. Consult `backend-go/config/config.go` rather than assuming every variable in `.env.sample` is consumed.
- Defaults are API port 3000 and metrics port 3001. PostgreSQL is inferred from a PostgreSQL DSN when `DB_DRIVER` is absent; otherwise SQLite is the default.
- The frontend is **not embedded in the Go binary**. `NewApp` searches, in order, `./frontend/dist`, `./dist`, `../frontend/dist`, `./static`, `/opt/gowave/frontend/dist`, and `/opt/gowave/dist`. The first existing directory wins at construction time.
- Static serving has an `/assets/` fallback to files in the static root for editor WASM/runtime/schema URLs. Missing file-like requests return 404; extensionless SPA routes fall back to `index.html`. Keep this behavior when modifying asset paths or router fallback.
- Running the executable as root attempts to create/update `/usr/local/bin/gowave`. `scripts/migrate-node.sh` is a privileged systemd installer for a separate Remnanode binary and can stop a Docker node; it is not a local development setup script.
- `.github/workflows/release.yml` triggers on `v*` tags and publishes separate frontend tarballs and Linux amd64/arm64 backend binaries. Version metadata is injected with `-X gowave/internal/system.AppVersion=${VERSION}`. The release first runs `ci.yml` (via `workflow_call`), verifies the embedded version with `--version`, publishes `SHA256SUMS`, and pushes a multi-arch image to `ghcr.io/<owner>/gowave`; tags containing `-` become pre-releases. The frontend Makefile's `tag-release` creates `v<package.json version>`.
