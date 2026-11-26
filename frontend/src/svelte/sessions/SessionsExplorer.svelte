<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { onDestroy } from 'svelte'
    import { locale } from '../i18n.svelte'
    import NodeDetails from '../nodes/NodeDetails.svelte'
    import { nodesQuery } from '../nodes/queries'
    import RouteLoading from '../RouteLoading.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import EmptyState from '../ui/EmptyState.svelte'
    import FormCard from '../ui/FormCard.svelte'
    import Icon from '../ui/Icon.svelte'
    import MetricCard from '../ui/MetricCard.svelte'
    import PageHeader from '../ui/PageHeader.svelte'
    import Tooltip from '../ui/Tooltip.svelte'
    import UserDialog from '../users/UserDialog.svelte'
    import { loadNodeSessions } from './queries'
    import SessionGroup from './SessionGroup.svelte'
    import SessionScanProgress from './SessionScanProgress.svelte'
    import { type ScanProgress, scanSessions } from './scan'

    const nodes = createQuery(() => ({ ...nodesQuery, refetchInterval: 15_000 }))
    let online = $derived(nodes.data?.filter((node) => node.isConnected && !node.isDisabled) ?? [])
    let controller: AbortController | undefined
    let phase = $state<'idle' | 'running' | 'completed' | 'failed'>('idle')
    let progress = $state<ScanProgress>()
    let result = $state.raw<Awaited<ReturnType<typeof scanSessions>>>()
    let search = $state('')
    let term = $derived(search.trim())
    let page = $state(0)
    const pageSize = 24
    let matches = $derived(
        result?.users.filter(
            (user) => !term || user.nodes.some(({ ips }) => ips.some(({ ip }) => ip.includes(term)))
        ) ?? []
    )
    let pages = $derived(Math.max(1, Math.ceil(matches.length / pageSize)))
    let highestCount = $derived(result?.users[0]?.uniqueIps ?? 0)
    let lowestCount = $derived(result?.users.at(-1)?.uniqueIps ?? 0)
    let visible = $derived(matches.slice(page * pageSize, (page + 1) * pageSize))
    let t = $derived(locale.t)
    let selectedUserId = $state<number>()
    let selectedNodeId = $state('')
    let failure = $state('')
    let now = $state(Date.now())
    let format = $derived(new Intl.NumberFormat(locale.language))

    $effect(() => {
        if (phase !== 'completed') return
        now = Date.now()
        const clock = window.setInterval(() => (now = Date.now()), 30_000)
        return () => window.clearInterval(clock)
    })

    onDestroy(() => controller?.abort())

    function reset(): void {
        controller?.abort()
        controller = undefined
        phase = 'idle'
        progress = undefined
        result = undefined
        search = ''
        page = 0
        failure = ''
    }

    async function start(): Promise<void> {
        if (phase === 'running' || !online.length) return
        const targets = online
        reset()
        const aborter = new AbortController()
        controller = aborter
        phase = 'running'
        try {
            const data = await scanSessions(
                targets,
                loadNodeSessions,
                (next) => (progress = next),
                aborter.signal
            )
            if (aborter.signal.aborted) return
            result = data
            phase = data.completed > 0 ? 'completed' : 'failed'
        } catch (error) {
            if (aborter.signal.aborted) return
            failure = error instanceof Error ? error.message : String(error)
            phase = 'failed'
        } finally {
            if (controller === aborter) controller = undefined
        }
    }

    function countTone(count: number): 'teal' | 'yellow' | 'red' {
        const range = highestCount - lowestCount
        if (!range) return 'teal'
        const rank = (count - lowestCount) / range
        if (rank >= 0.66) return 'red'
        return rank >= 0.33 ? 'yellow' : 'teal'
    }
</script>

