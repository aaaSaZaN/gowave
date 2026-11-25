import {
    GetNodePluginCommand,
    GetNodePluginsCommand,
    GetSharedListCommand,
    GetSharedListsCommand
} from '@remnawave/backend-contract'
import { queryOptions } from '@tanstack/svelte-query'

import { callCommand } from '../commands'

export const nodePluginsKey = ['nodePlugins'] as const
export type Plugin = GetNodePluginCommand.Response['response']

export const nodePluginsQuery = queryOptions({
    queryKey: [...nodePluginsKey, 'getNodePlugins'],
    queryFn: ({ signal }) => callCommand(GetNodePluginsCommand, { signal }),
    staleTime: 15_000
})

export function pluginQuery(uuid: string) {
    const command = GetNodePluginCommand
    return queryOptions({
        queryKey: [...nodePluginsKey, 'getNodePlugin', { uuid }],
        queryFn: ({ signal }) => {
            const params = command.RequestParamSchema.parse({ uuid })
            return callCommand({ ...command, TSQ_url: command.url(params.uuid) }, { signal })
        },
        retry: false
    })
}

export const sharedListsQuery = queryOptions({
    queryKey: [...nodePluginsKey, 'getSharedLists'],
    queryFn: ({ signal }) => callCommand(GetSharedListsCommand, { signal }),
    staleTime: 15_000
})

export function sharedListQuery(name: string) {
    const command = GetSharedListCommand
    return queryOptions({
        queryKey: [...nodePluginsKey, 'getSharedList', { name }],
        queryFn: ({ signal }) =>
            callCommand(command, { params: command.RequestQuerySchema.parse({ name }), signal }),
        enabled: !!name,
        retry: false
    })
}
