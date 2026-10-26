export function encode(bytes: Uint8Array): string {
    // Chunking avoids the argument limit for large templates and vault backups.
    let binary = ''
    for (let offset = 0; offset < bytes.length; offset += 8192) {
        binary += String.fromCharCode(...bytes.subarray(offset, offset + 8192))
    }
    return btoa(binary)
}

export function decode(value: string): Uint8Array {
    if (value.length % 4 !== 0 || !/^[A-Za-z0-9+/]*={0,2}$/.test(value)) {
        throw new Error('Invalid base64')
    }
    return Uint8Array.from(atob(value), (char) => char.charCodeAt(0))
}

export const encodeURLSafe = (bytes: Uint8Array): string =>
    encode(bytes).replace(/\+/g, '-').replace(/\//g, '_')

export function decodeURLSafe(value: string): Uint8Array {
    if (!/^[A-Za-z0-9_-]*={0,2}$/.test(value)) throw new Error('Invalid base64url')
    return decode(value.replace(/-/g, '+').replace(/_/g, '/'))
}
