import {
    GetHttpStatsCommand,
    GetHwidDevicesCommand,
    GetHwidDevicesStatsCommand,
    GetSubscriptionRequestHistoryCommand,
    GetSubscriptionRequestHistoryStatsCommand,
    GetTopUsersByHwidDevicesCommand
} from '@remnawave/backend-contract'
import { createUrl } from '@shared/api/helpers/create-url'
import { queryOptions } from '@tanstack/svelte-query'

import { callCommand } from '../commands'
import type { inspectorRequest } from './table-state'

export type Device = GetHwidDevicesCommand.Response['response']['devices'][number]
export type HistoryRecord =
    GetSubscriptionRequestHistoryCommand.Response['response']['records'][number]
export type InspectorParams = ReturnType<typeof inspectorRequest>

export async function loadDevices(filters: InspectorParams, signal: AbortSignal) {
    const command = GetHwidDevicesCommand
    const TSQ_url = createUrl(command.TSQ_url, command.RequestQuerySchema.parse(filters))
    const data = await callCommand({ ...command, TSQ_url }, { signal })
    return { rows: data.devices, total: data.total }
}

export async function loadHistory(filters: InspectorParams, signal: AbortSignal) {
    const command = GetSubscriptionRequestHistoryCommand
    const TSQ_url = createUrl(command.TSQ_url, command.RequestQuerySchema.parse(filters))
    const data = await callCommand({ ...command, TSQ_url }, { signal })
    return { rows: data.records, total: data.total }
}

export const devicesStats = queryOptions({
    queryKey: ['hwid', 'stats'],
    queryFn: ({ signal }) => callCommand(GetHwidDevicesStatsCommand, { signal }),
    refetchInterval: 30_000,
    staleTime: 30_000
})

export const historyStats = queryOptions({
    queryKey: ['subscriptionRequestHistory', 'stats'],
    queryFn: ({ signal }) => callCommand(GetSubscriptionRequestHistoryStatsCommand, { signal }),
    refetchInterval: 30_000,
    staleTime: 30_000
})

export function deviceLeaders(start: number, size: number) {
    const command = GetTopUsersByHwidDevicesCommand
    return queryOptions({
        queryKey: ['hwid', 'topUsers', { start, size }],
        queryFn: ({ signal }) =>
            callCommand(command, {
                params: command.RequestQuerySchema.parse({ start, size }),
                signal
            }),
        staleTime: 30_000,
        refetchInterval: 30_000
    })
}

export const httpStats = queryOptions({
    queryKey: ['system', 'httpStats'],
    queryFn: ({ signal }) => callCommand(GetHttpStatsCommand, { signal }),
    staleTime: 5_000,
    refetchInterval: 5_000
})
