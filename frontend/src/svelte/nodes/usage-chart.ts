import { prettifyBytesUtil } from '@shared/utils/bytes/pretty-bytes/pretty-bytes.util'
import { formatTimeUtil } from '@shared/utils/time-utils/format-time.util'
import type { Options, Point } from 'highcharts'
import type { UsageSeries } from './analytics'

const body = 'var(--mantine-color-body)'
const text = 'var(--mantine-color-text)'
const dimmed = 'var(--mantine-color-dimmed)'
const edge = 'var(--mantine-color-gray-4)'

function escapeText(value: string): string {
    return value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;')
}

function formatBytes(value: number, compact = false): string {
    return (compact ? prettifyBytesUtil(value, true) : prettifyBytesUtil(value)) ?? ''
}

function trafficTooltip(
    point: Point,
    node: UsageSeries,
    categories: string[],
    language: string,
    hint: string,
    rgb: string
): string {
    const index = point.index
    const total = point.series.chart.series.reduce(
        (sum, series) => sum + (series.data[index]?.y ?? 0),
        0
    )
    const start = Math.max(0, index - 5)
    const end = index + 6
    const peak = Math.max(1, ...node.data.slice(start, end))
    const nearby = categories
        .slice(start, end)
        .map((time, offset) => {
            const day = start + offset
            const value = node.data[day] ?? 0
            if (!value) return ''
            const current = day === index
            const date = formatTimeUtil({ time, template: 'SHORT_DATE', language })
            const color = current ? text : `color-mix(in srgb, ${text} 50%, transparent)`
            const weight = current ? 600 : 400
            return `<div style="display:flex;align-items:center;gap:6px;color:${color}">
            <span style="width:42px;font-size:.7rem;text-align:right;font-weight:${weight}">${date}</span>
            <div style="flex:1;height:6px;background:${body};border-radius:3px;overflow:hidden">
                <div style="width:${Math.max((value / peak) * 100, 2)}%;height:100%;background:rgba(${rgb}, ${current ? 0.8 : 0.4});border-radius:3px"></div>
            </div>
            <span style="width:50px;font-size:.7rem;font-weight:${weight}">${formatBytes(value, true)}</span>
        </div>`
        })
        .reverse()
        .join('')

    const date = formatTimeUtil({ time: categories[index], template: 'FULL_DATE', language })
    return `<div style="font-size:.875rem;padding:4px;min-width:220px">
        <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:8px">
            <span style="font-weight:600">${date}</span>
            <span style="font-size:.85rem;color:${dimmed};display:flex;align-items:center;gap:4px">Σ ${formatBytes(total, true)}</span>
        </div>
        <div style="display:flex;align-items:center;gap:8px;margin-bottom:10px">
            <div style="width:10px;height:10px;background:rgba(${rgb}, .8);border-radius:50%;flex-shrink:0"></div>
            <span style="flex:1">${escapeText(node.name)}</span>
            <span style="font-weight:600">${formatBytes(point.y ?? 0)}</span>
        </div>
        <div style="display:flex;flex-direction:column;gap:3px;padding-top:8px;border-top:1px solid ${edge}">${nearby}</div>
        <div style="color:${dimmed};font-size:.7rem;text-align:center;margin-top:10px">${escapeText(hint)}</div>
    </div>`
}

export function usageChartOptions({
    categories,
    series,
    language,
    hint,
    user,
    selectDay
}: {
    categories: string[]
    series: UsageSeries[]
    language: string
    hint: string
    user: boolean
    selectDay: (index: number) => void
}): Options {
    const coloredSeries = series.map((node) => {
        const color = /^#[\da-f]{6}$/i.test(node.color) ? node.color : '#868e96'
        const rgb = [1, 3, 5]
            .map((start) => Number.parseInt(color.slice(start, start + 2), 16))
            .join(', ')
        return { node, color, rgb }
    })
    return {
        chart: {
            type: 'bar',
            height: user ? '90%' : '60%',
            backgroundColor: 'transparent',
            style: { fontFamily: 'inherit' }
        },
        title: { text: '' },
        accessibility: { enabled: false },
        plotOptions: {
            bar: { stacking: 'normal', borderRadius: 0, cursor: 'pointer' },
            series: {
                states: { hover: { enabled: true, brightness: 0.1 }, inactive: { opacity: 0.2 } },
                stickyTracking: false
            }
        },
        legend: {
            enabled: true,
            layout: 'horizontal',
            align: 'center',
            verticalAlign: 'bottom',
            backgroundColor: body,
            borderWidth: 2,
            padding: 20,
            borderRadius: 14,
            borderColor: 'var(--mantine-color-gray-7)',
            itemMarginTop: 4,
            itemMarginBottom: 4,
            itemStyle: { color: text, fontSize: '14px', fontWeight: '400' },
            itemHoverStyle: { color: text, fontWeight: '600', textDecoration: 'underline' },
            itemHiddenStyle: { color: dimmed },
            symbolRadius: 2,
            symbolHeight: 10,
            symbolWidth: 10,
            navigation: {
                enabled: true,
                activeColor: 'var(--mantine-color-primary-filled)',
                inactiveColor: dimmed,
                style: { fontWeight: '500', color: text, fontSize: '12px' }
            }
        },
        credits: { enabled: false },
        exporting: { enabled: false },
        xAxis: {
            reversed: false,
            reversedStacks: true,
            categories: categories.slice(),
            crosshair: true,
            labels: {
                style: { color: text },
                formatter: ({ value }) =>
                    formatTimeUtil({ time: value, template: 'SHORT_DATE', language })
            },
            gridLineColor: 'var(--mantine-color-gray-light-hover)',
            gridLineWidth: 1,
            gridLineDashStyle: 'LongDash'
        },
        yAxis: {
            title: { text: undefined },
            reversedStacks: false,
            labels: {
                autoRotation: [-45, 45],
                style: { color: text },
                formatter: ({ value }) => prettifyBytesUtil(value, true)
            },
            gridLineColor: 'transparent'
        },
        tooltip: {
            shared: false,
            backgroundColor: body,
            borderColor: edge,
            style: { color: text },
            useHTML: true,
            formatter() {
                const entry = coloredSeries[this.series.index]
                return (
                    entry && trafficTooltip(this, entry.node, categories, language, hint, entry.rgb)
                )
            }
        },
        series: coloredSeries.map(({ node, color, rgb }) => ({
            id: node.uuid,
            type: 'bar',
            name: node.name,
            data: node.data.slice(),
            color: `rgba(${rgb}, 0.5)`,
            borderWidth: 1,
            borderColor: color,
            point: {
                events: {
                    click() {
                        selectDay(this.index)
                    }
                }
            }
        }))
    }
}
