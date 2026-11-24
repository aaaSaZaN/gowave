<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { onDestroy } from 'svelte'
    import { locale } from '../i18n.svelte'
    import { nodesQuery } from '../nodes/queries'
    import RouteLoading from '../RouteLoading.svelte'
    import { p } from '../router'
    import Button from '../ui/Button.svelte'
    import FormCard from '../ui/FormCard.svelte'
    import Icon from '../ui/Icon.svelte'
    import PageHeader from '../ui/PageHeader.svelte'
    import { loadNodeSessions } from './queries'
    import SessionGroup from './SessionGroup.svelte'
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
    let visible = $derived(matches.slice(page * pageSize, (page + 1) * pageSize))
    let t = $derived(locale.t)
    let notice = $state('')
    let failure = $state('')
    let copyFailure = $state('')

    onDestroy(() => controller?.abort())

    function reset(): void {
        controller?.abort()
        controller = undefined
        phase = 'idle'
        progress = undefined
        result = undefined
        search = ''
        page = 0
        notice = ''
        failure = ''
        copyFailure = ''
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

    function copy(ip: string): void {
        notice = ''
        copyFailure = ''
        void navigator.clipboard.writeText(ip).then(
            () => (notice = t('common.message.copied')),
            (error: unknown) =>
                (copyFailure = error instanceof Error ? error.message : String(error))
        )
    }
</script>

<svelte:head><title>{t('constants.sessions-explorer')} | Gowave</title></svelte:head>
<main>
    {#if nodes.isPending}
        <RouteLoading label={t('common.message.loading')} />
    {:else}
        <PageHeader title={t('constants.sessions-explorer')} icon="TbRadar2" />

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
            <section class="surface">
                <h2>{t('sessions-explorer-progress.exploring-nodes')}</h2>
                <progress
                    value={progress.completed + progress.failed}
                    max={progress.total}
                    aria-label={t('sessions-explorer-progress.exploring-nodes')}
                ></progress>
                <p role="status">
                    {t('sessions-explorer-progress.nodes-processed', {
                        completed: progress.completed + progress.failed,
                        total: progress.total
                    })}
                    · {t('sessions-explorer-progress.nodes-failed', { count: progress.failed })}
                </p>
                <ul>
                    {#each progress.active as node (node.uuid)}
                        <li>{node.name}</li>
                    {/each}
                </ul>
                <button type="button" onclick={reset}>{t('common.action.cancel')}</button>
            </section>
        {:else if phase === 'failed'}
            <p role="alert">
                {failure || t('sessions-explorer.widget.all-nodes-failed-to-return-session-data')}
            </p>
            <button type="button" onclick={() => void start()} disabled={!online.length}>
                {t('common.action.try-again')}
            </button>
            <button type="button" onclick={reset}>
                {t('sessions-explorer.widget.clear-results')}
            </button>
        {:else if result}
            <dl class="metrics">
                <div>
                    <dt>{t('sessions-explorer.widget.total-ips')}</dt>
                    <dd>{result.totalIps.toLocaleString(locale.language)}</dd>
                </div>
                <div>
                    <dt>{t('sessions-explorer.widget.unique-ips')}</dt>
                    <dd>{result.uniqueIps.toLocaleString(locale.language)}</dd>
                </div>
                <div>
                    <dt>{t('sessions-explorer.widget.nodes-explored')}</dt>
                    <dd>{result.completed.toLocaleString(locale.language)}</dd>
                </div>
                <div>
                    <dt>{t('sessions-explorer.widget.total-users')}</dt>
                    <dd>{result.users.length.toLocaleString(locale.language)}</dd>
                </div>
            </dl>
            <div class="button-row">
                <button type="button" disabled={!online.length} onclick={() => void start()}>
                    {t('sessions-explorer.widget.restart-scan')}
                </button>
                <button type="button" onclick={reset}>
                    {t('sessions-explorer.widget.clear-results')}
                </button>
            </div>
            {#if result.users.length}
                <label class="search">
                    {t('sessions-explorer.widget.search-by-ip')}
                    <input type="search" bind:value={search} oninput={() => (page = 0)}>
                </label>
                {#if term}
                    <button
                        type="button"
                        onclick={() => {
                            search = ''
                            page = 0
                        }}
                    >
                        {t('action-group.feature.clear-filters')}
                    </button>
                {/if}
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
                <p>
                    {term
                        ? t('sessions-explorer.widget.no-results-for-ip', { ip: term })
                        : t('common.message.nothing-found')}
                </p>
            {/if}
            {#if notice}
                <p role="status">{notice}</p>
            {/if}
            {#if copyFailure}
                <p role="alert">{copyFailure}</p>
            {/if}
            <div class="results">
                {#each visible as user (user.userId)}
                    <article class="surface">
                        <header>
                            <h2>
                                <a
                                    href={p('/dashboard/open/user/:id', {
                                        params: { id: String(user.userId) }
                                    })}
                                    >User {user.userId}</a
                                >
                            </h2>
                            <span
                                >{t('sessions-explorer.widget.unique-ips')}
                                {user.uniqueIps.toLocaleString(locale.language)}
                                ·
                                {t('sessions-explorer.widget.total-ips')}
                                {user.totalIps.toLocaleString(locale.language)}</span
                            >
                        </header>
                        {#each user.nodes as { node, ips } (node.uuid)}
                            {#if ips.length}
                                <SessionGroup {node} {ips} search={term} {copy} />
                            {/if}
                        {/each}
                    </article>
                {/each}
            </div>
            {#if pages > 1}
                <div class="button-row pagination">
                    <button type="button" disabled={page === 0} onclick={() => (page -= 1)}>
                        Previous
                    </button>
                    <span
                        >{page + 1}
                        / {pages} · {matches.length.toLocaleString(locale.language)}</span
                    >
                    <button type="button" disabled={page + 1 >= pages} onclick={() => (page += 1)}>
                        Next
                    </button>
                </div>
            {/if}
        {/if}
    {/if}
</main>

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

    .surface,
    .metrics > div {
        border: 1px solid var(--gw-border);
        border-radius: 0.5rem;
        padding: 1rem;
        background: var(--gw-surface);
    }
    .results,
    .metrics {
        display: grid;
        gap: 0.75rem;
    }
    .results {
        grid-template-columns: repeat(auto-fit, minmax(min(100%, 27rem), 1fr));
    }
    .metrics {
        grid-template-columns: repeat(auto-fit, minmax(min(100%, 12rem), 1fr));
    }
    .metrics dd {
        margin: 0.25rem 0 0;
        font-size: 1.25rem;
        font-weight: bold;
    }
    article header {
        margin-block-end: 0.75rem;
    }
    h2 {
        font-size: 1.15rem;
        margin-block: 0 0.4rem;
    }
    .search {
        display: grid;
        gap: 0.25rem;
        max-width: 30rem;
        margin-block: 0.75rem;
    }
    .pagination {
        margin-block: 0.75rem;
        align-items: center;
    }
    details {
        margin-block: 0.75rem;
    }
    progress {
        width: min(100%, 32rem);
    }
</style>
