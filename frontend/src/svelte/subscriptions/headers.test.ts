import assert from 'node:assert/strict'
import { test } from 'node:test'

import { removedHeaderNames, responseHeaders } from './headers'

test('response header editing keeps the last value for each case-insensitive name', () => {
    assert.deepEqual(
        responseHeaders([
            { key: ' X-Test ', value: 'overwritten\ninvalid' },
            { key: '', value: 'ignored' },
            { key: 'x-test', value: ' final ' },
            { key: 'x-encoded', value: 'rwEncodeBase64:á\nline' },
            { key: '__proto__', value: '' }
        ]),
        { 'x-test': 'final', 'x-encoded': 'rwEncodeBase64:á\nline', ['__proto__']: '' }
    )
})

test('response header editing rejects invalid names and unencoded multiline values', () => {
    assert.throws(
        () => responseHeaders([{ key: 'not a key', value: 'value' }]),
        /Invalid header name/
    )
    assert.throws(() => responseHeaders([{ key: 'x-test', value: 'a\nb' }]), /rwEncodeBase64:/)
    assert.throws(() => responseHeaders([{ key: 'x-test', value: 'á' }]), /Invalid header value/)
})

test('inherited header removals accept commas and whitespace and collapse duplicates', () => {
    assert.deepEqual(removedHeaderNames(' X-Test, x-other\nX-TEST '), ['x-test', 'x-other'])
    assert.throws(() => removedHeaderNames('x-test:'), /Invalid header name/)
})
