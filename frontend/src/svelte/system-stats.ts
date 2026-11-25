import { GetStatsCommand } from '@remnawave/backend-contract'
import { getUserTimezoneUtil } from '@shared/utils/time-utils'
import { keepPreviousData, queryOptions } from '@tanstack/svelte-query'
import type { TFunction } from 'i18next'

import { callCommand } from './commands'
import type { IconName } from './ui/icons'

export const systemStatsQuery = queryOptions({
    queryKey: ['system', 'getSystemStats'],
    queryFn: ({ signal }) =>
        callCommand(GetStatsCommand, {
            params: GetStatsCommand.RequestQuerySchema.parse({ tz: getUserTimezoneUtil() }),
            signal
        }),
    staleTime: 5_000,
    refetchInterval: 5_100,
    placeholderData: keepPreviousData
})

export interface SummaryMetric {
    label: string
    value: number | string
    icon: IconName
    color: string
    comparison?: string
    period?: string
}

export function userMetrics(
    users: GetStatsCommand.Response['response']['users'] | undefined,
    t: TFunction
): SummaryMetric[] {
    return [
        {
            label: t('common.field.total'),
            value: users?.totalUsers ?? '—',
            icon: 'PiUsersDuotone',
            color: 'blue'
        },
        {
            label: 'Active',
            value: users?.statusCounts.ACTIVE ?? '—',
            icon: 'PiPulseDuotone',
            color: 'teal'
        },
        {
            label: 'Expired',
            value: users?.statusCounts.EXPIRED ?? '—',
            icon: 'PiClockUserDuotone',
            color: 'red'
        },
        {
            label: 'Limited',
            value: users?.statusCounts.LIMITED ?? '—',
            icon: 'PiClockCountdownDuotone',
            color: 'orange'
        },
        {
            label: 'Disabled',
            value: users?.statusCounts.DISABLED ?? '—',
            icon: 'PiProhibitDuotone',
            color: 'gray'
        }
    ]
}
