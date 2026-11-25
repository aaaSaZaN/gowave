<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { locale } from '../i18n.svelte'
    import RouteLoading from '../RouteLoading.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import EmptyState from '../ui/EmptyState.svelte'
    import MetricCard from '../ui/MetricCard.svelte'
    import PageHeader from '../ui/PageHeader.svelte'
    import { httpStats } from './queries'

    let t = $derived(locale.t)
    const query = createQuery(() => httpStats)
    let total = $derived(query.data?.total ?? 0)
    let routes = $derived(query.data?.routes ?? [])
    const tones: Record<string, string> = {
        GET: 'teal',
        POST: 'blue',
        PUT: 'orange',
        PATCH: 'yellow',
        DELETE: 'red',
        OPTIONS: 'grape',
        HEAD: 'indigo'
    }
    let peak = $derived(routes.reduce((max, { count }) => Math.max(max, count), 0))

    function format(value: number): string {
        return value.toLocaleString(locale.language)
    }
</script>

<svelte:head><title>{t('constants.http-stats')} | Gowave</title></svelte:head>
<main class="http-page">
    {#if query.isPending}
        <RouteLoading label={t('common.message.loading')} />
    {:else}
        <PageHeader
            title={t('constants.http-stats')}
            description={t('http-stats.page.component.http-stats-description')}
            icon="TbApi"
        >
            <ActionIcon
                icon="TbRefresh"
                tone="cyan"
                label={t('common.action.update')}
                disabled={query.isFetching}
                onclick={() => void query.refetch()}
            />
        </PageHeader>

        {#if query.error}
            <p role="alert">{query.error.message}</p>
        {/if}
        {#if query.data}
            <div class="metrics">
                <MetricCard
                    label={t('http-stats-table.widget.total-requests')}
                    value={format(total)}
                    icon="TbTrendingUp"
                    color="cyan"
                    valueFont="text"
                />
                <MetricCard
                    label={t('http-stats-table.widget.tracked-routes')}
                    value={format(routes.length)}
                    icon="TbRoute"
                    color="blue"
                    valueFont="text"
                />
                <MetricCard
                    label={t('http-stats-table.widget.peak-requests')}
                    value={format(peak)}
                    icon="TbFlame"
                    color="teal"
                    valueFont="text"
                />
                <MetricCard
                    label={t('http-stats-table.widget.avg-route')}
                    value={format(routes.length ? Math.round(total / routes.length) : 0)}
                    icon="TbDivide"
                    color="grape"
                    valueFont="text"
                />
            </div>
            {#if routes.length}
                <ul aria-label={t('constants.http-stats')}>
                    {#each routes as entry (`${entry.method} ${entry.route}`)}
                        {@const share = total > 0 ? entry.count / total : 0}
                        {@const tone = tones[entry.method] ?? 'gray'}
                        <li
                            style:--method-color={`var(--mantine-color-${tone}-6)`}
                            style:--badge-color={`var(--mantine-color-${tone}-light-color)`}
                        >
                            <span
                                class="fill"
                                style:width={`${Math.max(1.5, share * 100)}%`}
                            ></span>
                            <div class="row">
                                <code class="method">{entry.method}</code>
                                <code class="path" title={entry.route}>{entry.route}</code>
                                <div class="amount">
                                    <span class="share">{(share * 100).toFixed(1)}%</span>
                                    <strong>{format(entry.count)}</strong>
                                </div>
                            </div>
                        </li>
                    {/each}
                </ul>
            {:else}
                <EmptyState icon="TbRoute" label={t('common.message.no-data-available')} />
            {/if}
        {/if}
    {/if}
</main>

<style>
    .http-page,
    .metrics,
    ul {
        display: grid;
        min-width: 0;
        gap: var(--mantine-spacing-md);
    }
    ul {
        list-style: none;
        margin: 0;
        padding: var(--mantine-spacing-md);
        gap: var(--mantine-spacing-xs);
        background: var(--gw-raised-bg);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
    }
    li {
        position: relative;
        overflow: hidden;
        border-radius: var(--mantine-radius-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.06);
        background: var(--gw-raised-bg);
        padding: 10px 14px;
        transition:
            background 150ms ease,
            transform 150ms ease;
    }
    li:hover {
        background: rgba(var(--gw-fg-rgb), 0.04);
        transform: translateX(2px);
    }
    .fill {
        position: absolute;
        inset: 0 auto 0 0;
        background: var(--method-color);
        border-radius: inherit;
        opacity: 0.14;
        pointer-events: none;
    }
    .row {
        position: relative;
        display: flex;
        gap: var(--mantine-spacing-sm);
        align-items: center;
    }
    .amount {
        display: flex;
        align-items: baseline;
        flex: none;
        gap: var(--mantine-spacing-xs);
    }
    code {
        font-family: var(--mantine-font-family-monospace);
    }
    .path {
        flex: 1;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        font-weight: 500;
        font-size: var(--mantine-font-size-sm);
    }
    .method {
        flex: none;
        min-width: 72px;
        height: 26px;
        line-height: 24px;
        padding-inline: 6px;
        border: 1px solid color-mix(in srgb, var(--method-color) 30%, transparent);
        border-radius: var(--mantine-radius-sm);
        color: var(--badge-color);
        background: color-mix(in srgb, var(--method-color) 15%, transparent);
        font-size: var(--mantine-font-size-sm);
        text-align: center;
        font-weight: 700;
        letter-spacing: 0.02em;
    }
    .share {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        font-variant-numeric: tabular-nums;
    }
    @media (min-width: 48em) {
        .metrics {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }
    @media (min-width: 75em) {
        .metrics {
            grid-template-columns: repeat(4, minmax(0, 1fr));
        }
    }
</style>
