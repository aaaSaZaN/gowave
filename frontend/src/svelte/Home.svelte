<script lang="ts">
    import {
        GetBandwidthStatsCommand,
        GetRemnawaveHealthCommand
    } from '@remnawave/backend-contract'
    import { prettifyBytesUtil } from '@shared/utils/bytes'
    import { formatPercentage } from '@shared/utils/misc/number'
    import { getUserTimezoneUtil, getXrayUptimeUtil } from '@shared/utils/time-utils'
    import { createQuery, keepPreviousData } from '@tanstack/svelte-query'
    import { callCommand } from './commands'
    import home from './home.module.css'
    import { locale } from './i18n.svelte'
    import RouteLoading from './RouteLoading.svelte'
    import Disclaimer from './shell/Disclaimer.svelte'
    import { type SummaryMetric, systemStatsQuery, userMetrics } from './system-stats'
    import Button from './ui/Button.svelte'
    import type { IconName } from './ui/icons'
    import MetricCard from './ui/MetricCard.svelte'

    const timezone = getUserTimezoneUtil()
    const frequent = { staleTime: 5_000, refetchInterval: 5_100 }
    const stats = createQuery(() => systemStatsQuery)
    const bandwidth = createQuery(() => ({
        ...frequent,
        queryKey: ['system', 'getBandwidthStats'],
        queryFn: ({ signal }) =>
            callCommand(GetBandwidthStatsCommand, {
                params: GetBandwidthStatsCommand.RequestQuerySchema.parse({ tz: timezone }),
                signal
            }),
        placeholderData: keepPreviousData
    }))
    const health = createQuery(() => ({
        staleTime: 10_000,
        refetchInterval: 10_000,
        queryKey: ['system', 'getGowaveHealth'],
        queryFn: ({ signal }) => callCommand(GetRemnawaveHealthCommand, { signal }),
        placeholderData: keepPreviousData
    }))

    type Section = { title: string; columns: number; metrics: SummaryMetric[] }

    let t = $derived(locale.t)
    let error = $derived(stats.error || bandwidth.error || health.error)
    let sections = $derived.by((): Section[] => {
        const summary = stats.data
        const traffic = bandwidth.data
        const healthInfo = health.data
        if (!summary || !traffic || !healthInfo) return []

        const result: Section[] = []
        const runtime = healthInfo.runtimeMetrics
        if (runtime?.length) {
            result.push({
                title: t('home.page.gowave-usage'),
                columns: 4,
                metrics: [
                    {
                        label: t('pm2-metrics.total-memory'),
                        icon: 'PiMemoryDuotone',
                        color: 'cyan',
                        value: prettifyBytesUtil(
                            runtime.reduce((total, process) => total + Number(process.rss), 0),
                            true
                        )
                    },
                    {
                        label: t('pm2-metrics.heap-used'),
                        icon: 'PiTreeStructureDuotone',
                        color: 'green',
                        value: prettifyBytesUtil(
                            runtime.reduce((total, process) => total + Number(process.heapUsed), 0),
                            true
                        )
                    },
                    {
                        label: t('pm2-metrics.goroutines'),
                        icon: 'PiGearSixDuotone',
                        color: 'blue',
                        value: runtime.reduce(
                            (total, process) => total + Number(process.activeHandles),
                            0
                        )
                    },
                    {
                        label: t('pm2-metrics.uptime'),
                        icon: 'PiTimerDuotone',
                        color: 'orange',
                        value: getXrayUptimeUtil(
                            runtime.reduce(
                                (longest, process) => Math.max(longest, Number(process.uptime)),
                                0
                            )
                        )
                    }
                ]
            })
        }

        const periods: {
            label: string
            period: string
            data: typeof traffic.bandwidthLastTwoDays
            icon: IconName
            color: string
        }[] = [
            {
                label: t('common.field.today'),
                icon: 'PiCalendarDotDuotone',
                color: 'blue',
                period: t('bandwidth-metrics.from-yesterday'),
                data: traffic.bandwidthLastTwoDays
            },
            {
                label: t('bandwidth-metrics.last-7-days'),
                icon: 'PiChartDonutDuotone',
                color: 'green',
                period: t('bandwidth-metrics.from-last-week'),
                data: traffic.bandwidthLastSevenDays
            },
            {
                label: t('bandwidth-metrics.last-30-days'),
                icon: 'PiChartPieSliceDuotone',
                color: 'teal',
                period: t('bandwidth-metrics.from-last-month'),
                data: traffic.bandwidthLast30Days
            },
            {
                label: t('bandwidth-metrics.calendar-month'),
                icon: 'PiCalendarDotsDuotone',
                color: 'orange',
                period: t('bandwidth-metrics.from-last-month'),
                data: traffic.bandwidthCalendarMonth
            },
            {
                label: t('bandwidth-metrics.current-year'),
                icon: 'PiCalendarStarDuotone',
                color: 'cyan',
                period: t('bandwidth-metrics.from-last-year'),
                data: traffic.bandwidthCurrentYear
            }
        ]
        result.push({
            title: t('home.page.bandwidth'),
            columns: 3,
            metrics: periods.map(({ label, period, data, icon, color }) => ({
                label,
                value: data.current,
                icon,
                color,
                comparison: trend(data.difference),
                period
            }))
        })

        const { memory, nodes, onlineStats, users } = summary
        result.push(
            {
                title: t('home.page.online-stats'),
                columns: 4,
                metrics: [
                    {
                        label: t('online-metrics.online-now'),
                        icon: 'PiPulseDuotone',
                        color: 'teal',
                        value: onlineStats.onlineNow
                    },
                    {
                        label: t('online-metrics.online-today'),
                        icon: 'PiClockCountdownDuotone',
                        color: 'blue',
                        value: onlineStats.lastDay
                    },
                    {
                        label: t('online-metrics.online-this-week'),
                        icon: 'PiUsersDuotone',
                        color: 'indigo',
                        value: onlineStats.lastWeek
                    },
                    {
                        label: t('online-metrics.never-online'),
                        icon: 'PiProhibitDuotone',
                        color: 'red',
                        value: onlineStats.neverOnline
                    }
                ]
            },
            {
                title: t('common.field.system'),
                columns: 4,
                metrics: [
                    {
                        label: t('simple-metrics.total-online-on-nodes'),
                        icon: 'PiCpuDuotone',
                        color: 'blue',
                        value: nodes.totalOnline
                    },
                    {
                        label: t('common.field.total-traffic'),
                        icon: 'PiChartBarDuotone',
                        color: 'green',
                        value: prettifyBytesUtil(Number(nodes.totalBytesLifetime)) ?? 0
                    },
                    {
                        label: t('simple-metrics.ram-usage'),
                        icon: 'PiMemoryLight',
                        color: 'cyan',
                        value: prettifyBytesUtil(memory.used) ?? 0
                    },
                    {
                        label: t('use-nodes-table-widget.total-ram'),
                        icon: 'PiMemoryFill',
                        color: 'cyan',
                        value: prettifyBytesUtil(memory.total) ?? 0
                    }
                ]
            },
            {
                title: t('user-table.widget.table-title'),
                columns: 5,
                metrics: userMetrics(users, t)
            }
        )
        return result
    })

    function trend(difference: number | string): string {
        if (typeof difference === 'string') return difference
        return formatPercentage(difference, {
            prefix: difference > 0 ? '+' : '',
            precision: 0,
            thousandSeparator: ' '
        })
    }

    function refresh(): void {
        void stats.refetch()
        void bandwidth.refetch()
        void health.refetch()
    }
