import {
    GetInternalSquadAccessibleNodesCommand,
    GetInternalSquadCommand,
    GetInternalSquadsCommand,
    GetInternalSquadUsageCommand
} from '@remnawave/backend-contract'
import { infiniteQueryOptions, queryOptions } from '@tanstack/svelte-query'

import { callCommand } from '../commands'

export type Squad = GetInternalSquadsCommand.Response['response']['internalSquads'][number]

export const squadsQuery = queryOptions({
    queryKey: ['internalSquads', 'getInternalSquads'],
    queryFn: ({ signal }) => callCommand(GetInternalSquadsCommand, { signal }),
    staleTime: 5_000
})

export function internalSquadQuery(uuid: string) {
    const command = GetInternalSquadCommand
    return queryOptions({
        queryKey: ['internalSquads', 'getInternalSquad', { uuid }],
        queryFn: ({ signal }) => {
            const params = command.RequestParamSchema.parse({ uuid })
            return callCommand({ ...command, TSQ_url: command.url(params.uuid) }, { signal })
        },
        retry: false
    })
}

export function squadUsageQuery(
    uuid: string,
    filters: { start: string; end: string; minTotalBytes: number }
) {
    const command = GetInternalSquadUsageCommand
    return infiniteQueryOptions({
        queryKey: ['internalSquads', 'usage', uuid, { ...filters }],
        initialPageParam: undefined as string | undefined,
        queryFn: ({ signal, pageParam }) => {
            const params = command.RequestParamSchema.parse({ uuid })
            const query = command.RequestQuerySchema.parse({
                ...filters,
                limit: 250,
                cursor: pageParam
            })
            return callCommand(
                { ...command, TSQ_url: command.url(params.uuid) },
                { signal, params: query }
            )
        },
        getNextPageParam: (lastPage) => (lastPage.hasMore ? lastPage.nextCursor : undefined),
        staleTime: 60_000
    })
}

export function squadNodesQuery(uuid: string) {
    const command = GetInternalSquadAccessibleNodesCommand
    return queryOptions({
        queryKey: ['internalSquads', 'accessibleNodes', { uuid }],
        queryFn: ({ signal }) => {
            const params = command.RequestParamSchema.parse({ uuid })
            return callCommand({ ...command, TSQ_url: command.url(params.uuid) }, { signal })
        },
        staleTime: 15_000
    })
}
