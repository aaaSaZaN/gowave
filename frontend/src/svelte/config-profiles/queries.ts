import {
    GetComputedConfigProfileByUuidCommand,
    GetConfigProfileByUuidCommand,
    GetConfigProfilesCommand,
    GetInboundsByProfileUuidCommand,
    GetSnippetsCommand
} from '@remnawave/backend-contract'
import { queryOptions } from '@tanstack/svelte-query'

import { callCommand } from '../commands'

export type Profile = GetConfigProfilesCommand.Response['response']['configProfiles'][number]

export const profilesQuery = queryOptions({
    queryKey: ['configProfiles', 'getConfigProfiles'],
    queryFn: ({ signal }) => callCommand(GetConfigProfilesCommand, { signal }),
    staleTime: 5_000
})

export const snippetsQuery = queryOptions({
    queryKey: ['snippets', 'getSnippets'],
    queryFn: ({ signal }) => callCommand(GetSnippetsCommand, { signal }),
    staleTime: 30_000
})

export function profileQuery(uuid: string) {
    return queryOptions({
        queryKey: ['configProfiles', 'getConfigProfile', { uuid }],
        queryFn: ({ signal }) => {
            const params = GetConfigProfileByUuidCommand.RequestParamSchema.parse({ uuid })
            return callCommand(
                {
                    ...GetConfigProfileByUuidCommand,
                    TSQ_url: GetConfigProfileByUuidCommand.url(params.uuid)
                },
                { signal }
            )
        },
        staleTime: 30_000
    })
}

export function computedProfileQuery(uuid: string) {
    return queryOptions({
        queryKey: ['configProfiles', 'getComputedConfigProfile', { uuid }],
        queryFn: ({ signal }) => {
            const params = GetComputedConfigProfileByUuidCommand.RequestParamSchema.parse({ uuid })
            return callCommand(
                {
                    ...GetComputedConfigProfileByUuidCommand,
                    TSQ_url: GetComputedConfigProfileByUuidCommand.url(params.uuid)
                },
                { signal }
            )
        }
    })
}

export function profileInboundsQuery(uuid: string) {
    const command = GetInboundsByProfileUuidCommand
    return queryOptions({
        queryKey: ['configProfiles', 'getInboundsByProfile', { uuid }],
        queryFn: ({ signal }) => {
            const params = command.RequestParamSchema.parse({ uuid })
            return callCommand({ ...command, TSQ_url: command.url(params.uuid) }, { signal })
        }
    })
}
