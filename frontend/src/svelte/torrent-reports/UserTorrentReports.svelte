<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { locale } from '../i18n.svelte'
    import { p } from '../router'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import EmptyState from '../ui/EmptyState.svelte'
    import FormCard from '../ui/FormCard.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import { ipDetailsUrl, reportsForUser } from './queries'

    let { userId, close }: { userId: number; close: () => void } = $props()
    let t = $derived(locale.t)
    let page = $state(0)
    const size = 25
    const reports = createQuery(() => reportsForUser(userId, page, size))
    let pages = $derived(Math.max(1, Math.ceil((reports.data?.total ?? 0) / size)))
    let dates = $derived(
        new Intl.DateTimeFormat(locale.language, { dateStyle: 'medium', timeStyle: 'short' })
    )

    $effect(() => {
        if (reports.data && page >= pages) page = pages - 1
    })

    function copy(value: string): void {
        void navigator.clipboard.writeText(value).then(
            () =>
                notifications.show({
                    title: t('common.message.copied'),
                    message: value,
                    color: 'teal'
                }),
            (error: unknown) =>
                notifications.show({
                    title: t('common.message.error'),
                    message: error instanceof Error ? error.message : String(error),
                    color: 'red'
                })
        )
    }
</script>

<Overlay title={t('constants.tb-reports')} icon="TbFlame" tone="red" width={500} drawer {close}>
    {#if reports.isPending}
        <p role="status">{t('common.message.loading')}</p>
    {/if}
    {#if reports.error}
        <p role="alert">{reports.error.message}</p>
    {/if}
    {#if reports.data}
        <div class="report-content">
            <FormCard
                title={reports.data.total.toLocaleString(locale.language)}
                description={t('user-torrent-blocker-reports.drawer.widget.total-reports')}
                icon="TbFlame"
                tone="indigo"
                headingSize="xl"
            >
                {#snippet actions()}
                    <ActionIcon
                        icon="TbRefresh"
                        tone="indigo"
                        size={34}
                        iconSize={20}
                        label={t('common.action.refresh')}
                        disabled={reports.isFetching}
                        onclick={() => void reports.refetch()}
                    />
                {/snippet}
            </FormCard>
            {#if reports.data.records.length}
                <ul>
                    {#each reports.data.records as record (record.id)}
                        {@const xray = record.report.xrayReport}
                        {@const action = record.report.actionReport}
                        {@const processedAt = dates.format(new Date(action.processedAt))}
                        {@const willUnblockAt = dates.format(new Date(action.willUnblockAt))}
                        <li>
                            <h3>
                                <a
                                    href={p('/dashboard/open/node/:id', {
                                        params: { id: record.node.uuid }
                                    })}
                                    >{record.node.name}</a
                                >
                            </h3>
                            <dl>
                                <dt>Inbound tag</dt>
                                <dd>{xray.inboundTag || '—'}</dd>
                                <dt>{t('common.field.ip-address')}</dt>
                                <dd>
                                    <button
                                        type="button"
                                        class="copy-value"
                                        title={t('common.action.copy')}
                                        onclick={() => copy(action.ip)}
                                    >
                                        {action.ip}
                                    </button>
                                    <a
                                        href={ipDetailsUrl(action.ip)}
                                        target="_blank"
                                        rel="noreferrer"
                                        >IP info</a
                                    >
                                </dd>
                                <dt>{t('user-torrent-blocker-report-item.processed-at')}</dt>
                                <dd>
                                    <button
                                        type="button"
                                        class="copy-value"
                                        title={t('common.action.copy')}
                                        onclick={() => copy(processedAt)}
                                    >
                                        {processedAt}
                                    </button>
                                </dd>
                                <dt>{t('user-torrent-blocker-report-item.unblocked-at')}</dt>
                                <dd>
                                    <button
                                        type="button"
                                        class="copy-value"
                                        title={t('common.action.copy')}
                                        onclick={() => copy(willUnblockAt)}
                                    >
                                        {willUnblockAt}
                                    </button>
                                </dd>
                            </dl>
                            <details>
                                <summary>Raw report</summary>
                                <pre>{JSON.stringify(record.report, null, 2)}</pre>
                            </details>
                        </li>
                    {/each}
                </ul>
            {:else}
                <EmptyState icon="TbFlame" label={t('common.message.nothing-found')} />
            {/if}
            {#if pages > 1}
                <div class="button-row controls">
                    <ActionIcon
                        icon="PiCaretLeft"
                        label="Previous"
                        disabled={page === 0 || reports.isFetching}
                        onclick={() => (page -= 1)}
                    />
                    <span>{page + 1} / {pages}</span>
                    <ActionIcon
                        icon="TbChevronRight"
                        label={t('common.action.next')}
                        disabled={page + 1 >= pages || reports.isFetching}
                        onclick={() => (page += 1)}
                    />
                </div>
            {/if}
        </div>
    {/if}
</Overlay>

<style>
    .report-content {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    h3 {
        margin: 0;
        font-size: 1rem;
    }
    ul {
        margin: 0;
        padding: 0;
        list-style: none;
    }
    li {
        margin-block-end: 0.5rem;
        padding: 0.75rem;
        border: 1px solid var(--gw-border);
        border-radius: 0.5rem;
    }
    dl {
        display: grid;
        grid-template-columns: minmax(7rem, auto) 1fr;
        gap: 0.35rem 0.75rem;
    }
    dd {
        margin: 0;
        overflow-wrap: anywhere;
    }
    pre {
        white-space: pre-wrap;
        overflow-wrap: anywhere;
    }
    .copy-value {
        padding: 0.2rem 0.25rem;
        min-height: 1.5rem;
    }
    .controls {
        align-items: center;
    }
</style>
