import type { GetNodesMetricsCommand, GetStatsNodesUsageCommand } from '@remnawave/backend-contract'

type Usage = GetStatsNodesUsageCommand.Response['response']
export type UsageSeries = Usage['series'][number]
export type NodeMetricsRow = GetNodesMetricsCommand.Response['response']['nodes'][number]

export function usageSummary(values: number[]) {
    let total = 0
    let peak = 0
    for (const value of values) {
        total += value
        peak = Math.max(peak, value)
    }
    return { total, peak, average: values.length ? total / values.length : 0 }
}

export function usageDays(categories: string[], series: UsageSeries[]) {
    return categories.map((date, index) => ({
        date,
        index,
        total: series.reduce((sum, node) => sum + (node.data[index] ?? 0), 0)
    }))
}

export function usageForDay(series: UsageSeries[], index: number) {
    return series
        .map(({ uuid, name, color, countryCode, data }) => ({
            uuid,
            name,
            color,
            countryCode,
            bytes: data[index] ?? 0
        }))
        .filter(({ bytes }) => bytes > 0)
        .sort((a, b) => b.bytes - a.bytes)
}

export function metricsSummary(rows: NodeMetricsRow[]) {
    return rows.reduce(
        (summary, row) => ({
            total: summary.total + 1,
            active: summary.active + (row.usersOnline > 0 ? 1 : 0),
            online: summary.online + row.usersOnline,
            inbounds: summary.inbounds + row.inboundsStats.length
        }),
        { total: 0, active: 0, online: 0, inbounds: 0 }
    )
}

export function visibleTraffic(rows: NodeMetricsRow['inboundsStats'], hideZero: boolean) {
    return hideZero
        ? rows.filter(
              ({ upload, download }) =>
                  Number.parseFloat(upload) !== 0 || Number.parseFloat(download) !== 0
          )
        : rows
}

export function flag(country: string): string {
    if (!/^[A-Z]{2}$/.test(country) || country === 'XX') return ''
    return String.fromCodePoint(...Array.from(country, (letter) => 127397 + letter.charCodeAt(0)))
}
