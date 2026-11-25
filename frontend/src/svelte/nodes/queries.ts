import {
    GetInfraProvidersCommand,
    GetNodeCommand,
    GetNodeSecretKeyCommand,
    GetNodesCommand,
    GetNodesMetricsCommand,
    GetStatsNodesUsageCommand,
    GetStatsNodesUsersUsageCommand,
    GetStatsNodeUsersUsageCommand,
    GetStatsUserUsageCommand
} from '@remnawave/backend-contract'
import { requestResponse } from '@shared/api/request-response'
import { GetNodeResponseSchema } from '@shared/api/schemas/node.schema'
import { queryOptions } from '@tanstack/svelte-query'
import { z } from 'zod'

import { callCommand } from '../commands'

export type Node = GetNodesCommand.Response['response'][number]
export type NodeDetail = z.output<typeof GetNodeResponseSchema>['response']
export const nodesKey = ['nodes'] as const

export const nodesQuery = queryOptions({
    queryKey: [...nodesKey, 'getAllNodes'],
    queryFn: ({ signal }) => callCommand(GetNodesCommand, { signal }),
    staleTime: 5_000
})

const trafficSummarySchema = z.object({
    response: z.object({
        nodes: z.array(z.object({ uuid: z.string(), lifetimeTrafficBytes: z.number() }))
    })
})

export const trafficSummaryQuery = queryOptions({
    queryKey: [...nodesKey, 'getTrafficSummary'],
    queryFn: ({ signal }) =>
        requestResponse(
            { url: '/api/nodes/traffic-summary', method: 'GET', signal },
            trafficSummarySchema
        ),
    staleTime: 60_000
})

export function nodeQuery(uuid: string) {
    const command = GetNodeCommand
    return queryOptions({
        queryKey: [...nodesKey, 'getNode', { uuid }],
        queryFn: ({ signal }) => {
            const route = command.RequestParamSchema.parse({ uuid })
            return callCommand(
                {
                    ...command,
                    TSQ_url: command.url(route.uuid),
                    ResponseSchema: GetNodeResponseSchema
                },
                { signal }
            )
        },
        staleTime: 5_000,
        retry: false
    })
}

export const nodeSecretQuery = queryOptions({
    queryKey: [...nodesKey, 'getNodeSecretKey'],
    queryFn: ({ signal }) => callCommand(GetNodeSecretKeyCommand, { signal }),
    gcTime: 0,
    staleTime: 60_000
})

export const providersQuery = queryOptions({
    queryKey: ['infraBilling', 'getInfraProviders'],
    queryFn: ({ signal }) => callCommand(GetInfraProvidersCommand, { signal }),
    staleTime: 15_000
})

export const nodeMetricsQuery = queryOptions({
    queryKey: ['system', 'getNodesMetrics'],
    queryFn: ({ signal }) => callCommand(GetNodesMetricsCommand, { signal }),
    staleTime: 30_000
})

export function usageQuery(filters: GetStatsNodesUsageCommand.RequestQuery, userId?: number) {
    return queryOptions({
        queryKey:
            userId === undefined
                ? ['bandwidthStats', 'getStatsNodesUsage', filters]
                : ['bandwidthStats', 'getStatsUserUsage', { userId }, filters],
        queryFn: ({ signal }) => {
            if (userId === undefined) {
                return callCommand(GetStatsNodesUsageCommand, {
                    params: GetStatsNodesUsageCommand.RequestQuerySchema.parse(filters),
                    signal
                })
            }
            const command = GetStatsUserUsageCommand
            const params = command.RequestParamSchema.parse({ userId })
            return callCommand(
                { ...command, TSQ_url: command.url(String(params.userId)) },
                { params: command.RequestQuerySchema.parse(filters), signal }
            )
        }
    })
}

export function nodeUsersUsageQuery(
    node: string | string[],
    filters: GetStatsNodeUsersUsageCommand.RequestQuery
) {
    const target = typeof node === 'string' ? node : [...new Set(node)].sort()
    return queryOptions({
        queryKey: ['bandwidthStats', 'getStatsNodeUsersUsage', target, filters],
        queryFn: ({ signal }) => {
            if (typeof target !== 'string') {
                const command = GetStatsNodesUsersUsageCommand
                return callCommand(command, {
                    data: command.RequestBodySchema.parse({ nodesUuids: target }),
                    params: command.RequestQuerySchema.parse(filters),
                    signal
                })
            }
            const command = GetStatsNodeUsersUsageCommand
            const route = command.RequestParamSchema.parse({ uuid: target })
            return callCommand(
                { ...command, TSQ_url: command.url(route.uuid) },
                { params: command.RequestQuerySchema.parse(filters), signal }
            )
        }
    })
}
