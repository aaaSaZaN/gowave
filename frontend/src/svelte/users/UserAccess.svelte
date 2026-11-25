<script lang="ts">
    import {
        GetUserAccessibleNodesCommand,
        GetUserSubscriptionRequestHistoryCommand
    } from '@remnawave/backend-contract'
    import { formatTimeUtil } from '@shared/utils/time-utils'
    import { createQuery } from '@tanstack/svelte-query'
    import { callCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { flag } from '../nodes/analytics'
    import EmptyState from '../ui/EmptyState.svelte'
    import Icon from '../ui/Icon.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import { usersQueryKey } from './queries'

    let { userId, section }: { userId: number; section: 'access' | 'history' } = $props()
    let t = $derived(locale.t)
    type Request = GetUserSubscriptionRequestHistoryCommand.Response['response']['records'][number]
    type Column = 'id' | 'requestIp' | 'userAgent' | 'requestAt' | 'srrRuleName' | 'srrResponseType'
    let columns = $derived([
        ['id', 'ID'],
        ['requestIp', t('common.field.ip-address')],
        ['userAgent', t('common.field.user-agent')],
        ['requestAt', t('common.field.request-at')],
        ['srrRuleName', t('use-srh-inspector-table-columns.rule-name')],
        ['srrResponseType', t('use-srh-inspector-table-columns.response-type')]
    ] as const satisfies readonly [Column, string][])
    let sort = $state<Column>('requestAt')
    let ascending = $state(false)
    let search = $state('')
    let collator = $derived(new Intl.Collator(locale.language, { numeric: true }))

    function display(row: Request, column: Column): string {
        if (column === 'requestAt')
            return formatTimeUtil({
                time: row.requestAt,
                template: 'TIME_FIRST_DATETIME',
                language: locale.language
            })
        return String(row[column] ?? '—')
    }

    function order(column: Column): void {
        if (sort === column) ascending = !ascending
        else {
            sort = column
            ascending = true
        }
    }

    function copy(text: string): void {
        void navigator.clipboard.writeText(text).then(
            () =>
                notifications.show({
                    title: t('common.message.copied'),
                    message: text,
                    color: 'teal'
                }),
            (error: unknown) =>
                notifications.show({
                    title: t('common.message.error'),
                    message: error instanceof Error ? error.message : t('common.message.error'),
                    color: 'red'
                })
        )
    }

    const history = createQuery(() => ({
        queryKey: [...usersQueryKey, 'getUserSubscriptionRequestHistory', { userId }],
        queryFn: ({ signal }) =>
            callCommand(
                {
                    ...GetUserSubscriptionRequestHistoryCommand,
                    TSQ_url: GetUserSubscriptionRequestHistoryCommand.url(String(userId))
                },
                { signal }
            ),
        enabled: section === 'history',
        staleTime: 60_000
    }))

    let rows = $derived.by(() => {
        const term = search.trim().toLocaleLowerCase(locale.language)
        return (history.data?.records ?? [])
            .filter(
                (row) =>
                    !term ||
                    columns.some(([column]) =>
                        display(row, column).toLocaleLowerCase(locale.language).includes(term)
                    )
            )
            .sort((a, b) => {
                const result =
                    sort === 'requestAt'
                        ? a.requestAt.getTime() - b.requestAt.getTime()
                        : collator.compare(display(a, sort), display(b, sort))
                return ascending ? result : -result
            })
    })

    const nodes = createQuery(() => ({
        queryKey: [...usersQueryKey, 'getUserAccessibleNodes', { userId }],
        queryFn: ({ signal }) =>
            callCommand(
                {
                    ...GetUserAccessibleNodesCommand,
                    TSQ_url: GetUserAccessibleNodesCommand.url(String(userId))
                },
                { signal }
            ),
        enabled: section === 'access',
        staleTime: 60_000
    }))
</script>

{#if section === 'access'}
    {#if nodes.isPending}
        <p role="status">{t('common.message.loading')}</p>
    {:else if nodes.isError}
        <p role="alert">{nodes.error.message}</p>
    {:else if nodes.data.activeNodes.length}
        <div class="tree">
            {#each nodes.data.activeNodes as node (node.uuid)}
                <details>
                    <summary>
                        <Icon name="TbChevronRight" size={16} />
                        {flag(node.countryCode)} {node.nodeName}
                        <span class="profile"
                            ><Icon name="XrayLogo" size={14} />{node.configProfileName}</span
                        >
                        <b>{node.activeSquads.length}</b>
                    </summary>
                    <div class="branch">
                        {#each node.activeSquads as squad (squad.squadName)}
                            <details>
                                <summary>
                                    <Icon name="TbChevronRight" size={16} />
                                    <Icon name="TbCirclesRelation" size={16} />{squad.squadName}
                                    <b>{squad.activeInbounds.length}</b>
                                </summary>
                                <ul class="branch">
                                    {#each squad.activeInbounds as inbound}
                                        <li><Icon name="PiTag" size={14} />{inbound}</li>
                                    {/each}
                                </ul>
                            </details>
                        {/each}
                    </div>
                </details>
            {/each}
        </div>
    {:else}
        <EmptyState icon="TbServer" label={t('common.message.no-data-available')} />
    {/if}
{:else if history.isPending}
    <p role="status">{t('common.message.loading')}</p>
{:else if history.isError}
    <p role="alert">{history.error.message}</p>
{:else if !history.data.records.length}
    <EmptyState icon="TbTimeline" label={t('common.message.no-data-available')} />
{:else}
    <input
        class="search"
        type="search"
        aria-label={t('common.action.search')}
        placeholder={t('common.action.search')}
        bind:value={search}
    >
    <div class="requests">
        <table>
            <thead>
                <tr>
                    {#each columns as [column, label] (column)}
                        <th
                            scope="col"
                            aria-sort={column === sort
                                ? ascending
                                    ? 'ascending'
                                    : 'descending'
                                : undefined}
                        >
                            <button
                                type="button"
                                class="mantine-UnstyledButton-root"
                                onclick={() => order(column)}
                            >
                                {label}
                                {#if column === sort}
                                    <Icon
                                        name={ascending ? 'TbChevronUp' : 'TbChevronDown'}
                                        size={14}
                                    />
                                {/if}
                            </button>
                        </th>
                    {/each}
                </tr>
            </thead>
            <tbody>
                {#each rows as record (record.id)}
                    <tr>
                        {#each columns as [column, label] (column)}
                            <td>
                                {#if column === 'requestIp' && record.requestIp}
                                    <a
                                        target="_blank"
                                        rel="noreferrer"
                                        href={`https://ipinfo.io/${encodeURIComponent(record.requestIp)}`}
                                        >{record.requestIp}</a
                                    >
                                {:else if column === 'userAgent' ||
                                    column === 'srrRuleName' ||
                                    column === 'srrResponseType'}
                                    <button
                                        class="mantine-UnstyledButton-root copy"
                                        type="button"
                                        title={t('common.action.copy')}
                                        aria-label={`${label}: ${display(record, column)}`}
                                        onclick={() => copy(display(record, column))}
                                    >
                                        {display(record, column)}
                                    </button>
                                {:else}
                                    {display(record, column)}
                                {/if}
                            </td>
                        {/each}
                    </tr>
                {:else}
                    <tr>
                        <td colspan={columns.length}>{t('common.message.nothing-found')}</td>
                    </tr>
                {/each}
            </tbody>
        </table>
    </div>
{/if}

<style>
    .tree,
    .requests {
        border: 1px solid var(--gw-border);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }
    .tree {
        padding: var(--mantine-spacing-xs);
    }
    .branch {
        margin: 0 0 0 8px;
        padding-inline-start: 17px;
        border-inline-start: 1px solid var(--gw-border);
        list-style: none;
    }
    summary,
    li,
    .profile {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        font-size: var(--mantine-font-size-sm);
    }
    summary,
    li {
        padding: 4px 0;
    }
    summary {
        cursor: pointer;
        list-style: none;
    }
    summary::-webkit-details-marker {
        display: none;
    }
    details[open] > summary > :global(svg:first-child) {
        transform: rotate(90deg);
    }
    .profile {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }
    b {
        margin-inline-start: auto;
        padding-inline: 6px;
        border-radius: var(--mantine-radius-sm);
        background: var(--mantine-color-default-hover);
        font-size: var(--mantine-font-size-xs);
    }
    .search {
        width: 100%;
        margin-block-end: var(--mantine-spacing-sm);
    }
    .requests {
        overflow: auto;
    }
    table {
        width: 100%;
        border-collapse: collapse;
        text-align: start;
        font-size: var(--mantine-font-size-sm);
    }
    :is(th, td) {
        padding: var(--mantine-spacing-xs);
        border-block-end: 1px solid var(--gw-border);
        text-align: start;
    }
    th button {
        display: inline-flex;
        align-items: center;
        gap: 4px;
        white-space: nowrap;
    }
    td a,
    td:nth-child(4) {
        font-family: var(--mantine-font-family-monospace);
        white-space: nowrap;
    }
    .copy {
        overflow-wrap: anywhere;
        text-align: start;
    }
    .copy:hover {
        color: var(--gw-accent);
    }
</style>
