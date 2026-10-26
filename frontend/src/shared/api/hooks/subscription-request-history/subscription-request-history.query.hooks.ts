import {
    GetSubscriptionRequestHistoryCommand,
    GetSubscriptionRequestHistoryStatsCommand
} from '@remnawave/backend-contract'
import { createQueryKeys } from '@shared/api/query-keys'
import { sToMs } from '@shared/utils/time-utils'
import { keepPreviousData } from '@tanstack/react-query'

import { createGetQueryHook, errorHandler } from '../../tsq-helpers'

export const subscriptionRequestHistoryQueryKeys = createQueryKeys('subscriptionRequestHistory', {
    getSubscriptionRequestHistory: (
        filters: GetSubscriptionRequestHistoryCommand.RequestQuery
    ) => ({
        queryKey: [filters]
    }),
    getSubscriptionRequestHistoryStats: {
        queryKey: null
    }
})

const STALE_TIME = 15_000
const REFETCH_INTERVAL = 15_100

export const useGetSubscriptionRequestHistory = createGetQueryHook({
    endpoint: GetSubscriptionRequestHistoryCommand.TSQ_url,
    responseSchema: GetSubscriptionRequestHistoryCommand.ResponseSchema,
    requestQuerySchema: GetSubscriptionRequestHistoryCommand.RequestQuerySchema,
    getQueryKey: ({ query }) =>
        query
            ? subscriptionRequestHistoryQueryKeys.getSubscriptionRequestHistory(query).queryKey
            : subscriptionRequestHistoryQueryKeys.getSubscriptionRequestHistory._def,
    rQueryParams: {
        staleTime: sToMs(20),
        refetchInterval: sToMs(25),
        placeholderData: keepPreviousData,
        refetchOnMount: true
    },
    errorHandler: (error) => errorHandler(error, 'Get Subscription Request History')
})

export const useGetSubscriptionRequestHistoryStats = createGetQueryHook({
    endpoint: GetSubscriptionRequestHistoryStatsCommand.TSQ_url,
    responseSchema: GetSubscriptionRequestHistoryStatsCommand.ResponseSchema,
    getQueryKey: () =>
        subscriptionRequestHistoryQueryKeys.getSubscriptionRequestHistoryStats.queryKey,
    rQueryParams: {
        placeholderData: keepPreviousData,
        staleTime: STALE_TIME,
        refetchInterval: REFETCH_INTERVAL
    },
    errorHandler: (error) => errorHandler(error, 'Get Subscription Request History Stats')
})
