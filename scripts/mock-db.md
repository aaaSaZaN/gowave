# Mock development database

From `frontend/`, run `bun run dev:mock` and open <http://127.0.0.1:3333>.
The initial panel administrator is **root / root**. The backend reports version
and channel `dev`.

The first launch generates `.dev/mock/gowave.db`, a private application secret,
one random VLESS/WebSocket config, a linked host, a squad, and four subscribers
(active, disabled, expired and limited). The example host has no live node.
Subsequent launches reuse the database and keep your edits.

Stop dev and run `bun run dev:mock --recreate` to overwrite the mock database
with fresh fixtures. Other databases and existing `.env` files are untouched.

To generate the database without starting dev, run from the repository root:

```sh
bun scripts/mock-db.mjs
bun scripts/mock-db.mjs --recreate
```

The script requires Bun, Go and installed frontend dependencies when serving.
It runs the backend's noninteractive rescue CLI to apply the real migrations
and seed defaults, then inserts fixtures using Bun's built-in SQLite support.
It does not duplicate the migration runner or require another Go entry point.

The API runs from an isolated directory with its own environment; production
database settings and root/backend `.env` files are not loaded. Vite uses the
local API proxy. Both servers bind to loopback and stop together. A mock lock
prevents recreation while the stand is running. `.dev/` stays outside Git.
