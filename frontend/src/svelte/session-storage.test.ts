import assert from 'node:assert/strict'
import { test } from 'node:test'

import { readSessionToken, SESSION_STORAGE_KEY, writeSessionToken } from './session-storage'

test('session tokens can be carried from React to Svelte and back', () => {
    const data = new Map([[SESSION_STORAGE_KEY, '{"state":{"token":"existing"},"version":0}']])
    const storage = {
        getItem: (key: string) => data.get(key) ?? null,
        setItem: (key: string, value: string) => {
            data.set(key, value)
        }
    }

    assert.equal(readSessionToken(storage), 'existing')
    writeSessionToken(storage, 'replacement')
    assert.equal(data.get(SESSION_STORAGE_KEY), '{"state":{"token":"replacement"},"version":0}')
    writeSessionToken(storage, '')
    assert.equal(readSessionToken(storage), '')
})

test('unavailable or malformed session storage leaves the user signed out', () => {
    for (const raw of [null, '{', '42', '{"state":null}', '{"state":{"token":false}}']) {
        assert.equal(readSessionToken({ getItem: () => raw }), '')
    }
    assert.equal(
        readSessionToken({
            getItem() {
                throw new Error('Storage is disabled')
            }
        }),
        ''
    )
})