<svelte:head><title>{t('constants.sessions-explorer')} | Gowave</title></svelte:head>
<main>
    {#if nodes.isPending}
        <RouteLoading label={t('common.message.loading')} />
    {:else}
        <PageHeader title={t('constants.sessions-explorer')} icon="TbRadar2">
            {#if phase === 'completed'}
                <ActionIcon
                    icon="TbRefresh"
                    label={t('sessions-explorer.widget.restart-scan')}
                    tone="teal"
                    iconSize={20}
                    disabled={!online.length}
                    onclick={() => void start()}
                />
                <ActionIcon
                    icon="TbTrash"
                    label={t('sessions-explorer.widget.clear-results')}
                    tone="red"
                    iconSize={20}
                    onclick={reset}
                />
            {/if}
        </PageHeader>

        {#if nodes.error}
            <p role="alert">{nodes.error.message}</p>
        {/if}
        {#if phase === 'idle'}
            <FormCard
                class="requirements"
                title={t('active-sessions-drawer.widget.requirements')}
                icon="TbAlertTriangle"
                tone="yellow"
            >
                <div class="requirement">
                    <span><Icon name="TbClock" size={16} /></span>
                    <p>
                        {#each t('active-sessions-drawer.widget.warning-activity').split(
                            /<\/?highlight>/g
                        ) as part, index}
                            <span class:highlight={index % 2 === 1}>{part}</span>
                        {/each}
                    </p>
                </div>
                <div class="requirement split-requirement">
                    <span class="yellow"><Icon name="TbRadar" size={16} /></span>
                    <p>{t('sessions-explorer-idle.description-1')}</p>
                </div>
            </FormCard>
            <Button
                variant="light"
                color="teal"
                block
                icon="TbRadar2"
                iconSize={20}
                size="md"
                disabled={!online.length}
                onclick={() => void start()}
            >
                {t('sessions-explorer-idle.start-exploring')}
            </Button>
        {:else if phase === 'running' && progress}
            <SessionScanProgress explorer={progress}>
                <Button variant="subtle" color="gray" icon="TbX" onclick={reset}>
                    {t('common.action.cancel')}
                </Button>
            </SessionScanProgress>
        {:else if phase === 'failed'}
            <FormCard
                title={t('sessions-explorer.widget.all-nodes-failed-to-return-session-data')}
                icon="TbAlertTriangle"
                tone="red"
            >
                {#if failure}
                    <p role="alert">{failure}</p>
                {/if}
                <div class="button-row">
                    <Button
                        variant="soft"
                        color="teal"
                        icon="TbRefresh"
                        disabled={!online.length}
                        onclick={() => void start()}
                    >
                        {t('common.action.try-again')}
                    </Button>
                    <Button onclick={reset}>{t('sessions-explorer.widget.clear-results')}</Button>
                </div>
            </FormCard>
        {:else if result}
            <div class="metrics">
                <MetricCard
                    label={t('sessions-explorer.widget.total-ips')}
                    value={format.format(result.totalIps)}
                    icon="TbNetwork"
                    color="blue"
                />
                <MetricCard
                    label={t('sessions-explorer.widget.unique-ips')}
                    value={format.format(result.uniqueIps)}
                    icon="TbFingerprint"
                    color="teal"
                />
                <MetricCard
                    label={t('sessions-explorer.widget.nodes-explored')}
                    value={format.format(result.completed)}
                    icon="TbServer"
                    color="indigo"
                />
                <MetricCard
                    label={t('sessions-explorer.widget.total-users')}
                    value={format.format(result.users.length)}
                    icon="TbUser"
                    color="violet"
                />
            </div>
            {#if result.users.length}
                <div class="search-row">
                    <div class="search">
                        <span class="search-icon" aria-hidden="true"><Icon name="TbSearch" /></span>
                        <input
                            class="gw-input"
                            type="search"
                            aria-label={t('sessions-explorer.widget.search-by-ip')}
                            placeholder={t('sessions-explorer.widget.search-by-ip')}
                            bind:value={search}
                            oninput={() => (page = 0)}
                        >
                        {#if search}
                            <ActionIcon
                                icon="TbX"
                                label={t('action-group.feature.clear-filters')}
                                size={28}
                                iconSize={16}
                                subtle
                                onclick={() => {
                                    search = ''
                                    page = 0
                                }}
                            />
                        {/if}
                    </div>
                    {#if term}
                        <span class="count teal" role="status"
                            >{format.format(matches.length)}</span
                        >
                    {/if}
                </div>
            {/if}
        {/if}
        {#if result?.errors.size}
            <details open={phase === 'failed'}>
                <summary>
                    {t('sessions-explorer-progress.nodes-failed', { count: result.errors.size })}
                </summary>
                <ul>
                    {#each result.errors as [node, error] (node.uuid)}
                        <li>{node.name} · {error}</li>
                    {/each}
                </ul>
            </details>
        {/if}
        {#if phase === 'completed'}
            {#if !visible.length}
                <EmptyState
                    icon="TbRadar"
                    label={term
                        ? t('sessions-explorer.widget.no-results-for-ip', { ip: term })
                        : t('common.message.nothing-found')}
                />
            {:else}
                <div class="results">
                    {#each visible as user (user.userId)}
                        {@const tone = countTone(user.uniqueIps)}
                        {@const uniqueLabel = t('sessions-explorer.widget.unique-ips')}
                        {@const totalLabel = t('sessions-explorer.widget.total-ips')}
                        <article>
                            <header>
                                <h2>
                                    <ActionIcon
                                        icon="PiUserCircle"
                                        label={t('node-active-session.item.widget.view-user')}
                                        tone="cyan"
                                        size={34}
                                        iconSize={20}
                                        onclick={() => (selectedUserId = user.userId)}
                                    />
                                    {user.userId}
                                </h2>
                                <div class="counts">
                                    <Tooltip text={uniqueLabel}>
                                        <span class={['count', tone]}>
                                            <Icon name="TbFingerprint" size={16} />
                                            <span class="sr-only">{uniqueLabel}</span>
                                            {format.format(user.uniqueIps)}
                                        </span>
                                    </Tooltip>
                                    <Tooltip text={totalLabel}>
                                        <span class="count">
                                            <Icon name="TbSum" size={16} />
                                            <span class="sr-only">{totalLabel}</span>
                                            {format.format(user.totalIps)}
                                        </span>
                                    </Tooltip>
                                </div>
                            </header>
                            <div class="groups">
                                {#each user.nodes as { node, ips } (node.uuid)}
                                    {#if ips.length}
                                        <SessionGroup
                                            {node}
                                            {ips}
                                            search={term}
                                            {now}
                                            onnode={(uuid) => (selectedNodeId = uuid)}
                                        />
                                    {/if}
                                {/each}
                            </div>
                        </article>
                    {/each}
                </div>
            {/if}
            {#if pages > 1}
                <div class="button-row pagination">
                    <Button disabled={page === 0} onclick={() => (page -= 1)}>
                        {t('common.action.previous')}
                    </Button>
                    <span
                        >{page + 1}
                        / {pages} · {matches.length.toLocaleString(locale.language)}</span
                    >
                    <Button disabled={page + 1 >= pages} onclick={() => (page += 1)}>
                        {t('common.action.next')}
                    </Button>
                </div>
            {/if}
        {/if}
    {/if}
</main>

{#if selectedUserId !== undefined}
    <UserDialog id={selectedUserId} close={() => (selectedUserId = undefined)} />
{/if}
{#if selectedNodeId}
    <NodeDetails nodeId={selectedNodeId} onclose={() => (selectedNodeId = '')} />
{/if}

<style>
    main :global(.requirements h3) {
        font-size: var(--mantine-h4-font-size);
        font-weight: var(--mantine-h4-font-weight);
        line-height: var(--mantine-h4-line-height);
    }
    .split-requirement {
        padding-block-start: var(--mantine-spacing-md);
        border-block-start: 1px solid rgba(var(--gw-fg-rgb), 0.06);
    }
    main {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    .requirement {
        display: flex;
        gap: var(--mantine-spacing-sm);
        align-items: center;
    }
    .requirement p {
        margin: 0;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
        line-height: var(--mantine-line-height-sm);
    }
    .requirement > span {
        display: grid;
        place-items: center;
        width: 28px;
        height: 28px;
        flex: none;
        border-radius: var(--mantine-radius-md);
        color: var(--mantine-color-cyan-4);
        background: var(--mantine-color-cyan-light);
        border: 1px solid rgb(21 170 191 / 30%);
    }
    .requirement > .yellow {
        color: var(--mantine-color-yellow-4);
        background: var(--mantine-color-yellow-light);
        border-color: rgb(250 176 5 / 30%);
    }
    .requirement .highlight {
        color: var(--mantine-color-bright);
        font-weight: 600;
    }

    .results,
    .metrics {
        display: grid;
        grid-template-columns: minmax(0, 1fr);
        gap: var(--mantine-spacing-sm);
    }
    .metrics {
        gap: var(--mantine-spacing-xs);
    }
    @media (min-width: 36em) {
        .metrics {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }
    @media (min-width: 56em) {
        .results {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }
    @media (min-width: 68em) {
        .metrics {
            grid-template-columns: repeat(4, minmax(0, 1fr));
        }
    }
    @media (min-width: 80em) {
        .results {
            grid-template-columns: repeat(3, minmax(0, 1fr));
        }
    }
    article {
        min-width: 0;
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        padding: var(--mantine-spacing-md);
        background: var(--gw-raised-bg);
    }
    article header,
    article h2,
    .counts,
    .search-row {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    article header {
        justify-content: space-between;
        flex-wrap: wrap;
        padding-block-end: var(--mantine-spacing-xs);
    }
    article h2 {
        margin: 0;
        font-size: var(--mantine-h5-font-size);
        color: var(--mantine-color-bright);
    }
    .groups {
        height: 400px;
        overflow: auto;
    }
    .count {
        display: inline-flex;
        align-items: center;
        gap: 0.35rem;
        padding: 0.18rem 0.55rem;
        border: 1px solid var(--count-border, var(--mantine-color-default-border));
        border-radius: var(--mantine-radius-md);
        background: var(--count-bg, var(--mantine-color-default));
        color: var(--count-ink, var(--mantine-color-text));
        font-weight: 700;
        font-size: var(--mantine-font-size-xs);
        font-variant-numeric: tabular-nums;
        white-space: nowrap;
    }
    .count.teal {
        --count-border: color-mix(in srgb, var(--mantine-color-teal-6) 30%, transparent);
        --count-bg: var(--mantine-color-teal-light);
        --count-ink: var(--mantine-color-teal-light-color);
    }
    .count.yellow {
        --count-border: color-mix(in srgb, var(--mantine-color-yellow-6) 30%, transparent);
        --count-bg: var(--mantine-color-yellow-light);
        --count-ink: var(--mantine-color-yellow-light-color);
    }
    .count.red {
        --count-border: color-mix(in srgb, var(--mantine-color-red-6) 30%, transparent);
        --count-bg: var(--mantine-color-red-light);
        --count-ink: var(--mantine-color-red-light-color);
    }
    .search {
        position: relative;
        display: grid;
        flex: 1;
        min-width: 0;
    }
    .search input {
        width: 100%;
        padding-inline: 2.15rem;
    }
    .search input::-webkit-search-cancel-button {
        appearance: none;
    }
    .search-icon,
    .search :global(.action-icon) {
        position: absolute;
        align-self: center;
    }
    .search-icon {
        display: flex;
        inset-inline-start: 0.65rem;
        color: var(--mantine-color-dimmed);
        pointer-events: none;
    }
    .search :global(.action-icon) {
        inset-inline-end: 0.25rem;
    }
    .pagination {
        align-items: center;
    }
    .sr-only {
        position: absolute;
        width: 1px;
        height: 1px;
        overflow: hidden;
        clip-path: inset(50%);
        white-space: nowrap;
    }
</style>
