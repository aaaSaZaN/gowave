<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { z } from 'zod'
    import { callCommand } from '../commands'
    import { locale } from '../i18n.svelte'

    let { uuid }: { uuid: string } = $props()
    let type = $state('xray')
    let lines = $state(500)
    let live = $state(false)
    let tail = $state(true)
    let t = $derived(locale.t)
    let viewer = $state<HTMLPreElement>()
    const responseSchema = z.object({ response: z.object({ logs: z.array(z.string()) }) })
    const logs = createQuery(() => ({
        queryKey: ['node-logs', uuid, type, lines],
        queryFn: ({ signal }) =>
            callCommand(
                {
                    TSQ_url: `/api/nodes/${uuid}/logs`,
                    endpointDetails: { REQUEST_METHOD: 'GET' },
                    ResponseSchema: responseSchema
                },
                { params: { type, lines }, signal }
            ),
        refetchInterval: live ? 2500 : false,
        refetchOnWindowFocus: false,
        retry: false
    }))
    $effect(() => {
        if (logs.data && tail && viewer) viewer.scrollTop = viewer.scrollHeight
    })
</script>

<div class="button-row">
    <label
        >Log type
        <select bind:value={type}>
            <option value="xray">Xray</option>
            <option value="node">Node</option>
        </select>
    </label>
    <label
        >Lines
        <select bind:value={lines}>
            {#each [100, 250, 500, 1000, 2000] as count (count)}
                <option value={count}>{count}</option>
            {/each}
        </select>
    </label>
    <label class="check"><input type="checkbox" bind:checked={live}> Auto refresh</label>
    <label class="check"><input type="checkbox" bind:checked={tail}> Auto scroll</label>
    <button type="button" disabled={logs.isFetching} onclick={() => void logs.refetch()}>
        {t('common.action.refresh')}
    </button>
</div>
{#if logs.isPending}
    <p role="status">{t('common.message.loading')}</p>
{/if}
{#if logs.error}
    <p role="alert">{logs.error.message}</p>
{/if}
{#if logs.data}
    <pre
        bind:this={viewer}
    >{logs.data.logs.join('\n') || t('common.message.no-data-available')}</pre>
{/if}

<style>
    .button-row,
    label.check {
        align-items: center;
    }
    label.check {
        display: flex;
    }
    pre {
        white-space: pre-wrap;
        overflow: auto;
        max-height: 30rem;
        padding: 0.75rem;
        background: var(--gw-background);
        border-radius: 0.5rem;
    }
</style>
