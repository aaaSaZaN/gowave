import { GetHostCommand, GetHostsCommand, GetHostsTagsCommand } from '@remnawave/backend-contract'
import { queryOptions } from '@tanstack/svelte-query'

import { callCommand } from '../commands'

export type Host = GetHostCommand.Response['response']
export const hostsKey = ['hosts'] as const

export const hostsQuery = queryOptions({
    queryKey: [...hostsKey, 'getAllHosts'],
    queryFn: ({ signal }) => callCommand(GetHostsCommand, { signal }),
    staleTime: 15_000
})

export const hostTagsQuery = queryOptions({
    queryKey: [...hostsKey, 'getAllTags'],
    queryFn: ({ signal }) => callCommand(GetHostsTagsCommand, { signal }),
    staleTime: 15_000
})

export function hostQuery(uuid: string) {
    const command = GetHostCommand
    return queryOptions({
        queryKey: [...hostsKey, 'getHost', { uuid }],
        queryFn: ({ signal }) => {
            const params = command.RequestParamSchema.parse({ uuid })
            return callCommand({ ...command, TSQ_url: command.url(params.uuid) }, { signal })
        },
        retry: false,
        staleTime: 15_000
    })
}
