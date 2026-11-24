<script lang="ts">
    import { ResolveUserCommand } from '@remnawave/backend-contract'
    import { prettifyBytesUtil } from '@shared/utils/bytes/pretty-bytes/pretty-bytes.util'
    import { getDefaultDateRange } from '@shared/utils/time-utils/get-default-date-range/get-default-date-range.util'
    import { createQuery } from '@tanstack/svelte-query'
    import { onMount } from 'svelte'
    import { submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { navigate } from '../router'
    import { nodeUsersUsageQuery } from './queries'
    import UsageFilters from './UsageFilters.svelte'
    import UsageSummary from './UsageSummary.svelte'

    let { target, name, close }: { target: string | string[]; name: string; close: () => void } =
        $props()
    let dialog: HTMLDialogElement
    let t = $derived(locale.t)
    const title = $props.id()
    const limits = [5, 10, 30, 50, 100, 500, 1000, 2000]
    const pageSize = 50
    let page = $state(0)
    let filters = $state({ ...getDefaultDateRange(), topUsersLimit: 100 })
    let resolving = $state(false)
    let resolutionError = $state('')
    let closed = false
    const stats = createQuery(() => nodeUsersUsageQuery(target, filters))
    let users = $derived(stats.data?.topUsers ?? [])

    onMount(() => {
        dialog.showModal()
        return () => {
            closed = true
        }
    })

    async function openUser(username: string): Promise<void> {
        if (resolving) return
        resolutionError = ''
        resolving = true
        try {
            const user = await submitCommand(ResolveUserCommand, { username })
            if (!closed) {
                await navigate('/dashboard/open/user/:id', { params: { id: String(user.id) } })
            }
        } catch (error) {
            resolutionError = error instanceof Error ? error.message : t('common.message.error')
        } finally {
            resolving = false
        }
    }
</script>

<dialog class="node-users-usage" bind:this={dialog} onclose={close} aria-labelledby={title}>
    <div class="header">
        <h2 id={title}>{t('common.field.usage-stats')} · {name}</h2>
        <button type="button" onclick={() => dialog.close()}>{t('common.action.close')}</button>
    </div>
    <UsageFilters
        range={filters}
        busy={stats.isFetching}
        apply={(range) => {
            page = 0
            if (filters.start === range.start && filters.end === range.end) void stats.refetch()
            else filters = { ...filters, ...range }
        }}
        refresh={() => {
            page = 0
            void stats.refetch()
        }}
    >
        {#snippet control()}
            <label>
                {t('torrent-blocker-stats.widget.top-users')}
                <select
                    name="topUsersLimit"
                    value={filters.topUsersLimit}
                    onchange={(event) => {
                        page = 0
                        filters = { ...filters, topUsersLimit: Number(event.currentTarget.value) }
                    }}
                >
                    {#each limits as limit (limit)}
                        <option value={limit}>{limit}</option>
                    {/each}
                </select>
            </label>
        {/snippet}
    </UsageFilters>
    {#if stats.isPending}
        <p role="status">{t('common.message.loading')}</p>
    {:else if stats.isError}
        <p role="alert">{stats.error.message}</p>
        <button type="button" onclick={() => void stats.refetch()}>
            {t('common.action.try-again')}
        </button>
    {:else}
        <UsageSummary values={stats.data.sparklineData} />
        <section aria-labelledby={`${title}-users`}>
            <h3 id={`${title}-users`}>{t('torrent-blocker-stats.widget.top-users')}</h3>
            {#if resolutionError}
                <p role="alert">{resolutionError}</p>
            {/if}
            {#if users.length}
                <ol start={page * pageSize + 1}>
                    {#each users.slice(
                        page * pageSize,
                        (page + 1) * pageSize
                    ) as user (user.username)}
                        <li>
                            <button
                                type="button"
                                disabled={resolving}
                                onclick={() => void openUser(user.username)}
                            >
                                {user.username}
                            </button>
                            <span>{prettifyBytesUtil(user.total) ?? '0 B'}</span>
                        </li>
                    {/each}
                </ol>
                {#if users.length > pageSize}
                    <div class="button-row">
                        <button type="button" disabled={page === 0} onclick={() => page--}>
                            Previous
                        </button>
                        <button
                            type="button"
                            disabled={(page + 1) * pageSize >= users.length}
                            onclick={() => page++}
                        >
                            {t('common.action.next')}
                        </button>
                    </div>
                {/if}
            {:else}
                <p>{t('common.message.no-data-available')}</p>
            {/if}
        </section>
    {/if}
</dialog>

<style>
    .node-users-usage {
        width: min(42rem, calc(100vw - 2rem));
    }
    .header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        flex-wrap: wrap;
        gap: 0.75rem;
        margin-block-end: 0.75rem;
    }
    h2 {
        margin: 0;
    }
    li {
        margin-block: 0.35rem;
        overflow-wrap: anywhere;
    }
    li span {
        margin-inline-start: 0.75rem;
        white-space: nowrap;
    }
</style>
