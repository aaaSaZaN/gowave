import assert from 'node:assert/strict'
import { test } from 'node:test'
import { UpdateHostCommand, UpdateManyHostsCommand } from '@remnawave/backend-contract'

import { type HostField, hostPatch } from './host-fields'

const uuid = 'bfbb53cf-c55e-482c-ae53-a7b33eec4ebc'

test('bulk host patches retain explicit false, zero, null and empty selections', () => {
    const controls = new FormData()
    for (const field of ['isHidden', 'vlessRouteId', 'sni', 'nodes', 'tags', 'mapper']) {
        controls.set(`apply:${field}`, 'on')
    }
    controls.set('vlessRouteId', '0')
    controls.set('remark', 'A different name')
    controls.set('host', 'unselected.example')
    const patch = hostPatch(controls, (key) => controls.has(`apply:${key}`))

    assert.deepEqual(UpdateManyHostsCommand.RequestBodySchema.parse({ uuids: [uuid], ...patch }), {
        uuids: [uuid],
        isHidden: false,
        vlessRouteId: 0,
        sni: null,
        nodes: [],
        tags: [],
        mapper: {}
    })
})

test('host edits send only changed fields and leave legacy metadata and dangling inbounds intact', () => {
    const controls = new FormData()
    controls.set('remark', '  New name  ')
    controls.set('port', '443')
    controls.set('inbound', '')
    controls.append('includeInSrrRules', 'First Rule')
    controls.append('includeInSrrRules', 'Case Sensitive, with comma')
    controls.append('includeInSrrRules', 'Second Rule')
    controls.set('tags', 'tag:1, TAG:1 two')
    controls.set('muxParams', '{"enabled":false}')
    controls.set('finalMask', 'bad stored JSON')
    const changed = new Set(['remark', 'includeInSrrRules', 'tags', 'muxParams'])
    const patch = hostPatch(controls, (key) => changed.has(key))

    assert.deepEqual(UpdateHostCommand.RequestBodySchema.parse({ uuid, ...patch }), {
        uuid,
        remark: 'New name',
        includeInSrrRules: ['First Rule', 'Case Sensitive, with comma', 'Second Rule'],
        tags: ['TAG:1', 'TWO'],
        muxParams: { enabled: false }
    })
})

test('removed host options serialize to their contract defaults', () => {
    const controls = new FormData()
    const removed = new Set<HostField>([
        'sni',
        'securityLayer',
        'internalSquads',
        'keepSniBlank',
        'mihomoIpVersion',
        'mapper',
        'muxParams',
        'includeInSrrRules'
    ])
    const patch = hostPatch(controls, (key) => removed.has(key))

    assert.deepEqual(UpdateHostCommand.RequestBodySchema.parse({ uuid, ...patch }), {
        uuid,
        sni: null,
        securityLayer: 'DEFAULT',
        internalSquads: { mode: 'EXCLUDE', squads: [] },
        keepSniBlank: false,
        mihomoIpVersion: null,
        mapper: {},
        muxParams: null,
        includeInSrrRules: []
    })
})

test('host input rejects invalid JSON and out-of-range ports before a request', () => {
    const controls = new FormData()
    controls.set('mapper', '{')
    controls.set('port', '65536')
    assert.throws(() => hostPatch(controls, (key) => key === 'mapper'), /Invalid JSON in mapper/)
    assert.throws(() => hostPatch(controls, (key) => key === 'port'), /Port must be between/)
})
