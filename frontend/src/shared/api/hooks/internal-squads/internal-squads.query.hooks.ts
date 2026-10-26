import {
    GetInternalSquadAccessibleNodesCommand,
    GetInternalSquadCommand,
    GetInternalSquadsCommand,
    GetInternalSquadsTagsCommand
} from '@remnawave/backend-contract'
import { createQueryKeys } from '@shared/api/query-keys'
import { sToMs } from '@shared/utils/time-utils'
import { keepPreviousData } from '@tanstack/react-query'

import { createGetQueryHook, errorHandler } from '../../tsq-helpers'

export const internalSquadsQueryKeys = createQueryKeys('internalSquads', {
    getInternalSquadsTags: {
        queryKey: null
    },
    getInternalSquads: {
        queryKey: null
    },
    getInternalSquad: (route: GetInternalSquadCommand.RequestParam) => ({
        queryKey: [route]
    }),
    getInternalSquadAccessibleNodes: (
        route: GetInternalSquadAccessibleNodesCommand.RequestParam
    ) => ({
        queryKey: [route]
    })
})

export const useGetInternalSquads = createGetQueryHook({
    endpoint: GetInternalSquadsCommand.TSQ_url,
    responseSchema: GetInternalSquadsCommand.ResponseSchema,
    getQueryKey: () => internalSquadsQueryKeys.getInternalSquads.queryKey,
    rQueryParams: {
        placeholderData: keepPreviousData,
        refetchOnMount: true,
        staleTime: sToMs(5)
    },

    errorHandler: (error) => errorHandler(error, 'Get All Internal Squads')
})

export const useGetInternalSquad = createGetQueryHook({
    endpoint: GetInternalSquadCommand.TSQ_url,
    responseSchema: GetInternalSquadCommand.ResponseSchema,
    routeParamsSchema: GetInternalSquadCommand.RequestParamSchema,
    getQueryKey: ({ route }) =>
        route
            ? internalSquadsQueryKeys.getInternalSquad(route).queryKey
            : internalSquadsQueryKeys.getInternalSquad._def,
    rQueryParams: {
        refetchOnMount: true,
        staleTime: sToMs(30)
    },
    errorHandler: (error) => errorHandler(error, 'Get Internal Squad')
})

export const useGetInternalSquadAccessibleNodes = createGetQueryHook({
    endpoint: GetInternalSquadAccessibleNodesCommand.TSQ_url,
    responseSchema: GetInternalSquadAccessibleNodesCommand.ResponseSchema,
    routeParamsSchema: GetInternalSquadAccessibleNodesCommand.RequestParamSchema,
    getQueryKey: ({ route }) =>
        route
            ? internalSquadsQueryKeys.getInternalSquadAccessibleNodes(route).queryKey
            : internalSquadsQueryKeys.getInternalSquadAccessibleNodes._def,
    rQueryParams: {
        staleTime: sToMs(15)
    },
    errorHandler: (error) => errorHandler(error, 'Get Internal Squad Accessible Nodes')
})

export const useGetInternalSquadsTags = createGetQueryHook({
    endpoint: GetInternalSquadsTagsCommand.TSQ_url,
    responseSchema: GetInternalSquadsTagsCommand.ResponseSchema,
    getQueryKey: () => internalSquadsQueryKeys.getInternalSquadsTags.queryKey,
    rQueryParams: {
        refetchOnMount: true,
        staleTime: sToMs(30)
    },
    errorHandler: (error) => errorHandler(error, 'Get InternalSquads Tags')
})
