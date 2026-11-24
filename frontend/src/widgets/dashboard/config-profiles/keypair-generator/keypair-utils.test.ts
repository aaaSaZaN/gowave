import assert from 'node:assert/strict'
import { test } from 'node:test'

import { decodeURLSafe } from '@shared/lib/base64'
import { generateX25519 } from './keypair-utils'

test('X25519 generator interoperates with native WebCrypto key import', async () => {
    const pair = generateX25519()
    assert.match(pair.privateKey, /^[\w-]{43}$/)
    assert.match(pair.password, /^[\w-]{43}$/)
    const secret = decodeURLSafe(`${pair.privateKey}=`)
    // RFC 8410 PKCS#8 header for an X25519 private key.
    const header = Uint8Array.from([
        0x30, 0x2e, 0x02, 0x01, 0x00, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x6e, 0x04, 0x22, 0x04,
        0x20
    ])
    const key = await crypto.subtle.importKey(
        'pkcs8',
        new Uint8Array([...header, ...secret]),
        'X25519',
        true,
        ['deriveBits']
    )
    const jwk = await crypto.subtle.exportKey('jwk', key)
    assert.equal(jwk.x, pair.password)
})
