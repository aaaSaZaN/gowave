<script lang="ts">
    import {
        GetUserAccessibleNodesCommand,
        GetUserSubscriptionRequestHistoryCommand
    } from '@remnawave/backend-contract'
    import { createQuery } from '@tanstack/svelte-query'
    import { callCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { usersQueryKey } from './queries'

    let { userId, section }: { userId: number; section: 'access' | 'history' } = $props()
    let t = $derived(locale.t)
    let dates = $derived(
        new Intl.DateTimeFormat(locale.language, { dateStyle: 'medium', timeStyle: 'short' })
    )

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
    {:else}
        <ul>
            {#each nodes.data.activeNodes as node (node.uuid)}
                <li>
                    <strong>{node.nodeName}</strong>
                    · {node.configProfileName}
                    <ul>
                        {#each node.activeSquads as squad (squad.squadName)}
                            <li>{squad.squadName} · {squad.activeInbounds.join(', ')}</li>
                        {/each}
                    </ul>
                </li>
            {:else}
                <li>{t('common.message.no-data-available')}</li>
            {/each}
        </ul>
    {/if}
{:else}
    {#if history.isPending}
        <p role="status">{t('common.message.loading')}</p>
    {:else if history.isError}
        <p role="alert">{history.error.message}</p>
    {:else}
        <ul>
            {#each history.data.records as record (record.id)}
                <li>
                    <time datetime={record.requestAt.toISOString()}
                        >{dates.format(record.requestAt)}</time
                    >
                    · {record.srrResponseType}
                    {#if record.requestIp}
                        · {record.requestIp}
                    {/if}
                    {#if record.srrRuleName}
                        · {record.srrRuleName}
                    {/if}
                    {#if record.userAgent}
                        <p>{record.userAgent}</p>
                    {/if}
                </li>
            {:else}
                <li>{t('common.message.no-data-available')}</li>
            {/each}
        </ul>
    {/if}
{/if}

<style>
    ul {
        padding-inline-start: var(--mantine-spacing-lg);
    }
    li > p {
        margin: 0.2rem 0;
        overflow-wrap: anywhere;
    }
</style>
