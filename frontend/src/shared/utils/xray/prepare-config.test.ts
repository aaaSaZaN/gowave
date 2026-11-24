import assert from 'node:assert/strict'
import { test } from 'node:test'

import { prepareXrayConfig } from './prepare-config'

test('validator expands Xray snippets without replacing explicitly configured or protected sections', () => {
    const snippets = new Map<string, unknown>([
        ['root', { log: { loglevel: 'error' }, inbounds: ['protected'], dns: { servers: [] } }],
        ['out', [{ tag: 'a' }, { tag: 'b' }]],
        ['rule', { outboundTag: 'a' }]
    ])
    const result = prepareXrayConfig(
        JSON.stringify({
            snippets: ['root'],
            log: { loglevel: 'info' },
            outbounds: [{ snippet: 'out' }, { tag: 'c' }],
            routing: { rules: [{ snippet: 'rule' }] }
        }),
        snippets
    )

    assert.ok('config' in result)
    assert.deepEqual(JSON.parse(result.config), {
        log: { loglevel: 'info' },
        dns: { servers: [] },
        outbounds: [{ tag: 'a' }, { tag: 'b' }, { tag: 'c' }],
        routing: { rules: [{ outboundTag: 'a' }] }
    })
})

test('validator rejects malformed JSON and non-object configuration drafts', () => {
    assert.deepEqual(prepareXrayConfig('{', new Map()), { error: 'Invalid JSON.' })
    assert.deepEqual(prepareXrayConfig('[]', new Map()), { error: 'Config must be a JSON object.' })
})
