import type { GetRemnawaveHealthCommand } from '@remnawave/backend-contract'
import type { IMetricCardProps } from '@shared/ui/metrics/metric-card'
import { prettifyBytesUtil } from '@shared/utils/bytes'
import { getXrayUptimeUtil } from '@shared/utils/time-utils'
import type { TFunction } from 'i18next'
import {
    PiGearSixDuotone,
    PiMemoryDuotone,
    PiTimerDuotone,
    PiTreeStructureDuotone
} from 'react-icons/pi'

export const getRuntimeSummaryMetrics = (
    runtimeMetrics: GetRemnawaveHealthCommand.Response['response']['runtimeMetrics'],
    t: TFunction
): IMetricCardProps[] => {
    if (!runtimeMetrics || runtimeMetrics.length === 0) {
        return []
    }

    const totalRss = runtimeMetrics.reduce((sum, p) => sum + Number(p.rss), 0)
    const totalHeapUsed = runtimeMetrics.reduce((sum, p) => sum + Number(p.heapUsed), 0)
    const totalGoroutines = runtimeMetrics.reduce((sum, p) => sum + Number(p.activeHandles), 0)
    const maxUptime = runtimeMetrics.reduce((max, p) => Math.max(max, Number(p.uptime)), 0)

    return [
        {
            value: prettifyBytesUtil(totalRss, true),
            IconComponent: PiMemoryDuotone,
            title: t('pm2-metrics.total-memory'),
            iconVariant: 'soft',
            iconColor: 'cyan'
        },
        {
            value: prettifyBytesUtil(totalHeapUsed, true),
            IconComponent: PiTreeStructureDuotone,
            title: 'Heap Used',
            iconVariant: 'soft',
            iconColor: 'green'
        },
        {
            value: totalGoroutines,
            IconComponent: PiGearSixDuotone,
            title: 'Goroutines',
            iconVariant: 'soft',
            iconColor: 'blue'
        },
        {
            value: getXrayUptimeUtil(maxUptime),
            IconComponent: PiTimerDuotone,
            title: t('pm2-metrics.uptime'),
            iconVariant: 'soft',
            iconColor: 'orange'
        }
    ]
}
