import assert from 'node:assert/strict'
import { test } from 'node:test'
import { GetHwidDevicesCommand, GetTorrentBlockerReportsCommand } from '@remnawave/backend-contract'
import { createUrl } from '@shared/api/helpers/create-url'

import { devicePlatforms } from './device-platforms'
import { validEntityId } from './quick-open'
import {
    historyStorage,
    hwidStorage,
    inspectorRequest,
    readInspectorState,
    torrentStorage
} from './table-state'

test('inspectors use saved layouts and pass only supported filters to Go', () => {
    const state = readInspectorState(hwidStorage, {
        getItem: (name) => {
            assert.equal(name, 'x-rmnw-hwid-inspector-table')
            return JSON.stringify({
                version: 2,
                state: {
                    columnFilter: [
                        { id: 'userId', value: 17 },
                        { id: 'platform', value: ' Android ' },
                        { id: 'deviceModel', value: 'test' }
                    ],
                    sorting: [{ id: 'hwid', desc: false }],
                    paginationState: { pageIndex: 3, pageSize: 10 },
                    columnVisibility: { userAgent: false },
                    columnOrder: ['hwid', 'userId']
                }
            })
        }
    })
    const query = GetHwidDevicesCommand.RequestQuerySchema.parse(
        inspectorRequest(state, new Set(['userId', 'platform']), new Set(['hwid']))
    )
    const request = new URL(createUrl(GetHwidDevicesCommand.TSQ_url, query), 'http://localhost')
    assert.equal(request.searchParams.get('start'), '30')
    assert.equal(request.searchParams.get('size'), '10')
    assert.deepEqual(JSON.parse(request.searchParams.get('filters') ?? ''), [
        { id: 'userId', value: '17' },
        { id: 'platform', value: 'Android' }
    ])
    assert.deepEqual(state.columnVisibility, { userAgent: false })
    assert.deepEqual(state.columnOrder, ['hwid', 'userId'])
})

test('obsolete or unreadable inspector storage falls back to a usable page', () => {
    const state = readInspectorState(historyStorage, {
        getItem: () => '{"version":1,"state":{"sorting":[]}}'
    })
    assert.deepEqual(state.sorting, [{ id: 'id', desc: true }])
    assert.deepEqual(state.paginationState, { pageIndex: 0, pageSize: 25 })
})

test('device platform previews keep frequent apps and preserve all apps for inspection', () => {
    const [platform, ...remaining] = devicePlatforms([
        { platform: 'Unused', count: 0, byApp: [] },
        {
            platform: '',
            count: 200,
            byApp: [
                { app: '', count: 191 },
                { app: 'Rare', count: 1 },
                { app: 'At threshold', count: 2 }
            ]
        },
        ...Array.from({ length: 9 }, (_, index) => ({
            platform: `Platform ${index}`,
            count: 1,
            byApp: []
        }))
    ])

    assert.equal(remaining.length, 7)
    assert.equal(platform?.name, 'Unknown')
    assert.deepEqual(
        platform?.preview.map(({ name }) => name),
        ['Unknown', 'At threshold']
    )
    assert.deepEqual(
        platform?.allApps.map(({ name }) => name),
        ['Unknown', 'Rare', 'At threshold']
    )
})

test('Quick Open accepts stable IDs and rejects path and number coercion', () => {
    assert.equal(validEntityId('user', '42'), true)
    assert.equal(validEntityId('user', '3e2'), false)
    assert.equal(validEntityId('user', '9007199254740993'), false)
    assert.equal(validEntityId('config-profile', 'bfbb53cf-c55e-482c-ae53-a7b33eec4ebc'), true)
    assert.equal(validEntityId('host', '../user/2'), false)
})

test('torrent table restores React layout and sends exact and partial matches to Go', () => {
    const state = readInspectorState(torrentStorage, {
        getItem: (key) => {
            assert.equal(key, 'x-rmnw-tb-reports-table')
            return JSON.stringify({
                version: 3,
                state: {
                    columnFilter: [
                        { id: 'user.username', value: ' user ' },
                        { id: 'report.actionReport.ip', value: '192.0.2' },
                        { id: 'report.xrayReport.protocol', value: 'bittorrent' }
                    ],
                    columnOrder: ['node.uuid', 'user.username'],
                    columnVisibility: { 'report.xrayReport.protocol': false },
                    paginationState: { pageIndex: 1, pageSize: 10 },
                    sorting: [{ id: 'createdAt', desc: true }]
                }
            })
        }
    })
    const filters = new Set(['user.username', 'report.actionReport.ip'])
    const params = GetTorrentBlockerReportsCommand.RequestQuerySchema.parse(
        inspectorRequest(
            state,
            filters,
            new Set(['createdAt']),
            new Map([['user.username', 'equals']])
        )
    )
    const url = new URL(
        createUrl(GetTorrentBlockerReportsCommand.TSQ_url, params),
        'http://localhost'
    )
    assert.equal(url.searchParams.get('start'), '10')
    assert.deepEqual(JSON.parse(url.searchParams.get('filters') ?? ''), [
        { id: 'user.username', value: 'user' },
        { id: 'report.actionReport.ip', value: '192.0.2' }
    ])
    assert.deepEqual(JSON.parse(url.searchParams.get('filterModes') ?? ''), {
        'user.username': 'equals',
        'report.actionReport.ip': 'contains'
    })
    assert.deepEqual(state.columnOrder, ['node.uuid', 'user.username'])
    assert.equal(state.columnVisibility['report.xrayReport.protocol'], false)
})
