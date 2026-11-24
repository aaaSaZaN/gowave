import {
    GetTorrentBlockerReportsCommand,
    GetTorrentBlockerReportsStatsCommand
} from '@remnawave/backend-contract'
import { createUrl } from '@shared/api/helpers/create-url'
import { queryOptions } from '@tanstack/svelte-query'

import { callCommand } from '../commands'
import type { InspectorParams } from '../tools/queries'

import { torrentStorage } from '../tools/table-state'

export type TorrentReport = GetTorrentBlockerReportsCommand.Response['response']['records'][number]

function loadReports(query: GetTorrentBlockerReportsCommand.RequestQuery, signal: AbortSignal) {
    const command = GetTorrentBlockerReportsCommand
    const TSQ_url = createUrl(command.TSQ_url, command.RequestQuerySchema.parse(query))
    return callCommand({ ...command, TSQ_url }, { signal })
}

export async function reportRows(params: InspectorParams, signal: AbortSignal) {
    const { records, total } = await loadReports(params, signal)
    return { rows: records, total }
}

export function reportsForUser(userId: number, page: number, size: number) {
    return queryOptions({
        queryKey: [torrentStorage.name, 'user', userId, page, size],
        queryFn: ({ signal }) =>
            loadReports(
                {
                    start: page * size,
                    size,
                    filters: [{ id: 'userId', value: userId }],
                    filterModes: { userId: 'equals' },
                    sorting: [{ id: 'id', desc: true }]
                },
                signal
            )
    })
}

export const reportStats = queryOptions({
    queryKey: [torrentStorage.name, 'stats'],
    queryFn: ({ signal }) => callCommand(GetTorrentBlockerReportsStatsCommand, { signal }),
    staleTime: 25_000,
    refetchInterval: 25_000
})

export function ipDetailsUrl(ip: string): string {
    return `https://ipinfo.io/${encodeURIComponent(ip)}`
}
