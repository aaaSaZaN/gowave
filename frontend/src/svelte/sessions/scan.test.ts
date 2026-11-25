import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
    aggregateSessions,
    type ScanProgress,
    type SessionNode,
    scanSessions,
    sessionActivityCounts
} from './scan'

function node(index: number): SessionNode {
    return { uuid: String(index), name: `Node ${index}`, countryCode: 'XX' }
}

const seen = (ip: string) => ({ ip, lastSeen: new Date('2026-03-04T12:00:00Z') })

test('classifies session activity at the five- and sixty-minute boundaries', () => {
    const lastSeen = new Date('2026-03-04T12:00:00Z')
    const ips = [{ ip: '192.0.2.1', lastSeen }]
    assert.deepEqual(sessionActivityCounts(ips, lastSeen.getTime() + 5 * 60_000), {
        recent: 1,
        idle: 0,
        stale: 0
    })
    assert.deepEqual(sessionActivityCounts(ips, lastSeen.getTime() + 60 * 60_000), {
        recent: 0,
        idle: 1,
        stale: 0
    })
    assert.deepEqual(sessionActivityCounts(ips, lastSeen.getTime() + 60 * 60_000 + 1), {
        recent: 0,
        idle: 0,
        stale: 1
    })
})

test('keeps every connection while counting distinct IPs across nodes and users', () => {
    const alpha = node(1)
    const beta = node(2)
    const first = seen('192.0.2.1')
    const second = seen('2001:db8::2')
    const result = aggregateSessions([
        {
            node: alpha,
            users: [
                { userId: 2, ips: [first] },
                { userId: 1, ips: [first] },
                { userId: 1, ips: [second] }
            ]
        },
        { node: beta, users: [{ userId: 1, ips: [first] }] }
    ])
    assert.equal(result.totalIps, 4)
    assert.equal(result.uniqueIps, 2)
    assert.deepEqual(result.users, [
        {
            userId: 1,
            nodes: [
                { node: alpha, ips: [first, second] },
                { node: beta, ips: [first] }
            ],
            totalIps: 3,
            uniqueIps: 2
        },
        { userId: 2, nodes: [{ node: alpha, ips: [first] }], totalIps: 1, uniqueIps: 1 }
    ])
})

test('limits concurrent requests and preserves successful results when one node fails', async () => {
    const nodes = Array.from({ length: 14 }, (_, index) => node(index))
    const progress: ScanProgress[] = []
    let active = 0
    let peak = 0
    const result = await scanSessions(
        nodes,
        async ({ uuid }) => {
            active++
            peak = Math.max(peak, active)
            await Promise.resolve()
            active--
            if (uuid === '1') throw new Error('Offline')
            return [{ userId: 1, ips: [seen('192.0.2.1')] }]
        },
        (update) => progress.push(update),
        new AbortController().signal
    )
    assert.equal(peak, 10)
    assert.equal(result.completed, 13)
    assert.deepEqual([...result.errors.entries()], [[nodes[1], 'Offline']])
    assert.equal(result.users[0]?.totalIps, 13)
    assert.deepEqual(progress.at(-1), { active: [], completed: 13, failed: 1, total: 14 })
})

test('cancellation stops scheduling the queued nodes and suppresses late progress', async () => {
    const controller = new AbortController()
    const nodes = Array.from({ length: 12 }, (_, index) => node(index))
    let started = 0
    let reported = 0

    const pending = scanSessions(
        nodes,
        (_node, signal) =>
            new Promise((_resolve, reject) => {
                started++
                signal.addEventListener('abort', () => reject(signal.reason), { once: true })
            }),
        () => {
            reported++
        },
        controller.signal
    )
    controller.abort()
    await assert.rejects(pending, { name: 'AbortError' })
    assert.equal(started, 10)
    assert.equal(reported, 10)
})
