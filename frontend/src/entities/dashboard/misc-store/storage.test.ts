import assert from 'node:assert/strict'
import { test } from 'node:test'

import { MISC_STORAGE_KEY, readMiscPreferences, writeMiscPreferences } from './storage'

test('updating a dashboard preference retains unrelated flags and the Zustand version', () => {
    let raw = JSON.stringify({
        version: 2,
        state: { disclaimerAccepted: true, mobileWarningClosed: true }
    })
    const storage = {
        getItem: () => raw,
        setItem(key: string, value: string) {
            assert.equal(key, MISC_STORAGE_KEY)
            raw = value
        }
    }

    writeMiscPreferences(storage, { srrAdvancedModalClosed: true })
    assert.deepEqual(JSON.parse(raw), {
        version: 3,
        state: {
            disclaimerAccepted: false,
            mobileWarningClosed: true,
            srrAdvancedModalClosed: true
        }
    })
    assert.equal(readMiscPreferences(storage).srrAdvancedModalClosed, true)
})

test('obsolete or malformed values do not skip the Response Rules warning', () => {
    for (const value of [
        null,
        'malformed',
        '{"version":1,"state":{"srrAdvancedModalClosed":true}}'
    ]) {
        assert.equal(readMiscPreferences({ getItem: () => value }).srrAdvancedModalClosed, false)
    }
})
