# GoWave frontend

React frontend for [GoWave](https://github.com/aaaSaZaN/gowave).
See the root README for installation and development instructions.
Upstream copyright notices are preserved in [LICENCE](LICENCE).

Dependencies are pinned to exact versions in `package.json`; `bun.lock` locks the
transitive graph. Install with Bun 1.4.2 using `bun install --frozen-lockfile`.
Keep both files together when deliberately updating a dependency.
The initial dependency reduction removed 49 direct dependencies (122 to 73).
Biome replaces the former Oxfmt/Oxlint toolchain and its React hook plugin.

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

Biome 2.6.3 handles linting, formatting and import organization through
`biome.json`. `bun run check` fails on warnings and runs in CI. Use `bun run fix`
for safe fixes, or `format:fix` and `lint:fix` separately. Existing exclusions for
Persian, Russian and Chinese translation files remain in place. Generated editor
assets and build output are excluded. YAML and Markdown retain two-space indentation; the experimental HTML formatter
is left disabled.

`tsconfig.base.json` shares strict checks between application code and Vite:
unchecked indexed access, exact optional properties, override/return/unused checks,
index-signature bracket access, side-effect import resolution and type-only imports.
Biome also checks React hooks/compiler rules, exhaustive switches, unhandled or
misused promises, unsafe returns, implicit/explicit `any` and non-null assertions.
Optional JSX properties are omitted when undefined through `optionalProp`, rather
than relaxing exact optional-property checking at third-party component boundaries.

`skipLibCheck` remains enabled because the pinned dependencies have incompatible
declarations, including legacy React JSX references, missing optional peer types,
and Highcharts optional-property conflicts. Application code, tests and Vite
configuration still undergo strict checking. `useLiteralKeys` is disabled because
it conflicts with `noPropertyAccessFromIndexSignature`. Local lint exceptions explain
JSON Schema's standard `then` keyword, fixed positional UI slots and necessary
overrides of third-party component styling.

CSS modules have exact generated `*.module.d.css.ts` declarations. After changing
class names, run `bun run css:types` and commit the declarations alongside the CSS.
`bun run typecheck` first checks for stale or orphaned declarations, then runs both
TypeScript configurations. The generator uses the existing PostCSS dependency.
