<script lang="ts">
    import {
        GeocheckByNodeCommand,
        GeocheckByNodeResultCommand,
        type GetNodeCommand
    } from '@remnawave/backend-contract'
    import { resolveConnectivity } from '@shared/lib/geocheck-trace'
    import { createQuery, queryOptions } from '@tanstack/svelte-query'
    import { onMount } from 'svelte'
    import { callCommand, submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'

    let { node, close }: { node: GetNodeCommand.Response['response']; close: () => void } = $props()
    const title = $props.id()
    let t = $derived(locale.t)
    let dialog: HTMLDialogElement
    let source = $state<'default' | 'ip' | 'interface'>('default')
    let ip = $state('')
    let networkInterface = $state('')
    let job = $state('')
    let starting = $state(false)
    let failure = $state('')
    let active = true
    const result = createQuery(() => {
        const jobId = job
        const command = GeocheckByNodeResultCommand
        return queryOptions({
            queryKey: ['node-geocheck', jobId],
            enabled: Boolean(jobId),
            queryFn: ({ signal }) =>
                callCommand({ ...command, TSQ_url: command.url(jobId) }, { signal }),
            refetchInterval: ({ state }) =>
                state.data?.isCompleted || state.data?.isFailed || state.error ? false : 1000
        })
    })
    let report = $derived(result.data?.result)
    let connectivity = $derived(resolveConnectivity(report?.rawReport))
    let image = $derived(
        report?.image && ['image/png', 'image/jpeg', 'image/webp'].includes(report.image.media_type)
            ? `data:${report.image.media_type};base64,${report.image.data}`
            : undefined
    )

    onMount(() => {
        dialog.showModal()
        return () => {
            active = false
        }
    })

    async function start(event: SubmitEvent): Promise<void> {
        event.preventDefault()
        if (starting) return
        starting = true
        failure = ''
        const command = GeocheckByNodeCommand
        try {
            const input =
                source === 'ip'
                    ? { ip: ip.trim() }
                    : source === 'interface'
                      ? { interface: networkInterface.trim() }
                      : {}
            const { jobId } = await submitCommand(
                { ...command, TSQ_url: command.url(node.uuid) },
                input
            )
            if (active) job = jobId
        } catch (error) {
            failure = error instanceof Error ? error.message : String(error)
        } finally {
            starting = false
        }
    }
</script>

<dialog bind:this={dialog} onclose={close} aria-labelledby={title}>
    <div class="button-row">
        <h2 id={title}>{t('node-geocheck.title')} · {node.name}</h2>
        <button type="button" onclick={() => dialog.close()}>{t('common.action.close')}</button>
    </div>
    {#if !job}
        <form onsubmit={(event) => void start(event)}>
            <label>
                Source
                <select bind:value={source} disabled={starting}>
                    <option value="default">{t('node-geocheck.source-default')}</option>
                    <option value="ip">{t('common.field.ip-address')}</option>
                    <option value="interface">{t('common.field.interface')}</option>
                </select>
            </label>
            {#if source === 'ip'}
                <label>
                    {t('common.field.ip-address')}
                    <input bind:value={ip} list={`${title}-ips`} required disabled={starting}>
                </label>
                <datalist id={`${title}-ips`}>
                    {#each node.ips as entry (entry.ip)}
                        <option value={entry.ip}></option>
                    {/each}
                </datalist>
            {:else if source === 'interface'}
                <label>
                    {t('common.field.interface')}
                    <input
                        bind:value={networkInterface}
                        list={`${title}-interfaces`}
                        required
                        disabled={starting}
                    >
                </label>
                <datalist id={`${title}-interfaces`}>
                    {#each node.system?.info.networkInterfaces ?? [] as name (name)}
                        <option value={name}></option>
                    {/each}
                </datalist>
            {/if}
            <button
                type="submit"
                disabled={starting ||
                    (source === 'ip' && !ip.trim()) ||
                    (source === 'interface' && !networkInterface.trim())}
            >
                {t('node-geocheck.run')}
            </button>
        </form>
        {#if failure}
            <p role="alert">{failure}</p>
        {/if}
    {:else if result.error || result.data?.isFailed}
        <p role="alert">
            {result.error?.message || report?.message || t('node-geocheck.failed-description')}
        </p>
        <button type="button" onclick={() => (job = '')}>{t('common.action.try-again')}</button>
    {:else if result.data?.isCompleted}
        {#if image}
            <img src={image} alt={t('node-geocheck.title')}>
            <a href={image} download={`geocheck-${node.uuid}`}>{t('common.action.download')}</a>
        {:else}
            <p>{t('node-geocheck.no-image')}</p>
        {/if}
        {#if connectivity?.targets.length}
            <section aria-label="Connectivity trace">
                {#each connectivity.targets as target}
                    <details>
                        <summary>
                            {target.name || target.host}
                            · {target.verdict} · {target.rtt_ms} ms
                        </summary>
                        <p>{target.host} · {target.resolved} · {target.method}</p>
                        {#if Array.isArray(target.hops) && target.hops.length}
                            <ol>
                                {#each target.hops as hop}
                                    <li value={hop.ttl}>
                                        {hop.host || hop.addr || 'Timed out'}
                                        {#if hop.asn}
                                            · AS{hop.asn} {hop.as_name ?? ''}
                                        {/if}
                                        {#if hop.recv}
                                            · {hop.avg_ms} ms
                                        {/if}
                                    </li>
                                {/each}
                            </ol>
                        {/if}
                    </details>
                {/each}
            </section>
        {/if}
        {#if report?.rawReport}
            <details>
                <summary>JSON</summary>
                <pre>{JSON.stringify(report.rawReport, null, 2)}</pre>
            </details>
        {/if}
        <button type="button" onclick={() => (job = '')}>{t('common.action.try-again')}</button>
    {:else}
        <p role="status">{t('node-geocheck.running')}</p>
    {/if}
</dialog>

<style>
    dialog {
        width: min(54rem, calc(100vw - 2rem));
    }
    form,
    label {
        display: grid;
        gap: 0.5rem;
    }
    img {
        display: block;
        max-width: 100%;
        margin-block: 0.75rem;
    }
    pre {
        overflow: auto;
        max-height: 40dvh;
    }
</style>
