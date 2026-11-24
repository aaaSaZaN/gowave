<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { onMount } from 'svelte'
    import { locale } from '../i18n.svelte'
    import { p } from '../router'
    import { ipDetailsUrl, reportsForUser } from './queries'

    let { userId, close }: { userId: number; close: () => void } = $props()
    let t = $derived(locale.t)
    let dialog: HTMLDialogElement
    let copyStatus = $state('')
    let copyError = $state('')
    let page = $state(0)
    const size = 25
    const reports = createQuery(() => reportsForUser(userId, page, size))
    let pages = $derived(Math.max(1, Math.ceil((reports.data?.total ?? 0) / size)))
    let dates = $derived(
        new Intl.DateTimeFormat(locale.language, { dateStyle: 'medium', timeStyle: 'short' })
    )

    onMount(() => dialog.showModal())

    $effect(() => {
        if (reports.data && page >= pages) page = pages - 1
    })

    function copy(value: string): void {
        copyError = ''
        copyStatus = ''
        void navigator.clipboard.writeText(value).then(
            () => (copyStatus = t('common.message.copied')),
            (error: unknown) => (copyError = error instanceof Error ? error.message : String(error))
        )
    }
</script>

<dialog bind:this={dialog} onclose={close} aria-labelledby="user-torrent-reports">
    <h2 id="user-torrent-reports">{t('constants.tb-reports')}</h2>
    {#if reports.isPending}
        <p role="status">{t('common.message.loading')}</p>
    {/if}
    {#if reports.error}
        <p role="alert">{reports.error.message}</p>
    {/if}
    {#if copyStatus}
        <p role="status">{copyStatus}</p>
    {/if}
    {#if copyError}
        <p role="alert">{copyError}</p>
    {/if}
    {#if reports.data}
        <p>
            {t('user-torrent-blocker-reports.drawer.widget.total-reports')}:
            {reports.data.total.toLocaleString(locale.language)}
        </p>
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
                            <a href={ipDetailsUrl(action.ip)} target="_blank" rel="noreferrer"
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
            {:else}
                <li>{t('common.message.nothing-found')}</li>
            {/each}
        </ul>
    {/if}
    <div class="button-row controls">
        <button
            type="button"
            disabled={page === 0 || reports.isPending}
            onclick={() => (page -= 1)}
        >
            Previous
        </button>
        <span>{page + 1} / {pages}</span>
        <button
            type="button"
            disabled={page + 1 >= pages || reports.isPending}
            onclick={() => (page += 1)}
        >
            Next
        </button>
        <button type="button" disabled={reports.isFetching} onclick={() => void reports.refetch()}>
            {t('common.action.refresh')}
        </button>
        <button type="button" onclick={() => dialog.close()}>{t('common.action.close')}</button>
    </div>
</dialog>

<style>
    dialog {
        width: min(95vw, 35rem);
    }
    h2 {
        margin-block: 0;
    }
    h3 {
        margin: 0;
        font-size: 1rem;
    }
    ul {
        padding: 0;
        max-height: 56vh;
        overflow: auto;
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
