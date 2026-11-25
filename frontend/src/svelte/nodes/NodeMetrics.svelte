<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { MediaQuery } from 'svelte/reactivity'
    import { locale } from '../i18n.svelte'
    import RouteLoading from '../RouteLoading.svelte'
    import Icon from '../ui/Icon.svelte'
    import MetricCard from '../ui/MetricCard.svelte'
    import PageHeader from '../ui/PageHeader.svelte'
    import { metricsSummary } from './analytics'
    import NodeMetricCard from './NodeMetricCard.svelte'
    import { nodeMetricsQuery } from './queries'

    let activeNode = $state('')
    let infoOpen = $state(false)
    let infoDismissed = $state(false)
    const desktop = new MediaQuery('(width > 64em)')
    const heights = $state<Record<string, number>>({})
    const tooltipId = $props.id()
    let t = $derived(locale.t)
    const metrics = createQuery(() => ({
        ...nodeMetricsQuery,
        refetchInterval: 30_000
    }))
    let nodes = $derived(metrics.data?.nodes ?? [])
    let summary = $derived(metricsSummary(nodes))
    const masonry = $derived.by(() => {
        if (!desktop.current || nodes.some(({ nodeUuid }) => !heights[nodeUuid])) return

        const columns = [0, 0] as [number, number]
        const cards = nodes.map(({ nodeUuid }) => {
            const column = columns[0] <= columns[1] ? 0 : 1
            const top = columns[column]
            columns[column] += heights[nodeUuid] ?? 0
            return { column, top }
        })
        return { cards, height: Math.max(...columns) }
    })
</script>

