import assert from 'node:assert/strict'
import { test } from 'node:test'

import { connectionKeyLabel, qrImage } from './delivery'

test('keeps the entire key fragment and tolerates malformed percent escapes', () => {
    assert.equal(
        connectionKeyLabel('vless://key#Primary%20%E2%80%93%20Edge', 'Host'),
        'Primary – Edge'
    )
    assert.equal(connectionKeyLabel('vless://key#Broken%G0#tag', 'Host'), 'Broken%G0#tag')
    assert.equal(connectionKeyLabel('vless://key', 'Host'), 'Host')
    assert.equal(connectionKeyLabel('vless://key#', 'Host'), 'Host')
})

test('renders customizable local QR images and incorporates the complete URL', () => {
    const first = qrImage('https://example.net/sub/alpha#first', '#000000', '#ffffff')
    const second = qrImage('https://example.net/sub/alpha#second', '#000000', '#ffffff')
    assert.ok(first.startsWith('data:image/svg+xml,'))
    assert.notEqual(first, second)
    const recolored = decodeURIComponent(
        qrImage('https://example.net/sub/alpha#first', '#112233', '#ddeeff')
    )
    assert.match(recolored, /<svg/)
    assert.match(recolored, /#112233/)
    assert.match(recolored, /#ddeeff/)
})
