<script lang="ts">
    import ColorHash from '@shared/lib/color'
    import { formatInt } from '@shared/utils/misc/number'
    import { createQuery } from '@tanstack/svelte-query'
    import dayjs from 'dayjs'
    import type { Options } from 'highcharts'
    import { MediaQuery } from 'svelte/reactivity'
    import { locale } from '../i18n.svelte'
    import RouteLoading from '../RouteLoading.svelte'
    import Chart from '../ui/Chart.svelte'
    import ChartPlaceholder from '../ui/ChartPlaceholder.svelte'
    import Icon from '../ui/Icon.svelte'
    import Leaderboard from '../ui/Leaderboard.svelte'
    import MobileTableNotice from '../users/MobileTableNotice.svelte'
    import type { Column } from './columns'
    import InspectorTable from './InspectorTable.svelte'
    import { type HistoryRecord, historyStats, loadHistory } from './queries'

    import { historyStorage } from './table-state'

    const smallScreen = new MediaQuery('(max-width: 500px)')
    let t = $derived(locale.t)
    const stats = createQuery(() => historyStats)
    const appColors = new ColorHash({ lightness: [0.65, 0.65, 0.65] })
    let hours = $derived(stats.data?.hourlyRequestStats ?? [])
    let requests = $derived(t('srh-inspector-metrics.widget.requests'))
    const axisStyle = {
        labels: { style: { color: 'var(--mantine-color-text)' } },
        gridLineColor: 'var(--mantine-color-gray-6)',
        lineColor: 'var(--mantine-color-gray-6)'
    }
    let hourlyChart = $derived<Options>({
        chart: {
            type: 'column',
            backgroundColor: 'transparent',
            height: 300,
            style: { fontFamily: 'inherit' }
        },
        time: { timezone: Intl.DateTimeFormat().resolvedOptions().timeZone },
        accessibility: { enabled: false },
        title: { text: '' },
        xAxis: {
            ...axisStyle,
            type: 'datetime',
            dateTimeLabelFormats: { hour: '%H:00' },
            units: [['hour', [1, 2, 3, 4, 6, 8, 12]]]
        },
        yAxis: {
            ...axisStyle,
            title: { text: requests, style: { color: 'var(--mantine-color-text)' } }
        },
        tooltip: {
            shared: true,
            backgroundColor: 'var(--mantine-color-body)',
            borderColor: 'var(--mantine-color-gray-4)',
            headerFormat: '',
            style: { color: 'var(--mantine-color-text)' },
            pointFormatter() {
                return `<b>${dayjs(this.x).format('DD.MM.YYYY, HH:mm')}</b><br/>${requests}: <b>${this.y}</b>`
            }
        },
        plotOptions: {
            column: {
                borderWidth: 0,
                borderRadius: 4,
                pointPadding: 0.1,
                groupPadding: 0.1,
                color: 'var(--mantine-color-indigo-6)',
                states: { hover: { color: 'var(--mantine-color-indigo-3)' } },
                dataLabels: { enabled: false }
            }
        },
        legend: { enabled: false },
        credits: { enabled: false },
        series: [
            {
                type: 'column',
                name: requests,
                data: hours.map(({ dateTime, requestCount }) => [dateTime.getTime(), requestCount])
            }
        ]
    })
    let columns: Column<HistoryRecord>[] = $derived([
        { key: 'userId', label: 'User ID', filter: 'number', width: 130 },
        {
            key: 'requestIp',
            label: t('use-srh-inspector-table-columns.request-ip'),
            filter: 'text'
        },
        { key: 'userAgent', label: t('common.field.user-agent'), filter: 'text', width: 400 },
        { key: 'requestAt', label: t('common.field.request-at'), width: 250 },
        {
            key: 'srrResponseType',
            label: t('use-srh-inspector-table-columns.response-type'),
            filter: 'text',
            width: 200
        },
        {
            key: 'srrRuleName',
            label: t('use-srh-inspector-table-columns.rule-name'),
            filter: 'text',
            width: 200
        },
        { key: 'id', label: 'ID', width: 80 }
    ])

    function count(value: number): string {
        return formatInt(value, { thousandSeparator: ' ' })
    }
</script>

<svelte:head><title>{t('constants.srh-inspector')} | Gowave</title></svelte:head>
<main>
    {#if smallScreen.current}
        <MobileTableNotice />
    {/if}
    {#if stats.isPending}
        <RouteLoading compact />
    {/if}
    {#if stats.error}
        <p role="alert">{stats.error.message}</p>
    {/if}
    {#if stats.data}
        <div class="panels">
            <section aria-labelledby="request-apps-heading">
                <h2 id="request-apps-heading">
                    <span class="app-icon"><Icon name="PiAppWindowDuotone" size={20} /></span>
                    <span class="panel-title"
                        >{t('hwid-inspector-metrics.widget.app-distribution')}</span
                    >
                </h2>
                <Leaderboard
                    items={stats.data.byParsedApp.map(({ app, count }) => ({
                        name: app || 'Unknown',
                        total: count,
                        color: appColors.hex(app || 'Unknown')
                    }))}
                    format={count}
                    maxHeight={300}
                    emptyText={t('hwid-inspector-metrics.widget.no-app-data')}
                />
            </section>
            <section aria-labelledby="hourly-request-heading">
                <h2 id="hourly-request-heading">
                    <span class="chart-icon"><Icon name="PiChartBar" size={20} /></span>
                    <span class="panel-title"
                        >{t('srh-inspector-metrics.widget.hourly-request-statistics')}</span
                    >
                </h2>
                {#if hours.length}
                    <div class="hourly-chart">
                        <Chart options={hourlyChart} />
                    </div>
                {:else}
                    <ChartPlaceholder
                        height={300}
                        icon="PiChartBar"
                        iconSize={48}
                        label={t('srh-inspector-metrics.widget.no-hourly-data-available')}
                    />
                {/if}
            </section>
        </div>
    {/if}
    <InspectorTable
        icon="TbReportAnalytics"
        title={t('common.field.subscription-request-history')}
        setup={historyStorage}
        {columns}
        rowKey={(row) => String(row.id)}
        ipAddress={(row) => row.requestIp}
        load={loadHistory}
    />
</main>

<style>
    main {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    @media (min-width: 64em) {
        .panels {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }
    .panels {
        display: grid;

        gap: var(--mantine-spacing-lg);
    }
    section {
        min-width: 0;
        padding: var(--mantine-spacing-lg);
        background: linear-gradient(
            135deg,
            var(--mantine-color-dark-6),
            var(--mantine-color-dark-7)
        );
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
    }
    .app-icon,
    .chart-icon {
        display: inline-grid;
        place-items: center;
        flex: none;
        width: 34px;
        height: 34px;
        border: 1px solid;
        border-radius: var(--mantine-radius-md);
    }
    .app-icon {
        color: var(--mantine-color-teal-outline);
    }
    .chart-icon {
        color: var(--mantine-color-blue-outline);
    }
    h2 {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        margin: 0 0 var(--mantine-spacing-lg);
        font-size: var(--mantine-font-size-lg);
        line-height: var(--mantine-line-height-lg);
        font-weight: 600;
    }
    .panel-title {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    section :global(.placeholder svg) {
        color: var(--mantine-color-gray-4);
    }
    .hourly-chart {
        min-height: 300px;
    }
</style>
