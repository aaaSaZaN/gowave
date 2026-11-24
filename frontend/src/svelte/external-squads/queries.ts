import {
    GetExternalSquadByUuidCommand,
    GetExternalSquadsCommand
} from '@remnawave/backend-contract'
import { queryOptions } from '@tanstack/svelte-query'

import { callCommand } from '../commands'

export type ExternalSquad = GetExternalSquadByUuidCommand.Response['response']
export const externalSquadsKey = ['externalSquads'] as const

export const externalSquadsQuery = queryOptions({
    queryKey: [...externalSquadsKey, 'getExternalSquads'],
    queryFn: ({ signal }) => callCommand(GetExternalSquadsCommand, { signal }),
    staleTime: 20_000
})

export function externalSquadQuery(uuid: string) {
    const command = GetExternalSquadByUuidCommand
    return queryOptions({
        queryKey: [...externalSquadsKey, 'getExternalSquad', { uuid }],
        queryFn: ({ signal }) => {
            const params = command.RequestParamSchema.parse({ uuid })
            return callCommand({ ...command, TSQ_url: command.url(params.uuid) }, { signal })
        }
    })
}
