<script lang="ts">
    import { prettifyBytesUtil } from '@shared/utils/bytes/pretty-bytes/pretty-bytes.util'
    import { formatTimeUtil } from '@shared/utils/time-utils/format-time.util'
    import { getDefaultDateRange } from '@shared/utils/time-utils/get-default-date-range/get-default-date-range.util'
    import { createQuery } from '@tanstack/svelte-query'
    import { locale } from '../i18n.svelte'
    import RouteLoading from '../RouteLoading.svelte'
    import { p } from '../router'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Chart from '../ui/Chart.svelte'
    import Icon from '../ui/Icon.svelte'
    import Leaderboard from '../ui/Leaderboard.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import SelectField from '../ui/SelectField.svelte'
    import { flag, usageDays, usageForDay, usageSummary } from './analytics'
    import { usageQuery } from './queries'
    import UsageFilters from './UsageFilters.svelte'
    import UsageSummary from './UsageSummary.svelte'
    import { usageChartOptions } from './usage-chart'

    let {
        userId,
        close
    }:
        | { userId?: undefined; close?: undefined }
        | {
              userId: number
              close: () => void
          } = $props()

    const initial = getDefaultDateRange()
    const limits = [5, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100].map((limit) => ({
        value: String(limit),
        label: `Top ${limit}`
    }))
    let t = $derived(locale.t)
    let filters = $state({ ...initial, topNodesLimit: 20 })
    let selectedDay = $state<number | undefined>()
    const stats = createQuery(() => usageQuery(filters, userId))
    let categories = $derived(stats.data?.categories ?? [])
    let series = $derived(stats.data?.series ?? [])
    let isUser = $derived(userId !== undefined)
    let hasChart = $derived(categories.length > 0 && series.length > 0)
    let days = $derived(usageDays(categories, series))
    let details = $derived(selectedDay === undefined ? [] : usageForDay(series, selectedDay))
    let totalTraffic = $derived(formatBytes(usageSummary(details.map(({ bytes }) => bytes)).total))
    let chart = $derived(
        usageChartOptions({
            categories,
            series,
            language: locale.language,
            hint: t('common.message.click-to-see-all'),
            user: isUser,
            selectDay: (index) => {
                if (days[index]?.total) selectedDay = index
            }
        })
    )

    function formatBytes(value: number): string {
        return prettifyBytesUtil(value) ?? '0 B'
    }
</script>

