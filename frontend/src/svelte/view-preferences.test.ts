import assert from 'node:assert/strict'
import { test } from 'node:test'
import { NO_TAG } from '@shared/constants/tag-filter'

import {
    readHostView,
    readLegacyLayout,
    writeHostView,
    writeLegacyLayout
} from './view-preferences'

test('React host preferences migrate while preserving node and layout settings', () => {
    let raw = JSON.stringify({
        version: 1,
        state: {
            layoutStyle: 'sidebar',
            hostsViewMode: 'table',
            nodesViewMode: 'table',
            hostsActiveTag: NO_TAG
        }
    })
    const storage = {
        getItem: () => raw,
        setItem: (name: string, value: string) => {
            assert.equal(name, 'viewPreferencesStore')
            raw = value
        }
    }
    assert.deepEqual(readHostView(storage), { mode: 'table', tag: NO_TAG })
    writeHostView('cards', '', storage)
    assert.deepEqual(JSON.parse(raw), {
        version: 2,
        state: {
            experimental: { legacyLayoutStyle: true },
            nodesViewMode: 'table',
            hostsViewMode: 'cards',
            hostsActiveTag: null
        }
    })
})

test('saving the sidebar drops retired preferences and preserves node filters', () => {
    let raw = JSON.stringify({
        version: 2,
        state: {
            nodesViewMode: 'table',
            nodesActiveTag: 'production',
            quickLinks: [],
            launcherColumns: 3,
            experimental: {
                legacyLayoutStyle: true,
                quickLauncher: true,
                sshTerminal: true,
                nodeIntegrations: true
            }
        }
    })
    const storage = {
        getItem: () => raw,
        setItem: (_key: string, value: string) => {
            raw = value
        }
    }

    assert.equal(readLegacyLayout(storage), true)
    writeLegacyLayout(false, storage)
    assert.equal(readLegacyLayout(storage), false)
    assert.deepEqual(JSON.parse(raw), {
        version: 2,
        state: {
            experimental: { legacyLayoutStyle: false },
            nodesViewMode: 'table',
            nodesActiveTag: 'production'
        }
    })
})
