import {
    GetSubpageConfigCommand,
    GetSubpageConfigsCommand,
    GetSubscriptionSettingsCommand,
    GetSubscriptionTemplateCommand,
    GetSubscriptionTemplatesCommand
} from '@remnawave/backend-contract'
import { SubscriptionSettingsResponseSchema } from '@shared/api/schemas/subscription-settings.schema'
import { queryOptions } from '@tanstack/svelte-query'

import { callCommand } from '../commands'

export const subscriptionTemplatesQuery = queryOptions({
    queryKey: ['subscriptionTemplate', 'getSubscriptionTemplates'],
    queryFn: ({ signal }) => callCommand(GetSubscriptionTemplatesCommand, { signal }),
    staleTime: 30_000,
    refetchOnMount: false
})

export const subpageConfigsQuery = queryOptions({
    queryKey: ['subpageConfigs', 'getSubpageConfigs'],
    queryFn: ({ signal }) => callCommand(GetSubpageConfigsCommand, { signal }),
    staleTime: 15_000
})

export const subscriptionSettingsQuery = queryOptions({
    queryKey: ['subscriptionSettings', 'getSubscriptionSettings'],
    queryFn: ({ signal }) =>
        callCommand(
            {
                ...GetSubscriptionSettingsCommand,
                ResponseSchema: SubscriptionSettingsResponseSchema
            },
            { signal }
        ),
    staleTime: 30_000,
    refetchOnMount: false
})

export function templateQuery(uuid: string) {
    const command = GetSubscriptionTemplateCommand
    return queryOptions({
        queryKey: ['subscriptionTemplate', 'getSubscriptionTemplate', { uuid }],
        queryFn: ({ signal }) => {
            const params = command.RequestParamSchema.parse({ uuid })
            return callCommand({ ...command, TSQ_url: command.url(params.uuid) }, { signal })
        },
        retry: false
    })
}

export function subpageQuery(uuid: string) {
    const command = GetSubpageConfigCommand
    return queryOptions({
        queryKey: ['subpageConfigs', 'getSubpageConfig', { uuid }],
        queryFn: ({ signal }) => {
            const params = command.RequestParamSchema.parse({ uuid })
            return callCommand({ ...command, TSQ_url: command.url(params.uuid) }, { signal })
        },
        retry: false
    })
}