</script>

<svelte:head><title>{t('constants.home')} | Gowave</title></svelte:head>

<main class="home">
    {#if error && !sections.length}
        <p role="alert">{error.message}</p>
        <Button onclick={refresh}>{t('common.action.try-again')}</Button>
    {:else if !sections.length}
        <RouteLoading label={t('common.message.loading')} />
    {:else}
        {#each sections as section (section.title)}
            <section class={home['section']}>
                <h2 class={home['title']}>{section.title}</h2>
                <div class="metrics" style:--metric-columns={section.columns}>
                    {#each section.metrics as metric, index (metric.label)}
                        <div class={home['card']} style:--card-index={index}>
                            <MetricCard {...metric} />
                        </div>
                    {/each}
                </div>
            </section>
        {/each}
        <Disclaimer />
    {/if}
</main>

<style>
    .home {
        display: flex;
        flex-direction: column;
        gap: var(--mantine-spacing-sm);
    }

    h2 {
        margin: var(--mantine-spacing-xs) 0;
        font-size: var(--mantine-h4-font-size);
        font-weight: var(--mantine-h4-font-weight);
        line-height: var(--mantine-h4-line-height);
    }

    .metrics {
        display: grid;
        grid-template-columns: minmax(0, 1fr);
        gap: var(--mantine-spacing-xs);
    }

    @media (min-width: 40em) {
        .metrics {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }

    @media (min-width: 80em) {
        .metrics {
            grid-template-columns: repeat(var(--metric-columns), minmax(0, 1fr));
        }
    }
</style>
