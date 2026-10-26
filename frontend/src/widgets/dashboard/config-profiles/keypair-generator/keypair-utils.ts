import { x25519 } from '@noble/curves/ed25519.js'
import { ml_dsa65 } from '@noble/post-quantum/ml-dsa.js'
import { ml_kem768 } from '@noble/post-quantum/ml-kem.js'
/* eslint-disable camelcase */
import { randomBytes } from '@noble/post-quantum/utils.js'

import { encodeURLSafe } from '@shared/lib/base64'

export const generateX25519 = () => {
    const secretKey = x25519.utils.randomSecretKey()
    const kp = { secretKey, publicKey: x25519.getPublicKey(secretKey) }
    return {
        privateKey: encodeURLSafe(kp.secretKey).replace(/=/g, '').replace(/\n/g, ''),
        password: encodeURLSafe(kp.publicKey).replace(/=/g, '').replace(/\n/g, '')
    }
}

export const generateMlDsa65 = () => {
    const seed = randomBytes(32)
    const kp = ml_dsa65.keygen(seed)
    return {
        mldsa65Verify: encodeURLSafe(kp.publicKey).replace(/=/g, '').replace(/\n/g, ''),
        mldsa65Seed: encodeURLSafe(seed).replace(/=/g, '').replace(/\n/g, '')
    }
}

export const generateMlKem768 = () => {
    const seed = randomBytes(64)
    const kp = ml_kem768.keygen(seed)
    return {
        mlkem768PublicKey: encodeURLSafe(kp.publicKey).replace(/=/g, '').replace(/\n/g, ''),
        mlkem768Seed: encodeURLSafe(seed).replace(/=/g, '').replace(/\n/g, '')
    }
}
