import assert from 'node:assert/strict'
import { test } from 'node:test'

import { isCustomXrayVersion, isGoNodeVersion } from '../utils/misc/node-versions'
import { virtualLayout } from './virtual-layout'

test('virtual rows cover the viewport with bounded overscan', () => {
    const options = { count: 1000, estimateSize: () => 40, overscan: 2 }
    const layout = virtualLayout(options, { offset: 400, height: 100 }, new Map())
    assert.equal(layout.totalSize, 40000)
    assert.deepEqual(
        layout.virtualItems.map((item) => item.index),
        [8, 9, 10, 11, 12, 13, 14]
    )
    assert.equal(layout.virtualItems[0]?.start, 320)
    assert.equal(layout.virtualItems.at(-1)?.end, 600)
})

test('virtual row sizes follow keys through filtering and reordering', () => {
    const heights = new Map([
        ['a', 75],
        ['b', 120]
    ])
    const keys = ['b', 'a', 'c']
    const layout = virtualLayout(
        {
            count: keys.length,
            estimateSize: () => 40,
            getItemKey: (index) => keys[index] ?? index,
            scrollMargin: 200
        },
        { offset: 200, height: 300 },
        heights
    )
    assert.equal(layout.totalSize, 235)
    assert.deepEqual(
        layout.virtualItems.map(({ key, start, size }) => ({ key, start, size })),
        [
            { key: 'b', start: 200, size: 120 },
            { key: 'a', start: 320, size: 75 },
            { key: 'c', start: 395, size: 40 }
        ]
    )
    assert.equal(heights.size, 2)
})

test('virtual layout handles empty lists, hidden viewports, boundaries and shrinking data', () => {
    const options = { count: 0, estimateSize: () => 60, overscan: 1 }
    assert.deepEqual(virtualLayout(options, { offset: 0, height: 100 }, new Map()), {
        totalSize: 0,
        virtualItems: []
    })
    assert.deepEqual(
        virtualLayout({ ...options, count: 10 }, { offset: 0, height: 0 }, new Map()),
        { totalSize: 600, virtualItems: [] }
    )
    const boundary = virtualLayout(
        { ...options, count: 10, overscan: 0 },
        { offset: 60, height: 60 },
        new Map()
    )
    assert.deepEqual(
        boundary.virtualItems.map((item) => item.index),
        [1]
    )
    const shrunk = virtualLayout({ ...options, count: 2 }, { offset: 1000, height: 200 }, new Map())
    assert.deepEqual(
        shrunk.virtualItems.map((item) => item.index),
        [0, 1]
    )
})

test('node version metadata supports contract versions and optional GoWave flags', () => {
    assert.equal(isGoNodeVersion(null), false)
    assert.equal(isGoNodeVersion({ node: '3.3.0' }), false)
    assert.equal(isGoNodeVersion({ node: '1.0.0 (Go)' }), true)
    assert.equal(isGoNodeVersion({ nodeType: 'go' }), true)
    assert.equal(isCustomXrayVersion(undefined), false)
    assert.equal(isCustomXrayVersion({ xray: '25.10.1' }), false)
    assert.equal(isCustomXrayVersion({ xray: '25.10.1 Custom' }), true)
    assert.equal(isCustomXrayVersion({ isCustom: true }), true)
    assert.equal(isCustomXrayVersion({ isCustomXray: true }), true)
})
