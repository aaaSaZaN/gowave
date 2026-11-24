<script lang="ts">
    import { DotLottie } from '@lottiefiles/dotlottie-web'
    import wasmUrl from '@lottiefiles/dotlottie-web/dotlottie-player.wasm?url'
    import { panelUrl } from '@shared/lib/panel-path'
    import { onMount } from 'svelte'
    import { locale } from '../i18n.svelte'
    import Icon from '../ui/Icon.svelte'

    let {
        completed = 0,
        total = 0,
        percent = 0
    }: {
        completed?: number
        total?: number
        percent?: number
    } = $props()
    let t = $derived(locale.t)
    let canvas: HTMLCanvasElement
    let ready = $state(false)
    let progress = $derived(Math.min(100, Math.max(0, percent)))

    onMount(() => {
        const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
        DotLottie.setWasmUrl(wasmUrl)
        const animation = new DotLottie({
            canvas,
            autoplay: !reducedMotion,
            loop: !reducedMotion,
            src: panelUrl('lotties/globe.lottie'),
            renderConfig: { autoResize: true, devicePixelRatio: window.devicePixelRatio || 2 }
        })
        animation.addEventListener('load', () => (ready = true))
        return () => animation.destroy()
    })
</script>

<section class="scan" aria-label={t('active-sessions-drawer.widget.fetching')}>
    <div class="globe" aria-hidden="true">
        <canvas bind:this={canvas} class:ready></canvas>
        {#if !ready}
            <Icon name="TbWorld" size={80} />
        {/if}
    </div>
    <div class="description" role="status">
        <strong>{t('active-sessions-drawer.widget.fetching')}</strong>
        <span>{t('active-sessions-drawer.widget.progress', { completed, total })}</span>
    </div>
    <div class="progress">
        <div
            class="track"
            role="progressbar"
            aria-label={t('active-sessions-drawer.widget.fetching')}
            aria-valuenow={progress}
        >
            <span style:width={`${progress}%`}></span>
        </div>
        <b>{Math.round(progress)}%</b>
    </div>
</section>

<style>
    .scan {
        display: grid;
        gap: var(--mantine-spacing-md);
        padding: calc(var(--mantine-spacing-xl) + var(--mantine-spacing-md))
            var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
        text-align: center;
    }
    .globe {
        position: relative;
        display: grid;
        height: 120px;
        place-items: center;
        color: var(--mantine-color-blue-5);
    }
    .globe canvas {
        position: absolute;
        width: 96px;
        height: 115px;
        visibility: hidden;
    }
    .globe canvas.ready {
        visibility: visible;
    }
    .description {
        display: grid;
        gap: 4px;
    }
    .description strong {
        color: var(--mantine-color-bright);
        font-weight: 600;
    }
    .description span {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
    }
    .progress {
        display: grid;
        gap: 6px;
        margin-block-start: 6px;
        color: var(--mantine-color-bright);
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-sm);
    }
    .track {
        height: 18px;
        overflow: hidden;
        border-radius: var(--mantine-radius-md);
        background: rgba(var(--gw-fg-rgb), 0.05);
    }
    .track span {
        display: block;
        height: 100%;
        border-radius: inherit;
        background: repeating-linear-gradient(
                45deg,
                transparent 0 7px,
                rgb(255 255 255 / 0.25) 7px 14px
            )
            var(--mantine-color-teal-8);
    }
    @media (prefers-reduced-motion: no-preference) {
        .track span {
            animation: stripes 1s linear infinite;
            transition: width 400ms ease;
        }
    }
    @keyframes stripes {
        to {
            background-position: 20px 0;
        }
    }
</style>
