import assert from 'node:assert/strict'
import { test } from 'node:test'

import { executorCommand, readPluginConfig, readSharedListConfig } from './config'

test('retains unknown plugin configuration keys and requires an object', () => {
    assert.deepEqual(readPluginConfig('{"existing":{"enabled":false},"version":1}'), {
        existing: { enabled: false },
        version: 1
    })
    assert.throws(() => readPluginConfig('[]'))
    assert.throws(() => readPluginConfig('{"unfinished":'))
})

test('validates IP and ASN shared lists', () => {
    assert.deepEqual(
        readSharedListConfig('{"type":"ipList","items":["192.0.2.0/24", "2001:db8::1"]}'),
        { type: 'ipList', items: ['192.0.2.0/24', '2001:db8::1'] }
    )
    assert.deepEqual(readSharedListConfig('{"type":"asList","items":[64496]}'), {
        type: 'asList',
        items: [64496]
    })
    assert.throws(() => readSharedListConfig('{"type":"ipList","items":["bad address"]}'))
    assert.throws(() => readSharedListConfig('{"type":"asList","items":[-1]}'))
})

test('builds contract-valid executor commands and rejects invalid selections', () => {
    assert.deepEqual(executorCommand('blockIps', ' 192.0.2.1;60\n\n2001:db8::1 '), {
        command: 'blockIps',
        ips: [
            { ip: '192.0.2.1', timeout: 60 },
            { ip: '2001:db8::1', timeout: 0 }
        ]
    })
    assert.deepEqual(executorCommand('unblockIps', ' 192.0.2.1\n'), {
        command: 'unblockIps',
        ips: ['192.0.2.1']
    })
    assert.deepEqual(executorCommand('recreateTables', ''), { command: 'recreateTables' })
    assert.throws(() => executorCommand('blockIps', '192.0.2.1;-1'))
    assert.throws(() => executorCommand('blockIps', '192.0.2.1;60;120'))
    assert.throws(() => executorCommand('unblockIps', 'example.com'))
    assert.throws(() => executorCommand('unblockIps', ''))
})
