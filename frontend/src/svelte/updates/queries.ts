import { GetMetadataCommand } from '@remnawave/backend-contract'
import { fetchUpdateCheck, type UpdateChannel } from '@shared/api/panel-update'
import { queryOptions } from '@tanstack/svelte-query'

import { callCommand } from '../commands'

export type Metadata = GetMetadataCommand.Response['response']
const interval = 10 * 60_000

export const metadataQuery = queryOptions({
    queryKey: ['system', 'getGowaveMetadata'],
    queryFn: ({ signal }) => callCommand(GetMetadataCommand, { signal }),
    staleTime: 60 * 60_000
})

export function updateCheckQuery(channel?: UpdateChannel) {
    return queryOptions({
        queryKey: ['panel-update-check', channel ?? 'current'],
        queryFn: ({ signal }) => fetchUpdateCheck(channel, false, signal),
        retry: false,
        staleTime: interval / 2,
        refetchInterval: interval,
        refetchIntervalInBackground: false
    })
}
