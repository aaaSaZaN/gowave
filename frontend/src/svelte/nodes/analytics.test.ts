import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
    metricsSummary,
    type NodeMetricsRow,
    usageDays,
    usageForDay,
    usageSummary,
    visibleTraffic
} from './analytics'
import { usageChartOptions } from './usage-chart'

test('node history keeps all-node totals separate from filtered chart series', () => {
    const series = [
        { uuid: 'a', name: 'A', countryCode: 'DE', color: '#243151', total: 15, data: [10, 5] },
        { uuid: 'b', name: 'B', countryCode: 'FR', color: '#44515a', total: 50, data: [50] },
        { uuid: 'c', name: 'C', countryCode: 'XX', color: '#333333', total: 0, data: [] }
    ]
    assert.deepEqual(usageSummary([60, 40, 0]), { total: 100, peak: 60, average: 100 / 3 })
    assert.deepEqual(
        usageDays(['2026-09-01', '2026-09-02', '2026-09-03'], series).map(({ total }) => total),
        [60, 5, 0]
    )
    assert.deepEqual(
        usageForDay(series, 0).map(({ name, bytes }) => [name, bytes]),
        [
            ['B', 50],
            ['A', 10]
        ]
    )
    assert.deepEqual(usageForDay(series, 2), [])
})

test('traffic charts keep zero-value dates and isolate query data from the chart', () => {
    const series = [
        { uuid: 'a', name: 'A', countryCode: 'DE', color: '#243151', total: 12, data: [0, 12] }
    ]
    const chart = usageChartOptions({
        categories: ['2026-09-01', '2026-09-02'],
        series,
        language: 'en',
        hint: 'Click to see all',
        user: true,
        selectDay: () => {}
    })
    const axis = chart.xAxis
    assert.ok(axis && !Array.isArray(axis))
    assert.deepEqual(axis.categories, ['2026-09-01', '2026-09-02'])
    const node = chart.series?.[0]
    assert.ok(node && 'type' in node && node.type === 'bar' && 'color' in node && 'data' in node)
    assert.equal(node.color, 'rgba(36, 49, 81, 0.5)')
    assert.deepEqual(node.data, [0, 12])
    assert.notEqual(node.data, series[0]?.data)
    assert.equal(chart.chart?.height, '90%')
})

test('live metrics totals count online nodes and zero filtering keeps either traffic direction', () => {
    const rows = [
        { tag: 'unused', upload: '0 B', download: '0.0 B' },
        { tag: 'upload', upload: '12 KiB', download: '0 B' },
        { tag: 'download', upload: '0 B', download: '50 B' }
    ]
    const base: NodeMetricsRow = {
        nodeUuid: 'a',
        nodeName: 'A',
        countryEmoji: '',
        providerName: '',
        usersOnline: 4,
        inboundsStats: rows,
        outboundsStats: []
    }
    assert.deepEqual(
        metricsSummary([base, { ...base, nodeUuid: 'b', usersOnline: 0, inboundsStats: [] }]),
        {
            total: 2,
            active: 1,
            online: 4,
            inbounds: 3
        }
    )
    assert.deepEqual(
        visibleTraffic(rows, true).map(({ tag }) => tag),
        ['upload', 'download']
    )
    assert.equal(visibleTraffic(rows, false), rows)
})
