<script lang="ts">
    import { prettifyBytesUtil } from '@shared/utils/bytes/pretty-bytes/pretty-bytes.util'
    import { getDefaultDateRange } from '@shared/utils/time-utils/get-default-date-range/get-default-date-range.util'
    import { createInfiniteQuery } from '@tanstack/svelte-query'
    import { onMount } from 'svelte'
    import type resources from '../../../@types/resources'
    import { locale } from '../i18n.svelte'
    import UsageFilters from '../nodes/UsageFilters.svelte'
    import RouteLoading from '../RouteLoading.svelte'
    import { p } from '../router'
    import { cancelDialog, dismissDialog } from '../ui/dialog-motion'
    import { squadUsageQuery } from './queries'

    let { uuid, name, close }: { uuid: string; name: string; close: () => void } = $props()
    let t = $derived(locale.t)
    const title = $props.id()
    const gib = 1024 ** 3
    let threshold = $state<number | undefined>(100)
    let filters = $state({ ...getDefaultDateRange(), minTotalBytes: 100 * gib })
    let thresholdError = $state(false)
    let dialog: HTMLDialogElement
    const usage = createInfiniteQuery(() => squadUsageQuery(uuid, filters))
    let users = $derived(usage.data?.pages.flatMap(({ users }) => users) ?? [])

    onMount(() => dialog.showModal())

    function apply(range: { start: string; end: string }): void {
        const minTotalBytes = Math.round((threshold ?? 0) * gib)
        thresholdError = minTotalBytes < 0 || !Number.isSafeInteger(minTotalBytes)
        if (thresholdError) return
        if (
            range.start === filters.start &&
            range.end === filters.end &&
            minTotalBytes === filters.minTotalBytes
        ) {
            void usage.refetch()
        } else {
            filters = { ...range, minTotalBytes }
        }
    }
</script>

<dialog oncancel={cancelDialog} bind:this={dialog} onclose={close} aria-labelledby={title}>
    <div class="button-row">
        <h2 id={title}>{t('common.field.usage-stats')} · {name}</h2>
        <button type="button" onclick={() => dismissDialog(dialog)}>
            {t('common.action.close')}
        </button>
    </div>
    <UsageFilters
        range={filters}
        busy={usage.isFetching}
        {apply}
        refresh={() => void usage.refetch()}
    >
        {#snippet control()}
            <label>
                Minimum traffic (GiB)
                <input type="number" name="minTrafficGib" min="0" step="any" bind:value={threshold}>
            </label>
        {/snippet}
    </UsageFilters>
    {#if thresholdError}
        <p role="alert">Enter a valid non-negative traffic threshold.</p>
    {/if}
    <details>
        <summary>{t('internal-squads-usage-drawer.info.title')}</summary>
        {#snippet info(
            key: `internal-squads-usage-drawer.info.${keyof (typeof resources.gowave)['internal-squads-usage-drawer']['info']}`
        )}
            {String(t(key)).replace(/<\/?(?:b|hl|code)>/g, '')}
        {/snippet}
        <p>{@render info('internal-squads-usage-drawer.info.summary')}</p>
        <h3>{t('internal-squads-usage-drawer.info.nodes-title')}</h3>
        <p>{@render info('internal-squads-usage-drawer.info.nodes-intro')}</p>
        <ol>
            <li>{@render info('internal-squads-usage-drawer.info.nodes-step-1')}</li>
            <li>{@render info('internal-squads-usage-drawer.info.nodes-step-2')}</li>
            <li>{@render info('internal-squads-usage-drawer.info.nodes-step-3')}</li>
        </ol>
        <p>{@render info('internal-squads-usage-drawer.info.nodes-outro')}</p>
        <h3>{t('internal-squads-usage-drawer.info.traffic-title')}</h3>
        <p>{@render info('internal-squads-usage-drawer.info.traffic-granularity')}</p>
        <p>{@render info('internal-squads-usage-drawer.info.traffic-sum')}</p>
        <p>{@render info('internal-squads-usage-drawer.info.traffic-threshold')}</p>
        <h3>{t('internal-squads-usage-drawer.info.caveats-title')}</h3>
        <ul>
            <li>{@render info('internal-squads-usage-drawer.info.caveats-per-node')}</li>
            <li>{@render info('internal-squads-usage-drawer.info.caveats-membership')}</li>
            <li>{@render info('internal-squads-usage-drawer.info.caveats-sorting')}</li>
        </ul>
    </details>
    {#if usage.isPending}
        <RouteLoading compact />
    {:else if usage.error}
        <p role="alert">{usage.error.message}</p>
    {/if}
    {#if users.length}
        <ul>
            {#each users as user (user.id)}
                <li>
                    <a href={p('/dashboard/open/user/:id', { params: { id: String(user.id) } })}
                        >{user.id}</a
                    >
                    · {prettifyBytesUtil(user.totalBytes) ?? '0 B'}
                </li>
            {/each}
        </ul>
    {:else if usage.isSuccess}
        <p>{t('common.message.no-data-available')}</p>
    {/if}
    {#if usage.hasNextPage}
        <button
            type="button"
            disabled={usage.isFetching}
            onclick={() => void usage.fetchNextPage()}
        >
            {t('common.action.load-more')}
        </button>
    {/if}
</dialog>

<style>
    dialog {
        width: min(42rem, calc(100vw - 2rem));
    }
</style>
