# GoWave frontend

React frontend for [GoWave](https://github.com/aaaSaZaN/gowave).
See the root README for installation and development instructions.
Upstream copyright notices are preserved in [LICENCE](LICENCE).

Dependencies are pinned to exact versions in `package.json`; `bun.lock` locks the
transitive graph. Install with Bun 1.4.2 using `bun install --frozen-lockfile`.
Keep both files together when deliberately updating a dependency.
The reduction removes 49 direct dependencies (122 to 73) and 157 locked package
entries (677 to 520).

The dependency reduction replaces utility packages, query-key construction,
locale loading/detection, table path access and sorting, progress, error handling,
country flag rendering, list animation, masonry layouts, chart lifecycle management
and sparklines with local implementations. Virtual lists consume immutable scroll
and row-size snapshots to support React memoization. Icons and motion use a single existing
library each, and X25519 uses the existing Noble implementation. Unused packages
and obsolete toolchain dependencies were removed.

The remaining packages support the React/Mantine application, contract validation,
large tables and virtualization, editors, terminal emulation, Markdown parsing,
SVG sanitization, authentication/cryptography, specialized exports and the build,
lint and translation workflows. These retain their existing locked resolutions.
Color hashes now use a local deterministic palette; generated UI colors may differ
from earlier builds. API and vault data formats are preserved.

Run frontend checks from this directory:

```sh
bun install --frozen-lockfile
bun run typecheck
bun run test
bun run test:browser # requires Chromium; set CHROMIUM_BIN for a different path
bun run check
bun run build
```

The browser suite builds an isolated test page and uses Chromium's debugging
protocol through native WebSocket, without a browser automation dependency. It
covers layout, animation cleanup, error recovery, SVG detection, charts, progress,
flags, locale loading, virtual scrolling and form resets. Unit tests also check
Base64 compatibility, vault blobs, native X25519 key import, query invalidation
keys, sorting, virtual row layouts, node metadata and release comparison.
