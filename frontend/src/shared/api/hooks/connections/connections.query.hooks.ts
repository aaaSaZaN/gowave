import {
    ConnectionsByNodeResultCommand,
    ConnectionsByUserResultCommand,
    GeocheckByNodeResultCommand
} from '@remnawave/backend-contract'

import { createQueryKeys } from '@shared/api/query-keys'
import { sToMs } from '@shared/utils/time-utils'

import { createGetQueryHook, errorHandler } from '../../tsq-helpers'

export const connectionsQueryKeys = createQueryKeys('connections', {
    connectionsByUserResult: (route: ConnectionsByUserResultCommand.RequestParam) => ({
        queryKey: [route]
    }),
    connectionsByNodeResult: (route: ConnectionsByNodeResultCommand.RequestParam) => ({
        queryKey: [route]
    }),
    geocheckByNodeResult: (route: GeocheckByNodeResultCommand.RequestParam) => ({
        queryKey: [route]
    })
})

export const useConnectionsByUserResult = createGetQueryHook({
    endpoint: ConnectionsByUserResultCommand.TSQ_url,
    responseSchema: ConnectionsByUserResultCommand.ResponseSchema,
    routeParamsSchema: ConnectionsByUserResultCommand.RequestParamSchema,
    getQueryKey: ({ route, query }) =>
        route && query
            ? connectionsQueryKeys.connectionsByUserResult({ ...route, ...query }).queryKey
            : connectionsQueryKeys.connectionsByUserResult._def,
    rQueryParams: {
        staleTime: sToMs(60)
    },
    errorHandler: (error) => errorHandler(error, 'Connections By User Result')
})

export const useConnectionsByNodeResult = createGetQueryHook({
    endpoint: ConnectionsByNodeResultCommand.TSQ_url,
    responseSchema: ConnectionsByNodeResultCommand.ResponseSchema,
    routeParamsSchema: ConnectionsByNodeResultCommand.RequestParamSchema,
    getQueryKey: ({ route, query }) =>
        route && query
            ? connectionsQueryKeys.connectionsByNodeResult({ ...route, ...query }).queryKey
            : connectionsQueryKeys.connectionsByNodeResult._def,
    rQueryParams: {
        staleTime: sToMs(60)
    },
    errorHandler: (error) => errorHandler(error, 'Connections By Node Result')
})

export const useGeocheckByNodeResult = createGetQueryHook({
    endpoint: GeocheckByNodeResultCommand.TSQ_url,
    responseSchema: GeocheckByNodeResultCommand.ResponseSchema,
    routeParamsSchema: GeocheckByNodeResultCommand.RequestParamSchema,
    getQueryKey: ({ route, query }) =>
        route && query
            ? connectionsQueryKeys.geocheckByNodeResult({ ...route, ...query }).queryKey
            : connectionsQueryKeys.geocheckByNodeResult._def,
    rQueryParams: {
        staleTime: sToMs(60)
    },
    errorHandler: (error) => errorHandler(error, 'Geocheck By Node Result')
})
