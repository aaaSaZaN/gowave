import assert from 'node:assert/strict'
import { test } from 'node:test'
import type { IPanelUpdateStatus, IPanelUpdateStatusResponse } from '@shared/api/panel-update'

import { deriveUpdateView, type PanelUpdateRequest, type PollState } from './panel-update-progress'

const request: PanelUpdateRequest = {
    channel: 'dev',
    requestedAt: '2026-03-01T12:00:00Z',
    fromVersion: 'v1.1.0'
}
const start = Date.parse(request.requestedAt)
const status: IPanelUpdateStatus = {
    channel: 'dev',
    startedAt: request.requestedAt,
    updatedAt: request.requestedAt,
    fromVersion: 'v1.1.0',
    state: 'running',
    step: 'download',
    message: 'Downloading',
    progress: 20,
    log: []
}
const response: IPanelUpdateStatusResponse = {
    channel: 'stable',
    currentVersion: request.fromVersion,
    helperInstalled: true,
    requestPending: false,
    serverTime: request.requestedAt,
    status
}
const poll: PollState = { response, now: start, lastError: null, unreachableSince: null }

test('tracks the current update during a channel switch and ignores unrelated helper reports', () => {
    assert.deepEqual(deriveUpdateView(request, poll, start), { kind: 'running', status })

    for (const unrelated of [
        { ...status, state: 'failed', channel: 'stable' } as const,
        { ...status, state: 'failed', startedAt: '2026-03-01T11:00:00Z' } as const
    ]) {
        const next = { ...poll, response: { ...response, status: unrelated } }
        assert.equal(deriveUpdateView(request, next, start).kind, 'waiting')
    }

    const restarted = {
        ...poll,
        response: { ...response, channel: 'dev', currentVersion: 'dev-123', status: null }
    } as const
    assert.deepEqual(deriveUpdateView(request, restarted, start), {
        kind: 'done',
        version: 'dev-123'
    })
})

test('distinguishes reconnecting, an idle helper, and a response from a legacy helper', () => {
    const waiting = {
        ...poll,
        response: { ...response, status: null, requestPending: true },
        now: start + 31_000
    }
    assert.equal(deriveUpdateView(request, waiting, start).kind, 'helperIdle')
    assert.equal(
        deriveUpdateView(
            request,
            { ...waiting, response: { ...waiting.response, requestPending: false } },
            start
        ).kind,
        'legacy'
    )
    const unreachable = { ...poll, now: start + 60_000, response: null, unreachableSince: start }
    assert.equal(deriveUpdateView(request, unreachable, start).kind, 'restarting')
    assert.equal(
        deriveUpdateView(request, { ...unreachable, now: start + 181_000 }, start).kind,
        'failed'
    )
})
