// Loads upstream TypeScript modules (decorators + tsconfig path
// aliases) through jiti from UPSTREAM_DIR, a checkout of the upstream backend
// at the pinned commit with `npm ci` done.
import { createRequire } from 'node:module';
import path from 'node:path';

export function loadUpstream(upstreamDir) {
    const require = createRequire(path.join(upstreamDir, 'package.json'));
    const { createJiti } = require('jiti');
    const src = (p) => path.join(upstreamDir, p);
    return createJiti(path.join(upstreamDir, 'index.js'), {
        interopDefault: true,
        moduleCache: true,
        alias: {
            '@common': src('src/common'),
            '@integration-modules': src('src/integration-modules'),
            '@modules': src('src/modules'),
            '@libs/contracts': src('libs/contract'),
            '@libs/node-plugins': src('libs/node-plugins'),
            '@libs/subscription-page': src('libs/subscription-page'),
            '@contract': src('libs/contract'),
            '@queue': src('src/queue'),
            '@scheduler': src('src/scheduler'),
        },
    });
}
