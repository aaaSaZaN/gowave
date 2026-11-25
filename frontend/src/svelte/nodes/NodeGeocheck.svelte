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
    import Button from '../ui/Button.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import SelectField from '../ui/SelectField.svelte'

    let { node, close }: { node: GetNodeCommand.Response['response']; close: () => void } = $props()
    let t = $derived(locale.t)
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

<Overlay
    title={t('node-geocheck.title')}
    subtitle={node.name}
    icon="TbMapSearch"
    tone="teal"
    width={report ? 1000 : 520}
    {close}
>
    {#if !job}
        <form onsubmit={(event) => void start(event)}>
            <div class="sources">
                <Button
                    variant={source === 'default' ? 'default' : 'subtle'}
                    aria-pressed={source === 'default'}
                    disabled={starting}
                    onclick={() => (source = 'default')}
                >
                    {t('node-geocheck.source-default')}
                </Button>
                <Button
                    variant={source === 'ip' ? 'default' : 'subtle'}
                    aria-pressed={source === 'ip'}
                    disabled={starting}
                    onclick={() => (source = 'ip')}
                >
                    {t('common.field.ip-address')}
                </Button>
                <Button
                    variant={source === 'interface' ? 'default' : 'subtle'}
                    aria-pressed={source === 'interface'}
                    disabled={starting}
                    onclick={() => (source = 'interface')}
                >
                    {t('common.field.interface')}
                </Button>
            </div>
            {#if source === 'default'}
                <label>
                    {t('node-geocheck.source-default')}
                    <small>{t('node-geocheck.default-description')}</small>
                    <input placeholder={t('node-geocheck.default-placeholder')} disabled>
                </label>
            {:else if source === 'ip'}
                <SelectField
                    label={t('common.field.ip-address')}
                    description={t('node-geocheck.ip-description')}
                    leading="TbWorld"
                    placeholder="1.2.3.4"
                    autocomplete="suggest"
                    disabled={starting}
                    bind:value={ip}
                    options={Array.from(new Set(node.ips.map(({ ip }) => ip)), (value) => ({
                        value,
                        label: value
                    }))}
                />
            {:else}
                <SelectField
                    label={t('common.field.interface')}
                    description={t('node-geocheck.interface-description')}
                    leading="TbNetwork"
                    placeholder="eth0"
                    autocomplete="suggest"
                    disabled={starting}
                    bind:value={networkInterface}
                    options={(node.system?.info.networkInterfaces ?? []).map((value) => ({
                        value,
                        label: value
                    }))}
                />
            {/if}
            <Button
                type="submit"
                color="teal"
                variant="soft"
                icon="TbRoute"
                disabled={starting ||
                    (source === 'ip' && !ip.trim()) ||
                    (source === 'interface' && !networkInterface.trim())}
                >{t('node-geocheck.run')}</Button
            >
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
</Overlay>

<style>
    .sources {
        display: grid;
        grid-template-columns: repeat(3, minmax(0, 1fr));
        gap: 2px;
        padding: 2px;
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }
    small {
        color: var(--mantine-color-dimmed);
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
