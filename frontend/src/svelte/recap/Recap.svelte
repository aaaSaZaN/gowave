<script lang="ts">
    import { GetRecapCommand } from '@remnawave/backend-contract'
    import {
        BG_STYLES,
        type BgStyle,
        CARD_SECTIONS,
        type CardSection,
        DEFAULT_SECTIONS,
        MASKABLE_FIELDS,
        type MaskableField,
        SWATCHES
    } from '@shared/lib/recap-options'
    import { prettifyBytesUtil } from '@shared/utils/bytes'
    import { createQuery } from '@tanstack/svelte-query'
    import { onMount, tick } from 'svelte'
    import { SvelteSet } from 'svelte/reactivity'
    import { callCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import BrandIcon from '../ui/BrandIcon.svelte'
    import Button from '../ui/Button.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import Switch from '../ui/Switch.svelte'
    import AccentPicker from './AccentPicker.svelte'

    let { close }: { close: () => void } = $props()
    let card: HTMLDivElement | undefined = $state()
    const id = $props.id()
    const sections = new SvelteSet<CardSection>(DEFAULT_SECTIONS)
    const masked = new SvelteSet<MaskableField>()
    let style = $state<BgStyle>('solid')
    let accent = $state('#15aabf')
    let note = $state('')
    let busy = $state(false)
    let error = $state('')
    let copied = $state(false)
    let exporter: typeof import('@shared/utils/copy-screenshot.util') | undefined = $state()
    let t = $derived(locale.t)
    let integers = $derived(new Intl.NumberFormat(locale.language, { notation: 'compact' }))
    let date = $derived(new Intl.DateTimeFormat(locale.language, { dateStyle: 'medium' }))
    let month = $derived(
        new Intl.DateTimeFormat(locale.language, { month: 'long', year: 'numeric' })
    )
    const swatches = SWATCHES.map(
        (color) =>
            '#' +
            (color.match(/\d+/g) ?? [])
                .map((component) => Number(component).toString(16).padStart(2, '0'))
                .join('')
    )
    const recap = createQuery(() => ({
        queryKey: ['system', 'getRecap'],
        queryFn: ({ signal }) => callCommand(GetRecapCommand, { signal })
    }))

    onMount(() => {
        void import('@shared/utils/copy-screenshot.util').then(
            (module) => (exporter = module),
            (failure: unknown) =>
                (error = failure instanceof Error ? failure.message : t('common.message.error'))
        )
    })

    function metric(field: MaskableField, value: string | undefined): string {
        return masked.has(field) ? '🙈' : (value ?? '—')
    }

    async function exportCard(action: 'copy' | 'download'): Promise<void> {
        if (!card || !exporter || busy || !exporter.isScreenshotSupported) return
        const filename = `gowave-recap-${new Date().toISOString().slice(0, 10)}.png`
        const element = card
        busy = true
        copied = false
        error = ''
        try {
            if (action === 'copy') {
                await exporter.copyScreenshotToClipboard(async () => {
                    await tick()
                    return element
                }, filename)
                copied = true
            } else {
                await tick()
                await exporter.downloadScreenshot(element, filename)
            }
        } catch (failure) {
            error = failure instanceof Error ? failure.message : t('common.message.error')
        } finally {
            busy = false
        }
    }
</script>

<Overlay
    class="recap"
    title="Recap"
    icon="TbSparkles"
    tone="indigo"
    width={980}
    fullscreenOnMobile={false}
    {close}
>
    {#if recap.isPending}
        <div class="loading" role="status" aria-label={t('common.message.loading')}>
            <span></span>
        </div>
    {:else if recap.isError}
        <p role="alert">{recap.error.message}</p>
        <Button onclick={() => void recap.refetch()}>{t('common.action.try-again')}</Button>
    {:else}
        {@const data = recap.data}
        <div class="layout">
            <div class="controls">
                <section class="controlPanel">
                    <div class="controlLabel">Sections</div>
                    <div class="switches">
                        {#each CARD_SECTIONS as item (item.value)}
                            <div>
                                <Switch
                                    id={id + item.value}
                                    label={item.label}
                                    switchSize="sm"
                                    tone="cyan"
                                    disabled={busy}
                                    checked={sections.has(item.value)}
                                    onchange={(event) =>
                                        event.currentTarget.checked
                                            ? sections.add(item.value)
                                            : sections.delete(item.value)}
                                />
                                <label for={id + item.value}>{item.label}</label>
                            </div>
                        {/each}
                    </div>
                </section>
                <section class="controlPanel">
                    <div class="controlLabel">Mask fields</div>
                    <div class="choices">
                        {#each MASKABLE_FIELDS as item (item.value)}
                            {@const hidden = masked.has(item.value)}
                            <Button
                                variant="soft"
                                size="compact-sm"
                                color={hidden ? 'gray' : 'teal'}
                                icon={hidden ? 'TbX' : 'TbCheck'}
                                aria-pressed={hidden}
                                disabled={busy}
                                onclick={() =>
                                    hidden ? masked.delete(item.value) : masked.add(item.value)}
                                >{item.label}</Button
                            >
                        {/each}
                    </div>
                </section>
            </div>
            <div
                class="recap-card card"
                data-style={style}
                style:--accent={accent}
                bind:this={card}
            >
                {#if data.initDate.valueOf() < Date.UTC(2025, 3, 1)}
                    <div class="ribbon">Early Adopter</div>
                {/if}
                <div class="glow glowTop"></div>
                <div class="glow glowBottom"></div>
                <div class="bgOverlay"></div>
                <div class="brand">
                    <BrandIcon /><span class="brandName"><span>GO</span>WAVE</span>
                </div>
                <div class="hero">
                    <div class="heroValue" data-metric="totalUsers">
                        {metric('totalUsers', integers.format(data.total.users))}
                    </div>
                    <div class="heroLabel">total users</div>
                </div>
                {#if sections.has('stats')}
                    <div class="statsRow">
                        <div class="stat">
                            <div class="statValue">
                                {metric('nodes', integers.format(data.total.nodes))}
                            </div>
                            <div class="statLabel">nodes</div>
                        </div>
                        <div class="stat">
                            <div class="statValue">
                                {metric(
                                    'totalTraffic',
                                    prettifyBytesUtil(data.total.traffic, true)
                                )}
                            </div>
                            <div class="statLabel">traffic</div>
                        </div>
                    </div>
                {/if}
                {#if sections.has('month')}
                    <div class="divider"></div>
                    <div class="section">
                        <div class="sectionTitle">{month.format(new Date())}</div>
                        <div class="monthGrid">
                            <div class="monthItem">
                                <div class="monthValue">
                                    {metric('monthUsers', integers.format(data.thisMonth.users))}
                                </div>
                                <div class="monthLabel">new users</div>
                            </div>
                            <div class="monthItem">
                                <div class="monthValue">
                                    {metric(
                                        'monthTraffic',
                                        prettifyBytesUtil(data.thisMonth.traffic)
                                    )}
                                </div>
                                <div class="monthLabel">traffic</div>
                            </div>
                        </div>
                    </div>
                {/if}
                {#if sections.has('infra')}
                    <div class="divider"></div>
                    <div class="section">
                        <div class="sectionTitle">Infrastructure</div>
                        <div class="infraGrid">
                            <div>
                                <div class="infraValue">
                                    {metric(
                                        'countries',
                                        integers.format(data.total.distinctCountries)
                                    )}
                                </div>
                                <div class="infraLabel">Countries</div>
                            </div>
                            <div>
                                <div class="infraValue">
                                    {metric('cpuCores', integers.format(data.total.nodesCpuCores))}
                                </div>
                                <div class="infraLabel">CPU cores</div>
                            </div>
                            <div>
                                <div class="infraValue">
                                    {metric('ram', prettifyBytesUtil(data.total.nodesRam, true))}
                                </div>
                                <div class="infraLabel">RAM</div>
                            </div>
                        </div>
                    </div>
                {/if}
                {#if sections.size}
                    <div class="divider"></div>
                {/if}
                {#if note}
                    <div class="customNote">{note}</div>
                {/if}
                <div class="footer">
                    <span class="since">Since {date.format(data.initDate)}</span>
                    <span class="version"
                        >{data.version.startsWith('v') ? data.version : `v${data.version}`}</span
                    >
                </div>
            </div>
            <div class="controls">
                <section class="controlPanel">
                    <div class="controlLabel">Background</div>
                    <div class="choices">
                        {#each BG_STYLES as item (item.value)}
                            <Button
                                size="compact-sm"
                                variant="soft"
                                color={style === item.value ? 'teal' : 'gray'}
                                aria-pressed={style === item.value}
                                disabled={busy}
                                onclick={() => (style = item.value)}
                                >{item.label}</Button
                            >
                        {/each}
                    </div>
                </section>
                <section class="controlPanel">
                    <label class="controlLabel" for={`${id}note`}>Custom note</label>
                    <input
                        id={`${id}note`}
                        name="recap-note"
                        maxlength="40"
                        placeholder="RW &lt;3"
                        bind:value={note}
                        disabled={busy}
                    >
                </section>
                <section class="controlPanel">
                    <div class="controlLabel">Accent color</div>
                    <AccentPicker
                        {swatches}
                        disabled={busy}
                        onchange={(color) => (accent = color)}
                    />
                </section>
                <div class="exports">
                    <Button
                        block
                        variant="filled"
                        color={accent}
                        icon="TbCopy"
                        iconSize={14}
                        disabled={!exporter?.isScreenshotSupported || busy}
                        onclick={() => exportCard('copy')}
                        >{t('common.action.copy')}</Button
                    >
                    <Button
                        block
                        icon="TbDownload"
                        iconSize={14}
                        disabled={!exporter?.isScreenshotSupported || busy}
                        onclick={() => exportCard('download')}
                        >{t('common.action.download')}</Button
                    >
                </div>
            </div>
        </div>
    {/if}
    {#if copied}
        <p role="status">{t('common.message.copied')}</p>
    {/if}
    {#if error}
        <p role="alert">{error}</p>
    {/if}
</Overlay>

<style>
    .card {
        width: 380px;
        padding: 36px 32px;
        border-radius: 14px;
        background: #08080f;
        position: relative;
        user-select: none;
        overflow: hidden;
        font-family: "Unbounded", sans-serif;
    }

    .controlPanel {
        background: rgba(var(--gw-fg-rgb), 0.03);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.06);
        border-radius: 10px;
        padding: 14px;
    }

    .controlLabel {
        display: block;
        font-size: 10px;
        font-weight: 600;
        letter-spacing: 1.5px;
        text-transform: uppercase;
        color: rgba(var(--gw-fg-rgb), 0.45);
        margin-bottom: 8px;
    }

    .bgOverlay {
        position: absolute;
        inset: 0;
        pointer-events: none;
        z-index: 0;
    }

    .glow {
        position: absolute;
        border-radius: 50%;
        pointer-events: none;
    }

    .glowTop {
        top: -80px;
        right: -60px;
        width: 220px;
        height: 220px;
        filter: blur(80px);
        opacity: 0.25;
    }

    .glowBottom {
        bottom: -100px;
        left: -60px;
        width: 200px;
        height: 200px;
        filter: blur(100px);
        opacity: 0.12;
    }

    .brand {
        display: flex;
        align-items: center;
        gap: 10px;
        margin-bottom: 32px;
        position: relative;
        z-index: 1;
    }

    .brandName {
        font-size: 16px;
        font-weight: 700;
        letter-spacing: 3px;
        font-family: "Unbounded", sans-serif;
        color: rgba(255, 255, 255, 0.85);
    }

    .hero {
        text-align: center;
        margin-bottom: 28px;
        position: relative;
        z-index: 1;
    }

    .heroValue {
        font-size: 72px;
        font-weight: 800;
        line-height: 1;
        margin-bottom: 6px;
    }

    .heroLabel {
        font-size: 11px;
        font-weight: 600;
        letter-spacing: 2.5px;
        text-transform: uppercase;
        color: rgba(255, 255, 255, 0.4);
    }

    .statsRow {
        display: flex;
        justify-content: center;
        gap: 48px;
        margin-bottom: 28px;
        position: relative;
        z-index: 1;
    }

    .stat {
        text-align: center;
    }

    .statValue {
        font-size: 28px;
        font-weight: 700;
        line-height: 1.2;
        color: rgba(255, 255, 255, 0.9);
    }

    .statLabel {
        font-size: 10px;
        font-weight: 600;
        letter-spacing: 2px;
        text-transform: uppercase;
        color: rgba(255, 255, 255, 0.35);
        margin-top: 2px;
    }

    .divider {
        height: 1px;
        margin-bottom: 20px;
        position: relative;
        z-index: 1;
    }

    .section {
        margin-bottom: 20px;
        position: relative;
        z-index: 1;
    }

    .sectionTitle {
        font-size: 10px;
        font-weight: 700;
        letter-spacing: 2.5px;
        text-transform: uppercase;
        color: rgba(255, 255, 255, 0.3);
        margin-bottom: 12px;
    }

    .monthGrid {
        display: flex;
        justify-content: center;
        gap: 40px;
    }

    .monthItem {
        text-align: center;
    }

    .monthValue {
        font-size: 22px;
        font-weight: 700;
        color: rgba(255, 255, 255, 0.85);
        line-height: 1.2;
    }

    .monthLabel {
        font-size: 9px;
        font-weight: 600;
        letter-spacing: 1.5px;
        text-transform: uppercase;
        color: rgba(255, 255, 255, 0.3);
        margin-top: 2px;
    }

    .infraGrid {
        display: grid;
        grid-template-columns: repeat(3, 1fr);
        gap: 8px;
        text-align: center;
    }

    .infraValue {
        font-size: 20px;
        font-weight: 700;
        color: rgba(255, 255, 255, 0.85);
        line-height: 1.2;
        white-space: nowrap;
    }

    .infraLabel {
        font-size: 9px;
        font-weight: 600;
        letter-spacing: 1.5px;
        text-transform: uppercase;
        color: rgba(255, 255, 255, 0.3);
        margin-top: 2px;
    }

    .customNote {
        text-align: left;
        font-size: 18px;
        font-weight: 600;
        letter-spacing: 2px;
        color: rgba(255, 255, 255, 1);
        position: relative;
        z-index: 1;
    }

    .footer {
        display: flex;
        justify-content: space-between;
        align-items: center;
        position: relative;
        z-index: 1;
    }

    .since {
        font-size: 14px;
        font-weight: 500;
    }

    .version {
        font-size: 14px;
        font-weight: 600;
        padding: 4px 10px;
        border-radius: 8px;
        letter-spacing: 0.5px;
    }

    .ribbon {
        position: absolute;
        top: 30px;
        right: -60px;
        width: 200px;
        padding: 4px 0;
        text-align: center;
        transform: rotate(45deg);
        font-size: 8px;
        font-weight: 700;
        letter-spacing: 1.5px;
        text-transform: uppercase;
        color: #08080f;
        z-index: 2;
        pointer-events: none;
    }
    .layout {
        display: grid;
        grid-template-columns: minmax(0, 270px) minmax(0, 380px) minmax(0, 270px);
        align-items: center;
        justify-content: center;
        gap: var(--mantine-spacing-sm);
    }
    .controls,
    .switches {
        display: grid;
        gap: var(--mantine-spacing-sm);
    }
    .switches {
        gap: var(--mantine-spacing-xs);
    }
    .switches div,
    .choices {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        font-size: var(--mantine-font-size-sm);
    }
    .choices {
        flex-wrap: wrap;
        gap: 4px;
    }
    .controlPanel > input {
        width: 100%;
        min-height: 30px;
        height: 30px;
        padding: 0 10px;
        font-size: var(--mantine-font-size-xs);
        background: var(--mantine-color-default);
    }
    .exports {
        display: grid;
        gap: var(--mantine-spacing-xs);
    }
    .card {
        max-width: 100%;
        border: 3px solid color-mix(in srgb, var(--accent) 30%, transparent);
    }
    .glow,
    .ribbon {
        background: var(--accent);
    }
    .heroValue,
    .since,
    .customNote,
    .brandName span {
        color: var(--accent);
    }
    .customNote {
        overflow-wrap: anywhere;
    }
    .version {
        background: color-mix(in srgb, var(--accent) 10%, transparent);
        color: color-mix(in srgb, var(--accent) 70%, transparent);
    }
    .divider {
        background: linear-gradient(
            90deg,
            transparent,
            color-mix(in srgb, var(--accent) 30%, transparent),
            transparent
        );
    }
    [data-style="dots"] .bgOverlay {
        background-image: radial-gradient(
            color-mix(in srgb, var(--accent) 12%, transparent) 1px,
            transparent 1px
        );
        background-size: 16px 16px;
    }
    [data-style="grid"] .bgOverlay {
        --line: color-mix(in srgb, var(--accent) 6%, transparent);
        background-image:
            linear-gradient(var(--line) 1px, transparent 1px),
            linear-gradient(90deg, var(--line) 1px, transparent 1px);
        background-size: 24px 24px;
    }
    [data-style="gradient"] .bgOverlay {
        background: linear-gradient(
            135deg,
            transparent,
            color-mix(in srgb, var(--accent) 8%, transparent),
            transparent
        );
    }
    .loading {
        height: 600px;
        display: grid;
        place-items: center;
    }
    .loading span {
        width: 22px;
        height: 22px;
        border-radius: 50%;
        border: 3px solid var(--mantine-color-cyan-light);
        border-top-color: var(--mantine-color-cyan-6);
        animation: spin 750ms linear infinite;
    }
    @keyframes spin {
        to {
            transform: rotate(360deg);
        }
    }
    @media (max-width: 52em) {
        .layout {
            grid-template-columns: minmax(0, 380px);
        }
        .card {
            grid-row: 1;
        }
    }
</style>
