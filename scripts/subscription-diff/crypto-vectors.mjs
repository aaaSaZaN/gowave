// Regenerates backend-go/internal/subtpl/testdata/crypto-vectors.json with the
// upstream helpers (node:crypto + @noble/post-quantum):
//   UPSTREAM_DIR=... node scripts/subscription-diff/crypto-vectors.mjs > backend-go/internal/subtpl/testdata/crypto-vectors.json
import { randomBytes } from 'node:crypto';
import { loadUpstream } from './load-upstream.mjs';

const dir = process.env.UPSTREAM_DIR;
const jiti = loadUpstream(dir);
const keys = await jiti.import(dir + '/src/common/helpers/xray-config/resolve-public-key.ts');
const enc = await jiti.import(dir + '/src/common/helpers/vless-encryption/generate-encryption-from-decryption.ts');

const out = { x25519: [], mldsa65: [], encryption: [] };
for (let i = 0; i < 4; i++) {
    const priv = randomBytes(32).toString(i % 2 ? 'base64url' : 'base64');
    const inbound = { tag: 't', streamSettings: { realitySettings: { privateKey: priv } } };
    const map = await keys.resolveInboundAndPublicKey([inbound]);
    out.x25519.push({ in: priv, out: map.get('t') ?? null });
    const seed = randomBytes(32).toString('base64');
    out.mldsa65.push({ in: seed, out: keys.getMlDsa65PublicKey(seed) });
}
const x = () => randomBytes(32).toString('base64url');
const m = () => randomBytes(64).toString('base64url');
for (const d of [
    `mlkem768x25519plus.native.600s.${x()}`,
    `mlkem768x25519plus.xorpub.0s.${m()}`,
    `mlkem768x25519plus.random.300-600s.100-111-1111.75-0-111.50-0-3333.${x()}.${m()}`,
    `mlkem768x25519plus.bogus.600s.${x()}`,
]) {
    let result = null;
    try { result = (await enc.generateEncryptionFromDecryption(d)).encryption; } catch {}
    out.encryption.push({ in: d, out: result });
}
console.log(JSON.stringify(out, null, 1));