{#snippet report()}
    <UsageFilters
        page={!isUser}
        hideRefresh={isUser}
        range={filters}
        busy={stats.isFetching}
        apply={(range) => {
            if (filters.start === range.start && filters.end === range.end) void stats.refetch()
            else filters = { ...filters, ...range }
        }}
        refresh={() => void stats.refetch()}
    >
        {#snippet control()}
            <div class="top-limit">
                <SelectField
                    name="topNodesLimit"
                    label={t('statistic-nodes.component.top-nodes')}
                    placeholder=""
                    leading="TbServer2"
                    leadingSize={20}
                    size="md"
                    options={limits}
                    value={String(filters.topNodesLimit)}
                    searchable={false}
                    clearable={false}
                    hideLabel
                    onchoose={(value) => (filters = { ...filters, topNodesLimit: Number(value) })}
                />
            </div>
        {/snippet}
    </UsageFilters>

    {#if stats.isPending}
        <RouteLoading compact />
    {:else if stats.isError}
        <p role="alert">{stats.error.message}</p>
        <button type="button" onclick={() => void stats.refetch()}>
            {t('common.action.try-again')}
        </button>
    {:else}
        <div class="summary">
            <UsageSummary
                values={stats.data.sparklineData}
                title={t(
                    isUser
                        ? 'common.field.total-traffic'
                        : 'statistic-nodes.component.total-traffic'
                )}
            />
            <section aria-label={t('statistic-nodes.component.top-nodes')}>
                <Leaderboard
                    maxHeight={230}
                    emptyText={t('common.message.no-data-available')}
                    format={formatBytes}
                    items={stats.data.topNodes.map(({ uuid, name, color, countryCode, total }) => ({
                        name,
                        color,
                        total,
                        flag: flag(countryCode),
                        href: p('/dashboard/open/node/:id', { params: { id: uuid } })
                    }))}
                />
            </section>
        </div>
        <section
            aria-label="Daily traffic by node"
            class:empty={!hasChart}
            class="daily"
            style:--empty-chart-height={isUser ? '400px' : '600px'}
        >
            {#if hasChart}
                <Chart options={chart} />
            {:else}
                <div class="placeholder">
                    <Icon name="PiEmpty" size="2rem" />
                    <p>{t('common.message.no-data-available-for-the-selected-period')}</p>
                </div>
            {/if}
        </section>
    {/if}
{/snippet}

{#if close}
    <Overlay
        drawer
        width={900}
        icon="TbChartArcs"
        tone="teal"
        title={t('common.field.usage-stats')}
        {close}
    >
        {#snippet actions()}
            <ActionIcon
                icon="TbRefresh"
                size={34}
                iconSize={20}
                tone="cyan"
                label={t('common.action.refresh')}
                pending={stats.isFetching}
                onclick={() => void stats.refetch()}
            />
        {/snippet}
        <div class="user-traffic">
            {@render report()}
        </div>
    </Overlay>
{:else}
    {@render report()}
{/if}

{#if selectedDay !== undefined}
    <Overlay
        icon="TbChartBar"
        tone="teal"
        width={600}
        fullscreenOnMobile={false}
        title={formatTimeUtil({
            time: categories[selectedDay],
            template: 'FULL_DATE',
            language: locale.language
        })}
        subtitle={isUser
            ? `Σ ${totalTraffic}`
            : t('statistic-nodes.component.total-traffic-placeholder', { totalTraffic })}
        close={() => (selectedDay = undefined)}
    >
        <div class="details">
            <table>
                <thead>
                    <tr>
                        <th scope="col">{t('statistic-nodes.component.node')}</th>
                        <th scope="col">{t('statistic-nodes.component.traffic')}</th>
                    </tr>
                </thead>
                <tbody>
                    {#each details as row (row.uuid)}
                        <tr>
                            <td>
                                <a
                                    href={p('/dashboard/open/node/:id', {
                                        params: { id: row.uuid }
                                    })}
                                    ><span class="node-color" style:background={row.color}></span>
                                    {flag(row.countryCode)} {row.name}</a
                                >
                            </td>
                            <td>{formatBytes(row.bytes)}</td>
                        </tr>
                    {/each}
                </tbody>
            </table>
        </div>
    </Overlay>
{/if}

<style>
    .user-traffic {
        display: grid;
        grid-template-columns: minmax(0, 1fr);
        gap: var(--mantine-spacing-md);
        padding-block-start: 1px;
    }
    .summary {
        display: grid;
        grid-template-columns: minmax(0, 1fr);
        gap: var(--mantine-spacing-md);
    }
    @media (min-width: 40em) {
        .summary {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }
    section {
        min-width: 0;
        padding: var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }
    .daily {
        padding: var(--mantine-spacing-xs);
    }
    .daily.empty {
        min-height: calc(var(--empty-chart-height) + 2 * var(--mantine-spacing-xs) + 2px);
    }
    section.empty {
        display: grid;
        place-items: center;
    }
    .placeholder {
        display: grid;
        justify-items: center;
        gap: var(--mantine-spacing-xs);
        color: var(--mantine-color-dimmed);
        text-align: center;
        font-size: var(--mantine-font-size-sm);
    }
    .daily .placeholder {
        gap: 8px;
        font-size: var(--mantine-font-size-md);
    }
    .daily .placeholder :global(svg) {
        color: var(--mantine-color-text);
    }
    .placeholder p {
        margin: 0;
    }
    .top-limit {
        width: 150px;
    }
    .details {
        height: min(400px, 55dvh);
        overflow: auto;
        scrollbar-gutter: stable;
    }
    table {
        width: 100%;
        border-collapse: collapse;
        border: 1px solid var(--gw-border);
    }
    th,
    td {
        text-align: start;
        padding: var(--mantine-spacing-xs);
        border-block-end: 1px solid var(--gw-border);
    }
    th:last-child,
    td:last-child {
        text-align: end;
    }
    tbody tr:nth-child(odd) {
        background: var(--gw-raised-bg);
    }
    td a {
        display: inline-flex;
        align-items: center;
        gap: 8px;
    }
    .node-color {
        width: 12px;
        height: 12px;
        flex: none;
        border-radius: 50%;
    }
</style>
