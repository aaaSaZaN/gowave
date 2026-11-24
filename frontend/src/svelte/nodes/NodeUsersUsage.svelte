<script lang="ts">
    import { ResolveUserCommand } from '@remnawave/backend-contract'
    import { prettifyBytesUtil } from '@shared/utils/bytes/pretty-bytes/pretty-bytes.util'
    import { getDefaultDateRange } from '@shared/utils/time-utils/get-default-date-range/get-default-date-range.util'
    import { createQuery } from '@tanstack/svelte-query'
    import { onDestroy } from 'svelte'
    import { submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { navigate } from '../router'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import EmptyState from '../ui/EmptyState.svelte'
    import Icon from '../ui/Icon.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import SelectField from '../ui/SelectField.svelte'
    import { nodeUsersUsageQuery } from './queries'
    import UsageFilters from './UsageFilters.svelte'
    import UsageSummary from './UsageSummary.svelte'

    let { target, name, close }: { target: string | string[]; name: string; close: () => void } =
        $props()
    let t = $derived(locale.t)
    const title = $props.id()
    const limits = [5, 10, 30, 50, 100, 500, 1000, 2000].map((limit) => ({
        value: String(limit),
        label: `Top ${limit}`
    }))
    const pageSize = 50
    let page = $state(0)
    let filters = $state({ ...getDefaultDateRange(), topUsersLimit: 100 })
    let resolving = $state(false)
    let resolutionError = $state('')
    let closed = false
    const stats = createQuery(() => nodeUsersUsageQuery(target, filters))
    let users = $derived(stats.data?.topUsers ?? [])

    onDestroy(() => {
        closed = true
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

<Overlay
    title={t('common.field.usage-stats')}
    icon="TbChartArcs"
    tone="teal"
    width={600}
    drawer
    subtitle={name}
    {close}
>
    {#snippet actions()}
        <ActionIcon
            icon="TbRefresh"
            tone="cyan"
            size={34}
            iconSize={20}
            label={t('common.action.refresh')}
            disabled={stats.isFetching}
            onclick={() => {
                page = 0
                void stats.refetch()
            }}
        />
    {/snippet}
    <UsageFilters
        hideRefresh
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
            <SelectField
                label={t('torrent-blocker-stats.widget.top-users')}
                leading="TbUsers"
                leadingSize={20}
                size="md"
                hideLabel
                placeholder=""
                searchable={false}
                clearable={false}
                options={limits}
                value={String(filters.topUsersLimit)}
                onchoose={(value) => {
                    page = 0
                    filters = { ...filters, topUsersLimit: Number(value) }
                }}
            />
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
                <ol>
                    {#each users.slice(
                        page * pageSize,
                        (page + 1) * pageSize
                    ) as user, index (user.username)}
                        <li>
                            <button
                                type="button"
                                class="mantine-UnstyledButton-root rank"
                                disabled={resolving}
                                onclick={() => void openUser(user.username)}
                            >
                                <span class="place">{page * pageSize + index + 1}</span>
                                <Icon name="PiUserCircle" size={18} />
                                <span class="username">{user.username}</span>
                                <strong>{prettifyBytesUtil(user.total) ?? '0 B'}</strong>
                            </button>
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
                <EmptyState icon="TbChartArcs" label={t('common.message.no-data-available')} />
            {/if}
        </section>
    {/if}
</Overlay>

<style>
    h3 {
        font-size: var(--mantine-font-size-md);
    }
    ol {
        display: grid;
        gap: 6px;
        padding: 0;
        list-style: none;
    }
    .rank {
        display: flex;
        width: 100%;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        padding: var(--mantine-spacing-xs);
        border: 1px solid var(--gw-border);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
        font-size: var(--mantine-font-size-sm);
    }
    .rank:hover {
        background: var(--mantine-color-default-hover);
    }
    .username {
        flex: 1;
        min-width: 0;
        overflow-wrap: anywhere;
    }
    .place {
        color: var(--mantine-color-dimmed);
    }
    strong {
        white-space: nowrap;
    }
</style>
