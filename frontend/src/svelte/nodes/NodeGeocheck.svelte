<script lang="ts">
    import {
        GeocheckByNodeCommand,
        GeocheckByNodeResultCommand,
        type GetNodeCommand
    } from '@remnawave/backend-contract'
    import {
        groupHopsByAs,
        isTimedOutHop,
        resolveConnectivity,
        resolveVerdictColor
    } from '@shared/lib/geocheck-trace'
    import { createQuery, queryOptions } from '@tanstack/svelte-query'
    import { onMount } from 'svelte'
    import { callCommand, submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import SessionScanProgress from '../sessions/SessionScanProgress.svelte'
    import Button from '../ui/Button.svelte'
    import Icon from '../ui/Icon.svelte'
    import JsonTree from '../ui/JsonTree.svelte'
    import { notifications } from '../ui/notification-state.svelte'
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
    let reportImage = $state<HTMLImageElement>()
    let exporting = $state(false)
    let exportError = $state('')
    let view = $state<'image' | 'trace' | 'json'>('image')
    let targetIndex = $state(0)
    let exporter = $state<typeof import('@shared/utils/copy-screenshot.util')>()
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
    let selected = $derived(connectivity?.targets[targetIndex])
    let hops = $derived(Array.isArray(selected?.hops) ? selected.hops : [])
    let segments = $derived(groupHopsByAs(hops))
    let scale = $derived(
        Math.max(
            1,
            selected?.rtt_ms || 0,
            ...hops.map((hop) => (isTimedOutHop(hop) ? 0 : hop.avg_ms || 0))
        )
    )
    let image = $derived(
        report?.image?.data ? `data:image/svg+xml;base64,${report.image.data}` : undefined
    )

    onMount(() => {
        return () => {
            active = false
        }
    })

    $effect(() => {
        if (!image || exporter) return
        void import('@shared/utils/copy-screenshot.util').then(
            (module) => (exporter = module),
            (error: unknown) =>
                (exportError = error instanceof Error ? error.message : String(error))
        )
    })

    async function exportImage(action: 'copy' | 'download'): Promise<void> {
        if (!reportImage || !exporter || exporting) return
        exporting = true
        exportError = ''
        const filename = `geocheck-${node.uuid}.png`
        try {
            if (action === 'copy') {
                await exporter.copyImageScreenshotToClipboard(reportImage, filename)
                notifications.show({
                    title: t('common.message.copied'),
                    message: t('node-geocheck.title'),
                    color: 'teal'
                })
            } else {
                await exporter.downloadImageScreenshot(reportImage, filename)
            }
        } catch (error) {
            exportError = error instanceof Error ? error.message : String(error)
        } finally {
            exporting = false
        }
    }

    async function start(event: SubmitEvent): Promise<void> {
        event.preventDefault()
        if (starting) return
        starting = true
        failure = ''
        exportError = ''
        view = 'image'
        targetIndex = 0
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

    function milliseconds(value: number | undefined): string {
        return value !== undefined && Number.isFinite(value)
            ? `${value.toFixed(value >= 100 ? 0 : 2)} ms`
            : '—'
    }

    function loss(value: number | undefined): string {
        return value !== undefined && Number.isFinite(value) ? `${Math.round(value * 100)}%` : '—'
    }
</script>

{#snippet verdict(label: string, tone: string)}
    <span class="verdict" style:--trace-tone={`var(--mantine-color-${tone}-5)`}>{label}</span>
{/snippet}

{#snippet statistic(label: string, value: string | number)}
    <div class="statistic">
        <dt>{label}</dt>
        <dd>{value}</dd>
    </div>
{/snippet}

<Overlay
    title={t('node-geocheck.title')}
    subtitle={node.name}
    icon="TbMapSearch"
    tone="teal"
    width={report ? 1000 : 520}
    {close}
>
    {#if starting || (job && !result.error && !result.data?.isFailed && !result.data?.isCompleted)}
        <SessionScanProgress
            indeterminate={{
                title: t('node-geocheck.running'),
                description: t('node-geocheck.running-description')
            }}
        />
    {:else if !job}
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
        <div class="report-toolbar">
            <fieldset class="sources report-views" aria-label={t('node-geocheck.title')}>
                <Button
                    size="sm"
                    icon="TbEye"
                    variant={view === 'image' ? 'default' : 'subtle'}
                    aria-pressed={view === 'image'}
                    onclick={() => (view = 'image')}
                    >{t('node-geocheck.view-image')}</Button
                >
                {#if connectivity}
                    <Button
                        size="sm"
                        icon="TbRoute"
                        variant={view === 'trace' ? 'default' : 'subtle'}
                        aria-pressed={view === 'trace'}
                        onclick={() => (view = 'trace')}
                        >Traceroute</Button
                    >
                {/if}
                <Button
                    size="sm"
                    icon="TbCode"
                    variant={view === 'json' ? 'default' : 'subtle'}
                    aria-pressed={view === 'json'}
                    onclick={() => (view = 'json')}
                    >JSON</Button
                >
            </fieldset>
            <div class="report-actions">
                {#if view === 'image' && image}
                    <Button
                        size="sm"
                        variant="default"
                        icon="TbCopy"
                        disabled={!exporter || exporting}
                        onclick={() => void exportImage('copy')}
                        >{t('common.action.copy')}</Button
                    >
                    <Button
                        size="sm"
                        variant="default"
                        icon="TbDownload"
                        disabled={!exporter || exporting}
                        onclick={() => void exportImage('download')}
                        >{t('common.action.download')}</Button
                    >
                {/if}
                <Button
                    size="sm"
                    color="teal"
                    variant="soft"
                    icon="TbRefresh"
                    disabled={exporting}
                    onclick={() => (job = '')}
                    >{t('common.action.try-again')}</Button
                >
            </div>
        </div>
        {#if view === 'image'}
            {#if image}
                <img bind:this={reportImage} src={image} alt={t('node-geocheck.title')}>
            {:else}
                <div class="report-empty">
                    <Button icon="TbCode" variant="subtle" onclick={() => (view = 'json')}
                        >{t('node-geocheck.no-image')}</Button
                    >
                </div>
            {/if}
            {#if exportError}
                <p role="alert">{exportError}</p>
            {/if}
        {:else if view === 'trace' && connectivity}
            {#if selected}
                <section class="trace" aria-label="Traceroute" dir="ltr">
                    <div class="trace-summary">
                        <Icon name="TbRoute" size={18} />
                        <dl class="trace-stats">
                            {@render statistic('Score', connectivity.score ?? '—')}
                            {#if connectivity.latency_floor_ms !== undefined}
                                {@render statistic(
                                    'Floor',
                                    milliseconds(connectivity.latency_floor_ms)
                                )}
                            {/if}
                        </dl>
                        {#each Object.entries(connectivity.breakdown ?? {}) as [name, count] (name)}
                            {#if count > 0}
                                {@render verdict(`${name} ${count}`, resolveVerdictColor(name))}
                            {/if}
                        {/each}
                        {#if connectivity.icmp_available === false}
                            {@render verdict('no ICMP', 'red')}
                        {/if}
                    </div>
                    <div class="trace-body">
                        <fieldset class="trace-targets" aria-label="Traceroute">
                            {#each connectivity.targets as target, index}
                                <button
                                    type="button"
                                    class="mantine-UnstyledButton-root trace-target"
                                    aria-pressed={targetIndex === index}
                                    onclick={() => (targetIndex = index)}
                                >
                                    <span class="trace-target-line">
                                        <span
                                            class="trace-dot"
                                            style:--trace-tone={`var(--mantine-color-${resolveVerdictColor(target.verdict)}-5)`}
                                        ></span>
                                        <strong>{target.name || target.host}</strong>
                                        <code>{milliseconds(target.rtt_ms)}</code>
                                    </span>
                                    <span class="trace-subtitle">
                                        {target.verdict}
                                        {#if target.transits?.length}
                                            · {target.transits.map(({ name }) => name).join(', ')}
                                        {/if}
                                    </span>
                                </button>
                            {/each}
                        </fieldset>
                        {#key targetIndex}
                            <article class="trace-detail">
                                <header class="trace-detail-heading">
                                    <strong>{selected.name || selected.host}</strong>
                                    {@render verdict(
                                        selected.verdict,
                                        resolveVerdictColor(selected.verdict)
                                    )}
                                    {#if selected.anycast}
                                        {@render verdict('anycast', 'grape')}
                                    {/if}
                                    {#if selected.method}
                                        {@render verdict(selected.method, 'gray')}
                                    {/if}
                                </header>
                                <code class="trace-address">
                                    {selected.resolved || selected.host}
                                    {#if selected.dest_asn}
                                        · AS{selected.dest_asn} {selected.dest_as_name ?? ''}
                                    {/if}
                                </code>
                                <dl class="trace-stats">
                                    {@render statistic('RTT', milliseconds(selected.rtt_ms))}
                                    {@render statistic('Excess', milliseconds(selected.excess_ms))}
                                    {@render statistic('Jitter', milliseconds(selected.jitter_ms))}
                                    {@render statistic('Loss', loss(selected.loss))}
                                    {@render statistic('Score', selected.score ?? '—')}
                                    {@render statistic('Hops', hops.length)}
                                </dl>
                                {#each segments as segment}
                                    <div class="hop-segment">
                                        <div class="hop-network">
                                            {segment.asn
                                                ? `AS${segment.asn} · ${segment.asName || '—'}`
                                                : '—'}
                                        </div>
                                        {#each segment.hops as hop}
                                            {@const timeout = isTimedOutHop(hop)}
                                            <div class="hop" class:timeout>
                                                <code>{hop.ttl}</code>
                                                <span class="hop-address">
                                                    <code>{hop.addr || '* * *'}</code>
                                                    {#if hop.host && hop.host !== hop.addr}
                                                        <small>{hop.host}</small>
                                                    {/if}
                                                </span>
                                                <span class="hop-track" aria-hidden="true">
                                                    <span
                                                        style:width={`${timeout ? 0 : ((hop.avg_ms || 0) * 100) / scale}%`}
                                                    ></span>
                                                </span>
                                                <code
                                                    >{timeout
                                                        ? '—'
                                                        : milliseconds(hop.avg_ms)}</code
                                                >
                                                <code title={`${hop.recv ?? 0}/${hop.sent ?? 0}`}
                                                    >{loss(hop.loss)}</code
                                                >
                                            </div>
                                        {/each}
                                    </div>
                                {/each}
                                {#each selected.notes ?? [] as note}
                                    <small>{note}</small>
                                {/each}
                            </article>
                        {/key}
                    </div>
                </section>
            {:else}
                <p class="report-empty">{t('common.message.no-data-available')}</p>
            {/if}
        {:else if view === 'json'}
            <div class="report-json"><JsonTree value={report?.rawReport ?? {}} indent={4} /></div>
        {/if}
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
    .report-toolbar,
    .report-actions,
    .trace-summary,
    .trace-detail-heading,
    .trace-stats {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .report-toolbar {
        justify-content: space-between;
        margin-block-end: var(--mantine-spacing-sm);
    }
    .report-views {
        display: flex;
        flex-wrap: wrap;
        margin: 0;
        border: 0;
    }
    .report-json {
        overflow: auto;
        max-height: 62dvh;
    }
    .report-empty {
        display: grid;
        min-height: 14rem;
        place-items: center;
        color: var(--mantine-color-dimmed);
    }
    .trace {
        display: grid;
        grid-template-rows: auto minmax(0, 1fr);
        gap: var(--mantine-spacing-sm);
        height: min(60dvh, 44rem);
        min-height: 20rem;
    }
    .trace-summary,
    .trace-targets,
    .trace-detail {
        min-width: 0;
        border: 1px solid var(--gw-border);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }
    .trace-summary {
        padding: 8px 12px;
    }
    .trace-body {
        display: grid;
        min-height: 0;
        grid-template-columns: minmax(0, 30%) minmax(0, 1fr);
        gap: var(--mantine-spacing-sm);
    }
    .trace-targets,
    .trace-detail {
        overflow-y: auto;
        overscroll-behavior: contain;
        scrollbar-width: thin;
    }
    .trace-targets {
        margin: 0;
        padding: 4px;
    }
    .trace-target {
        display: grid;
        width: 100%;
        gap: 2px;
        padding: 7px 9px;
        border: 1px solid transparent;
        border-radius: var(--mantine-radius-sm);
        font-size: var(--mantine-font-size-xs);
    }
    .trace-target:hover {
        background: var(--mantine-color-default-hover);
    }
    .trace-target[aria-pressed="true"] {
        border-color: var(--mantine-color-cyan-outline);
        background: var(--mantine-color-cyan-light);
    }
    .trace-target-line {
        display: grid;
        grid-template-columns: 6px minmax(0, 1fr) auto;
        align-items: center;
        gap: 6px;
    }
    .trace-dot {
        width: 6px;
        height: 6px;
        border-radius: 50%;
        background: var(--trace-tone);
    }
    .trace-target-line strong,
    .trace-subtitle,
    .hop-network,
    .hop-address > * {
        display: block;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .trace-target-line code,
    .trace-subtitle,
    .hop-network {
        color: var(--mantine-color-dimmed);
        font-size: 10px;
    }
    .trace-subtitle {
        padding-inline-start: 12px;
    }
    .trace-detail {
        display: grid;
        align-content: start;
        gap: var(--mantine-spacing-xs);
        padding: var(--mantine-spacing-md);
        font-size: var(--mantine-font-size-sm);
    }
    .trace-detail-heading strong,
    .trace-address {
        overflow-wrap: anywhere;
    }
    .trace-address {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }
    .trace-stats {
        gap: 8px 20px;
        margin: 0;
    }
    .statistic dt {
        color: var(--mantine-color-dimmed);
        font-size: 10px;
        text-transform: uppercase;
    }
    .statistic dd {
        margin: 0;
        font: var(--mantine-font-size-xs) var(--mantine-font-family-monospace);
    }
    .verdict {
        padding: 1px 8px;
        border-radius: var(--mantine-radius-xl);
        background: color-mix(in srgb, var(--trace-tone) 15%, transparent);
        color: color-mix(in srgb, var(--trace-tone) 62%, var(--mantine-color-text));
        font-size: 11px;
        font-weight: 600;
    }
    .hop-segment {
        padding-inline-start: 8px;
        border-inline-start: 2px solid var(--mantine-color-cyan-light-color);
    }
    .hop {
        display: grid;
        grid-template-columns: 2ch minmax(0, 1fr) minmax(24px, 74px) 7ch 4ch;
        align-items: center;
        gap: 6px;
        padding: 3px 4px;
        font-size: 11px;
    }
    .hop.timeout {
        opacity: 0.55;
    }
    .hop code {
        white-space: nowrap;
    }
    .hop-address {
        min-width: 0;
    }
    .hop-address small {
        font-size: 10px;
    }
    .hop-track,
    .hop-track > span {
        display: block;
        height: 4px;
        overflow: hidden;
        border-radius: 2px;
    }
    .hop-track {
        background: var(--mantine-color-default-border);
    }
    .hop-track > span {
        background: var(--mantine-color-teal-5);
    }
    @media (max-width: 48em) {
        .trace {
            height: auto;
            min-height: 0;
            grid-template-rows: auto;
        }
        .trace-body {
            grid-template-columns: minmax(0, 1fr);
        }
        .trace-targets {
            max-height: 13rem;
        }
        .hop {
            grid-template-columns: 2ch minmax(0, 1fr) 7ch 4ch;
        }
        .hop-track {
            display: none;
        }
    }
</style>
