import {
    GetConnectionKeysByUserIdCommand,
    GetRawSubscriptionByShortUuidCommand,
    GetUserByIdCommand,
    GetUserHwidDevicesCommand,
    GetUserMetadataCommand,
    GetUsersCommand,
    GetUsersTagsCommand
} from '@remnawave/backend-contract'
import { createUrl } from '@shared/api/helpers/create-url'
import { keepPreviousData, queryOptions } from '@tanstack/svelte-query'

import { callCommand } from '../commands'
import type { UserTableState } from './table-state'
import { usersRequest } from './table-state'

export type User = GetUsersCommand.Response['response']['users'][number]
export type UserStatus = User['status']
export const userStatuses = ['ACTIVE', 'DISABLED', 'LIMITED', 'EXPIRED'] as const
export const trafficStrategies = ['NO_RESET', 'DAY', 'WEEK', 'MONTH', 'MONTH_ROLLING'] as const
export const usersQueryKey = ['users'] as const

export function usersQuery(state: UserTableState, modes: Record<string, string>, polling: boolean) {
    const params = usersRequest(state, modes)
    return queryOptions({
        queryKey: [...usersQueryKey, 'getAllUsers', params],
        queryFn: ({ signal }) =>
            callCommand(
                {
                    ...GetUsersCommand,
                    TSQ_url: createUrl(GetUsersCommand.TSQ_url, params)
                },
                { signal }
            ),
        staleTime: 20_000,
        refetchInterval: polling ? 25_000 : false,
        placeholderData: keepPreviousData
    })
}

export function userQuery(userId: string) {
    return queryOptions({
        queryKey: [...usersQueryKey, 'getUserById', { userId: Number(userId) }],
        queryFn: ({ signal }) => {
            const params = GetUserByIdCommand.RequestParamSchema.parse({ userId })
            return callCommand(
                { ...GetUserByIdCommand, TSQ_url: GetUserByIdCommand.url(String(params.userId)) },
                { signal }
            )
        },
        staleTime: 3_000,
        refetchInterval: 35_000
    })
}

export function userMetadataQuery(userId: number) {
    const command = GetUserMetadataCommand
    return queryOptions({
        queryKey: ['metadata', { kind: 'user', userId }],
        queryFn: ({ signal }) => {
            const params = command.RequestParamsSchema.parse({ userId })
            return callCommand(
                { ...command, TSQ_url: command.url(String(params.userId)) },
                { signal }
            )
        }
    })
}

export const userTagsQuery = queryOptions({
    queryKey: [...usersQueryKey, 'getUserTags'],
    queryFn: ({ signal }) => callCommand(GetUsersTagsCommand, { signal }),
    staleTime: 15_000,
    refetchInterval: 15_000
})

export function connectionKeysQuery(userId: number) {
    const command = GetConnectionKeysByUserIdCommand
    return queryOptions({
        queryKey: [...usersQueryKey, 'getConnectionKeysByUserId', { userId }],
        queryFn: ({ signal }) => {
            const params = command.RequestParamSchema.parse({ userId })
            return callCommand(
                { ...command, TSQ_url: command.url(String(params.userId)) },
                { signal }
            )
        },
        gcTime: 0
    })
}

export function rawSubscriptionQuery(shortUuid: string) {
    const command = GetRawSubscriptionByShortUuidCommand
    return queryOptions({
        queryKey: [...usersQueryKey, 'getRawSubscription', { shortUuid }],
        queryFn: ({ signal }) => {
            const params = command.RequestParamSchema.parse({ shortUuid })
            return callCommand(
                { ...command, TSQ_url: command.url(encodeURIComponent(params.shortUuid)) },
                { signal }
            )
        },
        gcTime: 0
    })
}

export type HwidDevice = GetUserHwidDevicesCommand.Response['response']['devices'][number]

export function userDevicesQuery(userId: number) {
    const command = GetUserHwidDevicesCommand
    return queryOptions({
        queryKey: ['hwid', 'userDevices', { userId }],
        queryFn: ({ signal }) => {
            const params = command.RequestParamSchema.parse({ userId })
            return callCommand(
                { ...command, TSQ_url: command.url(String(params.userId)) },
                { signal }
            )
        }
    })
}
