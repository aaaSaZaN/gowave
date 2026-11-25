import assert from 'node:assert/strict'
import { test } from 'node:test'
import { BulkNodesUpdateCommand } from '@remnawave/backend-contract'
import { UpdateNodeBodySchema } from '@shared/api/schemas/node.schema'

import { readConfigProfile, readNodePatch } from './fields'

const uuid = 'bfbb53cf-c55e-482c-ae53-a7b33eec4ebc'

test('node traffic updates distinguish zero, false, and omitted inputs', () => {
    const form = new FormData()
    const limitBytes = 2.5 * 1024 ** 3
    form.set('trafficLimitBytes', String(limitBytes))
    form.set('notifyPercent', '0')
    form.set('countryCode', 'US')
    form.set('trafficTrackingStart', 'zero')
    const included = new Set([
        'trafficLimitBytes',
        'notifyPercent',
        'isTrafficTrackingActive',
        'trafficTrackingStart'
    ])
    const patch = readNodePatch(form, (field) => included.has(field), 'edit')
    assert.deepEqual(UpdateNodeBodySchema.parse({ uuid, ...patch }), {
        uuid,
        trafficLimitBytes: limitBytes,
        notifyPercent: 0,
        isTrafficTrackingActive: false,
        trafficTrackingStart: 'zero'
    })
})

test('node bulk edits preserve explicit clears and deduplicate tags', () => {
    const form = new FormData()
    form.set('tags', 'alpha, Alpha, beta')
    const included = new Set(['tags', 'note', 'providerUuid'])
    const fields = readNodePatch(form, (field) => included.has(field), 'bulk')
    assert.deepEqual(BulkNodesUpdateCommand.RequestBodySchema.parse({ uuids: [uuid], fields }), {
        uuids: [uuid],
        fields: { tags: ['ALPHA', 'BETA'], note: null, providerUuid: null }
    })
})

test('profile controls submit only checked inbounds', () => {
    const form = new FormData()
    form.set('profile', uuid)
    form.append('inbounds', 'first')
    form.append('inbounds', 'second')
    assert.deepEqual(readConfigProfile(form), {
        activeConfigProfileUuid: uuid,
        activeInbounds: ['first', 'second']
    })
})
