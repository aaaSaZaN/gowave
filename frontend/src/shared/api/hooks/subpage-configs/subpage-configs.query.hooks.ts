import {
    GetSubpageConfigCommand,
    GetSubpageConfigsCommand,
    GetSubpageConfigsTagsCommand
} from '@remnawave/backend-contract'

import { createQueryKeys } from '@shared/api/query-keys'
import { sToMs } from '@shared/utils/time-utils'

import { createGetQueryHook, errorHandler } from '../../tsq-helpers'

export const subpageConfigsQueryKeys = createQueryKeys('subpageConfigs', {
    getSubpageConfigsTags: {
        queryKey: null
    },
    getSubpageConfig: (route: GetSubpageConfigCommand.RequestParam) => ({
        queryKey: [route]
    }),
    getSubpageConfigs: {
        queryKey: null
    }
})

export const useGetSubscriptionPageConfig = createGetQueryHook({
    endpoint: GetSubpageConfigCommand.TSQ_url,
    routeParamsSchema: GetSubpageConfigCommand.RequestParamSchema,
    responseSchema: GetSubpageConfigCommand.ResponseSchema,
    getQueryKey: ({ route }) =>
        route
            ? subpageConfigsQueryKeys.getSubpageConfig(route).queryKey
            : subpageConfigsQueryKeys.getSubpageConfig._def,
    rQueryParams: {
        refetchOnMount: false,
        staleTime: sToMs(5)
    },
    errorHandler: (error) => errorHandler(error, 'Get Subscription Page Config')
})

export const useGetSubpageConfigs = createGetQueryHook({
    endpoint: GetSubpageConfigsCommand.TSQ_url,
    responseSchema: GetSubpageConfigsCommand.ResponseSchema,
    getQueryKey: () => subpageConfigsQueryKeys.getSubpageConfigs.queryKey,
    rQueryParams: {
        refetchOnMount: false,
        staleTime: sToMs(15)
    },
    errorHandler: (error) => errorHandler(error, 'Get Subscription Page Configs')
})

export const useGetSubpageConfigsTags = createGetQueryHook({
    endpoint: GetSubpageConfigsTagsCommand.TSQ_url,
    responseSchema: GetSubpageConfigsTagsCommand.ResponseSchema,
    getQueryKey: () => subpageConfigsQueryKeys.getSubpageConfigsTags.queryKey,
    rQueryParams: {
        refetchOnMount: true,
        staleTime: sToMs(30)
    },
    errorHandler: (error) => errorHandler(error, 'Get SubpageConfigs Tags')
})