<svelte:head><title>{t('constants.nodes-metrics')} | Gowave</title></svelte:head>
<main class="metrics-page">
    <PageHeader title={t('node-metrics.title')} icon="PiSpeedometer" />
    {#if metrics.isPending}
        <RouteLoading height="80vh" label={t('common.message.loading')} />
    {/if}
    {#if metrics.error}
        <p role="alert">{metrics.error.message}</p>
    {/if}
    {#if metrics.data}
        {#if !nodes.length}
            <div class="empty">
                <span><Icon name="PiProhibitDuotone" size={32} /></span>
                <p>{t('node-metrics.empty')}</p>
            </div>
        {:else}
            <div class="summary">
                <MetricCard
                    label={t('node-metrics.total')}
                    value={summary.total}
                    icon="TbServer"
                    color="indigo"
                />
                <MetricCard
                    label={t('node-metrics.active')}
                    value={summary.active}
                    icon="PiPulseDuotone"
                    color="teal"
                />
                <MetricCard
                    label={t('node-metrics.online')}
                    value={summary.online}
                    icon="PiUsersDuotone"
                    color="blue"
                />
                <MetricCard
                    label={t('node-metrics.inbounds')}
                    value={summary.inbounds}
                    icon="PiGlobeSimple"
                    color="violet"
                />
            </div>
            <div class="metrics-source">
                <div class="metrics-label">
                    <div
                        class="info"
                        class:info-open={infoOpen}
                        class:info-dismissed={infoDismissed}
                    >
                        <button
                            type="button"
                            aria-label={t('node-metrics.info')}
                            aria-describedby={tooltipId}
                            onclick={() => {
                                infoDismissed = false
                                infoOpen = !infoOpen
                            }}
                            onblur={() => {
                                infoDismissed = false
                                infoOpen = false
                            }}
                            onkeydown={(event) => {
                                if (event.key === 'Escape') {
                                    infoOpen = false
                                    infoDismissed = true
                                }
                            }}
                        >
                            <Icon name="PiInfo" size={19} />
                        </button>
                        <div role="tooltip" id={tooltipId}>
                            <strong
                                ><Icon name="PiInfo" size={12} /> {t('node-metrics.info')}</strong
                            >
                            <ul>
                                <li>{t('node-metrics.interval')}</li>
                                <li>{t('node-metrics.partial')}</li>
                                <li>
                                    {t('node-metrics.since-start')}
                                </li>
                            </ul>
                        </div>
                    </div>
                    {t('node-metrics.source')}
                </div>
                <span class="metrics-live"><i aria-hidden="true"></i>{t('node-metrics.live')}</span>
            </div>
            <div
                class="nodes"
                class:masonry={masonry}
                style:height={masonry ? `${masonry.height}px` : undefined}
            >
                {#each nodes as node, index (node.nodeUuid)}
                    {@const position = masonry?.cards[index]}
                    <div
                        class="node-item"
                        bind:offsetHeight={heights[node.nodeUuid]}
                        style:inset-block-start={position ? `${position.top}px` : undefined}
                        style:inset-inline-start={position ? `${position.column * 50}%` : undefined}
                    >
                        <NodeMetricCard {node} open={() => (activeNode = node.nodeUuid)} />
                    </div>
                {/each}
            </div>
        {/if}
    {/if}
</main>

{#if activeNode}
    {#await import('./NodeDetails.svelte') then { default: NodeDetails }}
        <NodeDetails nodeId={activeNode} onclose={() => (activeNode = '')} />
    {/await}
{/if}

<style>
    .metrics-page,
    .summary,
    .nodes {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    .summary {
        gap: var(--mantine-spacing-xs);
    }
    .nodes {
        align-items: start;
        position: relative;
        gap: 0;
    }
    .node-item {
        min-width: 0;
        padding: 5px;
    }
    .masonry .node-item {
        position: absolute;
        width: 50%;
    }
    .metrics-source {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        padding: var(--mantine-spacing-md);
        border: 1px solid color-mix(in srgb, var(--mantine-color-blue-6) 20%, transparent);
        border-radius: var(--mantine-radius-md);
        background: color-mix(in srgb, var(--mantine-color-blue-6) 5%, transparent);
    }
    .metrics-label,
    .metrics-live {
        display: flex;
        align-items: center;
    }
    .metrics-label {
        gap: var(--mantine-spacing-sm);
        color: var(--mantine-color-blue-4);
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
    }
    .metrics-live {
        gap: var(--mantine-spacing-xs);
        color: var(--mantine-color-gray-4);
        font-size: var(--mantine-font-size-xs);
    }
    .metrics-live i {
        width: 8px;
        height: 8px;
        border-radius: 50%;
        background: var(--mantine-color-teal-5);
        animation: pulse 1s ease-out infinite;
    }
    @keyframes pulse {
        from {
            box-shadow: 0 0 0 0 var(--mantine-color-teal-5);
        }
        to {
            box-shadow: 0 0 0 6px transparent;
        }
    }
    .info {
        position: relative;
    }
    .info button {
        display: grid;
        width: 28px;
        height: 28px;
        min-height: 28px;
        place-items: center;
        padding: 0;
        border: 0;
        border-radius: var(--mantine-radius-md);
        color: var(--mantine-color-teal-light-color);
        background: var(--mantine-color-teal-light);
        cursor: pointer;
    }
    .info [role="tooltip"] {
        visibility: hidden;
        position: absolute;
        z-index: 6;
        inset-block-start: 100%;
        inset-inline-start: 0;
        width: min(20rem, calc(100vw - 4.5rem));
        padding: var(--mantine-spacing-sm);
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        box-shadow: var(--mantine-shadow-lg);
        background: var(--gw-raised-bg);
        color: var(--mantine-color-text);
    }
    .info-open [role="tooltip"],
    .info:not(.info-dismissed):hover [role="tooltip"],
    .info:not(.info-dismissed):has(button:focus-visible) [role="tooltip"] {
        visibility: visible;
    }
    .info strong {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .info ul {
        margin: var(--mantine-spacing-xs) 0 0;
        padding-inline-start: var(--mantine-spacing-md);
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }
    .info li + li {
        margin-block-start: var(--mantine-spacing-xs);
    }
    .empty {
        display: grid;
        justify-items: center;
        gap: var(--mantine-spacing-md);
        padding: var(--mantine-spacing-xl);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
        color: var(--mantine-color-gray-5);
        font-size: var(--mantine-font-size-lg);
    }
    .empty span {
        display: grid;
        width: 44px;
        height: 44px;
        place-items: center;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-gray-light);
    }
    .empty p {
        margin: 0;
    }
    @media (width < 36em) {
        .metrics-live i {
            display: none;
        }
    }
    @media (prefers-reduced-motion: reduce) {
        .metrics-live i {
            animation: none;
        }
    }
    @media (min-width: 62em) {
        .summary {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }
    @media (width > 64em) {
        .nodes {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }
    @media (min-width: 75em) {
        .summary {
            grid-template-columns: repeat(4, minmax(0, 1fr));
        }
    }
</style>
