import assert from 'node:assert/strict'
import { test } from 'node:test'
import { USERS_TABLE_STORAGE_KEY } from '@entities/dashboard/users/users-table-store/defaults'
import { USERS_TEMPLATES_STORAGE_KEY } from '@entities/dashboard/users/users-table-templates-store/storage-key'
import { createUrl } from '@shared/api/helpers/create-url'

import { togglePageSelection, userSelection } from './selection.svelte'
import {
    applyUserTable,
    captureUserTable,
    readUserTable,
    readUserTemplates,
    usersRequest,
    writeUserTable,
    writeUserTemplates
} from './table-state'

test('React users table state supplies server paging, sort, and structured filters', () => {
    const saved = {
        columnFilter: [
            { id: 'username', value: 'name with spaces' },
            { id: 'status', value: ['ACTIVE', 'LIMITED'] },
            { id: 'trafficLimitBytes', value: [0, null] },
            { id: 'email', value: '' }
        ],
        columnPinning: { left: ['username'], right: ['tag'] },
        columnOrder: ['tag', 'username'],
        columnSize: { username: 170 },
        columnVisibility: { description: false },
        paginationState: { pageIndex: 2, pageSize: 100 },
        showColumnFilters: false,
        sorting: [{ id: 'username', desc: true }]
    }
    const stored = JSON.stringify({ version: 13, state: saved })
    const table = readUserTable({ getItem: () => stored })
    assert.deepEqual(table, saved)

    const query = usersRequest(table, { trafficLimitBytes: 'between' })
    const url = new URL(createUrl('/api/users/', query), 'http://localhost')
    assert.equal(url.searchParams.get('start'), '200')
    assert.equal(url.searchParams.get('size'), '100')
    assert.equal(url.searchParams.get('sorting'), JSON.stringify(saved.sorting))
    assert.equal(url.searchParams.get('filters'), JSON.stringify(saved.columnFilter.slice(0, 3)))

    writeUserTable(
        {
            setItem: (name, raw) => {
                assert.equal(name, USERS_TABLE_STORAGE_KEY)
                assert.deepEqual(JSON.parse(raw), { version: 13, state: saved })
            }
        },
        table
    )
})

test('applying a saved view restores its filters, columns, and pinning and resets the current page', () => {
    const table = readUserTable()
    table.columnFilter = [{ id: 'tag', value: ['ALPHA'] }]
    table.columnPinning = { left: ['username'], right: [] }
    table.columnOrder = ['username', 'tag']
    table.sorting = [{ id: 'tag', desc: false }]
    const snapshot = captureUserTable(table, { tag: 'equals' })
    const saved = [{ id: 'view-1', name: 'Alpha tags', color: '#0af', snapshot }]
    let stored = ''
    writeUserTemplates(
        {
            setItem: (name, raw) => {
                assert.equal(name, USERS_TEMPLATES_STORAGE_KEY)
                stored = raw
            }
        },
        saved
    )
    assert.deepEqual(JSON.parse(stored), { version: 1, state: { templates: saved } })
    assert.deepEqual(readUserTemplates({ getItem: () => stored }), saved)

    const layout = applyUserTable(
        { ...table, paginationState: { pageIndex: 12, pageSize: 500 } },
        snapshot
    )
    assert.deepEqual(layout.paginationState, { pageIndex: 0, pageSize: 500 })
    assert.deepEqual(layout.columnFilter, [{ id: 'tag', value: ['ALPHA'] }])
    assert.deepEqual(layout.columnPinning, { left: ['username'], right: [] })
    assert.deepEqual(layout.sorting, [{ id: 'tag', desc: false }])
})

test('obsolete and malformed table envelopes fall back to usable defaults', () => {
    for (const raw of [
        'invalid',
        '{"version":13,"state":9}',
        '{"version":12,"state":{"paginationState":{"pageIndex":3,"pageSize":100}}}'
    ]) {
        assert.deepEqual(readUserTable({ getItem: () => raw }).paginationState, {
            pageIndex: 0,
            pageSize: 25
        })
    }
})

test('toggling a page preserves rows selected on other pages', () => {
    userSelection.add(1)
    userSelection.add(3)
    try {
        const page = [{ id: 3 }, { id: 4 }]
        togglePageSelection(page)
        assert.deepEqual([...userSelection], [1, 3, 4])
        togglePageSelection(page)
        assert.deepEqual([...userSelection], [1])
        togglePageSelection([])
        assert.deepEqual([...userSelection], [1])
    } finally {
        userSelection.clear()
    }
})
