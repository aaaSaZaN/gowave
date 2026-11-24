import {
    GetInfraBillingNodesCommand,
    GetInfraBillingRecordsCommand,
    GetInfraProvidersCommand
} from '@remnawave/backend-contract'
import { createUrl } from '@shared/api/helpers/create-url'
import { infiniteQueryOptions, queryOptions } from '@tanstack/svelte-query'

import { callCommand } from '../commands'

export type BillingNode = GetInfraBillingNodesCommand.Response['response']['billingNodes'][number]
export type AvailableNode =
    GetInfraBillingNodesCommand.Response['response']['availableBillingNodes'][number]
export type Provider = GetInfraProvidersCommand.Response['response']['providers'][number]

export const billingKey = ['infraBilling']
export const billingNodes = queryOptions({
    queryKey: [...billingKey, 'nodes'],
    queryFn: ({ signal }) => callCommand(GetInfraBillingNodesCommand, { signal }),
    staleTime: 30_000
})

export const billingProviders = queryOptions({
    queryKey: [...billingKey, 'providers'],
    queryFn: ({ signal }) => callCommand(GetInfraProvidersCommand, { signal }),
    staleTime: 30_000
})

export const billingRecords = infiniteQueryOptions({
    queryKey: [...billingKey, 'records'],
    initialPageParam: 0,
    queryFn: ({ pageParam, signal }) => {
        const command = GetInfraBillingRecordsCommand
        const TSQ_url = createUrl(command.TSQ_url, { start: pageParam, size: 100 })
        return callCommand({ ...command, TSQ_url }, { signal })
    },
    getNextPageParam: (last, pages) => {
        const loaded = pages.reduce((count, page) => count + page.records.length, 0)
        return last.records.length > 0 && loaded < last.total ? loaded : undefined
    },
    staleTime: 20_000
})
